package azure

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
		"azurerm_mssql_database":             true,
		"azurerm_postgresql_server":          true,
		"azurerm_postgresql_flexible_server": true,
		"azurerm_mysql_flexible_server":      true,
		"azurerm_managed_disk":               true,
		"azurerm_storage_account":            true,
	}
	return normalizer.NormalizedResource{
		Address:      resourceType + ".test",
		Provider:     "azure",
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
	findings := r.Evaluate(ctx(nr("azurerm_linux_virtual_machine", parser.ChangeDelete, nil)))
	if !hasSeverity(findings, rules.SeverityWarning) {
		t.Error("expected warning for non-stateful delete")
	}
}

func TestDestructive_Delete_StatefulIsCritical(t *testing.T) {
	r := &DestructiveChangeRule{}
	findings := r.Evaluate(ctx(nr("azurerm_managed_disk", parser.ChangeDelete, nil)))
	if !hasSeverity(findings, rules.SeverityCritical) {
		t.Error("expected critical for stateful delete")
	}
}

func TestDestructive_Replace_Stateful(t *testing.T) {
	r := &DestructiveChangeRule{}
	findings := r.Evaluate(ctx(nr("azurerm_mssql_database", parser.ChangeReplace, nil)))
	if !hasSeverity(findings, rules.SeverityCritical) {
		t.Error("expected critical for stateful replace")
	}
}

func TestDestructive_Create_NoFinding(t *testing.T) {
	r := &DestructiveChangeRule{}
	findings := r.Evaluate(ctx(nr("azurerm_linux_virtual_machine", parser.ChangeCreate, nil)))
	if len(findings) != 0 {
		t.Errorf("expected no findings for create, got %d", len(findings))
	}
}

func TestDestructive_IgnoresNonAzure(t *testing.T) {
	r := &DestructiveChangeRule{}
	awsNR := normalizer.NormalizedResource{
		Address:    "aws_instance.web",
		Provider:   "aws",
		ChangeType: parser.ChangeDelete,
	}
	findings := r.Evaluate(rules.EvaluateContext{Resources: []normalizer.NormalizedResource{awsNR}})
	if len(findings) != 0 {
		t.Error("Azure rule should ignore non-Azure resources")
	}
}

func TestBackupRetention_TooLow(t *testing.T) {
	r := &MissingBackupRetentionRule{MinRetentionDays: 7}
	findings := r.Evaluate(ctx(nr("azurerm_postgresql_flexible_server", parser.ChangeCreate, map[string]interface{}{
		"backup_retention_days": float64(3),
	})))
	if len(findings) == 0 {
		t.Error("expected finding for backup_retention_days=3")
	}
}

func TestBackupRetention_Sufficient(t *testing.T) {
	r := &MissingBackupRetentionRule{MinRetentionDays: 7}
	findings := r.Evaluate(ctx(nr("azurerm_postgresql_flexible_server", parser.ChangeCreate, map[string]interface{}{
		"backup_retention_days": float64(14),
	})))
	if len(findings) != 0 {
		t.Errorf("expected no finding for backup_retention_days=14, got %d", len(findings))
	}
}

func TestBackupRetention_SkipsDelete(t *testing.T) {
	r := &MissingBackupRetentionRule{MinRetentionDays: 7}
	findings := r.Evaluate(ctx(nr("azurerm_postgresql_flexible_server", parser.ChangeDelete, map[string]interface{}{
		"backup_retention_days": float64(0),
	})))
	if len(findings) != 0 {
		t.Error("should skip delete operations")
	}
}

func TestBackupRetention_SkipsNonDatabases(t *testing.T) {
	r := &MissingBackupRetentionRule{MinRetentionDays: 7}
	findings := r.Evaluate(ctx(nr("azurerm_linux_virtual_machine", parser.ChangeCreate, map[string]interface{}{
		"backup_retention_days": float64(0),
	})))
	if len(findings) != 0 {
		t.Error("backup retention rule should only apply to database servers")
	}
}

