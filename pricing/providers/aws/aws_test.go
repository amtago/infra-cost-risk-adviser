package aws

import (
	"testing"

	"github.com/amt/tf-cost-risk/normalizer"
	"github.com/amt/tf-cost-risk/parser"
)

var p = &Pricer{}

func nr(resourceType, address, size string, raw map[string]interface{}) normalizer.NormalizedResource {
	return normalizer.NormalizedResource{
		Address:      address,
		ResourceType: resourceType,
		Size:         size,
		ChangeType:   parser.ChangeCreate,
		Raw:          raw,
	}
}

// -- EC2 --

func TestEstimate_EC2_KnownSize(t *testing.T) {
	e := p.Estimate(nr("aws_instance", "aws_instance.web", "t3.medium", nil))
	if e.Unknown {
		t.Error("t3.medium should have a known price")
	}
	if e.MonthlyCostUSD != 30.37 {
		t.Errorf("expected 30.37, got %.2f", e.MonthlyCostUSD)
	}
	if e.ResourceAddress != "aws_instance.web" {
		t.Errorf("address not propagated: %s", e.ResourceAddress)
	}
}

func TestEstimate_EC2_UnknownSize(t *testing.T) {
	e := p.Estimate(nr("aws_instance", "aws_instance.x", "p4d.24xlarge", nil))
	if !e.Unknown {
		t.Error("p4d.24xlarge should be unknown (not in price table)")
	}
	if e.MonthlyCostUSD != 0 {
		t.Errorf("unknown estimate should have 0 cost, got %.2f", e.MonthlyCostUSD)
	}
}

func TestEstimate_EC2_LargeInstance(t *testing.T) {
	e := p.Estimate(nr("aws_instance", "aws_instance.heavy", "m5.4xlarge", nil))
	if e.Unknown {
		t.Error("m5.4xlarge should have a known price")
	}
	if e.MonthlyCostUSD != 560.64 {
		t.Errorf("expected 560.64, got %.2f", e.MonthlyCostUSD)
	}
}

// -- RDS --

func TestEstimate_RDS_KnownClass(t *testing.T) {
	e := p.Estimate(nr("aws_db_instance", "aws_db_instance.main", "db.t3.micro", nil))
	if e.Unknown {
		t.Error("db.t3.micro should have a known price")
	}
	if e.MonthlyCostUSD != 12.41 {
		t.Errorf("expected 12.41, got %.2f", e.MonthlyCostUSD)
	}
}

func TestEstimate_RDS_UnknownClass(t *testing.T) {
	e := p.Estimate(nr("aws_db_instance", "aws_db_instance.x", "db.x2g.16xlarge", nil))
	if !e.Unknown {
		t.Error("db.x2g.16xlarge should be unknown")
	}
}

func TestEstimate_RDSCluster(t *testing.T) {
	e := p.Estimate(nr("aws_rds_cluster", "aws_rds_cluster.aurora", "db.r6g.large", nil))
	if e.Unknown {
		t.Error("db.r6g.large aurora should have a known price")
	}
	if e.MonthlyCostUSD != 175.20 {
		t.Errorf("expected 175.20, got %.2f", e.MonthlyCostUSD)
	}
}

// -- EBS --

func TestEstimate_EBS_GP3(t *testing.T) {
	e := p.Estimate(nr("aws_ebs_volume", "aws_ebs_volume.data", "gp3", map[string]interface{}{
		"size": float64(100),
	}))
	if e.Unknown {
		t.Error("gp3 100GB should have a known price")
	}
	want := 0.08 * 100
	if e.MonthlyCostUSD != want {
		t.Errorf("expected %.2f, got %.2f", want, e.MonthlyCostUSD)
	}
}

