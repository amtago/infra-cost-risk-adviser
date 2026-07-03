package gcp

import (
	"fmt"
	"strings"

	"github.com/amt/tf-cost-risk/parser"
	"github.com/amt/tf-cost-risk/rules"
)

// sensitivePorts maps port numbers to service names for firewall rule checks.
var sensitivePorts = map[int]string{
	22:    "SSH",
	3389:  "RDP",
	3306:  "MySQL",
	5432:  "PostgreSQL",
	1433:  "MSSQL",
	6379:  "Redis",
	27017: "MongoDB",
}

// OpenFirewallRule flags google_compute_firewall rules that allow ingress from 0.0.0.0/0 on sensitive ports.
type OpenFirewallRule struct{}

func (r *OpenFirewallRule) Evaluate(ctx rules.EvaluateContext) []rules.Finding {
	var findings []rules.Finding
	for _, nr := range ctx.Resources {
		if nr.Provider != "gcp" || nr.ResourceType != "google_compute_firewall" {
			continue
		}
		if nr.ChangeType == parser.ChangeDelete {
			continue
		}
		// Only check ingress rules
		direction := strAttr(nr.Raw, "direction")
		if direction != "" && strings.ToUpper(direction) != "INGRESS" {
			continue
		}

		sourceRanges := toSlice(nr.Raw["source_ranges"])
		if !containsCIDR(sourceRanges, "0.0.0.0/0") && !containsCIDR(sourceRanges, "::/0") {
			continue
		}

		// Check each allow block for sensitive ports
		for _, item := range toSlice(nr.Raw["allow"]) {
			block, ok := item.(map[string]interface{})
			if !ok {
				continue
			}
			proto := strings.ToLower(strAttr(block, "protocol"))
			if proto != "tcp" && proto != "udp" && proto != "all" {
				continue
			}
			ports := toSlice(block["ports"])
			for port, name := range sensitivePorts {
				if proto == "all" || portInList(ports, port) {
					findings = append(findings, rules.Finding{
						Severity:        rules.SeverityCritical,
						Category:        rules.CategorySecurity,
						ResourceAddress: nr.Address,
						Explanation: fmt.Sprintf(
							"%s allows inbound %s (port %d) from 0.0.0.0/0 (the entire internet). Restrict source_ranges to known IP ranges.",
							nr.Address, name, port,
						),
					})
				}
			}
		}
	}
	return findings
}

// PublicStorageBucketRule flags Cloud Storage buckets with public IAM or uniform bucket-level access disabled.
type PublicStorageBucketRule struct{}

func (r *PublicStorageBucketRule) Evaluate(ctx rules.EvaluateContext) []rules.Finding {
	var findings []rules.Finding
	for _, nr := range ctx.Resources {
		if nr.Provider != "gcp" || nr.ResourceType != "google_storage_bucket" {
			continue
		}
		if nr.ChangeType == parser.ChangeDelete {
			continue
		}
		// uniform_bucket_level_access = false means legacy ACLs are active — higher risk of misconfiguration
		if !boolAttr(nr.Raw, "uniform_bucket_level_access") {
			findings = append(findings, rules.Finding{
				Severity:        rules.SeverityWarning,
				Category:        rules.CategorySecurity,
				ResourceAddress: nr.Address,
				Explanation: fmt.Sprintf(
					"%s has uniform_bucket_level_access disabled. Legacy ACLs can lead to unintended public access. Enable uniform bucket-level access.",
					nr.Address,
				),
			})
		}
	}
	return findings
}

// UnencryptedSQLRule flags Cloud SQL instances without SSL enforcement or backups enabled.
type UnencryptedSQLRule struct{}

func (r *UnencryptedSQLRule) Evaluate(ctx rules.EvaluateContext) []rules.Finding {
	var findings []rules.Finding
	for _, nr := range ctx.Resources {
		if nr.Provider != "gcp" || nr.ResourceType != "google_sql_database_instance" {
			continue
		}
		if nr.ChangeType == parser.ChangeDelete {
			continue
		}

		settings := firstBlock(nr.Raw, "settings")
		if settings == nil {
			continue
		}

		// Check SSL enforcement
		ipConfig := firstBlock(settings, "ip_configuration")
		if ipConfig != nil {
			if !boolAttr(ipConfig, "require_ssl") {
				findings = append(findings, rules.Finding{
					Severity:        rules.SeverityWarning,
					Category:        rules.CategorySecurity,
					ResourceAddress: nr.Address,
					Explanation: fmt.Sprintf(
						"%s (google_sql_database_instance) does not require SSL connections (require_ssl = false). Enable SSL to encrypt data in transit.",
						nr.Address,
					),
				})
			}
		}

		// Check backup configuration
		backupConfig := firstBlock(settings, "backup_configuration")
		if backupConfig == nil || !boolAttr(backupConfig, "enabled") {
			findings = append(findings, rules.Finding{
				Severity:        rules.SeverityWarning,
				Category:        rules.CategorySecurity,
				ResourceAddress: nr.Address,
				Explanation: fmt.Sprintf(
					"%s (google_sql_database_instance) does not have automated backups enabled. Enable backup_configuration to protect against data loss.",
					nr.Address,
				),
			})
		}
	}
	return findings
}

// containsCIDR checks if a slice of interface{} contains the given CIDR string.
func containsCIDR(items []interface{}, cidr string) bool {
	for _, item := range items {
		if s, ok := item.(string); ok && s == cidr {
			return true
		}
	}
	return false
}

// portInList checks if a port number is covered by a GCP firewall port list entry.
// Entries can be "22", "8080-8090", etc.
func portInList(ports []interface{}, port int) bool {
	if len(ports) == 0 {
		// No ports specified means all ports
		return true
	}
	for _, p := range ports {
		s, ok := p.(string)
		if !ok {
			continue
		}
		parts := strings.SplitN(s, "-", 2)
		if len(parts) == 1 {
			var n int
			fmt.Sscanf(parts[0], "%d", &n)
			if n == port {
				return true
			}
		} else {
			var from, to int
			fmt.Sscanf(parts[0], "%d", &from)
			fmt.Sscanf(parts[1], "%d", &to)
			if from <= port && port <= to {
				return true
			}
		}
	}
	return false
}