// ── Tier 2 ────────────────────────────────────────────────────────────────────

func TestOpenNSG_SSH_Open(t *testing.T) {
	r := &OpenNSGRule{}
	raw := map[string]interface{}{
		"security_rule": []interface{}{
			map[string]interface{}{
				"direction":                   "Inbound",
				"access":                      "Allow",
				"source_address_prefix":       "*",
				"destination_port_range":      "22",
			},
		},
	}
	findings := r.Evaluate(ctx(nr("azurerm_network_security_group", parser.ChangeCreate, raw)))
	if !hasSeverity(findings, rules.SeverityCritical) {
		t.Error("expected critical for open SSH NSG rule")
	}
	if !hasExplanationContaining(findings, "SSH") {
		t.Error("expected SSH in explanation")
	}
}

func TestOpenNSG_RDP_Open(t *testing.T) {
	r := &OpenNSGRule{}
	raw := map[string]interface{}{
		"security_rule": []interface{}{
			map[string]interface{}{
				"direction":              "Inbound",
				"access":                "Allow",
				"source_address_prefix": "Internet",
				"destination_port_range": "3389",
			},
		},
	}
	findings := r.Evaluate(ctx(nr("azurerm_network_security_group", parser.ChangeCreate, raw)))
	if !hasExplanationContaining(findings, "RDP") {
		t.Error("expected RDP in explanation")
	}
}

func TestOpenNSG_WildcardPort(t *testing.T) {
	r := &OpenNSGRule{}
	raw := map[string]interface{}{
		"security_rule": []interface{}{
			map[string]interface{}{
				"direction":              "Inbound",
				"access":                "Allow",
				"source_address_prefix": "0.0.0.0/0",
				"destination_port_range": "*",
			},
		},
	}
	findings := r.Evaluate(ctx(nr("azurerm_network_security_group", parser.ChangeCreate, raw)))
	if len(findings) == 0 {
		t.Error("expected findings for wildcard port range open to internet")
	}
}

func TestOpenNSG_RestrictedSource_NoFinding(t *testing.T) {
	r := &OpenNSGRule{}
	raw := map[string]interface{}{
		"security_rule": []interface{}{
			map[string]interface{}{
				"direction":              "Inbound",
				"access":                "Allow",
				"source_address_prefix": "10.0.0.0/8",
				"destination_port_range": "22",
			},
		},
	}
	findings := r.Evaluate(ctx(nr("azurerm_network_security_group", parser.ChangeCreate, raw)))
	if len(findings) != 0 {
		t.Error("no finding expected for private source range")
	}
}

func TestOpenNSG_OutboundRule_NoFinding(t *testing.T) {
	r := &OpenNSGRule{}
	raw := map[string]interface{}{
		"security_rule": []interface{}{
			map[string]interface{}{
				"direction":              "Outbound",
				"access":                "Allow",
				"source_address_prefix": "*",
				"destination_port_range": "22",
			},
		},
	}
	findings := r.Evaluate(ctx(nr("azurerm_network_security_group", parser.ChangeCreate, raw)))
	if len(findings) != 0 {
		t.Error("no finding expected for outbound rule")
	}
}

func TestPublicStorage_PublicBlobAccess(t *testing.T) {
	r := &PublicStorageAccountRule{}
	findings := r.Evaluate(ctx(nr("azurerm_storage_account", parser.ChangeCreate, map[string]interface{}{
		"allow_blob_public_access":  true,
		"enable_https_traffic_only": true,
	})))
	if !hasSeverity(findings, rules.SeverityCritical) {
		t.Error("expected critical for public blob access")
	}
}

func TestPublicStorage_NoHTTPS(t *testing.T) {
	r := &PublicStorageAccountRule{}
	findings := r.Evaluate(ctx(nr("azurerm_storage_account", parser.ChangeCreate, map[string]interface{}{
		"allow_blob_public_access":  false,
		"enable_https_traffic_only": false,
	})))
	if !hasSeverity(findings, rules.SeverityWarning) {
		t.Error("expected warning for missing HTTPS enforcement")
	}
}

