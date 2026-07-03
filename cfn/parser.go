// Package cfn parses AWS CloudFormation change set JSON into the common
// internal resource-change representation used by the tfx pipeline.
//
// Input: output of `aws cloudformation describe-change-set --change-set-name X --stack-name Y`
// Optionally paired with the stack template JSON for attribute-level rule coverage.
package cfn

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/amt/tf-cost-risk/parser"
)

// ChangeSet is the top-level structure from describe-change-set.
type changeSet struct {
	ChangeSetName string   `json:"ChangeSetName"`
	StackName     string   `json:"StackName"`
	Changes       []change `json:"Changes"`
}

type change struct {
	Type           string         `json:"Type"`
	ResourceChange resourceChange `json:"ResourceChange"`
}

type resourceChange struct {
	Action          string `json:"Action"`      // Add | Modify | Remove
	LogicalResourceId string `json:"LogicalResourceId"`
	PhysicalResourceId string `json:"PhysicalResourceId"`
	ResourceType    string `json:"ResourceType"` // AWS::EC2::Instance etc.
	Replacement     string `json:"Replacement"`  // True | False | Conditional
}

// cfnTemplate is the minimal structure we need from a CloudFormation template.
type cfnTemplate struct {
	Resources map[string]cfnResource `json:"Resources"`
}

type cfnResource struct {
	Type       string                 `json:"Type"`
	Properties map[string]interface{} `json:"Properties"`
}

// ParseFile reads a CloudFormation change set JSON file and optional template file,
// returning the common ResourceChange slice used by the rest of the tfx pipeline.
func ParseFile(changeSetPath, templatePath string) ([]parser.ResourceChange, error) {
	csData, err := os.ReadFile(changeSetPath)
	if err != nil {
		return nil, fmt.Errorf("reading change set file: %w", err)
	}

	var tmplData []byte
	if templatePath != "" {
		tmplData, err = os.ReadFile(templatePath)
		if err != nil {
			return nil, fmt.Errorf("reading template file: %w", err)
		}
	}

	return Parse(csData, tmplData)
}

// Parse parses raw change set JSON (and optional template JSON) into ResourceChanges.
func Parse(changeSetJSON, templateJSON []byte) ([]parser.ResourceChange, error) {
	var cs changeSet
	if err := json.Unmarshal(changeSetJSON, &cs); err != nil {
		return nil, fmt.Errorf("parsing change set JSON: %w", err)
	}

	// Load template properties if provided.
	templateProps := map[string]map[string]interface{}{}
	if len(templateJSON) > 0 {
		var tmpl cfnTemplate
		if err := json.Unmarshal(templateJSON, &tmpl); err != nil {
			return nil, fmt.Errorf("parsing template JSON: %w", err)
		}
		for logicalID, res := range tmpl.Resources {
			if res.Properties != nil {
				templateProps[logicalID] = res.Properties
			}
		}
	}

	var changes []parser.ResourceChange
	for _, c := range cs.Changes {
		if c.Type != "Resource" {
			continue
		}
		rc := c.ResourceChange

		ct := cfnActionToChangeType(rc.Action, rc.Replacement)
		if ct == parser.ChangeNoOp {
			continue
		}

		props := templateProps[rc.LogicalResourceId]

		// Normalise CFN property names → snake_case for rule compatibility.
		after := normaliseCFNProps(rc.ResourceType, props)
		var before map[string]interface{}
		if ct == parser.ChangeDelete || ct == parser.ChangeReplace {
			before = after // change set doesn't expose pre-change values
		}

		changes = append(changes, parser.ResourceChange{
			Address:      rc.LogicalResourceId,
			ProviderName: "cloudformation",
			Type:         cfnTypeToInternal(rc.ResourceType),
			Name:         rc.LogicalResourceId,
			ChangeType:   ct,
			Before:       before,
			After:        after,
		})
	}
	return changes, nil
}

