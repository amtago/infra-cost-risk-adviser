package gcp

import (
	"testing"

	"github.com/amt/tf-cost-risk/normalizer"
	"github.com/amt/tf-cost-risk/parser"
)

func estimate(resourceType, size string, raw map[string]interface{}) float64 {
	p := &Pricer{}
	nr := normalizer.NormalizedResource{
		Address:      resourceType + ".test",
		ResourceType: resourceType,
		ChangeType:   parser.ChangeCreate,
		Size:         size,
		Raw:          raw,
	}
	return p.Estimate(nr).MonthlyCostUSD
}

func isUnknown(resourceType, size string, raw map[string]interface{}) bool {
	p := &Pricer{}
	nr := normalizer.NormalizedResource{
		Address:      resourceType + ".test",
		ResourceType: resourceType,
		ChangeType:   parser.ChangeCreate,
		Size:         size,
		Raw:          raw,
	}
	return p.Estimate(nr).Unknown
}

// -- Compute Engine --

func TestEstimate_ComputeInstance_KnownType(t *testing.T) {
	cost := estimate("google_compute_instance", "n2-standard-4", nil)
	if cost != 116.80 {
		t.Errorf("expected 116.80, got %.2f", cost)
	}
}

func TestEstimate_ComputeInstance_E2Micro(t *testing.T) {
	cost := estimate("google_compute_instance", "e2-micro", nil)
	if cost != 6.11 {
		t.Errorf("expected 6.11, got %.2f", cost)
	}
}

func TestEstimate_ComputeInstance_UnknownType(t *testing.T) {
	if !isUnknown("google_compute_instance", "a3-megagpu-8g", nil) {
		t.Error("unknown machine type should return Unknown=true")
	}
}

// -- Cloud SQL --

func TestEstimate_SQLInstance_Standard(t *testing.T) {
	cost := estimate("google_sql_database_instance", "db-n1-standard-2", nil)
	if cost != 93.19 {
		t.Errorf("expected 93.19, got %.2f", cost)
	}
}

func TestEstimate_SQLInstance_Micro(t *testing.T) {
	cost := estimate("google_sql_database_instance", "db-f1-micro", nil)
	if cost != 7.67 {
		t.Errorf("expected 7.67, got %.2f", cost)
	}
}

func TestEstimate_SQLInstance_Unknown(t *testing.T) {
	if !isUnknown("google_sql_database_instance", "db-custom-2-8192", nil) {
		t.Error("custom SQL tier should return Unknown=true")
	}
}

// -- Persistent Disk --

func TestEstimate_Disk_SSD(t *testing.T) {
	cost := estimate("google_compute_disk", "pd-ssd", map[string]interface{}{"size": float64(100)})
	want := 0.170 * 100
	if cost != want {
		t.Errorf("expected %.2f, got %.2f", want, cost)
	}
}

func TestEstimate_Disk_Standard(t *testing.T) {
	cost := estimate("google_compute_disk", "pd-standard", map[string]interface{}{"size": float64(500)})
	want := 0.040 * 500
	if cost != want {
		t.Errorf("expected %.2f, got %.2f", want, cost)
	}
}

func TestEstimate_Disk_UnknownType(t *testing.T) {
	if !isUnknown("google_compute_disk", "hyperdisk-balanced", map[string]interface{}{"size": float64(100)}) {
		t.Error("unknown disk type should return Unknown=true")
	}
}

func TestEstimate_Disk_MissingSize(t *testing.T) {
	if !isUnknown("google_compute_disk", "pd-ssd", map[string]interface{}{}) {
		t.Error("missing size should return Unknown=true")
	}
}

// -- Filestore --

func TestEstimate_Filestore_BasicSSD(t *testing.T) {
	raw := map[string]interface{}{
		"file_shares": []interface{}{
			map[string]interface{}{"capacity_gb": float64(1024)},
		},
	}
	cost := estimate("google_filestore_instance", "BASIC_SSD", raw)
	want := 0.38 * 1024
	if cost != want {
		t.Errorf("expected %.2f, got %.2f", want, cost)
	}
}

func TestEstimate_Filestore_BasicHDD(t *testing.T) {
	raw := map[string]interface{}{
		"capacity_gb": float64(2048),
	}
	cost := estimate("google_filestore_instance", "BASIC_HDD", raw)
	want := 0.20 * 2048
	if cost != want {
		t.Errorf("expected %.2f, got %.2f", want, cost)
	}
}

// -- GKE cluster management fee --

func TestEstimate_GKECluster_ManagementFee(t *testing.T) {
	cost := estimate("google_container_cluster", "", nil)
	if cost != gkeMgmtFee {
		t.Errorf("expected %.2f (management fee), got %.2f", gkeMgmtFee, cost)
	}
}

// -- Node pool --

func TestEstimate_NodePool_KnownType(t *testing.T) {
	cost := estimate("google_container_node_pool", "n2-standard-4", nil)
	if cost != 116.80 {
		t.Errorf("expected 116.80, got %.2f", cost)
	}
}

// -- Usage-based (always unknown) --

func TestEstimate_StorageBucket_Unknown(t *testing.T) {
	if !isUnknown("google_storage_bucket", "", nil) {
		t.Error("google_storage_bucket should always be unknown")
	}
}

func TestEstimate_CloudFunction_Unknown(t *testing.T) {
	if !isUnknown("google_cloudfunctions_function", "512MB", nil) {
		t.Error("google_cloudfunctions_function should always be unknown")
	}
}

func TestEstimate_PubSub_Unknown(t *testing.T) {
	if !isUnknown("google_pubsub_topic", "", nil) {
		t.Error("google_pubsub_topic should always be unknown")
	}
}

// -- Unrecognised resource type --

func TestEstimate_UnknownResourceType(t *testing.T) {
	if !isUnknown("google_dns_record_set", "", nil) {
		t.Error("unrecognised resource type should return Unknown=true")
	}
}

// -- ChangeType propagated --

func TestEstimate_ChangeType_Propagated(t *testing.T) {
	p := &Pricer{}
	nr := normalizer.NormalizedResource{
		Address:      "google_compute_instance.web",
		ResourceType: "google_compute_instance",
		ChangeType:   parser.ChangeDelete,
		Size:         "n2-standard-4",
	}
	e := p.Estimate(nr)
	if e.ChangeType != parser.ChangeDelete {
		t.Errorf("expected ChangeType=delete, got %s", e.ChangeType)
	}
}
