-- Add ChemVisor Mk.I scanner with enhanced capabilities
-- Migration: 004_add_chemvisor_scanner.sql

-- Insert ChemVisor Mk.I scanner
INSERT INTO scanner_catalog (
    code, name, tagline, description, base_range_m, base_fov_deg, 
    caps_json, drain_mult, allowed_modules, slot_count, slot_types, is_basic,
    max_rarity, detect_artifacts, detect_gear
) VALUES (
    'chemvisor_mk1',
    'ChemVisor Mk.I',
    'Chemický detekčný skener',
    'Pokročilý skener schopný detekovať chemické anomálie a vzácne artefakty. Ideálny pre výskum nebezpečných zón.',
    150,  -- Väčší dosah
    45,   -- Širšie zorné pole
    '{"range_pct_max": 60, "fov_pct_max": 70, "server_poll_hz_max": 3.0}',
    1.2,  -- Väčšia spotreba energie
    '["mod_range_i", "mod_range_ii", "mod_range_iii", "mod_fov_i", "mod_fov_ii", "mod_response_i", "mod_response_ii", "mod_target_lock", "mod_nav_beacon", "mod_chemical_boost"]',
    6,    -- Viac slotov
    '["power", "range", "fov", "response", "utility", "chemical"]',
    FALSE, -- Nie je základný
    'epic', -- Môže detekovať až epic rarity
    TRUE,   -- Môže detekovať artefakty
    TRUE    -- Môže detekovať gear
) ON CONFLICT (code) DO NOTHING;

-- Add ChemVisor-specific modules
INSERT INTO module_catalog (code, name, type, effects_json, energy_cost, drain_mult, compatible_scanners) VALUES
('mod_range_iii', 'Range Booster Mk.III', 'range', '{"range_pct": 30}', 4, 0.30, '["chemvisor_mk1"]'),
('mod_fov_ii', 'FOV Lens Mk.II', 'fov', '{"fov_pct": 40}', 3, 0.10, '["chemvisor_mk1"]'),
('mod_response_ii', 'Response Driver Mk.II', 'response', '{"server_poll_hz_add": 0.5}', 3, 0.25, '["chemvisor_mk1"]'),
('mod_chemical_boost', 'Chemical Enhancement', 'chemical', '{"lock_on_threshold_delta": -0.15, "off_sector_hint": true, "haptics_boost": true}', 2, 0.00, '["chemvisor_mk1"]')
ON CONFLICT (code) DO NOTHING;

-- Log the migration
INSERT INTO schema_migrations (version, applied_at) 
VALUES ('004_add_chemvisor_scanner', NOW())
ON CONFLICT (version) DO NOTHING;
