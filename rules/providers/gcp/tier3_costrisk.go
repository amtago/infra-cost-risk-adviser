package gcp

import (
	"fmt"
	"sort"
	"strings"

	"github.com/amt/tf-cost-risk/parser"
	"github.com/amt/tf-cost-risk/rules"
)

// OversizedResourceRule flags GCP resources whose cost exceeds OversizeMultiple × the plan median.
type OversizedResourceRule struct {
	OversizeMultiple float64
}

func (r *OversizedResourceRule) Evaluate(ctx rules.EvaluateContext) []rules.Finding {
	multiple := r.OversizeMultiple
	if multiple <= 0 {
		multiple = 5
	}

	// Collect costs for GCP non-delete resources only.
	gcpAddrs := map[string]bool{}
	for _, nr := range ctx.Resources {
		if nr.Provider == "gcp" && nr.ChangeType != parser.ChangeDelete {
			gcpAddrs[nr.Address] = true
		}
	}

	var knownCosts []float64
	costByAddr := map[string]float64{}
	for _, e := range ctx.Estimates {
		if e.Unknown || !gcpAddrs[e.ResourceAddress] {
			continue
		}
		knownCosts = append(knownCosts, e.MonthlyCostUSD)
		costByAddr[e.ResourceAddress] = e.MonthlyCostUSD
	}

	if len(knownCosts) < 2 {
		return nil
	}

	median := computeMedian(knownCosts)
	if median == 0 {
		return nil
	}

	threshold := median * multiple
	var findings []rules.Finding
	for addr, cost := range costByAddr {
		if cost > threshold {
			findings = append(findings, rules.Finding{
				Severity:        rules.SeverityWarning,
				Category:        rules.CategoryCostRisk,
				ResourceAddress: addr,
				Explanation: fmt.Sprintf(
					"%s costs an estimated $%.2f/mo, which is %.1fx the median resource cost in this plan ($%.2f/mo). Verify this instance size is intentional.",
					addr, cost, cost/median, median,
				),
			})
		}
	}
	return findings
}

func computeMedian(values []float64) float64 {
	sorted := make([]float64, len(values))
	copy(sorted, values)
	sort.Float64s(sorted)
	n := len(sorted)
	if n%2 == 0 {
		return (sorted[n/2-1] + sorted[n/2]) / 2
	}
	return sorted[n/2]
}

// MissingLabelsRule flags GCP resources missing required cost-allocation labels.
// GCP uses "labels" instead of AWS "tags"; the normalizer maps them to nr.Tags.
type MissingLabelsRule struct {
	RequiredLabels []string
}

func (r *MissingLabelsRule) Evaluate(ctx rules.EvaluateContext) []rules.Finding {
	required := r.RequiredLabels
	if ctx.RequiredTags != nil {
		required = ctx.RequiredTags
	}
	if len(required) == 0 {
		return nil
	}

	var findings []rules.Finding
	for _, nr := range ctx.Resources {
		if nr.Provider != "gcp" {
			continue
		}
		if nr.ChangeType == parser.ChangeDelete {
			continue
		}
		if nr.Raw == nil {
			continue
		}
		// Only check resources that support labels
		if _, hasLabels := nr.Raw["labels"]; !hasLabels {
			continue
		}
		var missing []string
		for _, label := range required {
			if _, ok := nr.Tags[label]; !ok {
				missing = append(missing, label)
			}
		}
		if len(missing) > 0 {
			findings = append(findings, rules.Finding{
				Severity:        rules.SeverityInfo,
				Category:        rules.CategoryCostRisk,
				ResourceAddress: nr.Address,
				Explanation: fmt.Sprintf(
					"%s is missing cost-allocation label(s): %s. Without these labels, spend cannot be attributed to a team or environment.",
					nr.Address, strings.Join(missing, ", "),
				),
			})
		}
	}
	return findings
}

// UnboundedGKEAutoscalingRule flags GKE node pools with cluster autoscaler enabled but no max node count.
type UnboundedGKEAutoscalingRule struct{}

func (r *UnboundedGKEAutoscalingRule) Evaluate(ctx rules.EvaluateContext) []rules.Finding {
	var findings []rules.Finding
	for _, nr := range ctx.Resources {
		if nr.Provider != "gcp" || nr.ResourceType != "google_container_node_pool" {
			continue
		}
		if nr.ChangeType == parser.ChangeDelete {
			continue
		}
		autoscaling := firstBlock(nr.Raw, "autoscaling")
		if autoscaling == nil {
			continue
		}
		maxNodeCount := intAttr(autoscaling, "max_node_count")
		if maxNodeCount <= 0 {
			findings = append(findings, rules.Finding{
				Severity:        rules.SeverityWarning,
				Category:        rules.CategoryCostRisk,
				ResourceAddress: nr.Address,
				Explanation: fmt.Sprintf(
					"%s (google_container_node_pool) has cluster autoscaling enabled with no max_node_count set. An unexpected traffic spike could scale this pool indefinitely and cause runaway costs.",
					nr.Address,
				),
			})
		}
	}
	return findings
}

