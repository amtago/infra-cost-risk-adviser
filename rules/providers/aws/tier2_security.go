package aws

import (
	"fmt"

	"github.com/amt/tf-cost-risk/normalizer"
	"github.com/amt/tf-cost-risk/parser"
	"github.com/amt/tf-cost-risk/rules"
)

// sensitivePorts are ports where open ingress to 0.0.0.0/0 is a security risk.
var sensitivePorts = map[int]string{
	22:    "SSH",
	3389:  "RDP",
	3306:  "MySQL",
	5432:  "PostgreSQL",
	1433:  "MSSQL",
	6379:  "Redis",
	27017: "MongoDB",
}

// OpenSecurityGroupRule flags security groups with ingress open to 0.0.0.0/0 on sensitive ports.
type OpenSecurityGroupRule struct{}

func (r *OpenSecurityGroupRule) Evaluate(ctx rules.EvaluateContext) []rules.Finding {
	var findings []rules.Finding
	for _, nr := range ctx.Resources {
		if nr.ResourceType != "aws_security_group" {
			continue
		}
		if nr.ChangeType == parser.ChangeDelete {
			continue
		}
		findings = append(findings, checkIngressRules(nr)...)
	}
	return findings
}

func checkIngressRules(nr normalizer.NormalizedResource) []rules.Finding {
	ingress, ok := nr.Raw["ingress"]
	if !ok {
		return nil
	}
	items := toSlice(ingress)
	var findings []rules.Finding
	for _, item := range items {
		rule, ok := item.(map[string]interface{})
		if !ok {
			continue
		}
		if !hasCIDR(rule, "0.0.0.0/0") && !hasCIDR(rule, "::/0") {
			continue
		}
		from := intAttr(rule, "from_port")
		to := intAttr(rule, "to_port")
		for port, name := range sensitivePorts {
			if from <= port && port <= to {
				findings = append(findings, rules.Finding{
					Severity:        rules.SeverityCritical,
					Category:        rules.CategorySecurity,
					ResourceAddress: nr.Address,
					Explanation: fmt.Sprintf(
						"%s allows inbound %s (port %d) from 0.0.0.0/0 (the entire internet). Restrict to known IP ranges.",
						nr.Address, name, port,
					),
				})
			}
		}
	}
	return findings
}

// PublicS3BucketRule flags S3 buckets with a public-read or public-read-write ACL.
type PublicS3BucketRule struct{}

func (r *PublicS3BucketRule) Evaluate(ctx rules.EvaluateContext) []rules.Finding {
	var findings []rules.Finding
	for _, nr := range ctx.Resources {
		if nr.ResourceType != "aws_s3_bucket" {
			continue
		}
		if nr.ChangeType == parser.ChangeDelete {
			continue
		}
		acl := strAttr(nr.Raw, "acl")
		if acl == "public-read" || acl == "public-read-write" {
			findings = append(findings, rules.Finding{
				Severity:        rules.SeverityCritical,
				Category:        rules.CategorySecurity,
				ResourceAddress: nr.Address,
				Explanation: fmt.Sprintf(
					"%s has a public ACL (%q). This exposes bucket contents to the internet. Use private ACL and bucket policies instead.",
					nr.Address, acl,
				),
			})
		}
	}
	return findings
}

// UnencryptedStorageRule flags RDS instances and EBS volumes created without encryption.
type UnencryptedStorageRule struct{}

func (r *UnencryptedStorageRule) Evaluate(ctx rules.EvaluateContext) []rules.Finding {
	var findings []rules.Finding
	for _, nr := range ctx.Resources {
		if nr.ChangeType == parser.ChangeDelete {
			continue
		}
		switch nr.ResourceType {
		case "aws_db_instance", "aws_rds_cluster":
			if !boolAttr(nr.Raw, "storage_encrypted") {
				findings = append(findings, rules.Finding{
					Severity:        rules.SeverityWarning,
					Category:        rules.CategorySecurity,
					ResourceAddress: nr.Address,
					Explanation: fmt.Sprintf(
						"%s (%s) does not have storage encryption enabled (storage_encrypted = false). Enable encryption at rest.",
						nr.Address, nr.ResourceType,
					),
				})
			}
		case "aws_ebs_volume":
			if !boolAttr(nr.Raw, "encrypted") {
				findings = append(findings, rules.Finding{
					Severity:        rules.SeverityWarning,
					Category:        rules.CategorySecurity,
					ResourceAddress: nr.Address,
					Explanation: fmt.Sprintf(
						"%s (aws_ebs_volume) does not have encryption enabled. Set encrypted = true.",
						nr.Address,
					),
				})
			}
		}
	}
	return findings
}

// WildcardIAMRule flags IAM policies with Action:* or Resource:* which grant excessive permissions.
type WildcardIAMRule struct{}

func (r *WildcardIAMRule) Evaluate(ctx rules.EvaluateContext) []rules.Finding {
	var findings []rules.Finding
	for _, nr := range ctx.Resources {
		if nr.ResourceType != "aws_iam_policy" &&
			nr.ResourceType != "aws_iam_role_policy" &&
			nr.ResourceType != "aws_iam_user_policy" {
			continue
		}
		if nr.ChangeType == parser.ChangeDelete {
			continue
		}
		for _, stmt := range parsePolicyDocument(nr.Raw) {
			if sliceContains(stmt.actions, "*") {
				findings = append(findings, rules.Finding{
					Severity:        rules.SeverityCritical,
					Category:        rules.CategorySecurity,
					ResourceAddress: nr.Address,
					Explanation: fmt.Sprintf(
						"%s contains an IAM statement with Action:\"*\" (full permissions on all AWS services). Scope actions to only what is needed.",
						nr.Address,
					),
				})
			}
			if sliceContains(stmt.resources, "*") {
				findings = append(findings, rules.Finding{
					Severity:        rules.SeverityWarning,
					Category:        rules.CategorySecurity,
					ResourceAddress: nr.Address,
					Explanation: fmt.Sprintf(
						"%s contains an IAM statement with Resource:\"*\" (applies to all resources). Scope to specific ARNs where possible.",
						nr.Address,
					),
				})
			}
		}
	}
	return findings
}

type iamStatement struct {
	actions   []string
	resources []string
}

func parsePolicyDocument(raw map[string]interface{}) []iamStatement {
	policyStr, ok := raw["policy"].(string)
	if !ok || policyStr == "" {
		return nil
	}
	type stmt struct {
		Action   interface{} `json:"Action"`
		Resource interface{} `json:"Resource"`
	}
	type doc struct {
		Statement []stmt `json:"Statement"`
	}
	var d doc
	if err := jsonUnmarshal([]byte(policyStr), &d); err != nil {
		return nil
	}
	var out []iamStatement
	for _, s := range d.Statement {
		out = append(out, iamStatement{
			actions:   toStringSlice(s.Action),
			resources: toStringSlice(s.Resource),
		})
	}
	return out
}
