-- Loadout System Database Setup
-- Tento súbor obsahuje všetky tabuľky a dáta pre loadout systém

-- ==========================================
-- LOADOUT SLOTS
-- ==========================================
CREATE TABLE IF NOT EXISTS loadout_slots (
    id VARCHAR(50) PRIMARY KEY,
    name VARCHAR(100) NOT NULL,
    description TEXT,
    max_items INTEGER DEFAULT 1,
    is_required BOOLEAN DEFAULT FALSE,
    "order" INTEGER DEFAULT 0,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
);

-- Vlož základné sloty
INSERT INTO loadout_slots (id, name, description, max_items, is_required, "order") VALUES
('head', 'Hlava', 'Helma, čiapka, kukla', 1, FALSE, 1),
('face', 'Tvár', 'Maska, okuliare, respirátor', 1, FALSE, 2),
('body', 'Telo', 'Bunda, tričko, kabát', 1, FALSE, 3),
('vest', 'Vesta', 'Taktická vesta, nepriestrelná vesta', 1, FALSE, 4),
('hands', 'Ruky', 'Rukavice, chrániče', 1, FALSE, 5),
('legs', 'Nohy', 'Nohavice, chrániče', 1, FALSE, 6),
('feet', 'Nohy', 'Topánky, čižmy', 1, FALSE, 7),
('scanner', 'Scanner', 'Skenovacie zariadenie pre artefakty a predmety', 1, FALSE, 8)
ON CONFLICT (id) DO NOTHING;

-- ==========================================
-- LOADOUT ITEMS
-- ==========================================
CREATE TABLE IF NOT EXISTS loadout_items (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    deleted_at TIMESTAMP WITH TIME ZONE,
    
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    slot_id VARCHAR(50) NOT NULL REFERENCES loadout_slots(id),
    item_id UUID NOT NULL,
    item_type VARCHAR(50) NOT NULL,
    equipped_at TIMESTAMP WITH TIME ZONE NOT NULL,
    
    -- Durability systém
    durability INTEGER DEFAULT 100 CHECK (durability >= 0 AND durability <= 100),
    max_durability INTEGER DEFAULT 100 CHECK (max_durability >= 0),
    last_repaired TIMESTAMP WITH TIME ZONE,
    
    -- Odolnosť proti nepriateľom
    zombie_resistance INTEGER DEFAULT 0,
    bandit_resistance INTEGER DEFAULT 0,
    soldier_resistance INTEGER DEFAULT 0,
    monster_resistance INTEGER DEFAULT 0,
    
    -- Properties pre flexibilitu
    properties JSONB DEFAULT '{}'::jsonb,
    
    -- Indexy
    CONSTRAINT fk_loadout_items_user FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE,
    CONSTRAINT fk_loadout_items_slot FOREIGN KEY (slot_id) REFERENCES loadout_slots(id),
    CONSTRAINT unique_user_slot UNIQUE (user_id, slot_id)
);

-- ==========================================
-- GEAR CATEGORIES
-- ==========================================
CREATE TABLE IF NOT EXISTS gear_categories (
    id VARCHAR(100) PRIMARY KEY,
    name VARCHAR(100) NOT NULL,
    description TEXT,
    slot_id VARCHAR(50) NOT NULL REFERENCES loadout_slots(id),
    rarity VARCHAR(50) DEFAULT 'common',
    level INTEGER DEFAULT 1,
    
    -- Base stats
    base_durability INTEGER DEFAULT 100,
    base_zombie_resistance INTEGER DEFAULT 0,
    base_bandit_resistance INTEGER DEFAULT 0,
    base_soldier_resistance INTEGER DEFAULT 0,
    base_monster_resistance INTEGER DEFAULT 0,
    
    -- Biome specific
    biome VARCHAR(50) DEFAULT 'all',
    exclusive_to_biome BOOLEAN DEFAULT FALSE,
    
    properties JSONB DEFAULT '{}'::jsonb,
    is_active BOOLEAN DEFAULT TRUE,
    
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
);

