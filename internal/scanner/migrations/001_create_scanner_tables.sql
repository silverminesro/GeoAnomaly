-- Scanner System Migration
-- Vytvorenie tabuliek pre scanner systém

-- 1. Scanner katalóg
CREATE TABLE IF NOT EXISTS scanner_catalog (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    code VARCHAR(50) UNIQUE NOT NULL,
    name VARCHAR(100) NOT NULL,
    tagline VARCHAR(200),
    description TEXT,
    base_range_m INTEGER NOT NULL,
    base_fov_deg INTEGER NOT NULL,
    caps_json JSONB NOT NULL DEFAULT '{}',
    drain_mult DECIMAL(3,2) DEFAULT 1.0,
    allowed_modules JSONB NOT NULL DEFAULT '[]',
    slot_count INTEGER NOT NULL DEFAULT 5,
    slot_types JSONB NOT NULL DEFAULT '["power", "range", "fov", "response", "utility"]',
    is_basic BOOLEAN DEFAULT FALSE,
    version INTEGER DEFAULT 1,
    effective_from TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

-- 2. Module katalóg
CREATE TABLE IF NOT EXISTS module_catalog (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    code VARCHAR(50) UNIQUE NOT NULL,
    name VARCHAR(100) NOT NULL,
    type VARCHAR(50) NOT NULL,
    effects_json JSONB NOT NULL DEFAULT '{}',
    energy_cost INTEGER NOT NULL DEFAULT 0,
    drain_mult DECIMAL(3,2) DEFAULT 0.0,
    compatible_scanners JSONB NOT NULL DEFAULT '[]',
    craft_json JSONB,
    store_price INTEGER DEFAULT 0,
    version INTEGER DEFAULT 1,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

-- 3. Power cells katalóg
CREATE TABLE IF NOT EXISTS power_cells_catalog (
    code VARCHAR(50) PRIMARY KEY,
    name VARCHAR(100) NOT NULL,
    base_minutes INTEGER NOT NULL,
    craft_json JSONB,
    price_credits INTEGER DEFAULT 0,
    version INTEGER DEFAULT 1,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

-- 4. Hráčove scanner inštancie
CREATE TABLE IF NOT EXISTS scanner_instances (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    owner_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    scanner_code VARCHAR(50) NOT NULL,
    energy_cap INTEGER,
    power_cell_code VARCHAR(50),
    power_cell_started_at TIMESTAMP,
    power_cell_minutes_left INTEGER,
    is_active BOOLEAN DEFAULT TRUE,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

-- 5. Inštalované moduly
CREATE TABLE IF NOT EXISTS scanner_modules_installed (
    instance_id UUID NOT NULL REFERENCES scanner_instances(id) ON DELETE CASCADE,
    slot_index INTEGER NOT NULL,
    module_code VARCHAR(50) NOT NULL,
    installed_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY (instance_id, slot_index)
);

-- Indexy
CREATE INDEX IF NOT EXISTS idx_scanner_catalog_code ON scanner_catalog(code);
CREATE INDEX IF NOT EXISTS idx_scanner_catalog_basic ON scanner_catalog(is_basic);
CREATE INDEX IF NOT EXISTS idx_module_catalog_type ON module_catalog(type);
CREATE INDEX IF NOT EXISTS idx_scanner_instances_owner ON scanner_instances(owner_id);
CREATE INDEX IF NOT EXISTS idx_scanner_instances_active ON scanner_instances(is_active);

-- Triggers pre updated_at
CREATE OR REPLACE FUNCTION update_scanner_updated_at_column()
RETURNS TRIGGER AS $$
BEGIN
    NEW.updated_at = CURRENT_TIMESTAMP;
    RETURN NEW;
END;
$$ language 'plpgsql';

CREATE TRIGGER update_scanner_catalog_updated_at 
    BEFORE UPDATE ON scanner_catalog 
    FOR EACH ROW EXECUTE FUNCTION update_scanner_updated_at_column();

CREATE TRIGGER update_module_catalog_updated_at 
    BEFORE UPDATE ON module_catalog 
    FOR EACH ROW EXECUTE FUNCTION update_scanner_updated_at_column();

CREATE TRIGGER update_scanner_instances_updated_at 
    BEFORE UPDATE ON scanner_instances 
    FOR EACH ROW EXECUTE FUNCTION update_scanner_updated_at_column();
