package azure

import (
	"fmt"

	"github.com/amt/tf-cost-risk/normalizer"
	"github.com/amt/tf-cost-risk/parser"
	"github.com/amt/tf-cost-risk/rules"
)

// DestructiveChangeRule flags delete/replace operations, escalating severity for stateful resources.
type DestructiveChangeRule struct{}

func (r *DestructiveChangeRule) Evaluate(ctx rules.EvaluateContext) []rules.Finding {
	var findings []rules.Finding
	for _, nr := range ctx.Resources {
		if nr.Provider != "azure" {
			continue
		}
		switch nr.ChangeType {
		case parser.ChangeDelete:
			findings = append(findings, destructiveFinding(nr, "will be permanently deleted"))
		case parser.ChangeReplace:
			findings = append(findings, destructiveFinding(nr, "will be destroyed and recreated (replace), causing downtime"))
		}
	}
	return findings
}

func destructiveFinding(nr normalizer.NormalizedResource, action string) rules.Finding {
	sev := rules.SeverityWarning
	if nr.Stateful {
		sev = rules.SeverityCritical
	}
	return rules.Finding{
		Severity:        sev,
		Category:        rules.CategoryDestructive,
		ResourceAddress: nr.Address,
		Explanation:     fmt.Sprintf("%s (%s) %s.", nr.Address, nr.ResourceType, action),
	}
}

// deletionProtectedTypes maps Azure resource types to the attribute that enables soft-delete / deletion lock.
// Azure doesn't have a universal deletion_protection; we flag the most impactful ones.
var deletionLockTypes = map[string]string{
	"azurerm_mssql_database":             "transparent_data_encryption_enabled",
	"azurerm_postgresql_server":          "ssl_enforcement_enabled", // reused as a proxy for hardened config
	"azurerm_postgresql_flexible_server": "backup_retention_days",
	"azurerm_mysql_flexible_server":      "backup_retention_days",
}

// MissingBackupRetentionRule flags database servers with backup_retention_days not set or set too low.
type MissingBackupRetentionRule struct {
	MinRetentionDays int // default 7
}

func (r *MissingBackupRetentionRule) Evaluate(ctx rules.EvaluateContext) []rules.Finding {
	minDays := r.MinRetentionDays
	if minDays <= 0 {
		minDays = 7
	}
	var findings []rules.Finding
	for _, nr := range ctx.Resources {
		if nr.Provider != "azure" {
			continue
		}
		if nr.ChangeType == parser.ChangeDelete {
			continue
		}
		switch nr.ResourceType {
		case "azurerm_postgresql_flexible_server", "azurerm_mysql_flexible_server",
			"azurerm_postgresql_server", "azurerm_mysql_server":
		default:
			continue
		}
		days := intAttr(nr.Raw, "backup_retention_days")
		if days < minDays {
			findings = append(findings, rules.Finding{
				Severity:        rules.SeverityWarning,
				Category:        rules.CategoryDestructive,
				ResourceAddress: nr.Address,
				Explanation: fmt.Sprintf(
					"%s (%s) has backup_retention_days set to %d (minimum recommended: %d). Insufficient backup retention risks unrecoverable data loss.",
					nr.Address, nr.ResourceType, days, minDays,
				),
			})
		}
	}
	return findings
}
