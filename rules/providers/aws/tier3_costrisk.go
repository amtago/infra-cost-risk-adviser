package aws

import (
	"fmt"
	"sort"
	"strings"

	"github.com/amt/tf-cost-risk/parser"
	"github.com/amt/tf-cost-risk/rules"
)

// OversizedResourceRule flags resources whose estimated monthly cost exceeds OversizeMultiple × the
// median cost of all priced resources in the plan.
type OversizedResourceRule struct {
	OversizeMultiple float64 // default 5
}

func (r *OversizedResourceRule) Evaluate(ctx rules.EvaluateContext) []rules.Finding {
	multiple := r.OversizeMultiple
	if multiple <= 0 {
		multiple = 5
	}

	// Collect known costs (exclude Unknown and deletes).
	addrToNR := make(map[string]struct{ changeType parser.ChangeType })
	for _, nr := range ctx.Resources {
		addrToNR[nr.Address] = struct{ changeType parser.ChangeType }{nr.ChangeType}
	}

	var knownCosts []float64
	costByAddr := map[string]float64{}
	for _, e := range ctx.Estimates {
		if e.Unknown {
			continue
		}
		info, ok := addrToNR[e.ResourceAddress]
		if !ok || info.changeType == parser.ChangeDelete {
			continue
		}
		knownCosts = append(knownCosts, e.MonthlyCostUSD)
		costByAddr[e.ResourceAddress] = e.MonthlyCostUSD
	}

	if len(knownCosts) < 2 {
		// Need at least 2 priced resources to compute a meaningful median comparison.
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

// MissingCostTagsRule flags new or updated resources that are missing required cost-allocation tags.
type MissingCostTagsRule struct {
	RequiredTags []string // e.g. ["Env", "Team"]
}

func (r *MissingCostTagsRule) Evaluate(ctx rules.EvaluateContext) []rules.Finding {
	if len(r.RequiredTags) == 0 {
		return nil
	}
	var findings []rules.Finding
	for _, nr := range ctx.Resources {
		if nr.ChangeType == parser.ChangeDelete {
			continue
		}
		// Resources that don't support tags (Lambda@Edge, etc.) have no tags attr — skip.
		if nr.Raw == nil {
			continue
		}
		if _, hasTagsAttr := nr.Raw["tags"]; !hasTagsAttr {
			// No tags attribute at all on this resource type — skip rather than false-positive.
			continue
		}
		var missing []string
		for _, tag := range r.RequiredTags {
			if _, ok := nr.Tags[tag]; !ok {
				missing = append(missing, tag)
			}
		}
		if len(missing) > 0 {
			findings = append(findings, rules.Finding{
				Severity:        rules.SeverityInfo,
				Category:        rules.CategoryCostRisk,
				ResourceAddress: nr.Address,
				Explanation: fmt.Sprintf(
					"%s is missing cost-allocation tag(s): %s. Without these tags, spend cannot be attributed to a team or environment.",
					nr.Address, strings.Join(missing, ", "),
				),
			})
		}
	}
	return findings
}

// UnboundedAutoscalingRule flags autoscaling groups with no max_size set or max_size = 0.
type UnboundedAutoscalingRule struct{}

func (r *UnboundedAutoscalingRule) Evaluate(ctx rules.EvaluateContext) []rules.Finding {
	var findings []rules.Finding
	for _, nr := range ctx.Resources {
		if nr.ResourceType != "aws_autoscaling_group" {
			continue
		}
		if nr.ChangeType == parser.ChangeDelete {
			continue
		}
		maxSize := intAttr(nr.Raw, "max_size")
		if maxSize <= 0 {
			findings = append(findings, rules.Finding{
				Severity:        rules.SeverityWarning,
				Category:        rules.CategoryCostRisk,
				ResourceAddress: nr.Address,
				Explanation: fmt.Sprintf(
					"%s (aws_autoscaling_group) has no upper bound on scaling (max_size = %d). An unexpected traffic spike could scale this group indefinitely and cause runaway costs.",
					nr.Address, maxSize,
				),
			})
		}
	}
	return findings
}
