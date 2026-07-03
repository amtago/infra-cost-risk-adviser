// Package rules runs normalized resources through the finding rule engine.
package rules

import (
	"github.com/amt/tf-cost-risk/normalizer"
	"github.com/amt/tf-cost-risk/pricing"
)

// Severity indicates how critical a finding is.
type Severity string

const (
	SeverityCritical Severity = "critical"
	SeverityWarning  Severity = "warning"
	SeverityInfo     Severity = "info"
)

// Category classifies the type of finding.
type Category string

const (
	CategoryDestructive Category = "destructive"
	CategorySecurity    Category = "security"
	CategoryCostRisk    Category = "cost-risk"
)

// Finding is a single rule engine result.
type Finding struct {
	Severity        Severity
	Category        Category
	ResourceAddress string
	Explanation     string
}

// EvaluateContext bundles everything a rule may need to produce findings.
type EvaluateContext struct {
	Resources []normalizer.NormalizedResource
	Estimates []pricing.Estimate
}

// Rule evaluates the plan context and returns findings.
type Rule interface {
	Evaluate(ctx EvaluateContext) []Finding
}
