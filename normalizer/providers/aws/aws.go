// Package aws normalizes AWS Terraform resource changes into the common internal schema.
package aws

import (
	"fmt"

	"github.com/amt/tf-cost-risk/normalizer"
	"github.com/amt/tf-cost-risk/parser"
)

// Normalizer implements normalizer.Normalizer for AWS resources.
type Normalizer struct{}

var statefulTypes = map[string]bool{
	"aws_db_instance":           true,
	"aws_rds_cluster":           true,
	"aws_elasticache_cluster":   true,
	"aws_elasticache_replication_group": true,
	"aws_dynamodb_table":        true,
	"aws_ebs_volume":            true,
	"aws_efs_file_system":       true,
}

var categoryMap = map[string]normalizer.ResourceCategory{
	"aws_instance":              normalizer.CategoryCompute,
	"aws_db_instance":           normalizer.CategoryDatabase,
	"aws_rds_cluster":           normalizer.CategoryDatabase,
	"aws_elasticache_cluster":   normalizer.CategoryDatabase,
	"aws_elasticache_replication_group": normalizer.CategoryDatabase,
	"aws_dynamodb_table":        normalizer.CategoryDatabase,
	"aws_ebs_volume":            normalizer.CategoryStorage,
	"aws_efs_file_system":       normalizer.CategoryStorage,
	"aws_s3_bucket":             normalizer.CategoryStorage,
	"aws_lb":                    normalizer.CategoryNetwork,
	"aws_alb":                   normalizer.CategoryNetwork,
	"aws_lambda_function":       normalizer.CategoryFunction,
}

// Normalize maps an AWS resource change to a NormalizedResource.
func (n *Normalizer) Normalize(rc parser.ResourceChange, region string) (normalizer.NormalizedResource, error) {
	attrs := rc.After
	if attrs == nil {
		attrs = rc.Before
	}
	if attrs == nil {
		attrs = map[string]interface{}{}
	}

	cat, ok := categoryMap[rc.Type]
	if !ok {
		cat = normalizer.CategoryUnknown
	}

	nr := normalizer.NormalizedResource{
		Address:      rc.Address,
		ChangeType:   rc.ChangeType,
		Provider:     "aws",
		ResourceType: rc.Type,
		Category:     cat,
		Size:         extractSize(rc.Type, attrs),
		Region:       extractRegion(attrs, region),
		Tags:         extractTags(attrs),
		Stateful:     statefulTypes[rc.Type],
		Raw:          attrs,
	}
	return nr, nil
}

func extractSize(resourceType string, attrs map[string]interface{}) string {
	switch resourceType {
	case "aws_instance":
		return strAttr(attrs, "instance_type")
	case "aws_db_instance":
		return strAttr(attrs, "instance_class")
	case "aws_rds_cluster":
		return strAttr(attrs, "db_cluster_instance_class")
	case "aws_elasticache_cluster", "aws_elasticache_replication_group":
		return strAttr(attrs, "node_type")
	case "aws_ebs_volume":
		return strAttr(attrs, "type") // gp3, io2, etc.
	case "aws_s3_bucket":
		return "" // S3 pricing is by usage, not instance size
	case "aws_dynamodb_table":
		return strAttr(attrs, "billing_mode") // PAY_PER_REQUEST or PROVISIONED
	case "aws_lb", "aws_alb":
		return strAttr(attrs, "load_balancer_type") // application, network, gateway
	case "aws_lambda_function":
		if mem, ok := attrs["memory_size"]; ok {
			// JSON numbers unmarshal as float64
			return fmt.Sprintf("%dMB", int(mem.(float64)))
		}
	}
	return ""
}

func extractRegion(attrs map[string]interface{}, fallback string) string {
	if r := strAttr(attrs, "region"); r != "" {
		return r
	}
	return fallback
}

func extractTags(attrs map[string]interface{}) map[string]string {
	tags := map[string]string{}
	raw, ok := attrs["tags"]
	if !ok {
		return tags
	}
	m, ok := raw.(map[string]interface{})
	if !ok {
		return tags
	}
	for k, v := range m {
		if s, ok := v.(string); ok {
			tags[k] = s
		}
	}
	return tags
}

func strAttr(attrs map[string]interface{}, key string) string {
	v, ok := attrs[key]
	if !ok {
		return ""
	}
	s, _ := v.(string)
	return s
}