func TestEstimate_EBS_IO2(t *testing.T) {
	e := p.Estimate(nr("aws_ebs_volume", "aws_ebs_volume.perf", "io2", map[string]interface{}{
		"size": float64(500),
	}))
	if e.Unknown {
		t.Error("io2 500GB should have a known price")
	}
	want := 0.125 * 500
	if e.MonthlyCostUSD != want {
		t.Errorf("expected %.2f, got %.2f", want, e.MonthlyCostUSD)
	}
}

func TestEstimate_EBS_MissingSize(t *testing.T) {
	e := p.Estimate(nr("aws_ebs_volume", "aws_ebs_volume.x", "gp3", map[string]interface{}{}))
	if !e.Unknown {
		t.Error("EBS without size in raw attrs should be unknown")
	}
}

func TestEstimate_EBS_UnknownType(t *testing.T) {
	e := p.Estimate(nr("aws_ebs_volume", "aws_ebs_volume.x", "magnetic", map[string]interface{}{
		"size": float64(50),
	}))
	if !e.Unknown {
		t.Error("unknown EBS type should return unknown")
	}
}

// -- ALB / NLB --

func TestEstimate_ALB(t *testing.T) {
	e := p.Estimate(nr("aws_lb", "aws_lb.frontend", "application", nil))
	if e.Unknown {
		t.Error("ALB should have a known base price")
	}
	if e.MonthlyCostUSD != 16.20 {
		t.Errorf("expected 16.20, got %.2f", e.MonthlyCostUSD)
	}
}

func TestEstimate_NLB(t *testing.T) {
	e := p.Estimate(nr("aws_lb", "aws_lb.internal", "network", nil))
	if e.Unknown {
		t.Error("NLB should have a known base price")
	}
}

func TestEstimate_ALB_Alias(t *testing.T) {
	e := p.Estimate(nr("aws_alb", "aws_alb.old", "application", nil))
	if e.Unknown {
		t.Error("aws_alb alias should have a known price")
	}
}

// -- ElastiCache --

func TestEstimate_ElastiCache(t *testing.T) {
	e := p.Estimate(nr("aws_elasticache_cluster", "aws_elasticache_cluster.cache", "cache.t3.micro", nil))
	if e.Unknown {
		t.Error("cache.t3.micro should have a known price")
	}
	if e.MonthlyCostUSD != 12.24 {
		t.Errorf("expected 12.24, got %.2f", e.MonthlyCostUSD)
	}
}

func TestEstimate_ElastiCacheReplicationGroup(t *testing.T) {
	e := p.Estimate(nr("aws_elasticache_replication_group", "aws_elasticache_replication_group.redis", "cache.r6g.large", nil))
	if e.Unknown {
		t.Error("cache.r6g.large replication group should have a known price")
	}
	if e.MonthlyCostUSD != 131.40 {
		t.Errorf("expected 131.40, got %.2f", e.MonthlyCostUSD)
	}
}

// -- Usage-based resources return Unknown --

func TestEstimate_S3_Unknown(t *testing.T) {
	e := p.Estimate(nr("aws_s3_bucket", "aws_s3_bucket.assets", "", nil))
	if !e.Unknown {
		t.Error("S3 is usage-based and should return unknown")
	}
}

func TestEstimate_Lambda_Unknown(t *testing.T) {
	e := p.Estimate(nr("aws_lambda_function", "aws_lambda_function.handler", "512MB", nil))
	if !e.Unknown {
		t.Error("Lambda is usage-based and should return unknown")
	}
}

func TestEstimate_DynamoDB_Unknown(t *testing.T) {
	e := p.Estimate(nr("aws_dynamodb_table", "aws_dynamodb_table.events", "PAY_PER_REQUEST", nil))
	if !e.Unknown {
		t.Error("DynamoDB is usage-based and should return unknown")
	}
}

// -- Completely unknown resource type --

func TestEstimate_UnknownResourceType(t *testing.T) {
	e := p.Estimate(nr("aws_cloudfront_distribution", "aws_cloudfront_distribution.cdn", "", nil))
	if !e.Unknown {
		t.Error("unknown resource type should return unknown")
	}
}
