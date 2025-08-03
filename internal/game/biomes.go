package game

// Zone templates for each biome
func GetZoneTemplate(biome string) ZoneTemplate {
	templates := map[string]ZoneTemplate{
		BiomeForest: {
			Names: []string{
				"Whispering Woods", "Dark Pine Forest", "Birch Grove", "Old Growth Forest",
				"Misty Woodland", "Silent Thicket", "Abandoned Logging Camp", "Hunter's Rest",
				"Tainted Grove", "Cursed Forest", "Dead Tree Valley", "Wolf's Den",
				"Moss-Covered Ruins", "Fungal Forest", "Rotten Swamp", "Beast Territory",
			},
			Biome:            BiomeForest,
			DangerLevel:      DangerLow,
			MinTierRequired:  0,
			AllowedArtifacts: []string{"mushroom_sample", "tree_resin", "animal_bones", "herbal_extract", "dewdrop_pearl"},
			ArtifactSpawnRates: map[string]float64{
				"mushroom_sample": 0.8,
				"tree_resin":      0.6,
				"animal_bones":    0.4,
				"herbal_extract":  0.5,
				"dewdrop_pearl":   0.3,
			},
			GearSpawnRates: map[string]float64{
				"hunting_knife": 0.4,
				"leather_boots": 0.5,
				"wooden_bow":    0.3,
			},
			EnvironmentalEffects: map[string]interface{}{
				"fog":          true,
				"wild_animals": true,
				"thick_canopy": true,
			},
		},

		BiomeMountain: {
			Names: []string{
				"Rocky Peaks", "Abandoned Mine", "Mountain Pass", "Highland Plateau",
				"Glacier Valley", "Stone Quarry", "Alpine Meadow", "Cave System",
				"Frozen Peak", "Avalanche Zone", "Mining Shaft", "Crystal Cave",
				"Cliff Face", "Ice Fields", "Boulder Field", "Echo Chamber",
			},
			Biome:            BiomeMountain,
			DangerLevel:      DangerMedium,
			MinTierRequired:  1,
			AllowedArtifacts: []string{"mineral_ore", "crystal_shard", "stone_tablet", "mountain_herb", "ice_crystal"},
			ArtifactSpawnRates: map[string]float64{
				"mineral_ore":   0.7,
				"crystal_shard": 0.5,
				"stone_tablet":  0.3,
				"mountain_herb": 0.4,
				"ice_crystal":   0.2,
			},
			GearSpawnRates: map[string]float64{
				"climbing_gear": 0.6,
				"winter_coat":   0.4,
				"pickaxe":       0.3,
			},
			EnvironmentalEffects: map[string]interface{}{
				"altitude_sickness": true,
				"cold_weather":      true,
				"unstable_terrain":  true,
			},
		},

		BiomeIndustrial: {
			Names: []string{
				"Abandoned Factory", "Chemical Plant", "Power Station", "Scrapyard",
				"Oil Refinery", "Steel Mill", "Warehouse District", "Train Depot",
				"Rust Zone", "Machinery Graveyard", "Toxic Facility", "Electrical Substation",
				"Assembly Line", "Cooling Tower", "Furnace Room", "Pipeline Junction",
			},
			Biome:            BiomeIndustrial,
			DangerLevel:      DangerHigh,
			MinTierRequired:  2,
			AllowedArtifacts: []string{"rusty_gear", "chemical_sample", "machinery_parts", "electronic_component", "toxic_waste"},
			ArtifactSpawnRates: map[string]float64{
				"rusty_gear":           0.6,
				"chemical_sample":      0.4,
				"machinery_parts":      0.5,
				"electronic_component": 0.3,
				"toxic_waste":          0.2,
			},
			GearSpawnRates: map[string]float64{
				"hard_hat":      0.5,
				"safety_gloves": 0.4,
				"welding_mask":  0.3,
			},
			EnvironmentalEffects: map[string]interface{}{
				"toxic_air":         true,
				"radiation_low":     true,
				"structural_damage": true,
			},
		},

		BiomeUrban: {
			Names: []string{
				"Ghost Town", "Subway Tunnels", "Ruined Hospital", "School Complex",
				"Shopping District", "Residential Block", "Office Building", "Police Station",
				"Empty Mall", "Parking Garage", "Abandoned Metro", "Rooftop Garden",
				"City Hall", "Library Ruins", "Apartment Complex", "Market Square",
			},
			Biome:            BiomeUrban,
			DangerLevel:      DangerMedium,
			MinTierRequired:  1,
			AllowedArtifacts: []string{"old_documents", "medical_supplies", "electronics", "urban_artifact", "pocket_radio"},
			ArtifactSpawnRates: map[string]float64{
				"old_documents":    0.5,
				"medical_supplies": 0.3,
				"electronics":      0.4,
				"urban_artifact":   0.2,
				"pocket_radio":     0.1,
			},
			GearSpawnRates: map[string]float64{
				"flashlight":    0.6,
				"first_aid_kit": 0.4,
				"crowbar":       0.3,
			},
			EnvironmentalEffects: map[string]interface{}{
				"unstable_buildings": true,
				"debris":             true,
				"darkness":           true,
			},
		},

		BiomeWater: {
			Names: []string{
				"Contaminated Lake", "Swamp Lands", "River Delta", "Flooded Quarry",
				"Toxic Pond", "Marsh Area", "Abandoned Pier", "Water Treatment Plant",
				"Algae Bloom", "Sunken Village", "Wetland Preserve", "Drainage Canal",
				"Boat Graveyard", "Muddy Banks", "Stagnant Pool", "Overflow Channel",
			},
			Biome:            BiomeWater,
			DangerLevel:      DangerMedium,
			MinTierRequired:  1,
			AllowedArtifacts: []string{"water_sample", "aquatic_plant", "filtered_water", "abyss_pearl", "algae_biomass"},
			ArtifactSpawnRates: map[string]float64{
				"water_sample":   0.6,
				"aquatic_plant":  0.4,
				"filtered_water": 0.3,
				"abyss_pearl":    0.2,
				"algae_biomass":  0.3,
			},
			GearSpawnRates: map[string]float64{
				"waders":         0.5,
				"fishing_gear":   0.4,
				"water_purifier": 0.2,
			},
			EnvironmentalEffects: map[string]interface{}{
				"contaminated_water": true,
				"slippery_terrain":   true,
				"methane_gas":        true,
			},
		},

		BiomeRadioactive: {
			Names: []string{
				"Nuclear Facility", "Reactor Core", "Contamination Zone", "Hot Zone",
				"Radiation Field", "Exclusion Zone", "Fallout Shelter", "Atomic Testing Ground",
				"Waste Storage", "Decontamination Site", "Geiger Memorial", "Uranium Mine",
				"Cooling Pool", "Control Room", "Ventilation Shaft", "Emergency Bunker",
			},
			Biome:              BiomeRadioactive,
			DangerLevel:        DangerExtreme,
			MinTierRequired:    3,
			AllowedArtifacts:   []string{"uranium_ore", "radiation_detector", "contaminated_soil", "atomic_battery", "nuclear_fuel"},
			ExclusiveArtifacts: []string{"plutonium_core", "reactor_fragment", "control_rod"},
			ArtifactSpawnRates: map[string]float64{
				"uranium_ore":        0.4,
				"radiation_detector": 0.3,
				"contaminated_soil":  0.5,
				"atomic_battery":     0.2,
				"plutonium_core":     0.1,
				"reactor_fragment":   0.05,
			},
			GearSpawnRates: map[string]float64{
				"hazmat_suit":     0.3,
				"geiger_counter":  0.4,
				"radiation_pills": 0.2,
			},
			EnvironmentalEffects: map[string]interface{}{
				"radiation_high":  true,
				"decontamination": true,
				"mutation_risk":   true,
			},
		},

		BiomeChemical: {
			Names: []string{
				"Chemical Spill Site", "Toxic Waste Dump", "Lab Complex", "Hazmat Zone",
				"Poison Gas Area", "Acid Rain Zone", "Mutant Laboratory", "Biohazard Facility",
				"Synthesis Plant", "Pesticide Factory", "Pharmaceutical Lab", "Cleanup Site",
				"Reaction Chamber", "Distillation Unit", "Storage Tank", "Ventilation System",
			},
			Biome:              BiomeChemical,
			DangerLevel:        DangerExtreme,
			MinTierRequired:    4,
			AllowedArtifacts:   []string{"chemical_compound", "lab_equipment", "toxic_sample", "hazmat_suit", "catalyst"},
			ExclusiveArtifacts: []string{"pure_toxin", "experimental_serum", "bio_weapon"},
			ArtifactSpawnRates: map[string]float64{
				"chemical_compound":  0.5,
				"lab_equipment":      0.3,
				"toxic_sample":       0.4,
				"pure_toxin":         0.05,
				"experimental_serum": 0.03,
				"bio_weapon":         0.01,
			},
			GearSpawnRates: map[string]float64{
				"gas_mask":      0.4,
				"chemical_suit": 0.3,
				"neutralizer":   0.2,
			},
			EnvironmentalEffects: map[string]interface{}{
				"toxic_gas":        true,
				"chemical_burns":   true,
				"corrosive_damage": true,
			},
		},

		// BiomeNight: {
		//Names: []string{
		//"Moonlit Grove", "Shadow Valley", "Night Hunter's Rest",
		//"Nocturnal Clearing", "Whispering Midnight Woods",
		//},
		//Biome:           BiomeNight,
		//DangerLevel:     DangerMedium,
		//MinTierRequired: 2,
		//AllowedArtifacts: []string{
		//"moon_shard", "night_bloom", "shadow_essence", "owl_feather", "midnight_berry",
		//},
		//ArtifactSpawnRates: map[string]float64{
		//"moon_shard":     0.4,
		//"night_bloom":    0.6,
		//"shadow_essence": 0.2,
		//"owl_feather":    0.5,
		//"midnight_berry": 0.3,
		//},
		//GearSpawnRates: map[string]float64{
		//"night_goggles": 0.3,
		//"silent_boots":  0.4,
		//},
		//EnvironmentalEffects: map[string]interface{}{
		//"darkness":  true,
		//"chill":     true,
		//"owl_hoots": true,
		//"fireflies": true,
		//},
		//},
	}

	if template, exists := templates[biome]; exists {
		return template
	}
	return templates[BiomeForest] // Default fallback
}
