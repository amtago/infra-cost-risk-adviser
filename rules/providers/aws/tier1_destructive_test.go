package aws

import (
	"testing"

	"github.com/amt/tf-cost-risk/normalizer"
	"github.com/amt/tf-cost-risk/parser"
	"github.com/amt/tf-cost-risk/rules"
)

func ctx(resources ...normalizer.NormalizedResource) rules.EvaluateContext {
	return rules.EvaluateContext{Resources: resources}
}

func nrBasic(resourceType, address string, changeType parser.ChangeType, stateful bool, raw map[string]interface{}) normalizer.NormalizedResource {
	return normalizer.NormalizedResource{
		Address:      address,
		ResourceType: resourceType,
		ChangeType:   changeType,
		Stateful:     stateful,
		Raw:          raw,
	}
}

// -- DestructiveChangeRule --

func TestDestructive_Delete_NonStateful_Warning(t *testing.T) {
	r := &DestructiveChangeRule{}
	findings := r.Evaluate(ctx(nrBasic("aws_instance", "aws_instance.web", parser.ChangeDelete, false, nil)))
	if len(findings) != 1 {
		t.Fatalf("expected 1 finding, got %d", len(findings))
	}
	if findings[0].Severity != rules.SeverityWarning {
		t.Errorf("non-stateful delete should be warning, got %s", findings[0].Severity)
	}
	if findings[0].Category != rules.CategoryDestructive {
		t.Errorf("expected destructive category")
	}
}

func TestDestructive_Delete_Stateful_Critical(t *testing.T) {
	r := &DestructiveChangeRule{}
	findings := r.Evaluate(ctx(nrBasic("aws_db_instance", "aws_db_instance.prod", parser.ChangeDelete, true, nil)))
	if len(findings) != 1 {
		t.Fatalf("expected 1 finding, got %d", len(findings))
	}
	if findings[0].Severity != rules.SeverityCritical {
		t.Errorf("stateful delete should be critical, got %s", findings[0].Severity)
	}
}

func TestDestructive_Replace_Stateful_Critical(t *testing.T) {
	r := &DestructiveChangeRule{}
	findings := r.Evaluate(ctx(nrBasic("aws_rds_cluster", "aws_rds_cluster.main", parser.ChangeReplace, true, nil)))
	if len(findings) != 1 {
		t.Fatalf("expected 1 finding, got %d", len(findings))
	}
	if findings[0].Severity != rules.SeverityCritical {
		t.Errorf("stateful replace should be critical, got %s", findings[0].Severity)
	}
}

func TestDestructive_Replace_NonStateful_Warning(t *testing.T) {
	r := &DestructiveChangeRule{}
	findings := r.Evaluate(ctx(nrBasic("aws_instance", "aws_instance.web", parser.ChangeReplace, false, nil)))
	if findings[0].Severity != rules.SeverityWarning {
		t.Errorf("non-stateful replace should be warning")
	}
}

func TestDestructive_Create_NoFinding(t *testing.T) {
	r := &DestructiveChangeRule{}
	findings := r.Evaluate(ctx(nrBasic("aws_instance", "aws_instance.web", parser.ChangeCreate, false, nil)))
	if len(findings) != 0 {
		t.Errorf("create should produce no destructive findings, got %d", len(findings))
	}
}

func TestDestructive_Update_NoFinding(t *testing.T) {
	r := &DestructiveChangeRule{}
	findings := r.Evaluate(ctx(nrBasic("aws_instance", "aws_instance.web", parser.ChangeUpdate, false, nil)))
	if len(findings) != 0 {
		t.Errorf("update should produce no destructive findings, got %d", len(findings))
	}
}

func TestDestructive_MultipleResources(t *testing.T) {
	r := &DestructiveChangeRule{}
	findings := r.Evaluate(ctx(
		nrBasic("aws_instance", "aws_instance.a", parser.ChangeDelete, false, nil),
		nrBasic("aws_db_instance", "aws_db_instance.b", parser.ChangeDelete, true, nil),
		nrBasic("aws_instance", "aws_instance.c", parser.ChangeCreate, false, nil),
	))
	if len(findings) != 2 {
		t.Errorf("expected 2 findings (2 deletes), got %d", len(findings))
	}
}

// -- MissingDeletionProtectionRule --

func TestDeletionProtection_RDS_Missing_Warning(t *testing.T) {
	r := &MissingDeletionProtectionRule{}
	findings := r.Evaluate(ctx(nrBasic("aws_db_instance", "aws_db_instance.main", parser.ChangeCreate, true,
		map[string]interface{}{"deletion_protection": false})))
	if len(findings) != 1 {
		t.Fatalf("expected 1 finding, got %d", len(findings))
	}
	if findings[0].Severity != rules.SeverityWarning {
		t.Errorf("expected warning, got %s", findings[0].Severity)
	}
}

func TestDeletionProtection_RDS_Present_NoFinding(t *testing.T) {
	r := &MissingDeletionProtectionRule{}
	findings := r.Evaluate(ctx(nrBasic("aws_db_instance", "aws_db_instance.main", parser.ChangeCreate, true,
		map[string]interface{}{"deletion_protection": true})))
	if len(findings) != 0 {
		t.Errorf("deletion_protection=true should produce no finding, got %d", len(findings))
	}
}

func TestDeletionProtection_DynamoDB_Missing(t *testing.T) {
	r := &MissingDeletionProtectionRule{}
	findings := r.Evaluate(ctx(nrBasic("aws_dynamodb_table", "aws_dynamodb_table.events", parser.ChangeCreate, true,
		map[string]interface{}{"deletion_protection_enabled": false})))
	if len(findings) != 1 {
		t.Fatalf("expected 1 finding for DynamoDB, got %d", len(findings))
	}
}

func TestDeletionProtection_Delete_Skipped(t *testing.T) {
	// Deletes are already flagged by DestructiveChangeRule — don't double-flag.
	r := &MissingDeletionProtectionRule{}
	findings := r.Evaluate(ctx(nrBasic("aws_db_instance", "aws_db_instance.main", parser.ChangeDelete, true,
		map[string]interface{}{"deletion_protection": false})))
	if len(findings) != 0 {
		t.Errorf("deletes should be skipped by deletion protection rule, got %d", len(findings))
	}
}

func TestDeletionProtection_EC2_NoAttr_NoFinding(t *testing.T) {
	// EC2 doesn't support deletion_protection — no false positive.
	r := &MissingDeletionProtectionRule{}
	findings := r.Evaluate(ctx(nrBasic("aws_instance", "aws_instance.web", parser.ChangeCreate, false, nil)))
	if len(findings) != 0 {
		t.Errorf("EC2 should not trigger deletion protection rule, got %d", len(findings))
	}
}
