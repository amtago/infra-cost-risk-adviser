// Package azure will provide pricing estimates for Azure resources. Not yet implemented.
package azure

import (
	"github.com/amt/tf-cost-risk/normalizer"
	"github.com/amt/tf-cost-risk/pricing"
)

// Pricer is a stub for future Azure pricing support.
type Pricer struct{}

// Estimate is not yet implemented for Azure resources.
func (p *Pricer) Estimate(nr normalizer.NormalizedResource) pricing.Estimate {
	return pricing.Estimate{ResourceAddress: nr.Address, Unknown: true}
}
