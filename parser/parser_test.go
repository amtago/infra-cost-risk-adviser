package parser

import (
	"path/filepath"
	"runtime"
	"testing"
)

// -- unit tests for each change type --

func TestParse_Create(t *testing.T) {
	changes := mustParse(t, planFixture(`[{
		"address":"aws_instance.web",
		"provider_name":"registry.terraform.io/hashicorp/aws",
		"type":"aws_instance","name":"web",
		"change":{"actions":["create"],"before":null,"after":{"instance_type":"t3.medium"}}}]`))

	if len(changes) != 1 {
		t.Fatalf("expected 1 change, got %d", len(changes))
	}
	c := changes[0]
	if c.ChangeType != ChangeCreate {
		t.Errorf("expected create, got %s", c.ChangeType)
	}
	if c.Address != "aws_instance.web" {
		t.Errorf("unexpected address %s", c.Address)
	}
	if c.Type != "aws_instance" {
		t.Errorf("unexpected type %s", c.Type)
	}
	if c.Before != nil {
		t.Errorf("Before should be nil for create")
	}
	if c.After["instance_type"] != "t3.medium" {
		t.Errorf("After.instance_type not propagated")
	}
}

func TestParse_Update(t *testing.T) {
	changes := mustParse(t, planFixture(`[{
		"address":"aws_instance.web",
		"provider_name":"registry.terraform.io/hashicorp/aws",
		"type":"aws_instance","name":"web",
		"change":{"actions":["update"],
			"before":{"instance_type":"t3.small"},
			"after":{"instance_type":"t3.medium"}}}]`))

	if len(changes) != 1 {
		t.Fatalf("expected 1 change, got %d", len(changes))
	}
	c := changes[0]
	if c.ChangeType != ChangeUpdate {
		t.Errorf("expected update, got %s", c.ChangeType)
	}
	if c.Before["instance_type"] != "t3.small" {
		t.Errorf("Before.instance_type not propagated")
	}
	if c.After["instance_type"] != "t3.medium" {
		t.Errorf("After.instance_type not propagated")
	}
}

func TestParse_Delete(t *testing.T) {
	changes := mustParse(t, planFixture(`[{
		"address":"aws_instance.old",
		"provider_name":"registry.terraform.io/hashicorp/aws",
		"type":"aws_instance","name":"old",
		"change":{"actions":["delete"],"before":{"instance_type":"t3.small"},"after":null}}]`))

	if len(changes) != 1 {
		t.Fatalf("expected 1 change, got %d", len(changes))
	}
	c := changes[0]
	if c.ChangeType != ChangeDelete {
		t.Errorf("expected delete, got %s", c.ChangeType)
	}
	if c.After != nil {
		t.Errorf("After should be nil for delete")
	}
	if c.Before["instance_type"] != "t3.small" {
		t.Errorf("Before.instance_type not propagated")
	}
}

func TestParse_Replace_DeleteCreate(t *testing.T) {
	changes := mustParse(t, planFixture(`[{
		"address":"aws_db_instance.main",
		"provider_name":"registry.terraform.io/hashicorp/aws",
		"type":"aws_db_instance","name":"main",
		"change":{"actions":["delete","create"],
			"before":{"instance_class":"db.t3.micro"},
			"after":{"instance_class":"db.t3.small"}}}]`))

	if changes[0].ChangeType != ChangeReplace {
		t.Errorf("expected replace for delete+create, got %s", changes[0].ChangeType)
	}
}

func TestParse_Replace_CreateDelete(t *testing.T) {
	changes := mustParse(t, planFixture(`[{
		"address":"aws_db_instance.main",
		"provider_name":"registry.terraform.io/hashicorp/aws",
		"type":"aws_db_instance","name":"main",
		"change":{"actions":["create","delete"],
			"before":{"instance_class":"db.t3.micro"},
			"after":{"instance_class":"db.t3.small"}}}]`))

	if changes[0].ChangeType != ChangeReplace {
		t.Errorf("expected replace for create+delete, got %s", changes[0].ChangeType)
	}
}

func TestParse_NoOp_Excluded(t *testing.T) {
	changes := mustParse(t, planFixture(`[{
		"address":"aws_instance.stable",
		"provider_name":"registry.terraform.io/hashicorp/aws",
		"type":"aws_instance","name":"stable",
		"change":{"actions":["no-op"],
			"before":{"instance_type":"t3.small"},
			"after":{"instance_type":"t3.small"}}}]`))

	if len(changes) != 0 {
		t.Errorf("expected no changes for no-op, got %d", len(changes))
	}
}

