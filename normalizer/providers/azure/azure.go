// Package azure normalizes Azure Terraform resource changes into the common internal schema.
package azure

import (
	"strings"

	"github.com/amt/tf-cost-risk/normalizer"
	"github.com/amt/tf-cost-risk/parser"
)

// Normalizer implements normalizer.Normalizer for Azure resources.
type Normalizer struct{}

var statefulTypes = map[string]bool{
	"azurerm_sql_server":                 true,
	"azurerm_mssql_server":               true,
	"azurerm_mssql_database":             true,
	"azurerm_sql_database":               true,
	"azurerm_postgresql_server":          true,
	"azurerm_postgresql_flexible_server": true,
	"azurerm_mysql_server":               true,
	"azurerm_mysql_flexible_server":      true,
	"azurerm_cosmosdb_account":           true,
	"azurerm_managed_disk":               true,
	"azurerm_storage_account":            true,
}

var categoryMap = map[string]normalizer.ResourceCategory{
	"azurerm_linux_virtual_machine":      normalizer.CategoryCompute,
	"azurerm_windows_virtual_machine":    normalizer.CategoryCompute,
	"azurerm_virtual_machine":            normalizer.CategoryCompute,
	"azurerm_kubernetes_cluster":         normalizer.CategoryCompute,
	"azurerm_mssql_database":             normalizer.CategoryDatabase,
	"azurerm_sql_database":               normalizer.CategoryDatabase,
	"azurerm_postgresql_server":          normalizer.CategoryDatabase,
	"azurerm_postgresql_flexible_server": normalizer.CategoryDatabase,
	"azurerm_mysql_server":               normalizer.CategoryDatabase,
	"azurerm_mysql_flexible_server":      normalizer.CategoryDatabase,
	"azurerm_cosmosdb_account":           normalizer.CategoryDatabase,
	"azurerm_managed_disk":               normalizer.CategoryStorage,
	"azurerm_storage_account":            normalizer.CategoryStorage,
	"azurerm_network_security_group":     normalizer.CategoryNetwork,
	"azurerm_lb":                         normalizer.CategoryNetwork,
	"azurerm_application_gateway":        normalizer.CategoryNetwork,
	"azurerm_function_app":               normalizer.CategoryFunction,
	"azurerm_linux_function_app":         normalizer.CategoryFunction,
	"azurerm_windows_function_app":       normalizer.CategoryFunction,
}

// Normalize maps an Azure resource change to a NormalizedResource.
func (n *Normalizer) Normalize(rc parser.ResourceChange, region string) (normalizer.NormalizedResource, error) {
	attrs := rc.After
	if attrs == nil {
		attrs = rc.Before
	}
	if attrs == nil {
		attrs = map[string]interface{}{}
	}

	cat, ok := categoryMap[rc.Type]
	if !ok {
		cat = normalizer.CategoryUnknown
	}

	nr := normalizer.NormalizedResource{
		Address:      rc.Address,
		ChangeType:   rc.ChangeType,
		Provider:     "azure",
		ResourceType: rc.Type,
		Category:     cat,
		Size:         extractSize(rc.Type, attrs),
		Region:       extractRegion(attrs, region),
		Tags:         extractTags(attrs),
		Stateful:     statefulTypes[rc.Type],
		Raw:          attrs,
	}
	return nr, nil
}

func extractSize(resourceType string, attrs map[string]interface{}) string {
	switch resourceType {
	case "azurerm_linux_virtual_machine", "azurerm_windows_virtual_machine", "azurerm_virtual_machine":
		return strAttr(attrs, "size") // e.g. "Standard_D2s_v3"
	case "azurerm_mssql_database", "azurerm_sql_database":
		return strAttr(attrs, "sku_name") // e.g. "S2", "GP_Gen5_2"
	case "azurerm_postgresql_server", "azurerm_mysql_server":
		return strAttr(attrs, "sku_name") // e.g. "GP_Gen5_2"
	case "azurerm_postgresql_flexible_server", "azurerm_mysql_flexible_server":
		return strAttr(attrs, "sku_name") // e.g. "GP_Standard_D2s_v3"
	case "azurerm_kubernetes_cluster":
		// Primary node pool size
		if np := firstBlock(attrs, "default_node_pool"); np != nil {
			return strAttr(np, "vm_size")
		}
	case "azurerm_managed_disk":
		return strAttr(attrs, "storage_account_type") // e.g. "Premium_LRS"
	}
	return ""
}

func extractRegion(attrs map[string]interface{}, fallback string) string {
	if loc := strAttr(attrs, "location"); loc != "" {
		return normalizeLocation(loc)
	}
	return fallback
}

// normalizeLocation converts Azure location strings to a canonical lowercase slug.
// e.g. "East US" → "eastus", "eastus" → "eastus"
func normalizeLocation(loc string) string {
	return strings.ToLower(strings.ReplaceAll(loc, " ", ""))
}

func extractTags(attrs map[string]interface{}) map[string]string {
	result := map[string]string{}
	raw, ok := attrs["tags"]
	if !ok {
		return result
	}
	switch t := raw.(type) {
	case map[string]interface{}:
		for k, v := range t {
			if s, ok := v.(string); ok {
				result[k] = s
			}
		}
	}
	return result
}

// helpers

func strAttr(attrs map[string]interface{}, key string) string {
	if attrs == nil {
		return ""
	}
	v, ok := attrs[key]
	if !ok {
		return ""
	}
	s, _ := v.(string)
	return s
}

func boolAttr(attrs map[string]interface{}, key string) bool {
	if attrs == nil {
		return false
	}
	v, ok := attrs[key]
	if !ok {
		return false
	}
	b, _ := v.(bool)
	return b
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

func toSlice(v interface{}) []interface{} {
	if v == nil {
		return nil
	}
	s, _ := v.([]interface{})
	return s
}

func firstBlock(attrs map[string]interface{}, key string) map[string]interface{} {
	items := toSlice(attrs[key])
	if len(items) == 0 {
		return nil
	}
	m, _ := items[0].(map[string]interface{})
	return m
}
