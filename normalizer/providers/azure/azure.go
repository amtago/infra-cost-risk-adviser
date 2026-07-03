// Package azure will normalize Azure Terraform resource changes. Not yet implemented.
package azure

import (
	"errors"

	"github.com/amt/tf-cost-risk/normalizer"
	"github.com/amt/tf-cost-risk/parser"
)

// Normalizer is a stub for future Azure support.
type Normalizer struct{}

// Normalize is not yet implemented for Azure resources.
func (n *Normalizer) Normalize(rc parser.ResourceChange, region string) (normalizer.NormalizedResource, error) {
	return normalizer.NormalizedResource{}, errors.New("Azure normalizer not implemented")
}
