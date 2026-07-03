package aws

// staticPrices maps resource type → size/tier → approximate monthly USD cost.
// Source: AWS us-east-1 on-demand Linux pricing as of 2025-06.
// Hourly rates converted to monthly using 730 hours/month.
// Refresh manually or via script; do not rely on this for production billing.
var staticPrices = map[string]map[string]float64{
	// EC2 — on-demand Linux, us-east-1
	"aws_instance": {
		// T3 (burstable, general purpose)
		"t3.nano":    3.80,
		"t3.micro":   7.59,
		"t3.small":   15.18,
		"t3.medium":  30.37,
		"t3.large":   60.74,
		"t3.xlarge":  121.47,
		"t3.2xlarge": 242.94,
		// M5 (general purpose)
		"m5.large":    70.08,
		"m5.xlarge":   140.16,
		"m5.2xlarge":  280.32,
		"m5.4xlarge":  560.64,
		"m5.8xlarge":  1121.28,
		"m5.12xlarge": 1681.92,
		// C5 (compute optimized)
		"c5.large":    62.05,
		"c5.xlarge":   124.10,
		"c5.2xlarge":  248.20,
		"c5.4xlarge":  496.40,
		// R5 (memory optimized)
		"r5.large":   91.98,
		"r5.xlarge":  183.96,
		"r5.2xlarge": 367.92,
		"r5.4xlarge": 735.84,
	},

	// RDS — on-demand, single-AZ, us-east-1 (MySQL/PostgreSQL rates)
	"aws_db_instance": {
		"db.t3.micro":   12.41,
		"db.t3.small":   24.82,
		"db.t3.medium":  49.64,
		"db.t3.large":   99.28,
		"db.t3.xlarge":  198.56,
		"db.t3.2xlarge": 397.12,
		"db.r6g.large":  175.20,
		"db.r6g.xlarge": 350.40,
		"db.r6g.2xlarge": 700.80,
		"db.r6g.4xlarge": 1401.60,
		"db.m6g.large":  131.40,
		"db.m6g.xlarge": 262.80,
	},

	// RDS Aurora cluster — db cluster instance class pricing
	"aws_rds_cluster": {
		"db.r6g.large":   175.20,
		"db.r6g.xlarge":  350.40,
		"db.r6g.2xlarge": 700.80,
		"db.r5.large":    175.20,
		"db.r5.xlarge":   350.40,
		"db.t3.medium":   49.64,
	},

	// ElastiCache — on-demand, us-east-1 (Redis/Memcached)
	"aws_elasticache_cluster": {
		"cache.t3.micro":   12.24,
		"cache.t3.small":   24.48,
		"cache.t3.medium":  48.96,
		"cache.r6g.large":  131.40,
		"cache.r6g.xlarge": 262.80,
		"cache.m6g.large":  109.50,
	},
	"aws_elasticache_replication_group": {
		"cache.t3.micro":   12.24,
		"cache.t3.small":   24.48,
		"cache.t3.medium":  48.96,
		"cache.r6g.large":  131.40,
		"cache.r6g.xlarge": 262.80,
		"cache.m6g.large":  109.50,
	},

	// ALB / NLB — base monthly rate (LCU usage excluded; actual cost will be higher)
	"aws_lb": {
		"application": 16.20,
		"network":     16.20,
		"gateway":     16.20,
	},
	"aws_alb": {
		"application": 16.20,
		"network":     16.20,
	},
}

// ebsPricePerGB maps EBS volume type to $/GB-month (us-east-1).
var ebsPricePerGB = map[string]float64{
	"gp2": 0.10,
	"gp3": 0.08,
	"io1": 0.125,
	"io2": 0.125,
	"st1": 0.045,
	"sc1": 0.025,
}
