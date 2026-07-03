package aws

import (
	"fmt"

	"github.com/amt/tf-cost-risk/normalizer"
	"github.com/amt/tf-cost-risk/parser"
	"github.com/amt/tf-cost-risk/rules"
)

// deletionProtectedTypes are stateful resource types that support a deletion_protection attribute.
var deletionProtectedTypes = map[string]string{
	"aws_db_instance":  "deletion_protection",
	"aws_rds_cluster":  "deletion_protection",
	"aws_dynamodb_table": "deletion_protection_enabled",
}

// DestructiveChangeRule flags any delete or replace operation, escalating severity for stateful resources.
type DestructiveChangeRule struct{}

func (r *DestructiveChangeRule) Evaluate(ctx rules.EvaluateContext) []rules.Finding {
	var findings []rules.Finding
	for _, nr := range ctx.Resources {
		if nr.Provider != "aws" {
			continue
		}
		switch nr.ChangeType {
		case parser.ChangeDelete:
			findings = append(findings, destructiveFinding(nr, "will be permanently deleted"))
		case parser.ChangeReplace:
			findings = append(findings, destructiveFinding(nr, "will be destroyed and recreated (replace), causing downtime"))
		}
	}
	return findings
}

func destructiveFinding(nr normalizer.NormalizedResource, action string) rules.Finding {
	sev := rules.SeverityWarning
	if nr.Stateful {
		sev = rules.SeverityCritical
	}
	return rules.Finding{
		Severity:        sev,
		Category:        rules.CategoryDestructive,
		ResourceAddress: nr.Address,
		Explanation:     fmt.Sprintf("%s (%s) %s.", nr.Address, nr.ResourceType, action),
	}
}

// MissingDeletionProtectionRule flags stateful resources that are being created or updated
// without deletion protection enabled.
type MissingDeletionProtectionRule struct{}

func (r *MissingDeletionProtectionRule) Evaluate(ctx rules.EvaluateContext) []rules.Finding {
	var findings []rules.Finding
	for _, nr := range ctx.Resources {
		if nr.ChangeType == parser.ChangeDelete {
			continue // already flagged by DestructiveChangeRule
		}
		attr, ok := deletionProtectedTypes[nr.ResourceType]
		if !ok {
			continue
		}
		if !boolAttr(nr.Raw, attr) {
			findings = append(findings, rules.Finding{
				Severity:        rules.SeverityWarning,
				Category:        rules.CategoryDestructive,
				ResourceAddress: nr.Address,
				Explanation: fmt.Sprintf(
					"%s (%s) does not have deletion protection enabled (%s = false). A future plan could destroy this resource without a separate safeguard.",
					nr.Address, nr.ResourceType, attr,
				),
			})
		}
	}
	return findings
}
