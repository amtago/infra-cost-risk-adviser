package gcp

import (
	"strings"
	"testing"

	"github.com/amt/tf-cost-risk/normalizer"
	"github.com/amt/tf-cost-risk/parser"
	"github.com/amt/tf-cost-risk/pricing"
	"github.com/amt/tf-cost-risk/rules"
)

func nr(resourceType string, changeType parser.ChangeType, raw map[string]interface{}) normalizer.NormalizedResource {
	stateful := map[string]bool{
		"google_sql_database_instance": true,
		"google_compute_disk":          true,
		"google_container_cluster":     true,
	}
	return normalizer.NormalizedResource{
		Address:      resourceType + ".test",
		Provider:     "gcp",
		ResourceType: resourceType,
		ChangeType:   changeType,
		Stateful:     stateful[resourceType],
		Tags:         map[string]string{},
		Raw:          raw,
	}
}

func ctx(resources ...normalizer.NormalizedResource) rules.EvaluateContext {
	return rules.EvaluateContext{Resources: resources}
}

func hasSeverity(findings []rules.Finding, sev rules.Severity) bool {
	for _, f := range findings {
		if f.Severity == sev {
			return true
		}
	}
	return false
}

func hasExplanationContaining(findings []rules.Finding, substr string) bool {
	for _, f := range findings {
		if strings.Contains(f.Explanation, substr) {
			return true
		}
	}
	return false
}

// ── Tier 1 ────────────────────────────────────────────────────────────────────

func TestDestructive_Delete_Warning(t *testing.T) {
	r := &DestructiveChangeRule{}
	findings := r.Evaluate(ctx(nr("google_compute_instance", parser.ChangeDelete, nil)))
	if !hasSeverity(findings, rules.SeverityWarning) {
		t.Error("expected warning for non-stateful delete")
	}
}

func TestDestructive_Delete_StatefulIsCritical(t *testing.T) {
	r := &DestructiveChangeRule{}
	findings := r.Evaluate(ctx(nr("google_compute_disk", parser.ChangeDelete, nil)))
	if !hasSeverity(findings, rules.SeverityCritical) {
		t.Error("expected critical for stateful delete")
	}
}

func TestDestructive_Replace_StatefulIsCritical(t *testing.T) {
	r := &DestructiveChangeRule{}
	findings := r.Evaluate(ctx(nr("google_sql_database_instance", parser.ChangeReplace, nil)))
	if !hasSeverity(findings, rules.SeverityCritical) {
		t.Error("expected critical for stateful replace")
	}
}

func TestDestructive_Create_NoFinding(t *testing.T) {
	r := &DestructiveChangeRule{}
	findings := r.Evaluate(ctx(nr("google_compute_instance", parser.ChangeCreate, nil)))
	if len(findings) != 0 {
		t.Errorf("expected no findings for create, got %d", len(findings))
	}
}

func TestDestructive_IgnoresAWSResources(t *testing.T) {
	r := &DestructiveChangeRule{}
	awsNR := normalizer.NormalizedResource{
		Address:    "aws_instance.web",
		Provider:   "aws",
		ChangeType: parser.ChangeDelete,
	}
	findings := r.Evaluate(rules.EvaluateContext{Resources: []normalizer.NormalizedResource{awsNR}})
	if len(findings) != 0 {
		t.Error("GCP rule should ignore AWS resources")
	}
}

func TestMissingDeletionProtection_NoProtection(t *testing.T) {
	r := &MissingDeletionProtectionRule{}
	findings := r.Evaluate(ctx(nr("google_sql_database_instance", parser.ChangeCreate, map[string]interface{}{
		"deletion_protection": false,
	})))
	if len(findings) == 0 {
		t.Error("expected finding for missing deletion protection")
	}
}

func TestMissingDeletionProtection_WithProtection(t *testing.T) {
	r := &MissingDeletionProtectionRule{}
	findings := r.Evaluate(ctx(nr("google_sql_database_instance", parser.ChangeCreate, map[string]interface{}{
		"deletion_protection": true,
	})))
	if len(findings) != 0 {
		t.Errorf("expected no findings when deletion_protection=true, got %d", len(findings))
	}
}

