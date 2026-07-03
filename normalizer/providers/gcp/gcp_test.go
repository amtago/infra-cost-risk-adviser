package gcp

import (
	"testing"

	"github.com/amt/tf-cost-risk/normalizer"
	"github.com/amt/tf-cost-risk/parser"
)

func normalize(t *testing.T, rc parser.ResourceChange) normalizer.NormalizedResource {
	t.Helper()
	n := &Normalizer{}
	nr, err := n.Normalize(rc, "us-central1")
	if err != nil {
		t.Fatalf("Normalize() error: %v", err)
	}
	return nr
}

func change(resourceType string, attrs map[string]interface{}) parser.ResourceChange {
	return parser.ResourceChange{
		Address:    resourceType + ".example",
		Type:       resourceType,
		ChangeType: parser.ChangeCreate,
		After:      attrs,
	}
}

// -- provider field --

func TestNormalize_Provider(t *testing.T) {
	nr := normalize(t, change("google_compute_instance", map[string]interface{}{
		"machine_type": "n2-standard-4",
	}))
	if nr.Provider != "gcp" {
		t.Errorf("expected provider=gcp, got %s", nr.Provider)
	}
}

// -- category mapping --

func TestNormalize_Category_Compute(t *testing.T) {
	nr := normalize(t, change("google_compute_instance", map[string]interface{}{}))
	if nr.Category != normalizer.CategoryCompute {
		t.Errorf("expected compute, got %s", nr.Category)
	}
}

func TestNormalize_Category_Database(t *testing.T) {
	nr := normalize(t, change("google_sql_database_instance", map[string]interface{}{}))
	if nr.Category != normalizer.CategoryDatabase {
		t.Errorf("expected database, got %s", nr.Category)
	}
}

func TestNormalize_Category_Storage(t *testing.T) {
	nr := normalize(t, change("google_compute_disk", map[string]interface{}{}))
	if nr.Category != normalizer.CategoryStorage {
		t.Errorf("expected storage, got %s", nr.Category)
	}
}

func TestNormalize_Category_Network(t *testing.T) {
	nr := normalize(t, change("google_compute_firewall", map[string]interface{}{}))
	if nr.Category != normalizer.CategoryNetwork {
		t.Errorf("expected network, got %s", nr.Category)
	}
}

func TestNormalize_Category_Function(t *testing.T) {
	nr := normalize(t, change("google_cloudfunctions_function", map[string]interface{}{}))
	if nr.Category != normalizer.CategoryFunction {
		t.Errorf("expected function, got %s", nr.Category)
	}
}

func TestNormalize_Category_Unknown(t *testing.T) {
	nr := normalize(t, change("google_dns_record_set", map[string]interface{}{}))
	if nr.Category != normalizer.CategoryUnknown {
		t.Errorf("expected unknown, got %s", nr.Category)
	}
}

// -- stateful flag --

func TestNormalize_Stateful_SQLInstance(t *testing.T) {
	nr := normalize(t, change("google_sql_database_instance", map[string]interface{}{}))
	if !nr.Stateful {
		t.Error("google_sql_database_instance should be stateful")
	}
}

func TestNormalize_Stateful_ComputeDisk(t *testing.T) {
	nr := normalize(t, change("google_compute_disk", map[string]interface{}{}))
	if !nr.Stateful {
		t.Error("google_compute_disk should be stateful")
	}
}

func TestNormalize_Stateful_GKECluster(t *testing.T) {
	nr := normalize(t, change("google_container_cluster", map[string]interface{}{}))
	if !nr.Stateful {
		t.Error("google_container_cluster should be stateful")
	}
}

func TestNormalize_NotStateful_ComputeInstance(t *testing.T) {
	nr := normalize(t, change("google_compute_instance", map[string]interface{}{}))
	if nr.Stateful {
		t.Error("google_compute_instance should not be stateful")
	}
}

// -- size extraction --

