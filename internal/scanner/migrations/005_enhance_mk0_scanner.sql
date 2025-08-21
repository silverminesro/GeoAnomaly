-- Enhance EchoVane Mk.0 scanner for better real-time performance
-- Migration: 005_enhance_mk0_scanner.sql

-- Update EchoVane Mk.0 with enhanced capabilities for real-time scanning
UPDATE scanner_catalog SET
    description = 'Vylepšený základný sektorový skener s rýchlejšími aktualizáciami. Optimalizovaný pre hľadanie základných artefaktov s presnou navigáciou v reálnom čase.',
    base_range_m = 75,  -- Zvýšený dosah z 50m na 75m
    base_fov_deg = 35,  -- Mierne rozšírené zorné pole z 30° na 35°
    caps_json = '{"range_pct_max": 50, "fov_pct_max": 60, "server_poll_hz_max": 4.0}',  -- Zvýšený polling rate z 2.0 na 4.0 Hz
    max_rarity = 'rare',  -- Môže detekovať až rare rarity
    detect_artifacts = true,
    detect_gear = true,
    updated_at = NOW()
WHERE code = 'echovane_mk0';

-- Add new enhanced modules for Mk.0 scanner
INSERT INTO module_catalog (code, name, type, effects_json, energy_cost, drain_mult, compatible_scanners, store_price, version) VALUES
('mod_response_ii_mk0', 'Response Driver Mk.II (Mk.0)', 'response', '{"server_poll_hz_add": 0.5, "lock_on_threshold_delta": -0.05}', 3, 0.20, '["echovane_mk0"]', 450, 1),
('mod_range_iii_mk0', 'Range Booster Mk.III (Mk.0)', 'range', '{"range_pct": 25}', 4, 0.25, '["echovane_mk0"]', 600, 1),
('mod_fov_ii_mk0', 'FOV Lens Mk.II (Mk.0)', 'fov', '{"fov_pct": 30}', 3, 0.10, '["echovane_mk0"]', 400, 1),
('mod_real_time_boost', 'Real-Time Enhancement', 'utility', '{"server_poll_hz_add": 0.75, "haptics_boost": true, "turn_hint_distance_m": 50}', 2, 0.15, '["echovane_mk0"]', 350, 1),
('mod_artifact_focus', 'Artifact Focus Lens', 'utility', '{"lock_on_threshold_delta": -0.15, "off_sector_hint": true}', 2, 0.00, '["echovane_mk0"]', 300, 1)
ON CONFLICT (code) DO NOTHING;

-- Update allowed modules for Mk.0 scanner to include new modules
UPDATE scanner_catalog SET
    allowed_modules = '["mod_range_i", "mod_range_ii", "mod_range_iii_mk0", "mod_fov_i", "mod_fov_ii_mk0", "mod_response_i", "mod_response_ii_mk0", "mod_target_lock", "mod_nav_beacon", "mod_real_time_boost", "mod_artifact_focus"]',
    slot_count = 6,  -- Zvýšený počet slotov z 5 na 6
    slot_types = '["power", "range", "fov", "response", "utility", "enhancement"]',
    updated_at = NOW()
WHERE code = 'echovane_mk0';

-- Log the migration
INSERT INTO schema_migrations (version, applied_at) 
VALUES ('005_enhance_mk0_scanner', NOW())
ON CONFLICT (version) DO NOTHING;