func TestMissingDeletionProtection_SkipsDelete(t *testing.T) {
	r := &MissingDeletionProtectionRule{}
	findings := r.Evaluate(ctx(nr("google_sql_database_instance", parser.ChangeDelete, map[string]interface{}{
		"deletion_protection": false,
	})))
	if len(findings) != 0 {
		t.Error("should skip delete operations")
	}
}

// ── Tier 2 ────────────────────────────────────────────────────────────────────

func TestOpenFirewall_SSH_Open(t *testing.T) {
	r := &OpenFirewallRule{}
	raw := map[string]interface{}{
		"direction":     "INGRESS",
		"source_ranges": []interface{}{"0.0.0.0/0"},
		"allow": []interface{}{
			map[string]interface{}{
				"protocol": "tcp",
				"ports":    []interface{}{"22"},
			},
		},
	}
	findings := r.Evaluate(ctx(nr("google_compute_firewall", parser.ChangeCreate, raw)))
	if !hasSeverity(findings, rules.SeverityCritical) {
		t.Error("expected critical for open SSH firewall rule")
	}
	if !hasExplanationContaining(findings, "SSH") {
		t.Error("expected SSH in explanation")
	}
}

func TestOpenFirewall_PortRange_SSH(t *testing.T) {
	r := &OpenFirewallRule{}
	raw := map[string]interface{}{
		"source_ranges": []interface{}{"0.0.0.0/0"},
		"allow": []interface{}{
			map[string]interface{}{
				"protocol": "tcp",
				"ports":    []interface{}{"20-30"},
			},
		},
	}
	findings := r.Evaluate(ctx(nr("google_compute_firewall", parser.ChangeCreate, raw)))
	if !hasExplanationContaining(findings, "SSH") {
		t.Error("expected SSH finding for port range 20-30 covering port 22")
	}
}

func TestOpenFirewall_AllProtocol(t *testing.T) {
	r := &OpenFirewallRule{}
	raw := map[string]interface{}{
		"source_ranges": []interface{}{"0.0.0.0/0"},
		"allow": []interface{}{
			map[string]interface{}{
				"protocol": "all",
			},
		},
	}
	findings := r.Evaluate(ctx(nr("google_compute_firewall", parser.ChangeCreate, raw)))
	if len(findings) == 0 {
		t.Error("expected findings for protocol=all with open source ranges")
	}
}

func TestOpenFirewall_RestrictedCIDR_NoFinding(t *testing.T) {
	r := &OpenFirewallRule{}
	raw := map[string]interface{}{
		"source_ranges": []interface{}{"10.0.0.0/8"},
		"allow": []interface{}{
			map[string]interface{}{
				"protocol": "tcp",
				"ports":    []interface{}{"22"},
			},
		},
	}
	findings := r.Evaluate(ctx(nr("google_compute_firewall", parser.ChangeCreate, raw)))
	if len(findings) != 0 {
		t.Error("no finding expected for private CIDR range")
	}
}

func TestOpenFirewall_Egress_NoFinding(t *testing.T) {
	r := &OpenFirewallRule{}
	raw := map[string]interface{}{
		"direction":     "EGRESS",
		"source_ranges": []interface{}{"0.0.0.0/0"},
		"allow": []interface{}{
			map[string]interface{}{"protocol": "tcp", "ports": []interface{}{"22"}},
		},
	}
	findings := r.Evaluate(ctx(nr("google_compute_firewall", parser.ChangeCreate, raw)))
	if len(findings) != 0 {
		t.Error("no finding expected for egress rule")
	}
}

func TestPublicStorageBucket_NoUniform(t *testing.T) {
	r := &PublicStorageBucketRule{}
	raw := map[string]interface{}{
		"uniform_bucket_level_access": false,
	}
	findings := r.Evaluate(ctx(nr("google_storage_bucket", parser.ChangeCreate, raw)))
	if len(findings) == 0 {
		t.Error("expected finding when uniform_bucket_level_access=false")
	}
}

