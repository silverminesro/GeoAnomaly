-- Seed data pre základný scanner systém

-- 1. Základný scanner - EchoVane Mk.0
INSERT INTO scanner_catalog (
    code, name, tagline, description, base_range_m, base_fov_deg, 
    caps_json, drain_mult, allowed_modules, slot_count, slot_types, is_basic
) VALUES (
    'echovane_mk0',
    'EchoVane Mk.0',
    'Základný sektorový skener',
    'Minimalistický ručný pinger s 30° zorným klinom. Vždy ťa vedie k najbližšiemu nálezu.',
    100,
    30,
    '{"range_pct_max": 40, "fov_pct_max": 50, "server_poll_hz_max": 2.0}',
    1.0,
    '["mod_range_i", "mod_range_ii", "mod_fov_i", "mod_response_i", "mod_target_lock", "mod_nav_beacon"]',
    5,
    '["power", "range", "fov", "response", "utility"]',
    TRUE
) ON CONFLICT (code) DO NOTHING;

-- 2. Základné moduly
INSERT INTO module_catalog (code, name, type, effects_json, energy_cost, drain_mult, compatible_scanners) VALUES
('mod_range_i', 'Range Booster Mk.I', 'range', '{"range_pct": 10}', 2, 0.10, '["echovane_mk0"]'),
('mod_range_ii', 'Range Booster Mk.II', 'range', '{"range_pct": 20}', 3, 0.20, '["echovane_mk0"]'),
('mod_fov_i', 'FOV Lens Mk.I', 'fov', '{"fov_pct": 20}', 2, 0.05, '["echovane_mk0"]'),
('mod_response_i', 'Response Driver Mk.I', 'response', '{"server_poll_hz_add": 0.25}', 2, 0.15, '["echovane_mk0"]'),
('mod_target_lock', 'Target Lock Assist', 'utility', '{"lock_on_threshold_delta": -0.10, "off_sector_hint": true}', 2, 0.00, '["echovane_mk0"]'),
('mod_nav_beacon', 'Nav Beacon', 'navigation', '{"haptics_boost": true, "turn_hint_distance_m": 100}', 1, 0.00, '["echovane_mk0"]')
ON CONFLICT (code) DO NOTHING;

-- 3. Power cells
INSERT INTO power_cells_catalog (code, name, base_minutes, price_credits) VALUES
('cell_s', 'Standard Cell', 15, 120),
('cell_m', 'High-Density Cell', 25, 220),
('cell_l', 'Ultra Cell', 35, 360)
ON CONFLICT (code) DO NOTHING;

-- 4. Pridať scanner slot do loadout_slots ak neexistuje
INSERT INTO loadout_slots (id, name, description) VALUES
('scanner', 'Scanner', 'Scanner equipment slot')
ON CONFLICT (id) DO NOTHING;
