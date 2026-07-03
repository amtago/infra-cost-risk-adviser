package gcp

// staticPrices maps resource type → machine type/tier → approximate monthly USD cost.
// Source: GCP us-central1 on-demand pricing as of 2025-06.
// Hourly rates converted to monthly using 730 hours/month.
// Refresh manually; do not rely on this for production billing.
var staticPrices = map[string]map[string]float64{
	// Compute Engine — on-demand, us-central1, Linux
	"google_compute_instance": {
		// E2 (cost-optimised general purpose)
		"e2-micro":      6.11,
		"e2-small":      12.23,
		"e2-medium":     24.46,
		"e2-standard-2": 48.92,
		"e2-standard-4": 97.84,
		"e2-standard-8": 195.68,
		"e2-highmem-2":  60.74,
		"e2-highmem-4":  121.47,
		"e2-highmem-8":  242.94,
		// N2 (balanced general purpose)
		"n2-standard-2":  58.40,
		"n2-standard-4":  116.80,
		"n2-standard-8":  233.60,
		"n2-standard-16": 467.20,
		"n2-standard-32": 934.40,
		"n2-highmem-2":   80.30,
		"n2-highmem-4":   160.60,
		"n2-highmem-8":   321.20,
		// N1 (first-gen general purpose, still widely used)
		"n1-standard-1":  24.27,
		"n1-standard-2":  48.54,
		"n1-standard-4":  97.09,
		"n1-standard-8":  194.18,
		"n1-standard-16": 388.36,
		"n1-highmem-2":   60.73,
		"n1-highmem-4":   121.46,
		"n1-highmem-8":   242.92,
		// C2 (compute optimised)
		"c2-standard-4":  152.69,
		"c2-standard-8":  305.38,
		"c2-standard-16": 610.76,
	},

	// Same machine types apply to instance templates (priced same as instances)
	"google_compute_instance_template": {
		"e2-micro":      6.11,
		"e2-small":      12.23,
		"e2-medium":     24.46,
		"e2-standard-2": 48.92,
		"e2-standard-4": 97.84,
		"e2-standard-8": 195.68,
		"n2-standard-2": 58.40,
		"n2-standard-4": 116.80,
		"n2-standard-8": 233.60,
		"n1-standard-1": 24.27,
		"n1-standard-2": 48.54,
		"n1-standard-4": 97.09,
		"n1-standard-8": 194.18,
	},

	// GKE node pools — same Compute Engine rates (cluster management fee separate)
	"google_container_node_pool": {
		"e2-standard-2": 48.92,
		"e2-standard-4": 97.84,
		"e2-standard-8": 195.68,
		"n2-standard-2": 58.40,
		"n2-standard-4": 116.80,
		"n2-standard-8": 233.60,
		"n1-standard-1": 24.27,
		"n1-standard-2": 48.54,
		"n1-standard-4": 97.09,
	},

	// Cloud SQL — on-demand, us-central1, single instance
	"google_sql_database_instance": {
		"db-f1-micro":       7.67,
		"db-g1-small":       25.55,
		"db-n1-standard-1":  46.60,
		"db-n1-standard-2":  93.19,
		"db-n1-standard-4":  186.38,
		"db-n1-standard-8":  372.76,
		"db-n1-highmem-2":   117.34,
		"db-n1-highmem-4":   234.67,
		"db-n1-highmem-8":   469.34,
	},

	// Cloud Load Balancing — base monthly rate (traffic excluded)
	"google_compute_forwarding_rule": {
		// External HTTP(S) LB — $0.025/hr base = $18.25/mo
		"EXTERNAL": 18.25,
		"INTERNAL": 5.11,
	},
	"google_compute_global_forwarding_rule": {
		"EXTERNAL": 18.25,
	},
}

// pdPricePerGB maps Persistent Disk type to $/GB-month (us-central1).
var pdPricePerGB = map[string]float64{
	"pd-standard": 0.040,
	"pd-balanced": 0.100,
	"pd-ssd":      0.170,
	"pd-extreme":  0.125,
}

// filestorePricePerGB maps Filestore tier to $/GB-month (us-central1).
var filestorePricePerGB = map[string]float64{
	"BASIC_HDD":  0.20,
	"BASIC_SSD":  0.38,
	"ENTERPRISE": 0.60,
}

// gkeMgmtFee is the GKE cluster management fee ($/mo) — charged per cluster, not per node.
// Free for the first zonal cluster; $73/mo for regional or additional clusters.
const gkeMgmtFee = 73.00