-- Vlož základné kategórie gearu
INSERT INTO gear_categories (id, name, description, slot_id, rarity, level, base_durability, base_zombie_resistance, base_bandit_resistance, base_soldier_resistance, base_monster_resistance, biome) VALUES
-- Head gear
('military_helmet', 'Military Helmet', 'Taktická vojenská helma', 'head', 'rare', 3, 120, 15, 20, 25, 10, 'urban'),
('hazmat_hood', 'Hazmat Hood', 'Chranná kapucňa pre nebezpečné zóny', 'head', 'epic', 5, 80, 30, 5, 10, 40, 'radioactive'),
('tactical_cap', 'Tactical Cap', 'Taktická čiapka s ochranou', 'head', 'common', 1, 60, 5, 10, 5, 5, 'all'),

-- Face gear
('gas_mask', 'Gas Mask', 'Plynová maska', 'face', 'rare', 4, 100, 20, 15, 20, 25, 'chemical'),
('night_vision_goggles', 'Night Vision Goggles', 'Nočné videnie', 'face', 'epic', 6, 90, 10, 25, 30, 15, 'urban'),
('sunglasses', 'Tactical Sunglasses', 'Taktické slnečné okuliare', 'face', 'common', 1, 40, 0, 5, 0, 0, 'all'),

-- Body gear
('bulletproof_vest', 'Bulletproof Vest', 'Nepriestrelná vesta', 'body', 'epic', 7, 150, 10, 40, 50, 20, 'urban'),
('hazmat_suit', 'Hazmat Suit', 'Kompletný hazmat oblek', 'body', 'legendary', 8, 200, 50, 10, 15, 60, 'radioactive'),
('leather_jacket', 'Leather Jacket', 'Kožená bunda', 'body', 'common', 2, 80, 5, 15, 10, 5, 'all'),

-- Vest gear
('tactical_vest', 'Tactical Vest', 'Taktická vesta s kapsami', 'vest', 'rare', 4, 100, 10, 25, 30, 15, 'urban'),
('explosive_vest', 'Explosive Vest', 'Vesta s ochranou proti výbuchom', 'vest', 'epic', 6, 120, 15, 20, 35, 25, 'industrial'),

-- Hands gear
('combat_gloves', 'Combat Gloves', 'Bojové rukavice', 'hands', 'common', 2, 60, 5, 10, 15, 5, 'all'),
('hazmat_gloves', 'Hazmat Gloves', 'Chranné rukavice', 'hands', 'rare', 4, 80, 20, 5, 10, 25, 'chemical'),
('tactical_gloves', 'Tactical Gloves', 'Taktické rukavice', 'hands', 'uncommon', 3, 70, 8, 12, 18, 8, 'urban'),

-- Legs gear
('combat_pants', 'Combat Pants', 'Bojové nohavice', 'legs', 'common', 2, 80, 5, 10, 15, 5, 'all'),
('hazmat_pants', 'Hazmat Pants', 'Chranné nohavice', 'legs', 'rare', 4, 100, 20, 5, 10, 25, 'chemical'),
('tactical_pants', 'Tactical Pants', 'Taktické nohavice', 'legs', 'uncommon', 3, 90, 8, 12, 18, 8, 'urban'),

-- Feet gear
('combat_boots', 'Combat Boots', 'Bojové topánky', 'feet', 'common', 2, 100, 5, 10, 15, 5, 'all'),
('hazmat_boots', 'Hazmat Boots', 'Chranné topánky', 'feet', 'rare', 4, 120, 20, 5, 10, 25, 'chemical'),
('tactical_boots', 'Tactical Boots', 'Taktické topánky', 'feet', 'uncommon', 3, 110, 8, 12, 18, 8, 'urban'),

-- Scanner gear
('basic_scanner', 'Basic Scanner', 'Základné skenovacie zariadenie', 'scanner', 'common', 1, 80, 0, 0, 0, 0, 'all'),
('advanced_scanner', 'Advanced Scanner', 'Pokročilé skenovacie zariadenie s lepšou presnosťou', 'scanner', 'rare', 3, 100, 0, 0, 0, 0, 'all'),
('quantum_scanner', 'Quantum Scanner', 'Kvantové skenovacie zariadenie s maximálnou presnosťou', 'scanner', 'epic', 5, 120, 0, 0, 0, 0, 'all'),
('artifact_scanner', 'Artifact Scanner', 'Špecializované zariadenie pre skenovanie artefaktov', 'scanner', 'legendary', 7, 150, 0, 0, 0, 0, 'all')
ON CONFLICT (id) DO NOTHING;

-- ==========================================
-- TRIGGERS
-- ==========================================
-- Trigger pre automatické aktualizovanie updated_at
CREATE OR REPLACE FUNCTION update_loadout_updated_at_column()
RETURNS TRIGGER AS $$
BEGIN
    NEW.updated_at = CURRENT_TIMESTAMP;
    RETURN NEW;
