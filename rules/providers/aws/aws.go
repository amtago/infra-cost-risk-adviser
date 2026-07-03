// Package aws provides AWS-specific rule implementations for all three tiers.
package aws

import (
	"github.com/amt/tf-cost-risk/rules"
)

// AllRules returns the full ordered list of AWS rules (Tier 1 → 2 → 3).
func AllRules() []rules.Rule {
	return []rules.Rule{
		// Tier 1 — destructive / data-loss
		&DestructiveChangeRule{},
		&MissingDeletionProtectionRule{},
		// Tier 2 — security misconfig
		&OpenSecurityGroupRule{},
		&PublicS3BucketRule{},
		&UnencryptedStorageRule{},
		&WildcardIAMRule{},
		// Tier 3 — cost-risk hybrid
		&OversizedResourceRule{OversizeMultiple: 5},
		&MissingCostTagsRule{RequiredTags: []string{"Env", "Team"}},
		&UnboundedAutoscalingRule{},
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