func TestNormalize_Size_ComputeInstance(t *testing.T) {
	nr := normalize(t, change("google_compute_instance", map[string]interface{}{
		"machine_type": "n2-standard-4",
	}))
	if nr.Size != "n2-standard-4" {
		t.Errorf("expected n2-standard-4, got %s", nr.Size)
	}
}

func TestNormalize_Size_SQLInstance(t *testing.T) {
	nr := normalize(t, change("google_sql_database_instance", map[string]interface{}{
		"settings": []interface{}{
			map[string]interface{}{"tier": "db-n1-standard-2"},
		},
	}))
	if nr.Size != "db-n1-standard-2" {
		t.Errorf("expected db-n1-standard-2, got %s", nr.Size)
	}
}

func TestNormalize_Size_ComputeDisk(t *testing.T) {
	nr := normalize(t, change("google_compute_disk", map[string]interface{}{
		"type": "pd-ssd",
	}))
	if nr.Size != "pd-ssd" {
		t.Errorf("expected pd-ssd, got %s", nr.Size)
	}
}

func TestNormalize_Size_CloudFunction(t *testing.T) {
	nr := normalize(t, change("google_cloudfunctions_function", map[string]interface{}{
		"available_memory_mb": float64(512),
	}))
	if nr.Size != "512MB" {
		t.Errorf("expected 512MB, got %s", nr.Size)
	}
}

func TestNormalize_Size_NodePool(t *testing.T) {
	nr := normalize(t, change("google_container_node_pool", map[string]interface{}{
		"node_config": []interface{}{
			map[string]interface{}{"machine_type": "e2-standard-4"},
		},
	}))
	if nr.Size != "e2-standard-4" {
		t.Errorf("expected e2-standard-4, got %s", nr.Size)
	}
}

// -- region extraction --

func TestNormalize_Region_FromRegion(t *testing.T) {
	nr := normalize(t, change("google_compute_instance", map[string]interface{}{
		"region": "europe-west1",
	}))
	if nr.Region != "europe-west1" {
		t.Errorf("expected europe-west1, got %s", nr.Region)
	}
}

func TestNormalize_Region_FromLocation(t *testing.T) {
	nr := normalize(t, change("google_sql_database_instance", map[string]interface{}{
		"location": "us-east1",
	}))
	if nr.Region != "us-east1" {
		t.Errorf("expected us-east1, got %s", nr.Region)
	}
}

func TestNormalize_Region_FromZone(t *testing.T) {
	nr := normalize(t, change("google_compute_instance", map[string]interface{}{
		"zone": "us-central1-a",
	}))
	if nr.Region != "us-central1" {
		t.Errorf("expected us-central1 (zone suffix stripped), got %s", nr.Region)
	}
}

func TestNormalize_Region_Fallback(t *testing.T) {
	nr := normalize(t, change("google_compute_instance", map[string]interface{}{}))
	if nr.Region != "us-central1" {
		t.Errorf("expected fallback us-central1, got %s", nr.Region)
	}
}

// -- labels → Tags --

func TestNormalize_Labels_MappedToTags(t *testing.T) {
	nr := normalize(t, change("google_compute_instance", map[string]interface{}{
		"labels": map[string]interface{}{
			"env":  "prod",
			"team": "platform",
		},
	}))
	if nr.Tags["env"] != "prod" || nr.Tags["team"] != "platform" {
		t.Errorf("labels not mapped to tags, got %v", nr.Tags)
	}
}

func TestNormalize_NilAttrs_UsesBeforeForDeletes(t *testing.T) {
	rc := parser.ResourceChange{
		Address:    "google_compute_disk.old",
		Type:       "google_compute_disk",
		ChangeType: parser.ChangeDelete,
		Before:     map[string]interface{}{"type": "pd-ssd"},
		After:      nil,
	}
	nr := normalize(t, rc)
	if nr.Size != "pd-ssd" {
		t.Errorf("expected size from Before attrs, got %s", nr.Size)
	}
}
