package azure

import (
	"fmt"
	"sort"
	"strings"

	"github.com/amt/tf-cost-risk/parser"
	"github.com/amt/tf-cost-risk/rules"
)

// OversizedResourceRule flags Azure resources whose cost exceeds OversizeMultiple × the plan median.
type OversizedResourceRule struct {
	OversizeMultiple float64
}

func (r *OversizedResourceRule) Evaluate(ctx rules.EvaluateContext) []rules.Finding {
	multiple := r.OversizeMultiple
	if multiple <= 0 {
		multiple = 5
	}

	azureAddrs := map[string]bool{}
	for _, nr := range ctx.Resources {
		if nr.Provider == "azure" && nr.ChangeType != parser.ChangeDelete {
			azureAddrs[nr.Address] = true
		}
	}

	var knownCosts []float64
	costByAddr := map[string]float64{}
	for _, e := range ctx.Estimates {
		if e.Unknown || !azureAddrs[e.ResourceAddress] {
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
					"%s costs an estimated $%.2f/mo, which is %.1fx the median resource cost in this plan ($%.2f/mo). Verify this VM size is intentional.",
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

// MissingTagsRule flags Azure resources missing required cost-allocation tags.
type MissingTagsRule struct {
	RequiredTags []string
}

func (r *MissingTagsRule) Evaluate(ctx rules.EvaluateContext) []rules.Finding {
	required := r.RequiredTags
	if ctx.RequiredTags != nil {
		required = ctx.RequiredTags
	}
	if len(required) == 0 {
		return nil
	}

	var findings []rules.Finding
	for _, nr := range ctx.Resources {
		if nr.Provider != "azure" {
			continue
		}
		if nr.ChangeType == parser.ChangeDelete {
			continue
		}
		if nr.Raw == nil {
			continue
		}
		if _, hasTags := nr.Raw["tags"]; !hasTags {
			continue
		}
		var missing []string
		for _, tag := range required {
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
					"%s is missing cost-allocation tag(s): %s. Without these tags, Azure Cost Management cannot attribute spend to a team or environment.",
					nr.Address, strings.Join(missing, ", "),
				),
			})
		}
	}
	return findings
}

// UnboundedAKSAutoscalingRule flags AKS node pools with cluster autoscaler enabled but no max_count.
type UnboundedAKSAutoscalingRule struct{}

func (r *UnboundedAKSAutoscalingRule) Evaluate(ctx rules.EvaluateContext) []rules.Finding {
	var findings []rules.Finding
	for _, nr := range ctx.Resources {
		if nr.Provider != "azure" || nr.ResourceType != "azurerm_kubernetes_cluster" {
			continue
		}
		if nr.ChangeType == parser.ChangeDelete {
			continue
		}
		np := firstBlock(nr.Raw, "default_node_pool")
		if np == nil {
			continue
		}
		if !boolAttr(np, "enable_auto_scaling") {
			continue
		}
		maxCount := intAttr(np, "max_count")
		if maxCount <= 0 {
			findings = append(findings, rules.Finding{
				Severity:        rules.SeverityWarning,
				Category:        rules.CategoryCostRisk,
				ResourceAddress: nr.Address,
				Explanation: fmt.Sprintf(
					"%s (azurerm_kubernetes_cluster) has cluster autoscaling enabled on the default node pool with no max_count set. An unexpected traffic spike could scale nodes indefinitely and cause runaway costs.",
					nr.Address,
				),
			})
		}
	}
	return findings
}
