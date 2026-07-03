// Package gcp provides GCP-specific rule implementations for all three tiers.
package gcp

import (
	"github.com/amt/tf-cost-risk/rules"
)

// AllRules returns the full ordered list of GCP rules (Tier 1 → 2 → 3).
func AllRules() []rules.Rule {
	return []rules.Rule{
		// Tier 1 — destructive / data-loss
		&DestructiveChangeRule{},
		&MissingDeletionProtectionRule{},
		// Tier 2 — security misconfig
		&OpenFirewallRule{},
		&PublicStorageBucketRule{},
		&UnencryptedSQLRule{},
		// Tier 3 — cost-risk hybrid
		&OversizedResourceRule{OversizeMultiple: 5},
		&MissingLabelsRule{RequiredLabels: []string{"env", "team"}},
		&UnboundedGKEAutoscalingRule{},
	}
}

// Run executes all rules against the given context and returns combined findings.
func Run(ctx rules.EvaluateContext, ruleSet []rules.Rule) []rules.Finding {
	var findings []rules.Finding
	for _, r := range ruleSet {
		findings = append(findings, r.Evaluate(ctx)...)
	}
	return findings
}
