package azure

import (
	"testing"

	"github.com/amt/tf-cost-risk/normalizer"
	"github.com/amt/tf-cost-risk/parser"
)

func nr(resourceType, size string, raw map[string]interface{}) normalizer.NormalizedResource {
	return normalizer.NormalizedResource{
		Address:      resourceType + ".test",
		Provider:     "azure",
		ResourceType: resourceType,
		ChangeType:   parser.ChangeCreate,
		Size:         size,
		Raw:          raw,
	}
}

var pricer = &Pricer{}

func TestEstimate_VM_KnownSize(t *testing.T) {
	e := pricer.Estimate(nr("azurerm_linux_virtual_machine", "Standard_D2s_v3", nil))
	if e.Unknown {
		t.Error("expected known cost for Standard_D2s_v3")
	}
	if e.MonthlyCostUSD != 70.08 {
		t.Errorf("expected $70.08, got $%.2f", e.MonthlyCostUSD)
	}
}

func TestEstimate_VM_FuzzyCase(t *testing.T) {
	e := pricer.Estimate(nr("azurerm_linux_virtual_machine", "standard_d2s_v3", nil))
	if e.Unknown {
		t.Error("fuzzy match should resolve lowercase size")
	}
	if e.MonthlyCostUSD != 70.08 {
		t.Errorf("expected $70.08, got $%.2f", e.MonthlyCostUSD)
	}
}

func TestEstimate_VM_UnknownSize(t *testing.T) {
	e := pricer.Estimate(nr("azurerm_linux_virtual_machine", "Standard_M128s", nil))
	if !e.Unknown {
		t.Error("unknown VM size should return unknown cost")
	}
}

func TestEstimate_SQLDatabase_DTU(t *testing.T) {
	e := pricer.Estimate(nr("azurerm_mssql_database", "S2", nil))
	if e.Unknown {
		t.Error("expected known cost for S2 SQL SKU")
	}
	if e.MonthlyCostUSD != 58.88 {
		t.Errorf("expected $58.88, got $%.2f", e.MonthlyCostUSD)
	}
}

func TestEstimate_SQLDatabase_VCore(t *testing.T) {
	e := pricer.Estimate(nr("azurerm_mssql_database", "GP_Gen5_4", nil))
	if e.Unknown || e.MonthlyCostUSD <= 0 {
		t.Error("expected known cost for GP_Gen5_4")
	}
}

func TestEstimate_PostgreSQL(t *testing.T) {
	e := pricer.Estimate(nr("azurerm_postgresql_server", "GP_Gen5_2", nil))
	if e.Unknown {
		t.Error("expected known cost for GP_Gen5_2 postgres")
	}
}

func TestEstimate_ManagedDisk_PremiumLRS(t *testing.T) {
	e := pricer.Estimate(nr("azurerm_managed_disk", "Premium_LRS", map[string]interface{}{
		"disk_size_gb": float64(256),
	}))
	if e.Unknown {
		t.Error("expected known cost for Premium_LRS disk")
	}
	expected := 256 * 0.135
	if e.MonthlyCostUSD != expected {
		t.Errorf("expected $%.2f, got $%.2f", expected, e.MonthlyCostUSD)
	}
}

func TestEstimate_ManagedDisk_DefaultSize(t *testing.T) {
	e := pricer.Estimate(nr("azurerm_managed_disk", "Standard_LRS", map[string]interface{}{}))
	if e.Unknown {
		t.Error("expected known cost for Standard_LRS disk with default size")
	}
	expected := 128 * 0.04
	if e.MonthlyCostUSD != expected {
		t.Errorf("expected $%.2f (128GB default), got $%.2f", expected, e.MonthlyCostUSD)
	}
}

func TestEstimate_LoadBalancer(t *testing.T) {
	e := pricer.Estimate(nr("azurerm_lb", "", nil))
	if e.Unknown || e.MonthlyCostUSD != lbMonthly {
		t.Errorf("expected load balancer base cost $%.2f", lbMonthly)
	}
}

func TestEstimate_NSG_Free(t *testing.T) {
	e := pricer.Estimate(nr("azurerm_network_security_group", "", nil))
	if e.Unknown {
		t.Error("NSG should have known cost ($0)")
	}
	if e.MonthlyCostUSD != 0 {
		t.Errorf("NSG should cost $0, got $%.2f", e.MonthlyCostUSD)
	}
}

func TestEstimate_StorageAccount_UsageBased(t *testing.T) {
	e := pricer.Estimate(nr("azurerm_storage_account", "", nil))
	if !e.Unknown {
		t.Error("storage account should be usage-based (unknown)")
	}
}

func TestEstimate_FunctionApp_UsageBased(t *testing.T) {
	e := pricer.Estimate(nr("azurerm_linux_function_app", "", nil))
	if !e.Unknown {
		t.Error("function app should be usage-based (unknown)")
	}
}

func TestEstimate_AKS_NodeSize(t *testing.T) {
	e := pricer.Estimate(nr("azurerm_kubernetes_cluster", "Standard_D4s_v3", nil))
	if e.Unknown {
		t.Error("expected known cost for AKS with known node VM size")
	}
	if e.MonthlyCostUSD != vmPrices["Standard_D4s_v3"] {
		t.Errorf("expected AKS cost matching VM price, got $%.2f", e.MonthlyCostUSD)
	}
}