func TestPublicStorage_Compliant(t *testing.T) {
	r := &PublicStorageAccountRule{}
	findings := r.Evaluate(ctx(nr("azurerm_storage_account", parser.ChangeCreate, map[string]interface{}{
		"allow_blob_public_access":  false,
		"enable_https_traffic_only": true,
	})))
	if len(findings) != 0 {
		t.Errorf("expected no findings for compliant storage account, got %d", len(findings))
	}
}

func TestUnencryptedDB_PostgreSQL_NoSSL(t *testing.T) {
	r := &UnencryptedDatabaseRule{}
	findings := r.Evaluate(ctx(nr("azurerm_postgresql_server", parser.ChangeCreate, map[string]interface{}{
		"ssl_enforcement_enabled": false,
	})))
	if !hasExplanationContaining(findings, "SSL") {
		t.Error("expected SSL finding for PostgreSQL without SSL")
	}
}

func TestUnencryptedDB_MSSQL_NoTDE(t *testing.T) {
	r := &UnencryptedDatabaseRule{}
	findings := r.Evaluate(ctx(nr("azurerm_mssql_database", parser.ChangeCreate, map[string]interface{}{
		"transparent_data_encryption_enabled": false,
	})))
	if !hasExplanationContaining(findings, "Transparent Data Encryption") {
		t.Error("expected TDE finding for MSSQL without encryption")
	}
}

func TestUnencryptedDB_Compliant(t *testing.T) {
	r := &UnencryptedDatabaseRule{}
	findings := r.Evaluate(ctx(nr("azurerm_postgresql_server", parser.ChangeCreate, map[string]interface{}{
		"ssl_enforcement_enabled": true,
	})))
	if len(findings) != 0 {
		t.Errorf("expected no findings for compliant postgres, got %d", len(findings))
	}
}

// ── Tier 3 ────────────────────────────────────────────────────────────────────

func TestOversized_FlagsExpensiveResource(t *testing.T) {
	r := &OversizedResourceRule{OversizeMultiple: 5}
	resources := []normalizer.NormalizedResource{
		{Address: "azurerm_linux_virtual_machine.a", Provider: "azure", ChangeType: parser.ChangeCreate},
		{Address: "azurerm_linux_virtual_machine.b", Provider: "azure", ChangeType: parser.ChangeCreate},
		{Address: "azurerm_linux_virtual_machine.big", Provider: "azure", ChangeType: parser.ChangeCreate},
	}
	estimates := []pricing.Estimate{
		{ResourceAddress: "azurerm_linux_virtual_machine.a", MonthlyCostUSD: 10},
		{ResourceAddress: "azurerm_linux_virtual_machine.b", MonthlyCostUSD: 12},
		{ResourceAddress: "azurerm_linux_virtual_machine.big", MonthlyCostUSD: 1000},
	}
	findings := r.Evaluate(rules.EvaluateContext{Resources: resources, Estimates: estimates})
	if !hasExplanationContaining(findings, "azurerm_linux_virtual_machine.big") {
		t.Error("expected oversized finding for big VM")
	}
}

func TestOversized_IgnoresNonAzure(t *testing.T) {
	r := &OversizedResourceRule{OversizeMultiple: 5}
	resources := []normalizer.NormalizedResource{
		{Address: "aws_instance.a", Provider: "aws", ChangeType: parser.ChangeCreate},
		{Address: "aws_instance.big", Provider: "aws", ChangeType: parser.ChangeCreate},
	}
	estimates := []pricing.Estimate{
		{ResourceAddress: "aws_instance.a", MonthlyCostUSD: 10},
		{ResourceAddress: "aws_instance.big", MonthlyCostUSD: 1000},
	}
	findings := r.Evaluate(rules.EvaluateContext{Resources: resources, Estimates: estimates})
	if len(findings) != 0 {
		t.Error("Azure oversized rule should ignore non-Azure resources")
	}
}