END;
$$ language 'plpgsql';

-- Aplikuj trigger na všetky loadout tabuľky
CREATE TRIGGER update_loadout_slots_updated_at BEFORE UPDATE ON loadout_slots
    FOR EACH ROW EXECUTE FUNCTION update_loadout_updated_at_column();

CREATE TRIGGER update_loadout_items_updated_at BEFORE UPDATE ON loadout_items
    FOR EACH ROW EXECUTE FUNCTION update_loadout_updated_at_column();

CREATE TRIGGER update_gear_categories_updated_at BEFORE UPDATE ON gear_categories
    FOR EACH ROW EXECUTE FUNCTION update_loadout_updated_at_column();

-- ==========================================
-- INDEXY PRE VÝKONNOSŤ
-- ==========================================
CREATE INDEX IF NOT EXISTS idx_loadout_items_user_id ON loadout_items(user_id);
CREATE INDEX IF NOT EXISTS idx_loadout_items_slot_id ON loadout_items(slot_id);
CREATE INDEX IF NOT EXISTS idx_loadout_items_durability ON loadout_items(durability);
CREATE INDEX IF NOT EXISTS idx_gear_categories_slot_id ON gear_categories(slot_id);
CREATE INDEX IF NOT EXISTS idx_gear_categories_biome ON gear_categories(biome);
CREATE INDEX IF NOT EXISTS idx_gear_categories_rarity ON gear_categories(rarity);
CREATE INDEX IF NOT EXISTS idx_gear_categories_level ON gear_categories(level);

-- ==========================================
-- VIEWS PRE ŠTATISTIKY
-- ==========================================
CREATE OR REPLACE VIEW loadout_stats AS
SELECT 
    li.user_id,
    COUNT(*) as total_items,
    AVG(li.durability) as avg_durability,
    SUM(li.durability) as total_durability,
    SUM(li.zombie_resistance) as total_zombie_resistance,
    SUM(li.bandit_resistance) as total_bandit_resistance,
    SUM(li.soldier_resistance) as total_soldier_resistance,
    SUM(li.monster_resistance) as total_monster_resistance,
    COUNT(CASE WHEN li.durability < 50 THEN 1 END) as damaged_items,
    COUNT(CASE WHEN li.durability < 20 THEN 1 END) as critical_items
FROM loadout_items li
WHERE li.deleted_at IS NULL
GROUP BY li.user_id;

-- ==========================================
-- FUNCTIONS PRE DURABILITY
-- ==========================================
CREATE OR REPLACE FUNCTION apply_durability_damage(
    p_user_id UUID,
    p_danger_level INTEGER,
    p_biome VARCHAR(50)
) RETURNS VOID AS $$
DECLARE
    item RECORD;
    damage INTEGER;
    biome_modifier FLOAT;
    resistance_bonus INTEGER;
BEGIN
    -- Biome modifiers
    biome_modifier := CASE p_biome
        WHEN 'radioactive' THEN 1.5
        WHEN 'chemical' THEN 1.3
        WHEN 'urban' THEN 1.1
        WHEN 'forest' THEN 0.8
        WHEN 'mountain' THEN 0.9
        WHEN 'water' THEN 1.2
        WHEN 'industrial' THEN 1.0
        ELSE 1.0
    END;
    
    -- Aplikuj poškodenie na každý vybavený item
    FOR item IN 
        SELECT * FROM loadout_items 
        WHERE user_id = p_user_id AND deleted_at IS NULL
    LOOP
        -- Základné poškodenie
        damage := p_danger_level * 2;
        
        -- Aplikuj biome modifier
        damage := FLOOR(damage * biome_modifier);
        
        -- Odpočítaj odolnosť
        resistance_bonus := 0;
        IF p_biome IN ('urban', 'industrial') THEN
            resistance_bonus := item.bandit_resistance / 10;
        ELSIF p_biome = 'radioactive' THEN
            resistance_bonus := item.monster_resistance / 10;
        END IF;
        
        damage := GREATEST(0, damage - resistance_bonus);
        
        -- Aplikuj poškodenie
        UPDATE loadout_items 
        SET durability = GREATEST(0, durability - damage)
        WHERE id = item.id;
    END LOOP;
END;
$$ LANGUAGE plpgsql;
