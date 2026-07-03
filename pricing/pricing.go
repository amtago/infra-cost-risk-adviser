// Package pricing matches normalized resources to monthly cost estimates.
package pricing

import (
	"github.com/amt/tf-cost-risk/normalizer"
	"github.com/amt/tf-cost-risk/parser"
)

// Estimate holds the pricing result for a single resource.
type Estimate struct {
	ResourceAddress string
	ChangeType      parser.ChangeType
	MonthlyCostUSD  float64
	// Unknown is true when no price entry was found for this resource.
	Unknown bool
}

// Pricer returns a cost estimate for a normalized resource.
type Pricer interface {
	Estimate(nr normalizer.NormalizedResource) Estimate
}
