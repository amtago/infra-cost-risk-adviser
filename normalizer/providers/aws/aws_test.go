package aws

import (
	"testing"

	"github.com/amt/tf-cost-risk/normalizer"
	"github.com/amt/tf-cost-risk/parser"
)

var n = &Normalizer{}

// -- helpers --

func rc(resourceType, address string, changeType parser.ChangeType, after map[string]interface{}) parser.ResourceChange {
	return parser.ResourceChange{
		Address:      address,
		ProviderName: "registry.terraform.io/hashicorp/aws",
		Type:         resourceType,
		Name:         address,
		ChangeType:   changeType,
		After:        after,
	}
}

func rcWithBefore(resourceType, address string, changeType parser.ChangeType, before, after map[string]interface{}) parser.ResourceChange {
	r := rc(resourceType, address, changeType, after)
	r.Before = before
	return r
}

// -- EC2 --

func TestNormalize_EC2_Create(t *testing.T) {
	nr, err := n.Normalize(rc("aws_instance", "aws_instance.web", parser.ChangeCreate, map[string]interface{}{
		"instance_type": "t3.medium",
		"tags":          map[string]interface{}{"Env": "prod", "Team": "platform"},
	}), "us-east-1")
	if err != nil {
		t.Fatal(err)
	}
	if nr.Category != normalizer.CategoryCompute {
		t.Errorf("expected compute, got %s", nr.Category)
	}
	if nr.Size != "t3.medium" {
		t.Errorf("expected t3.medium, got %s", nr.Size)
	}
	if nr.Stateful {
		t.Error("EC2 should not be stateful")
	}
	if nr.Region != "us-east-1" {
		t.Errorf("expected us-east-1, got %s", nr.Region)
	}
	if nr.Tags["Env"] != "prod" || nr.Tags["Team"] != "platform" {
		t.Errorf("tags not extracted: %v", nr.Tags)
	}
	if nr.Provider != "aws" {
		t.Errorf("expected provider aws, got %s", nr.Provider)
	}
	if nr.ResourceType != "aws_instance" {
		t.Errorf("expected ResourceType aws_instance, got %s", nr.ResourceType)
	}
}

// -- RDS --

func TestNormalize_RDS_Stateful(t *testing.T) {
	nr, err := n.Normalize(rc("aws_db_instance", "aws_db_instance.main", parser.ChangeCreate, map[string]interface{}{
		"instance_class":     "db.t3.micro",
		"deletion_protection": true,
	}), "us-east-1")
	if err != nil {
		t.Fatal(err)
	}
	if nr.Category != normalizer.CategoryDatabase {
		t.Errorf("expected database, got %s", nr.Category)
	}
	if nr.Size != "db.t3.micro" {
		t.Errorf("expected db.t3.micro, got %s", nr.Size)
	}
	if !nr.Stateful {
		t.Error("RDS should be stateful")
	}
}

func TestNormalize_RDSCluster_Stateful(t *testing.T) {
	nr, err := n.Normalize(rc("aws_rds_cluster", "aws_rds_cluster.aurora", parser.ChangeCreate, map[string]interface{}{
		"db_cluster_instance_class": "db.r6g.large",
	}), "us-west-2")
	if err != nil {
		t.Fatal(err)
	}
	if !nr.Stateful {
		t.Error("RDS cluster should be stateful")
	}
	if nr.Size != "db.r6g.large" {
		t.Errorf("expected db.r6g.large, got %s", nr.Size)
	}
}

// -- EBS --

func TestNormalize_EBS_Stateful(t *testing.T) {
	nr, err := n.Normalize(rc("aws_ebs_volume", "aws_ebs_volume.data", parser.ChangeCreate, map[string]interface{}{
		"type": "gp3",
		"size": 100,
	}), "us-east-1")
	if err != nil {
		t.Fatal(err)
	}
	if nr.Category != normalizer.CategoryStorage {
		t.Errorf("expected storage, got %s", nr.Category)
	}
	if nr.Size != "gp3" {
		t.Errorf("expected gp3, got %s", nr.Size)
	}
	if !nr.Stateful {
		t.Error("EBS volume should be stateful")
	}
}

// -- S3 --

func TestNormalize_S3(t *testing.T) {
	nr, err := n.Normalize(rc("aws_s3_bucket", "aws_s3_bucket.assets", parser.ChangeCreate, map[string]interface{}{
		"bucket": "my-assets",
		"tags":   map[string]interface{}{"CostCenter": "eng"},
	}), "us-east-1")
	if err != nil {
		t.Fatal(err)
	}
	if nr.Category != normalizer.CategoryStorage {
		t.Errorf("expected storage, got %s", nr.Category)
	}
	if nr.Stateful {
		t.Error("S3 bucket should not be marked stateful (managed separately)")
	}
	if nr.Size != "" {
		t.Errorf("S3 size should be empty, got %s", nr.Size)
	}
	if nr.Tags["CostCenter"] != "eng" {
		t.Errorf("tag not extracted: %v", nr.Tags)
	}
}

// -- ALB/NLB --

func TestNormalize_ALB(t *testing.T) {
	nr, err := n.Normalize(rc("aws_lb", "aws_lb.frontend", parser.ChangeCreate, map[string]interface{}{
		"load_balancer_type": "application",
	}), "us-east-1")
	if err != nil {
		t.Fatal(err)
	}
	if nr.Category != normalizer.CategoryNetwork {
		t.Errorf("expected network, got %s", nr.Category)
	}
	if nr.Size != "application" {
		t.Errorf("expected application, got %s", nr.Size)
	}
	if nr.Stateful {
		t.Error("ALB should not be stateful")
	}
}