func TestMissingTags_Default(t *testing.T) {
	r := &MissingTagsRule{RequiredTags: []string{"costcenter", "squad"}}
	resource := nr("azurerm_linux_virtual_machine", parser.ChangeCreate, map[string]interface{}{
		"tags": map[string]interface{}{},
	})
	findings := r.Evaluate(rules.EvaluateContext{Resources: []normalizer.NormalizedResource{resource}})
	if !hasExplanationContaining(findings, "costcenter") {
		t.Error("expected missing costcenter tag finding")
	}
}

func TestMissingTags_CtxOverride(t *testing.T) {
	r := &MissingTagsRule{RequiredTags: []string{"costcenter", "squad"}}
	resource := nr("azurerm_linux_virtual_machine", parser.ChangeCreate, map[string]interface{}{
		"tags": map[string]interface{}{},
	})
	findings := r.Evaluate(rules.EvaluateContext{
		Resources:    []normalizer.NormalizedResource{resource},
		RequiredTags: []string{"owner"},
	})
	if !hasExplanationContaining(findings, "owner") {
		t.Error("expected ctx.RequiredTags to override rule defaults")
	}
	if hasExplanationContaining(findings, "costcenter") {
		t.Error("overridden default tags should not appear")
	}
}

func TestMissingTags_NoTagsAttr_Skipped(t *testing.T) {
	r := &MissingTagsRule{RequiredTags: []string{"env"}}
	resource := nr("azurerm_network_security_group", parser.ChangeCreate, map[string]interface{}{})
	findings := r.Evaluate(rules.EvaluateContext{Resources: []normalizer.NormalizedResource{resource}})
	if len(findings) != 0 {
		t.Error("resources without tags attr should be skipped")
	}
}

func TestUnboundedAKS_NoMaxCount(t *testing.T) {
	r := &UnboundedAKSAutoscalingRule{}
	raw := map[string]interface{}{
		"default_node_pool": []interface{}{
			map[string]interface{}{
				"enable_auto_scaling": true,
				"max_count":          float64(0),
			},
		},
	}
	findings := r.Evaluate(ctx(nr("azurerm_kubernetes_cluster", parser.ChangeCreate, raw)))
	if len(findings) == 0 {
		t.Error("expected finding for max_count=0 with autoscaling enabled")
	}
}

func TestUnboundedAKS_WithMaxCount(t *testing.T) {
	r := &UnboundedAKSAutoscalingRule{}
	raw := map[string]interface{}{
		"default_node_pool": []interface{}{
			map[string]interface{}{
				"enable_auto_scaling": true,
				"max_count":          float64(10),
			},
		},
	}
	findings := r.Evaluate(ctx(nr("azurerm_kubernetes_cluster", parser.ChangeCreate, raw)))
	if len(findings) != 0 {
		t.Errorf("expected no finding when max_count=10, got %d", len(findings))
	}
}

func TestUnboundedAKS_AutoscalingDisabled_NoFinding(t *testing.T) {
	r := &UnboundedAKSAutoscalingRule{}
	raw := map[string]interface{}{
		"default_node_pool": []interface{}{
			map[string]interface{}{
				"enable_auto_scaling": false,
				"max_count":          float64(0),
			},
		},
	}
	findings := r.Evaluate(ctx(nr("azurerm_kubernetes_cluster", parser.ChangeCreate, raw)))
	if len(findings) != 0 {
		t.Error("no finding expected when autoscaling is disabled")
	}
}

// ── AllRules smoke test ───────────────────────────────────────────────────────

func TestAllRules_ReturnEightRules(t *testing.T) {
	if len(AllRules()) != 8 {
		t.Errorf("expected 8 rules, got %d", len(AllRules()))
	}
}
