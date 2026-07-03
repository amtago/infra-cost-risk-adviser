// Package gcp will provide pricing estimates for GCP resources. Not yet implemented.
package gcp

import (
	"github.com/amt/tf-cost-risk/normalizer"
	"github.com/amt/tf-cost-risk/pricing"
)

// Pricer is a stub for future GCP pricing support.
type Pricer struct{}

// Estimate is not yet implemented for GCP resources.
func (p *Pricer) Estimate(nr normalizer.NormalizedResource) pricing.Estimate {
	return pricing.Estimate{ResourceAddress: nr.Address, Unknown: true}
}
