// Package aws provides static pricing estimates for AWS resources.
package aws

import (
	"github.com/amt/tf-cost-risk/normalizer"
	"github.com/amt/tf-cost-risk/pricing"
)

// Pricer implements pricing.Pricer using a static in-repo price table.
type Pricer struct{}

// usageBased lists resource types whose cost depends on runtime usage, not instance size.
// We cannot estimate these without usage data, so we return Unknown rather than $0.
var usageBased = map[string]bool{
	"aws_s3_bucket":      true,
	"aws_lambda_function": true,
	"aws_dynamodb_table": true, // PAY_PER_REQUEST mode especially; PROVISIONED could be estimated but omit for MVP
}

// Estimate returns a monthly cost estimate for an AWS resource.
// Prices are approximate us-east-1 on-demand Linux rates; refreshed manually.
func (p *Pricer) Estimate(nr normalizer.NormalizedResource) pricing.Estimate {
	unknown := pricing.Estimate{ResourceAddress: nr.Address, ChangeType: nr.ChangeType, Unknown: true}

	if usageBased[nr.ResourceType] {
		return unknown
	}

	// EBS volumes are priced per GB-month, not per instance type.
	if nr.ResourceType == "aws_ebs_volume" {
		e := estimateEBS(nr)
		e.ChangeType = nr.ChangeType
		return e
	}

	bySize, ok := staticPrices[nr.ResourceType]
	if !ok {
		return unknown
	}
	cost, ok := bySize[nr.Size]
	if !ok {
		return unknown
	}
	return pricing.Estimate{ResourceAddress: nr.Address, ChangeType: nr.ChangeType, MonthlyCostUSD: cost}
}

// estimateEBS computes monthly cost from volume type ($/GB-month) × size in GB.
func estimateEBS(nr normalizer.NormalizedResource) pricing.Estimate {
	unknown := pricing.Estimate{ResourceAddress: nr.Address, ChangeType: nr.ChangeType, Unknown: true}

	ratePerGB, ok := ebsPricePerGB[nr.Size]
	if !ok {
		return unknown
	}
	// size is stored in the raw attributes as a number
	rawSize, ok := nr.Raw["size"]
	if !ok {
		return unknown
	}
	var gb float64
	switch v := rawSize.(type) {
	case float64:
		gb = v
	case int:
		gb = float64(v)
	case int64:
		gb = float64(v)
	default:
		return unknown
	}
	return pricing.Estimate{ResourceAddress: nr.Address, MonthlyCostUSD: ratePerGB * gb}
}
