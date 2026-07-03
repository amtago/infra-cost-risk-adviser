// Package gcp provides static pricing estimates for GCP resources.
package gcp

import (
	"github.com/amt/tf-cost-risk/normalizer"
	"github.com/amt/tf-cost-risk/pricing"
)

// Pricer implements pricing.Pricer using a static in-repo price table.
type Pricer struct{}

// usageBased lists resource types whose cost depends on runtime usage.
// We return Unknown rather than $0 so the report is honest about uncertainty.
var usageBased = map[string]bool{
	"google_storage_bucket":           true,
	"google_cloudfunctions_function":  true,
	"google_cloudfunctions2_function": true,
	"google_pubsub_topic":             true,
	"google_bigquery_dataset":         true,
}

// Estimate returns a monthly cost estimate for a GCP resource.
// Prices are approximate us-central1 on-demand Linux rates; refreshed manually.
func (p *Pricer) Estimate(nr normalizer.NormalizedResource) pricing.Estimate {
	unknown := pricing.Estimate{ResourceAddress: nr.Address, ChangeType: nr.ChangeType, Unknown: true}

	if usageBased[nr.ResourceType] {
		return unknown
	}

	switch nr.ResourceType {
	case "google_compute_disk":
		return estimateDisk(nr)
	case "google_filestore_instance":
		return estimateFilestore(nr)
	case "google_container_cluster":
		// GKE management fee per cluster (regional/additional zonal)
		return pricing.Estimate{
			ResourceAddress: nr.Address,
			ChangeType:      nr.ChangeType,
			MonthlyCostUSD:  gkeMgmtFee,
		}
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

// estimateDisk computes monthly cost from disk type ($/GB-month) × size in GB.
func estimateDisk(nr normalizer.NormalizedResource) pricing.Estimate {
	unknown := pricing.Estimate{ResourceAddress: nr.Address, ChangeType: nr.ChangeType, Unknown: true}

	ratePerGB, ok := pdPricePerGB[nr.Size]
	if !ok {
		return unknown
	}
	rawSize, ok := nr.Raw["size"]
	if !ok {
		return unknown
	}
	gb := toGB(rawSize)
	if gb == 0 {
		return unknown
	}
	return pricing.Estimate{ResourceAddress: nr.Address, ChangeType: nr.ChangeType, MonthlyCostUSD: ratePerGB * gb}
}

// estimateFilestore computes monthly cost from tier ($/GB-month) × capacity in GB.
func estimateFilestore(nr normalizer.NormalizedResource) pricing.Estimate {
	unknown := pricing.Estimate{ResourceAddress: nr.Address, ChangeType: nr.ChangeType, Unknown: true}

	ratePerGB, ok := filestorePricePerGB[nr.Size]
	if !ok {
		return unknown
	}
	// Filestore capacity is in networks[0].capacity_gb or file_shares[0].capacity_gb
	gb := extractFilestoreCapacity(nr.Raw)
	if gb == 0 {
		return unknown
	}
	return pricing.Estimate{ResourceAddress: nr.Address, ChangeType: nr.ChangeType, MonthlyCostUSD: ratePerGB * gb}
}

func extractFilestoreCapacity(attrs map[string]interface{}) float64 {
	for _, key := range []string{"file_shares", "networks"} {
		if s, ok := attrs[key].([]interface{}); ok && len(s) > 0 {
			if m, ok := s[0].(map[string]interface{}); ok {
				if v, ok := m["capacity_gb"]; ok {
					return toGB(v)
				}
			}
		}
	}
	// Direct capacity_gb field
	if v, ok := attrs["capacity_gb"]; ok {
		return toGB(v)
	}
	return 0
}

func toGB(v interface{}) float64 {
	switch n := v.(type) {
	case float64:
		return n
	case int:
		return float64(n)
	case int64:
		return float64(n)
	}
	return 0
}
