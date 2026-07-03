package gcp

import (
	"fmt"

	"github.com/amt/tf-cost-risk/normalizer"
	"github.com/amt/tf-cost-risk/parser"
	"github.com/amt/tf-cost-risk/rules"
)

// DestructiveChangeRule flags delete/replace operations, escalating severity for stateful resources.
type DestructiveChangeRule struct{}

func (r *DestructiveChangeRule) Evaluate(ctx rules.EvaluateContext) []rules.Finding {
	var findings []rules.Finding
	for _, nr := range ctx.Resources {
		if nr.Provider != "gcp" {
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

// MissingDeletionProtectionRule flags Cloud SQL instances created without deletion protection.
type MissingDeletionProtectionRule struct{}

// deletionProtectedTypes maps GCP resource types to the attribute that enables deletion protection.
var deletionProtectedTypes = map[string]string{
	"google_sql_database_instance": "deletion_protection",
}

func (r *MissingDeletionProtectionRule) Evaluate(ctx rules.EvaluateContext) []rules.Finding {
	var findings []rules.Finding
	for _, nr := range ctx.Resources {
		if nr.Provider != "gcp" {
			continue
		}
		if nr.ChangeType == parser.ChangeDelete {
			continue
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
					"%s (%s) does not have deletion protection enabled (%s = false). A future plan could destroy this instance without a separate safeguard.",
					nr.Address, nr.ResourceType, attr,
				),
			})
		}
	}
	return findings
}
