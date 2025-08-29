package game

// Biome constants
const (
	BiomeForest      = "forest"
	BiomeMountain    = "mountain"
	BiomeUrban       = "urban"
	BiomeWater       = "water"
	BiomeIndustrial  = "industrial"
	BiomeRadioactive = "radioactive"
	BiomeChemical    = "chemical"
	//BiomeNight       = "night"
)

// Danger level constants
const (
	DangerLow     = "low"
	DangerMedium  = "medium"
	DangerHigh    = "high"
	DangerExtreme = "extreme"
)

// Zone constants
const (
	EarthRadiusKm      = 6371.0
	MaxScanRadius      = 100.0
	MaxCollectRadius   = 50.0   // Zvýšené z 50.0 na 1000.0 metrov
	AreaScanRadius     = 7000.0 // 7km - visibility radius
	MaxSpawnRadius     = 2000.0 // 2km - maximum spawning radius (NEW)
	AreaScanCooldown   = 1
	ZoneMinExpiryHours = 6
	ZoneMaxExpiryHours = 16
	MinZoneDistance    = 250.0 // minimálna vzdialenosť medzi zónami v metroch

	//Tier 0 zóny a ich životnosť
	Tier0MinExpiryMinutes = 90  // 1:30 hod
	Tier0MaxExpiryMinutes = 120 // 2 hod
)

// NEW: Tier-based spawning distance ranges (in meters)
const (
	// Tier 0 zones - closest to player
	Tier0MinDistance = 100.0
	Tier0MaxDistance = 300.0

	// Tier 1 zones - slightly overlapping with tier 0
	Tier1MinDistance = 200.0
	Tier1MaxDistance = 450.0

	// Tier 2 zones - updated range
	Tier2MinDistance = 400.0
	Tier2MaxDistance = 800.0

	// Tier 3 zones - updated range
	Tier3MinDistance = 700.0
	Tier3MaxDistance = 1200.0

	// Tier 4 zones - furthest from player, updated range
	Tier4MinDistance = 1300.0
	Tier4MaxDistance = 2000.0
)

// NEW: Tier-based zone radius ranges (in meters)
const (
	// Tier 0 zone sizes
	Tier0MinRadius = 150.0
	Tier0MaxRadius = 200.0

	// Tier 1 zone sizes
	Tier1MinRadius = 170.0
	Tier1MaxRadius = 250.0

	// Tier 2 zone sizes
	Tier2MinRadius = 200.0
	Tier2MaxRadius = 270.0

	// Tier 3 zone sizes
	Tier3MinRadius = 250.0
	Tier3MaxRadius = 320.0

	// Tier 4 zone sizes
	Tier4MinRadius = 280.0
	Tier4MaxRadius = 360.0
)