func TestParse_MixedChanges_NoOpFiltered(t *testing.T) {
	data := planFixture(`[
		{"address":"aws_instance.stable","provider_name":"registry.terraform.io/hashicorp/aws",
		 "type":"aws_instance","name":"stable",
		 "change":{"actions":["no-op"],"before":{"instance_type":"t3.small"},"after":{"instance_type":"t3.small"}}},
		{"address":"aws_instance.new","provider_name":"registry.terraform.io/hashicorp/aws",
		 "type":"aws_instance","name":"new",
		 "change":{"actions":["create"],"before":null,"after":{"instance_type":"t3.large"}}}
	]`)
	changes := mustParse(t, data)
	if len(changes) != 1 {
		t.Fatalf("expected 1 change (no-op filtered), got %d", len(changes))
	}
	if changes[0].Address != "aws_instance.new" {
		t.Errorf("wrong resource returned: %s", changes[0].Address)
	}
}

func TestParse_EmptyPlan(t *testing.T) {
	changes := mustParse(t, planFixture(`[]`))
	if len(changes) != 0 {
		t.Errorf("expected 0 changes for empty plan, got %d", len(changes))
	}
}

func TestParse_InvalidJSON(t *testing.T) {
	_, err := Parse([]byte(`not json`))
	if err == nil {
		t.Error("expected error for invalid JSON, got nil")
	}
}

// -- fixture file tests --

func TestParseFile_CleanPlan(t *testing.T) {
	changes, err := ParseFile(fixturePath("clean_plan.json"))
	if err != nil {
		t.Fatal(err)
	}
	// clean plan has only no-op changes — all should be filtered
	if len(changes) != 0 {
		t.Errorf("clean plan: expected 0 changes, got %d", len(changes))
	}
}

func TestParseFile_CostIncreasePlan(t *testing.T) {
	changes, err := ParseFile(fixturePath("cost_increase_plan.json"))
	if err != nil {
		t.Fatal(err)
	}
	if len(changes) != 5 {
		t.Fatalf("cost_increase plan: expected 5 changes, got %d", len(changes))
	}
	for _, c := range changes {
		if c.ChangeType != ChangeCreate {
			t.Errorf("expected all creates in cost_increase plan, got %s for %s", c.ChangeType, c.Address)
		}
	}
}

func TestParseFile_DestructivePlan(t *testing.T) {
	changes, err := ParseFile(fixturePath("destructive_plan.json"))
	if err != nil {
		t.Fatal(err)
	}
	if len(changes) != 3 {
		t.Fatalf("destructive plan: expected 3 changes, got %d", len(changes))
	}
	types := map[ChangeType]int{}
	for _, c := range changes {
		types[c.ChangeType]++
	}
	if types[ChangeReplace] != 1 {
		t.Errorf("expected 1 replace, got %d", types[ChangeReplace])
	}
	if types[ChangeDelete] != 1 {
		t.Errorf("expected 1 delete, got %d", types[ChangeDelete])
	}
	if types[ChangeUpdate] != 1 {
		t.Errorf("expected 1 update, got %d", types[ChangeUpdate])
	}
}

func TestParseFile_SecurityMisconfigPlan(t *testing.T) {
	changes, err := ParseFile(fixturePath("security_misconfig_plan.json"))
	if err != nil {
		t.Fatal(err)
	}
	if len(changes) != 5 {
		t.Fatalf("security_misconfig plan: expected 5 changes, got %d", len(changes))
	}
	for _, c := range changes {
		if c.ChangeType != ChangeCreate {
			t.Errorf("expected all creates, got %s for %s", c.ChangeType, c.Address)
		}
	}
}

func TestParseFile_NotFound(t *testing.T) {
	_, err := ParseFile("/does/not/exist.json")
	if err == nil {
		t.Error("expected error for missing file, got nil")
	}
}

// -- helpers --

func mustParse(t *testing.T, data []byte) []ResourceChange {
	t.Helper()
	changes, err := Parse(data)
	if err != nil {
		t.Fatal(err)
	}
	return changes
}

func fixturePath(name string) string {
	_, file, _, _ := runtime.Caller(0)
	return filepath.Join(filepath.Dir(file), "..", "fixtures", name)
}

func planFixture(resourceChangesJSON string) []byte {
	return []byte(`{"resource_changes":` + resourceChangesJSON + `}`)
}
