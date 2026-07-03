package azure

import (
	"testing"

	"github.com/amt/tf-cost-risk/normalizer"
	"github.com/amt/tf-cost-risk/parser"
)

func rc(resourceType string, changeType parser.ChangeType, after map[string]interface{}) parser.ResourceChange {
	return parser.ResourceChange{
		Address:    resourceType + ".test",
		Type:       resourceType,
		ChangeType: changeType,
		After:      after,
	}
}

var norm = &Normalizer{}

func TestNormalize_VM_BasicFields(t *testing.T) {
	nr, err := norm.Normalize(rc("azurerm_linux_virtual_machine", parser.ChangeCreate, map[string]interface{}{
		"size":     "Standard_D2s_v3",
		"location": "East US",
		"tags":     map[string]interface{}{"env": "prod"},
	}), "eastus")
	if err != nil {
		t.Fatal(err)
	}
	if nr.Provider != "azure" {
		t.Errorf("expected provider azure, got %s", nr.Provider)
	}
	if nr.Size != "Standard_D2s_v3" {
		t.Errorf("expected size Standard_D2s_v3, got %s", nr.Size)
	}
	if nr.Region != "eastus" {
		t.Errorf("expected region eastus, got %s", nr.Region)
	}
	if nr.Tags["env"] != "prod" {
		t.Errorf("expected tag env=prod")
	}
	if nr.Category != normalizer.CategoryCompute {
		t.Errorf("expected compute category")
	}
	if nr.Stateful {
		t.Error("VM should not be stateful")
	}
}

func TestNormalize_LocationNormalization(t *testing.T) {
	nr, err := norm.Normalize(rc("azurerm_linux_virtual_machine", parser.ChangeCreate, map[string]interface{}{
		"location": "West Europe",
	}), "eastus")
	if err != nil {
		t.Fatal(err)
	}
	if nr.Region != "westeurope" {
		t.Errorf("expected westeurope, got %s", nr.Region)
	}
}

func TestNormalize_LocationFallback(t *testing.T) {
	nr, err := norm.Normalize(rc("azurerm_linux_virtual_machine", parser.ChangeCreate, map[string]interface{}{}), "eastus")
	if err != nil {
		t.Fatal(err)
	}
	if nr.Region != "eastus" {
		t.Errorf("expected fallback region eastus, got %s", nr.Region)
	}
}

func TestNormalize_ManagedDisk_Stateful(t *testing.T) {
	nr, err := norm.Normalize(rc("azurerm_managed_disk", parser.ChangeCreate, map[string]interface{}{
		"storage_account_type": "Premium_LRS",
		"location":             "eastus",
	}), "eastus")
	if err != nil {
		t.Fatal(err)
	}
	if !nr.Stateful {
		t.Error("managed disk should be stateful")
	}
	if nr.Size != "Premium_LRS" {
		t.Errorf("expected size Premium_LRS, got %s", nr.Size)
	}
	if nr.Category != normalizer.CategoryStorage {
		t.Errorf("expected storage category")
	}
}

func TestNormalize_PostgreSQL_Stateful(t *testing.T) {
	nr, err := norm.Normalize(rc("azurerm_postgresql_server", parser.ChangeCreate, map[string]interface{}{
		"sku_name": "GP_Gen5_4",
		"location": "eastus",
	}), "eastus")
	if err != nil {
		t.Fatal(err)
	}
	if !nr.Stateful {
		t.Error("postgresql server should be stateful")
	}
	if nr.Size != "GP_Gen5_4" {
		t.Errorf("expected size GP_Gen5_4, got %s", nr.Size)
	}
	if nr.Category != normalizer.CategoryDatabase {
		t.Errorf("expected database category")
	}
}

func TestNormalize_AKS_DefaultNodePoolSize(t *testing.T) {
	nr, err := norm.Normalize(rc("azurerm_kubernetes_cluster", parser.ChangeCreate, map[string]interface{}{
		"location": "eastus",
		"default_node_pool": []interface{}{
			map[string]interface{}{"vm_size": "Standard_D4s_v3", "node_count": float64(3)},
		},
	}), "eastus")
	if err != nil {
		t.Fatal(err)
	}
	if nr.Size != "Standard_D4s_v3" {
		t.Errorf("expected AKS node size Standard_D4s_v3, got %s", nr.Size)
	}
}

func TestNormalize_NilAfterUseBefore(t *testing.T) {
	change := parser.ResourceChange{
		Address:    "azurerm_managed_disk.old",
		Type:       "azurerm_managed_disk",
		ChangeType: parser.ChangeDelete,
		Before:     map[string]interface{}{"storage_account_type": "Standard_LRS", "location": "eastus"},
		After:      nil,
	}
	nr, err := norm.Normalize(change, "eastus")
	if err != nil {
		t.Fatal(err)
	}
	if nr.Size != "Standard_LRS" {
		t.Errorf("expected size from Before attrs, got %s", nr.Size)
	}
}

func TestNormalize_UnknownType_CategoryUnknown(t *testing.T) {
	nr, err := norm.Normalize(rc("azurerm_some_new_resource", parser.ChangeCreate, map[string]interface{}{}), "eastus")
	if err != nil {
		t.Fatal(err)
	}
	if nr.Category != normalizer.CategoryUnknown {
		t.Errorf("unknown resource type should have CategoryUnknown")
	}
}

func TestNormalize_FunctionApp(t *testing.T) {
	nr, err := norm.Normalize(rc("azurerm_linux_function_app", parser.ChangeCreate, map[string]interface{}{
		"location": "eastus",
	}), "eastus")
	if err != nil {
		t.Fatal(err)
	}
	if nr.Category != normalizer.CategoryFunction {
		t.Errorf("expected function category")
	}
}

func TestNormalize_Tags_Empty(t *testing.T) {
	nr, err := norm.Normalize(rc("azurerm_linux_virtual_machine", parser.ChangeCreate, map[string]interface{}{
		"tags": map[string]interface{}{},
	}), "eastus")
	if err != nil {
		t.Fatal(err)
	}
	if len(nr.Tags) != 0 {
		t.Errorf("expected empty tags map, got %v", nr.Tags)
	}
}

func TestNormalize_Tags_NoTagsKey(t *testing.T) {
	nr, err := norm.Normalize(rc("azurerm_network_security_group", parser.ChangeCreate, map[string]interface{}{}), "eastus")
	if err != nil {
		t.Fatal(err)
	}
	if nr.Tags == nil {
		t.Error("Tags should be non-nil empty map even when no tags key in attrs")
	}
}
