// Package gcp will normalize GCP Terraform resource changes. Not yet implemented.
package gcp

import (
	"errors"

	"github.com/amt/tf-cost-risk/normalizer"
	"github.com/amt/tf-cost-risk/parser"
)

// Normalizer is a stub for future GCP support.
type Normalizer struct{}

// Normalize is not yet implemented for GCP resources.
func (n *Normalizer) Normalize(rc parser.ResourceChange, region string) (normalizer.NormalizedResource, error) {
	return normalizer.NormalizedResource{}, errors.New("GCP normalizer not implemented")
}