func TestNormalize_NLB_AlbAlias(t *testing.T) {
	nr, err := n.Normalize(rc("aws_alb", "aws_alb.internal", parser.ChangeCreate, map[string]interface{}{
		"load_balancer_type": "network",
	}), "us-east-1")
	if err != nil {
		t.Fatal(err)
	}
	if nr.Category != normalizer.CategoryNetwork {
		t.Errorf("expected network, got %s", nr.Category)
	}
	if nr.Size != "network" {
		t.Errorf("expected network, got %s", nr.Size)
	}
}

// -- Lambda --

func TestNormalize_Lambda(t *testing.T) {
	nr, err := n.Normalize(rc("aws_lambda_function", "aws_lambda_function.handler", parser.ChangeCreate, map[string]interface{}{
		"memory_size": float64(512),
		"runtime":     "go1.x",
	}), "us-east-1")
	if err != nil {
		t.Fatal(err)
	}
	if nr.Category != normalizer.CategoryFunction {
		t.Errorf("expected function, got %s", nr.Category)
	}
	if nr.Size != "512MB" {
		t.Errorf("expected 512MB, got %s", nr.Size)
	}
	if nr.Stateful {
		t.Error("Lambda should not be stateful")
	}
}

// -- DynamoDB --

func TestNormalize_DynamoDB_Stateful(t *testing.T) {
	nr, err := n.Normalize(rc("aws_dynamodb_table", "aws_dynamodb_table.events", parser.ChangeCreate, map[string]interface{}{
		"billing_mode": "PAY_PER_REQUEST",
	}), "us-east-1")
	if err != nil {
		t.Fatal(err)
	}
	if nr.Category != normalizer.CategoryDatabase {
		t.Errorf("expected database, got %s", nr.Category)
	}
	if !nr.Stateful {
		t.Error("DynamoDB should be stateful")
	}
	if nr.Size != "PAY_PER_REQUEST" {
		t.Errorf("expected PAY_PER_REQUEST, got %s", nr.Size)
	}
}

// -- ElastiCache --

func TestNormalize_ElastiCache_Stateful(t *testing.T) {
	nr, err := n.Normalize(rc("aws_elasticache_cluster", "aws_elasticache_cluster.cache", parser.ChangeCreate, map[string]interface{}{
		"node_type": "cache.t3.micro",
	}), "us-east-1")
	if err != nil {
		t.Fatal(err)
	}
	if !nr.Stateful {
		t.Error("ElastiCache cluster should be stateful")
	}
	if nr.Size != "cache.t3.micro" {
		t.Errorf("expected cache.t3.micro, got %s", nr.Size)
	}
}

// -- Delete uses Before attrs --

func TestNormalize_Delete_UsesBefore(t *testing.T) {
	nr, err := n.Normalize(rcWithBefore(
		"aws_instance", "aws_instance.old", parser.ChangeDelete,
		map[string]interface{}{"instance_type": "t3.small"},
		nil,
	), "us-east-1")
	if err != nil {
		t.Fatal(err)
	}
	if nr.Size != "t3.small" {
		t.Errorf("delete should use before attrs for size, got %s", nr.Size)
	}
	if nr.ChangeType != parser.ChangeDelete {
		t.Errorf("change type not propagated, got %s", nr.ChangeType)
	}
}

// -- Region fallback --

func TestNormalize_Region_Fallback(t *testing.T) {
	nr, err := n.Normalize(rc("aws_instance", "aws_instance.web", parser.ChangeCreate, map[string]interface{}{
		"instance_type": "t3.micro",
	}), "eu-west-1")
	if err != nil {
		t.Fatal(err)
	}
	if nr.Region != "eu-west-1" {
		t.Errorf("expected fallback region eu-west-1, got %s", nr.Region)
	}
}

// -- Unknown resource type --

func TestNormalize_UnknownType(t *testing.T) {
	nr, err := n.Normalize(rc("aws_cloudfront_distribution", "aws_cloudfront_distribution.cdn", parser.ChangeCreate, map[string]interface{}{}), "us-east-1")
	if err != nil {
		t.Fatal(err)
	}
	if nr.Category != normalizer.CategoryUnknown {
		t.Errorf("expected unknown category, got %s", nr.Category)
	}
}

// -- Tags edge cases --

func TestNormalize_NoTags(t *testing.T) {
	nr, err := n.Normalize(rc("aws_instance", "aws_instance.web", parser.ChangeCreate, map[string]interface{}{
		"instance_type": "t3.micro",
	}), "us-east-1")
	if err != nil {
		t.Fatal(err)
	}
	if nr.Tags == nil {
		t.Error("Tags should never be nil")
	}
	if len(nr.Tags) != 0 {
		t.Errorf("expected empty tags, got %v", nr.Tags)
	}
}

func TestNormalize_EmptyTags(t *testing.T) {
	nr, err := n.Normalize(rc("aws_instance", "aws_instance.web", parser.ChangeCreate, map[string]interface{}{
		"instance_type": "t3.micro",
		"tags":          map[string]interface{}{},
	}), "us-east-1")
	if err != nil {
		t.Fatal(err)
	}
	if len(nr.Tags) != 0 {
		t.Errorf("expected empty tags, got %v", nr.Tags)
	}
}
