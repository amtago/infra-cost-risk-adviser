package azure

import (
	"fmt"
	"strings"

	"github.com/amt/tf-cost-risk/parser"
	"github.com/amt/tf-cost-risk/rules"
)

// sensitivePorts maps port numbers to service names checked in NSG inbound rules.
var sensitivePorts = map[int]string{
	22:    "SSH",
	3389:  "RDP",
	1433:  "MSSQL",
	3306:  "MySQL",
	5432:  "PostgreSQL",
	6379:  "Redis",
	27017: "MongoDB",
}

// OpenNSGRule flags azurerm_network_security_group rules that allow inbound traffic
// from any source (0.0.0.0/0 or *) on sensitive ports.
type OpenNSGRule struct{}

func (r *OpenNSGRule) Evaluate(ctx rules.EvaluateContext) []rules.Finding {
	var findings []rules.Finding
	for _, nr := range ctx.Resources {
		if nr.Provider != "azure" || nr.ResourceType != "azurerm_network_security_group" {
			continue
		}
		if nr.ChangeType == parser.ChangeDelete {
			continue
		}
		for _, item := range toSlice(nr.Raw["security_rule"]) {
			rule, ok := item.(map[string]interface{})
			if !ok {
				continue
			}
			if strings.ToUpper(strAttr(rule, "direction")) != "Inbound" &&
				strings.ToUpper(strAttr(rule, "direction")) != "INBOUND" {
				continue
			}
			if strings.ToUpper(strAttr(rule, "access")) != "ALLOW" {
				continue
			}
			src := strAttr(rule, "source_address_prefix")
			if src != "*" && src != "0.0.0.0/0" && src != "Internet" {
				continue
			}
			portRange := strAttr(rule, "destination_port_range")
			for port, name := range sensitivePorts {
				if portRangeCoversPort(portRange, port) {
					findings = append(findings, rules.Finding{
						Severity:        rules.SeverityCritical,
						Category:        rules.CategorySecurity,
						ResourceAddress: nr.Address,
						Explanation: fmt.Sprintf(
							"%s has an inbound NSG rule allowing %s (port %d) from any source (%s). Restrict source_address_prefix to known IP ranges.",
							nr.Address, name, port, src,
						),
					})
				}
			}
		}
	}
	return findings
}

// portRangeCoversPort checks if an Azure port range string covers a given port.
// Azure uses "*" for all, or "22", or "22-80" notation.
func portRangeCoversPort(portRange string, port int) bool {
	if portRange == "*" {
		return true
	}
	parts := strings.SplitN(portRange, "-", 2)
	if len(parts) == 1 {
		var n int
		fmt.Sscanf(parts[0], "%d", &n)
		return n == port
	}
	var from, to int
	fmt.Sscanf(parts[0], "%d", &from)
	fmt.Sscanf(parts[1], "%d", &to)
	return from <= port && port <= to
}

// PublicStorageAccountRule flags Azure Storage Accounts with public blob access enabled.
type PublicStorageAccountRule struct{}

func (r *PublicStorageAccountRule) Evaluate(ctx rules.EvaluateContext) []rules.Finding {
	var findings []rules.Finding
	for _, nr := range ctx.Resources {
		if nr.Provider != "azure" || nr.ResourceType != "azurerm_storage_account" {
			continue
		}
		if nr.ChangeType == parser.ChangeDelete {
			continue
		}
		// allow_blob_public_access defaults to false in newer provider versions, but older configs may set it true.
		if boolAttr(nr.Raw, "allow_blob_public_access") || boolAttr(nr.Raw, "allow_nested_items_to_be_public") {
			findings = append(findings, rules.Finding{
				Severity:        rules.SeverityCritical,
				Category:        rules.CategorySecurity,
				ResourceAddress: nr.Address,
				Explanation: fmt.Sprintf(
					"%s (azurerm_storage_account) has public blob access enabled. This can expose storage contents to the internet. Set allow_blob_public_access = false.",
					nr.Address,
				),
			})
		}
		// HTTPS-only transport
		if !boolAttr(nr.Raw, "enable_https_traffic_only") {
			findings = append(findings, rules.Finding{
				Severity:        rules.SeverityWarning,
				Category:        rules.CategorySecurity,
				ResourceAddress: nr.Address,
				Explanation: fmt.Sprintf(
					"%s (azurerm_storage_account) does not enforce HTTPS-only traffic (enable_https_traffic_only = false). Enable HTTPS to encrypt data in transit.",
					nr.Address,
				),
			})
		}
	}
	return findings
}

// UnencryptedDatabaseRule flags Azure database servers without SSL/TLS enforcement.
type UnencryptedDatabaseRule struct{}

func (r *UnencryptedDatabaseRule) Evaluate(ctx rules.EvaluateContext) []rules.Finding {
	var findings []rules.Finding
	for _, nr := range ctx.Resources {
		if nr.Provider != "azure" {
			continue
		}
		if nr.ChangeType == parser.ChangeDelete {
			continue
		}
		switch nr.ResourceType {
		case "azurerm_postgresql_server", "azurerm_mysql_server":
			if !boolAttr(nr.Raw, "ssl_enforcement_enabled") {
				findings = append(findings, rules.Finding{
					Severity:        rules.SeverityWarning,
					Category:        rules.CategorySecurity,
					ResourceAddress: nr.Address,
					Explanation: fmt.Sprintf(
						"%s (%s) does not enforce SSL connections (ssl_enforcement_enabled = false). Enable SSL to encrypt database traffic in transit.",
						nr.Address, nr.ResourceType,
					),
				})
			}
		case "azurerm_mssql_database":
			if !boolAttr(nr.Raw, "transparent_data_encryption_enabled") {
				findings = append(findings, rules.Finding{
					Severity:        rules.SeverityWarning,
					Category:        rules.CategorySecurity,
					ResourceAddress: nr.Address,
					Explanation: fmt.Sprintf(
						"%s (azurerm_mssql_database) does not have Transparent Data Encryption enabled. Enable transparent_data_encryption_enabled to encrypt data at rest.",
						nr.Address,
					),
				})
			}
		}
	}
	return findings
}
