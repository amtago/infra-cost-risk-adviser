// Package gcp normalizes GCP Terraform resource changes into the common internal schema.
package gcp

import (
	"fmt"
	"strings"

	"github.com/amt/tf-cost-risk/normalizer"
	"github.com/amt/tf-cost-risk/parser"
)

// Normalizer implements normalizer.Normalizer for GCP resources.
type Normalizer struct{}

var statefulTypes = map[string]bool{
	"google_sql_database_instance": true,
	"google_compute_disk":          true,
	"google_container_cluster":     true,
	"google_filestore_instance":    true,
	"google_bigtable_instance":     true,
	"google_spanner_instance":      true,
}

var categoryMap = map[string]normalizer.ResourceCategory{
	"google_compute_instance":         normalizer.CategoryCompute,
	"google_compute_instance_template": normalizer.CategoryCompute,
	"google_container_cluster":        normalizer.CategoryCompute,
	"google_container_node_pool":      normalizer.CategoryCompute,
	"google_sql_database_instance":    normalizer.CategoryDatabase,
	"google_compute_disk":             normalizer.CategoryStorage,
	"google_storage_bucket":           normalizer.CategoryStorage,
	"google_filestore_instance":       normalizer.CategoryStorage,
	"google_compute_firewall":         normalizer.CategoryNetwork,
	"google_compute_forwarding_rule":  normalizer.CategoryNetwork,
	"google_compute_global_forwarding_rule": normalizer.CategoryNetwork,
	"google_cloudfunctions_function":  normalizer.CategoryFunction,
	"google_cloudfunctions2_function": normalizer.CategoryFunction,
	"google_pubsub_topic":             normalizer.CategoryUnknown,
	"google_bigquery_dataset":         normalizer.CategoryUnknown,
}

// Normalize maps a GCP resource change to a NormalizedResource.
func (n *Normalizer) Normalize(rc parser.ResourceChange, region string) (normalizer.NormalizedResource, error) {
	attrs := rc.After
	if attrs == nil {
		attrs = rc.Before
	}
	if attrs == nil {
		attrs = map[string]interface{}{}
	}

	cat, ok := categoryMap[rc.Type]
	if !ok {
		cat = normalizer.CategoryUnknown
	}

	nr := normalizer.NormalizedResource{
		Address:      rc.Address,
		ChangeType:   rc.ChangeType,
		Provider:     "gcp",
		ResourceType: rc.Type,
		Category:     cat,
		Size:         extractSize(rc.Type, attrs),
		Region:       extractRegion(attrs, region),
		Tags:         extractLabels(attrs),
		Stateful:     statefulTypes[rc.Type],
		Raw:          attrs,
	}
	return nr, nil
}

func extractSize(resourceType string, attrs map[string]interface{}) string {
	switch resourceType {
	case "google_compute_instance", "google_compute_instance_template":
		return strAttr(attrs, "machine_type") // e.g. "n2-standard-4"
	case "google_sql_database_instance":
		// Nested under settings[0].tier in Terraform state
		if settings := sliceAttr(attrs, "settings"); len(settings) > 0 {
			if s, ok := settings[0].(map[string]interface{}); ok {
				return strAttr(s, "tier") // e.g. "db-n1-standard-2"
			}
		}
	case "google_compute_disk":
		return strAttr(attrs, "type") // pd-ssd, pd-standard, pd-balanced
	case "google_container_cluster":
		return strAttr(attrs, "node_config.0.machine_type")
	case "google_container_node_pool":
		if nc := sliceAttr(attrs, "node_config"); len(nc) > 0 {
			if s, ok := nc[0].(map[string]interface{}); ok {
				return strAttr(s, "machine_type")
			}
		}
	case "google_filestore_instance":
		return strAttr(attrs, "tier") // BASIC_HDD, BASIC_SSD, ENTERPRISE
	case "google_cloudfunctions_function":
		if mem, ok := attrs["available_memory_mb"]; ok {
			return fmt.Sprintf("%dMB", int(toFloat64(mem)))
		}
	case "google_cloudfunctions2_function":
		if sc := sliceAttr(attrs, "service_config"); len(sc) > 0 {
			if s, ok := sc[0].(map[string]interface{}); ok {
				if mem, ok := s["available_memory"]; ok {
					return fmt.Sprintf("%v", mem)
				}
			}
		}
	}
	return ""
}

// extractRegion resolves the GCP region from resource attributes.
// GCP resources use "region", "location", or "zone" (zone → strip suffix to get region).
func extractRegion(attrs map[string]interface{}, fallback string) string {
	if r := strAttr(attrs, "region"); r != "" {
		return r
	}
	if l := strAttr(attrs, "location"); l != "" {
		// GKE and Cloud SQL use "location" which can be a region or zone
		return stripZoneSuffix(l)
	}
	if z := strAttr(attrs, "zone"); z != "" {
		return stripZoneSuffix(z)
	}
	return fallback
}

// stripZoneSuffix converts a GCP zone (us-central1-a) to a region (us-central1).
func stripZoneSuffix(z string) string {
	parts := strings.Split(z, "-")
	if len(parts) > 2 {
		// zone format: region-zone e.g. us-central1-a → us-central1
		return strings.Join(parts[:len(parts)-1], "-")
	}
	return z
}

// extractLabels maps GCP labels (map[string]string in Terraform state) to the common Tags field.
// GCP uses "labels" instead of AWS "tags" — we unify them here so tag rules work unchanged.
func extractLabels(attrs map[string]interface{}) map[string]string {
	out := map[string]string{}
	for _, key := range []string{"labels", "tags"} {
		raw, ok := attrs[key]
		if !ok {
			continue
		}
		m, ok := raw.(map[string]interface{})
		if !ok {
			continue
		}
		for k, v := range m {
			if s, ok := v.(string); ok {
				out[k] = s
			}
		}
	}
	return out
}

func strAttr(attrs map[string]interface{}, key string) string {
	v, ok := attrs[key]
	if !ok {
		return ""
	}
	s, _ := v.(string)
	return s
}

func sliceAttr(attrs map[string]interface{}, key string) []interface{} {
	v, ok := attrs[key]
	if !ok {
		return nil
	}
	s, _ := v.([]interface{})
	return s
}

func toFloat64(v interface{}) float64 {
	switch n := v.(type) {
	case float64:
		return n
	case int:
		return float64(n)
	}
	return 0
}
