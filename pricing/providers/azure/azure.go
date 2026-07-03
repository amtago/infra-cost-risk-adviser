// Package azure provides static pricing estimates for Azure resources.
package azure

import (
	"strings"

	"github.com/amt/tf-cost-risk/normalizer"
	"github.com/amt/tf-cost-risk/pricing"
)

// Pricer implements pricing.Pricer using a static in-repo price table.
type Pricer struct{}

// usageBased lists resource types whose cost depends entirely on runtime usage.
var usageBased = map[string]bool{
	"azurerm_storage_account":     true,
	"azurerm_cosmosdb_account":    true,
	"azurerm_function_app":        true,
	"azurerm_linux_function_app":  true,
	"azurerm_windows_function_app": true,
}

func (p *Pricer) Estimate(nr normalizer.NormalizedResource) pricing.Estimate {
	base := pricing.Estimate{ResourceAddress: nr.Address, ChangeType: nr.ChangeType}

	if usageBased[nr.ResourceType] {
		base.Unknown = true
		return base
	}

	switch nr.ResourceType {
	case "azurerm_linux_virtual_machine", "azurerm_windows_virtual_machine", "azurerm_virtual_machine":
		if cost, ok := vmPrices[nr.Size]; ok {
			base.MonthlyCostUSD = cost
			return base
		}
		// Fuzzy match: try normalising the size (e.g. "standard_d2s_v3" → "Standard_D2s_v3")
		if cost := lookupVMFuzzy(nr.Size); cost > 0 {
			base.MonthlyCostUSD = cost
			return base
		}

	case "azurerm_kubernetes_cluster":
		// AKS control plane is free; node pool cost depends on VM size × node count.
		// We price the default node pool VM size as a single-node approximation.
		if cost, ok := vmPrices[nr.Size]; ok {
			base.MonthlyCostUSD = cost
			return base
		}
		if cost := lookupVMFuzzy(nr.Size); cost > 0 {
			base.MonthlyCostUSD = cost
			return base
		}

	case "azurerm_mssql_database", "azurerm_sql_database":
		if cost, ok := sqlPrices[nr.Size]; ok {
			base.MonthlyCostUSD = cost
			return base
		}

	case "azurerm_postgresql_server", "azurerm_mysql_server",
		"azurerm_postgresql_flexible_server", "azurerm_mysql_flexible_server":
		if cost, ok := pgPrices[nr.Size]; ok {
			base.MonthlyCostUSD = cost
			return base
		}

	case "azurerm_managed_disk":
		return estimateDisk(nr, base)

	case "azurerm_lb", "azurerm_application_gateway":
		base.MonthlyCostUSD = lbMonthly
		return base

	case "azurerm_network_security_group":
		base.MonthlyCostUSD = 0
		return base
	}

	base.Unknown = true
	return base
}

func estimateDisk(nr normalizer.NormalizedResource, base pricing.Estimate) pricing.Estimate {
	diskType := nr.Size // storage_account_type
	if diskType == "" {
		diskType = "Standard_LRS"
	}
	pricePerGB, ok := diskPricePerGB[diskType]
	if !ok {
		base.Unknown = true
		return base
	}
	gb := intAttr(nr.Raw, "disk_size_gb")
	if gb <= 0 {
		gb = 128 // Azure default managed disk size
	}
	base.MonthlyCostUSD = float64(gb) * pricePerGB
	return base
}

// lookupVMFuzzy attempts a case-insensitive prefix match for VM sizes.
func lookupVMFuzzy(size string) float64 {
	lower := strings.ToLower(size)
	for k, v := range vmPrices {
		if strings.ToLower(k) == lower {
			return v
		}
	}
	return 0
}

func intAttr(attrs map[string]interface{}, key string) int {
	if attrs == nil {
		return 0
	}
	v, ok := attrs[key]
	if !ok {
		return 0
	}
	switch n := v.(type) {
	case float64:
		return int(n)
	case int:
		return n
	}
	return 0
}
