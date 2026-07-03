// Package normalizer maps provider-specific resource types into a common internal schema.
package normalizer

import "github.com/amt/tf-cost-risk/parser"

// ResourceCategory groups resource types for rule and pricing logic.
type ResourceCategory string

const (
	CategoryCompute  ResourceCategory = "compute"
	CategoryDatabase ResourceCategory = "database"
	CategoryStorage  ResourceCategory = "storage"
	CategoryNetwork  ResourceCategory = "network"
	CategoryFunction ResourceCategory = "function"
	CategoryUnknown  ResourceCategory = "unknown"
)

// NormalizedResource is the common internal representation used by pricing and rules.
type NormalizedResource struct {
	Address      string
	ChangeType   parser.ChangeType
	Provider     string
	ResourceType string // provider-specific type, e.g. "aws_instance"
	Category     ResourceCategory
	// Size is the instance type, class, or tier (e.g. "t3.medium", "db.t3.micro", "gp3").
	Size     string
	Region   string
	Tags     map[string]string
	Stateful bool
	// Raw holds the original after (or before for deletes) attributes for rule inspection.
	Raw map[string]interface{}
}

// Normalizer converts a provider resource change into a NormalizedResource.
type Normalizer interface {
	Normalize(rc parser.ResourceChange, region string) (NormalizedResource, error)
}
