-- Migration: Add Puls Scanner Types
-- Description: Adds puls scanner variants (mk0, mk1, mk2) to scanner_catalog
-- Date: 2025-01-XX

-- Add Puls Mk.0 (Basic Puls Scanner)
INSERT INTO scanner_catalog (
    code,
    name,
    description,
    base_range_m,
    base_fov_deg,
    caps_json,
    max_rarity,
    detect_artifacts,
    detect_gear,
    slot_count,
    slot_types,
    allowed_modules,
    created_at,
    updated_at
) VALUES (
    'puls_mk0',
    'Puls Mk.0',
    'Základný puls scanner s vlnovým systémom. Vysiela pomalé vlny a má vysoký šum, ale je spoľahlivý pre základné artefakty.',
    60,
    45,
    '{"range_pct_max": 50, "fov_pct_max": 60, "server_poll_hz_max": 1.5, "wave_duration_ms": 3000, "echo_delay_ms": 800, "max_waves": 2, "wave_speed_ms": 30, "noise_level": 0.5}',
    'common',
    true,
    true,
    4,
    '["power", "range", "fov", "utility"]',
    '["mod_range_i", "mod_fov_i", "mod_power_i", "mod_noise_filter"]',
    NOW(),
    NOW()
);

-- Add Puls Mk.1 (Enhanced Puls Scanner)
INSERT INTO scanner_catalog (
    code,
    name,
    description,
    base_range_m,
    base_fov_deg,
    caps_json,
    max_rarity,
    detect_artifacts,
    detect_gear,
    slot_count,
    slot_types,
    allowed_modules,
    created_at,
    updated_at
) VALUES (
    'puls_mk1',
    'Puls Mk.1',
    'Vylepšený puls scanner s pokročilým vlnovým systémom. Rýchlejšie vlny, lepšie filtrovanie šumu a detekcia vzácnych artefaktov.',
    80,
    60,
    '{"range_pct_max": 70, "fov_pct_max": 80, "server_poll_hz_max": 2.5, "wave_duration_ms": 2000, "echo_delay_ms": 500, "max_waves": 3, "wave_speed_ms": 50, "noise_level": 0.3, "real_time_capable": true, "advanced_echo": true}',
    'rare',
    true,
    true,
    5,
    '["power", "range", "fov", "response", "utility"]',
    '["mod_range_i", "mod_range_ii", "mod_fov_i", "mod_fov_ii", "mod_response_i", "mod_power_i", "mod_noise_filter", "mod_echo_enhancer"]',
    NOW(),
    NOW()
);

-- Add Puls Mk.2 (Advanced Puls Scanner)
INSERT INTO scanner_catalog (
    code,
    name,
    description,
    base_range_m,
    base_fov_deg,
    caps_json,
    max_rarity,
    detect_artifacts,
    detect_gear,
    slot_count,
    slot_types,
    allowed_modules,
    created_at,
    updated_at
) VALUES (
    'puls_mk2',
    'Puls Mk.2',
    'Najpokročilejší puls scanner s maximálnym vlnovým systémom. Ultra-rýchle vlny, minimálny šum a detekcia všetkých typov artefaktov.',
    100,
    75,
    '{"range_pct_max": 90, "fov_pct_max": 100, "server_poll_hz_max": 3.0, "wave_duration_ms": 1500, "echo_delay_ms": 300, "max_waves": 4, "wave_speed_ms": 80, "noise_level": 0.1, "real_time_capable": true, "advanced_echo": true, "noise_filter": true}',
    'epic',
    true,
    true,
    6,
    '["power", "range", "fov", "response", "utility", "special"]',
    '["mod_range_i", "mod_range_ii", "mod_range_iii", "mod_fov_i", "mod_fov_ii", "mod_response_i", "mod_response_ii", "mod_power_i", "mod_power_ii", "mod_noise_filter", "mod_echo_enhancer", "mod_wave_amplifier"]',
    NOW(),
    NOW()
);

-- Add Puls-specific modules
INSERT INTO module_catalog (
    code,
    name,
    description,
    type,
    caps_json,
    created_at,
    updated_at
) VALUES 
-- Noise Filter Module
(
    'mod_noise_filter',
    'Noise Filter',
    'Filtruje falošné echo signály a znižuje šum v puls scanneri.',
    'utility',
    '{"noise_reduction": 0.3, "echo_clarity": 0.2}',
    NOW(),
    NOW()
),
-- Echo Enhancer Module
(
    'mod_echo_enhancer',
    'Echo Enhancer',
    'Zosilňuje echo signály a zlepšuje detekciu vzdialených artefaktov.',
    'response',
    '{"echo_strength": 0.4, "range_boost": 0.2, "signal_clarity": 0.3}',
    NOW(),
    NOW()
),
-- Wave Amplifier Module
(
    'mod_wave_amplifier',
    'Wave Amplifier',
    'Zosilňuje vlnové impulzy pre lepšiu penetráciu a detekciu.',
    'power',
    '{"wave_power": 0.5, "penetration": 0.3, "energy_efficiency": 0.2}',
    NOW(),
    NOW()
);

-- Add Puls Scanner to Market Items
INSERT INTO market_items (
    name,
    description,
    type,
    category,
    rarity,
    level,
    credits_price,
    essence_price,
    is_active,
    is_limited,
    stock,
    max_per_user,
    tier_required,
    level_required,
    properties,
    image_url,
    model_url,
    created_at,
    updated_at
) VALUES (
    'Puls Scanner Mk.0',
    'Základný puls scanner s vlnovým systémom. Vysiela pomalé vlny a má vysoký šum, ale je spoľahlivý pre základné artefakty. Ideálny pre začiatočníkov.',
    'scanner',
    'scanners',
    'common',
    1,
    300,
    5,
    true,
    false,
    -1,
    -1,
    0,
    1,
    '{"scanner_code": "puls_mk0", "base_range_m": 60, "base_fov_deg": 45, "wave_duration_ms": 3000, "echo_delay_ms": 800, "max_waves": 2, "wave_speed_ms": 30, "noise_level": 0.5}',
    '/images/scanners/puls_mk0.png',
    '/models/scanners/puls_mk0.glb',
    NOW(),
    NOW()
);

-- Add comments for documentation
COMMENT ON TABLE scanner_catalog IS 'Scanner catalog including new puls scanner variants';
COMMENT ON COLUMN scanner_catalog.caps_json IS 'JSON capabilities including puls-specific wave parameters';
COMMENT ON TABLE module_catalog IS 'Module catalog including puls-specific modules';
COMMENT ON TABLE market_items IS 'Market items including puls scanner variants';
