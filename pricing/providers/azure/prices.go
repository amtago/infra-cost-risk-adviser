package azure

// vmPrices maps Azure VM size names to approximate monthly on-demand cost (USD, East US, Linux).
// Source: Azure pricing calculator, Pay As You Go rates. Updated 2024-Q4.
var vmPrices = map[string]float64{
	// B-series (burstable)
	"Standard_B1s":  7.59,
	"Standard_B1ms": 15.19,
	"Standard_B2s":  30.37,
	"Standard_B2ms": 60.74,
	"Standard_B4ms": 121.47,
	"Standard_B8ms": 242.94,
	// D-series v3
	"Standard_D2s_v3":  70.08,
	"Standard_D4s_v3":  140.16,
	"Standard_D8s_v3":  280.32,
	"Standard_D16s_v3": 560.64,
	"Standard_D32s_v3": 1121.28,
	// D-series v4/v5
	"Standard_D2s_v4":  70.08,
	"Standard_D4s_v4":  140.16,
	"Standard_D8s_v4":  280.32,
	"Standard_D2s_v5":  68.62,
	"Standard_D4s_v5":  137.24,
	"Standard_D8s_v5":  274.48,
	// E-series (memory-optimised)
	"Standard_E2s_v3":  101.76,
	"Standard_E4s_v3":  203.52,
	"Standard_E8s_v3":  407.04,
	"Standard_E16s_v3": 814.08,
	"Standard_E2s_v5":  96.36,
	"Standard_E4s_v5":  192.72,
	"Standard_E8s_v5":  385.44,
	// F-series (compute-optimised)
	"Standard_F2s_v2": 61.32,
	"Standard_F4s_v2": 122.64,
	"Standard_F8s_v2": 245.28,
	// NC-series (GPU)
	"Standard_NC6":   657.00,
	"Standard_NC12":  1314.00,
	"Standard_NC24":  2628.00,
}

// sqlPrices maps Azure SQL Database SKU names to approximate monthly cost (USD).
var sqlPrices = map[string]float64{
	// DTU-based
	"Basic":  4.90,
	"S0":     14.72,
	"S1":     29.44,
	"S2":     58.88,
	"S3":     117.76,
	"S4":     235.52,
	// vCore General Purpose
	"GP_Gen5_2":  183.96,
	"GP_Gen5_4":  367.92,
	"GP_Gen5_8":  735.84,
	"GP_Gen5_16": 1471.68,
	// vCore Business Critical
	"BC_Gen5_2":  551.88,
	"BC_Gen5_4":  1103.76,
	"BC_Gen5_8":  2207.52,
}

// pgPrices maps Azure PostgreSQL / MySQL SKU names to approximate monthly cost (USD, East US).
var pgPrices = map[string]float64{
	// General Purpose
	"GP_Gen5_2":         183.96,
	"GP_Gen5_4":         367.92,
	"GP_Gen5_8":         735.84,
	"GP_Standard_D2s_v3": 134.32,
	"GP_Standard_D4s_v3": 268.64,
	"GP_Standard_D8s_v3": 537.28,
	// Burstable
	"B_Standard_B1ms": 12.41,
	"B_Standard_B2s":  24.82,
}

// diskPricePerGB maps managed disk storage account types to price per GB per month (USD).
var diskPricePerGB = map[string]float64{
	"Standard_LRS":    0.04,
	"StandardSSD_LRS": 0.075,
	"Premium_LRS":     0.135,
	"UltraSSD_LRS":    0.125,
}

// lbMonthly is the approximate base monthly cost of an Azure Load Balancer (USD).
const lbMonthly = 18.25