func TestPublicStorageBucket_WithUniform(t *testing.T) {
	r := &PublicStorageBucketRule{}
	raw := map[string]interface{}{
		"uniform_bucket_level_access": true,
	}
	findings := r.Evaluate(ctx(nr("google_storage_bucket", parser.ChangeCreate, raw)))
	if len(findings) != 0 {
		t.Errorf("expected no finding when uniform_bucket_level_access=true, got %d", len(findings))
	}
}

func TestUnencryptedSQL_NoSSL(t *testing.T) {
	r := &UnencryptedSQLRule{}
	raw := map[string]interface{}{
		"settings": []interface{}{
			map[string]interface{}{
				"ip_configuration": []interface{}{
					map[string]interface{}{"require_ssl": false},
				},
				"backup_configuration": []interface{}{
					map[string]interface{}{"enabled": true},
				},
			},
		},
	}
	findings := r.Evaluate(ctx(nr("google_sql_database_instance", parser.ChangeCreate, raw)))
	if !hasExplanationContaining(findings, "SSL") {
		t.Error("expected SSL finding")
	}
}

func TestUnencryptedSQL_NoBackup(t *testing.T) {
	r := &UnencryptedSQLRule{}
	raw := map[string]interface{}{
		"settings": []interface{}{
			map[string]interface{}{
				"ip_configuration": []interface{}{
					map[string]interface{}{"require_ssl": true},
				},
				"backup_configuration": []interface{}{
					map[string]interface{}{"enabled": false},
				},
			},
		},
	}
	findings := r.Evaluate(ctx(nr("google_sql_database_instance", parser.ChangeCreate, raw)))
	if !hasExplanationContaining(findings, "backup") {
		t.Error("expected backup finding")
	}
}

func TestUnencryptedSQL_Compliant(t *testing.T) {
	r := &UnencryptedSQLRule{}
	raw := map[string]interface{}{
		"settings": []interface{}{
			map[string]interface{}{
				"ip_configuration": []interface{}{
					map[string]interface{}{"require_ssl": true},
				},
				"backup_configuration": []interface{}{
					map[string]interface{}{"enabled": true},
				},
			},
		},
	}
	findings := r.Evaluate(ctx(nr("google_sql_database_instance", parser.ChangeCreate, raw)))
	if len(findings) != 0 {
		t.Errorf("expected no findings for compliant SQL instance, got %d", len(findings))
	}
}

// ── Tier 3 ────────────────────────────────────────────────────────────────────

func TestOversized_FlagsExpensiveResource(t *testing.T) {
	r := &OversizedResourceRule{OversizeMultiple: 5}
	resources := []normalizer.NormalizedResource{
		{Address: "google_compute_instance.a", Provider: "gcp", ChangeType: parser.ChangeCreate},
		{Address: "google_compute_instance.b", Provider: "gcp", ChangeType: parser.ChangeCreate},
		{Address: "google_compute_instance.big", Provider: "gcp", ChangeType: parser.ChangeCreate},
	}
	estimates := []pricing.Estimate{
		{ResourceAddress: "google_compute_instance.a", MonthlyCostUSD: 10},
		{ResourceAddress: "google_compute_instance.b", MonthlyCostUSD: 12},
		{ResourceAddress: "google_compute_instance.big", MonthlyCostUSD: 1000},
	}
	findings := r.Evaluate(rules.EvaluateContext{Resources: resources, Estimates: estimates})
	if !hasExplanationContaining(findings, "google_compute_instance.big") {
		t.Error("expected oversized finding for big instance")
	}
}

