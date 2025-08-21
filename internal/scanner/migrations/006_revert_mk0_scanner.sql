-- Revert EchoVane Mk.0 scanner to original values
-- Migration: 006_revert_mk0_scanner.sql

-- Revert EchoVane Mk.0 to original capabilities
UPDATE scanner_catalog SET
    description = 'Minimalistický ručný pinger s 30° zorným klinom. Vždy ťa vedie k najbližšiemu nálezu.',
    base_range_m = 50,  -- Vrátené na pôvodných 50m
    base_fov_deg = 30,  -- Vrátené na pôvodných 30°
    caps_json = '{"range_pct_max": 40, "fov_pct_max": 50, "server_poll_hz_max": 2.0}',  -- Vrátené na pôvodné hodnoty
    max_rarity = 'common',  -- Vrátené na pôvodné common
    detect_artifacts = true,
    detect_gear = true,
    slot_count = 5,  -- Vrátené na pôvodných 5 slotov
    slot_types = '["power", "range", "fov", "response", "utility"]',  -- Vrátené na pôvodné typy
    allowed_modules = '["mod_range_i", "mod_range_ii", "mod_fov_i", "mod_response_i", "mod_target_lock", "mod_nav_beacon"]',  -- Vrátené na pôvodné moduly
    updated_at = NOW()
WHERE code = 'echovane_mk0';

-- Remove enhanced modules that were added
DELETE FROM module_catalog WHERE code IN (
    'mod_response_ii_mk0',
    'mod_range_iii_mk0', 
    'mod_fov_ii_mk0',
    'mod_real_time_boost',
    'mod_artifact_focus'
);

-- Log the migration
INSERT INTO schema_migrations (version, applied_at) 
VALUES ('006_revert_mk0_scanner', NOW())
ON CONFLICT (version) DO NOTHING;