func cfnActionToChangeType(action, replacement string) parser.ChangeType {
	switch strings.ToLower(action) {
	case "add":
		return parser.ChangeCreate
	case "modify":
		if strings.EqualFold(replacement, "true") || strings.EqualFold(replacement, "conditional") {
			return parser.ChangeReplace
		}
		return parser.ChangeUpdate
	case "remove":
		return parser.ChangeDelete
	}
	return parser.ChangeNoOp
}

// cfnTypeToInternal maps CloudFormation resource types to the internal type names
// used by the normalizer, pricing, and rule engine (matching Terraform provider names).
func cfnTypeToInternal(cfnType string) string {
	m := map[string]string{
		"AWS::EC2::Instance":                           "aws_instance",
		"AWS::EC2::Volume":                             "aws_ebs_volume",
		"AWS::EC2::SecurityGroup":                      "aws_security_group",
		"AWS::RDS::DBInstance":                         "aws_db_instance",
		"AWS::RDS::DBCluster":                          "aws_rds_cluster",
		"AWS::ElastiCache::CacheCluster":               "aws_elasticache_cluster",
		"AWS::ElastiCache::ReplicationGroup":           "aws_elasticache_replication_group",
		"AWS::DynamoDB::Table":                         "aws_dynamodb_table",
		"AWS::S3::Bucket":                              "aws_s3_bucket",
		"AWS::ElasticLoadBalancingV2::LoadBalancer":    "aws_lb",
		"AWS::Lambda::Function":                        "aws_lambda_function",
		"AWS::IAM::Policy":                             "aws_iam_policy",
		"AWS::IAM::Role":                               "aws_iam_role",
		"AWS::AutoScaling::AutoScalingGroup":           "aws_autoscaling_group",
		"AWS::EFS::FileSystem":                         "aws_efs_file_system",
	}
	if internal, ok := m[cfnType]; ok {
		return internal
	}
	// Return the original CFN type for unknown resources so it's still visible.
	return cfnType
}

// normaliseCFNProps translates CloudFormation PascalCase property names into the
// snake_case attribute names the rule engine expects (matching Terraform provider attrs).
func normaliseCFNProps(cfnType string, props map[string]interface{}) map[string]interface{} {
	if props == nil {
		return map[string]interface{}{}
	}

	out := map[string]interface{}{}

	// Universal mappings
	copyAs(props, out, "Tags", "tags", normaliseCFNTags)

	switch cfnType {
	case "AWS::EC2::Instance":
		copyScalar(props, out, "InstanceType", "instance_type")

	case "AWS::EC2::Volume":
		copyScalar(props, out, "VolumeType", "type")
		copyScalar(props, out, "Size", "size")
		copyBool(props, out, "Encrypted", "encrypted")

	case "AWS::EC2::SecurityGroup":
		if ingress, ok := props["SecurityGroupIngress"]; ok {
			out["ingress"] = normaliseCFNIngress(ingress)
		}

	case "AWS::RDS::DBInstance":
		copyScalar(props, out, "DBInstanceClass", "instance_class")
		copyBool(props, out, "StorageEncrypted", "storage_encrypted")
		copyBool(props, out, "DeletionProtection", "deletion_protection")

	case "AWS::RDS::DBCluster":
		copyScalar(props, out, "DBClusterInstanceClass", "db_cluster_instance_class")
		copyBool(props, out, "StorageEncrypted", "storage_encrypted")
		copyBool(props, out, "DeletionProtection", "deletion_protection")

	case "AWS::ElastiCache::CacheCluster":
		copyScalar(props, out, "CacheNodeType", "node_type")

	case "AWS::ElastiCache::ReplicationGroup":
		copyScalar(props, out, "CacheNodeType", "node_type")

	case "AWS::DynamoDB::Table":
		copyScalar(props, out, "BillingMode", "billing_mode")
		copyBool(props, out, "DeletionProtectionEnabled", "deletion_protection_enabled")

	case "AWS::ElasticLoadBalancingV2::LoadBalancer":
		copyScalar(props, out, "Type", "load_balancer_type")

	case "AWS::Lambda::Function":
		copyScalar(props, out, "MemorySize", "memory_size")

	case "AWS::AutoScaling::AutoScalingGroup":
		copyScalar(props, out, "MaxSize", "max_size")
		copyScalar(props, out, "MinSize", "min_size")

	case "AWS::IAM::Policy", "AWS::IAM::Role":
		// Store the policy document JSON string under "policy" so WildcardIAMRule can read it.
		if doc, ok := props["PolicyDocument"]; ok {
			b, err := json.Marshal(doc)
			if err == nil {
				out["policy"] = string(b)
			}
		}
		// IAM Role inline policies
		if policies, ok := props["Policies"]; ok {
			if ps, ok := policies.([]interface{}); ok && len(ps) > 0 {
				if first, ok := ps[0].(map[string]interface{}); ok {
					if doc, ok := first["PolicyDocument"]; ok {
						b, err := json.Marshal(doc)
						if err == nil {
							out["policy"] = string(b)
						}
					}
				}
			}
		}
	}

	return out
}