func TestOversized_IgnoresAWSResources(t *testing.T) {
	r := &OversizedResourceRule{OversizeMultiple: 5}
	resources := []normalizer.NormalizedResource{
		{Address: "aws_instance.small", Provider: "aws", ChangeType: parser.ChangeCreate},
		{Address: "aws_instance.big", Provider: "aws", ChangeType: parser.ChangeCreate},
	}
	estimates := []pricing.Estimate{
		{ResourceAddress: "aws_instance.small", MonthlyCostUSD: 10},
		{ResourceAddress: "aws_instance.big", MonthlyCostUSD: 1000},
	}
	findings := r.Evaluate(rules.EvaluateContext{Resources: resources, Estimates: estimates})
	if len(findings) != 0 {
		t.Error("GCP oversized rule should ignore AWS resources")
	}
}

func TestMissingLabels_Defaults(t *testing.T) {
	r := &MissingLabelsRule{RequiredLabels: []string{"costcenter", "squad"}}
	resource := nr("google_compute_instance", parser.ChangeCreate, map[string]interface{}{
		"labels": map[string]interface{}{},
	})
	findings := r.Evaluate(rules.EvaluateContext{Resources: []normalizer.NormalizedResource{resource}})
	if !hasExplanationContaining(findings, "costcenter") {
		t.Error("expected missing costcenter label finding")
	}
}

func TestMissingLabels_Override(t *testing.T) {
	r := &MissingLabelsRule{RequiredLabels: []string{"costcenter", "squad"}}
	resource := nr("google_compute_instance", parser.ChangeCreate, map[string]interface{}{
		"labels": map[string]interface{}{},
	})
	resource.Tags = map[string]string{}
	findings := r.Evaluate(rules.EvaluateContext{
		Resources:    []normalizer.NormalizedResource{resource},
		RequiredTags: []string{"owner"},
	})
	if !hasExplanationContaining(findings, "owner") {
		t.Error("expected ctx.RequiredTags to override rule defaults")
	}
	if hasExplanationContaining(findings, "costcenter") || hasExplanationContaining(findings, "squad") {
		t.Error("overridden default labels should not appear")
	}
}

func TestMissingLabels_NoLabelsAttr_Skipped(t *testing.T) {
	r := &MissingLabelsRule{RequiredLabels: []string{"env"}}
	// google_compute_firewall doesn't have a labels attribute
	resource := nr("google_compute_firewall", parser.ChangeCreate, map[string]interface{}{})
	findings := r.Evaluate(rules.EvaluateContext{Resources: []normalizer.NormalizedResource{resource}})
	if len(findings) != 0 {
		t.Error("resources without labels attr should be skipped")
	}
}

func TestUnboundedGKEAutoscaling_NoMax(t *testing.T) {
	r := &UnboundedGKEAutoscalingRule{}
	raw := map[string]interface{}{
		"autoscaling": []interface{}{
			map[string]interface{}{"min_node_count": float64(1), "max_node_count": float64(0)},
		},
	}
	findings := r.Evaluate(ctx(nr("google_container_node_pool", parser.ChangeCreate, raw)))
	if len(findings) == 0 {
		t.Error("expected finding for max_node_count=0")
	}
}

func TestUnboundedGKEAutoscaling_WithMax(t *testing.T) {
	r := &UnboundedGKEAutoscalingRule{}
	raw := map[string]interface{}{
		"autoscaling": []interface{}{
			map[string]interface{}{"min_node_count": float64(1), "max_node_count": float64(10)},
		},
	}
	findings := r.Evaluate(ctx(nr("google_container_node_pool", parser.ChangeCreate, raw)))
	if len(findings) != 0 {
		t.Errorf("expected no finding when max_node_count=10, got %d", len(findings))
	}
}

func TestUnboundedGKEAutoscaling_NoAutoscalingBlock_NoFinding(t *testing.T) {
	r := &UnboundedGKEAutoscalingRule{}
	findings := r.Evaluate(ctx(nr("google_container_node_pool", parser.ChangeCreate, map[string]interface{}{})))
	if len(findings) != 0 {
		t.Error("no finding expected when autoscaling block is absent")
	}
}

// ── AllRules smoke test ───────────────────────────────────────────────────────

func TestAllRules_ReturnEightRules(t *testing.T) {
	if len(AllRules()) != 8 {
		t.Errorf("expected 8 rules, got %d", len(AllRules()))
	}
}