// copyScalar copies a property value as-is.
func copyScalar(src, dst map[string]interface{}, srcKey, dstKey string) {
	if v, ok := src[srcKey]; ok {
		dst[dstKey] = v
	}
}

// copyBool copies a boolean property, handling both bool and string "true"/"false".
func copyBool(src, dst map[string]interface{}, srcKey, dstKey string) {
	v, ok := src[srcKey]
	if !ok {
		return
	}
	switch val := v.(type) {
	case bool:
		dst[dstKey] = val
	case string:
		dst[dstKey] = strings.EqualFold(val, "true")
	}
}

// copyAs copies a property through a transform function.
func copyAs(src, dst map[string]interface{}, srcKey, dstKey string, fn func(interface{}) interface{}) {
	if v, ok := src[srcKey]; ok {
		dst[dstKey] = fn(v)
	}
}

// normaliseCFNTags converts CFN Tags ([{Key:X,Value:Y}]) to a flat map.
func normaliseCFNTags(v interface{}) interface{} {
	items, ok := v.([]interface{})
	if !ok {
		return map[string]interface{}{}
	}
	out := map[string]interface{}{}
	for _, item := range items {
		m, ok := item.(map[string]interface{})
		if !ok {
			continue
		}
		k, _ := m["Key"].(string)
		val, _ := m["Value"].(string)
		if k != "" {
			out[k] = val
		}
	}
	return out
}

// normaliseCFNIngress converts CFN SecurityGroupIngress rules to the format
// the OpenSecurityGroupRule expects (same shape as Terraform's ingress blocks).
func normaliseCFNIngress(v interface{}) []interface{} {
	items, ok := v.([]interface{})
	if !ok {
		return nil
	}
	var out []interface{}
	for _, item := range items {
		m, ok := item.(map[string]interface{})
		if !ok {
			continue
		}
		rule := map[string]interface{}{}

		if fp, ok := m["FromPort"]; ok {
			rule["from_port"] = toFloat64(fp)
		}
		if tp, ok := m["ToPort"]; ok {
			rule["to_port"] = toFloat64(tp)
		}
		if cidr, ok := m["CidrIp"].(string); ok && cidr != "" {
			rule["cidr_blocks"] = []interface{}{cidr}
		}
		if cidr6, ok := m["CidrIpv6"].(string); ok && cidr6 != "" {
			rule["ipv6_cidr_blocks"] = []interface{}{cidr6}
		}
		out = append(out, rule)
	}
	return out
}

func toFloat64(v interface{}) float64 {
	switch n := v.(type) {
	case float64:
		return n
	case int:
		return float64(n)
	case string:
		var f float64
		fmt.Sscanf(n, "%f", &f)
		return f
	}
	return 0
}
