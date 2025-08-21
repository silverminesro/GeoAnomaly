-- Add scanner detection capabilities to scanner_catalog table
-- Migration: 003_add_scanner_capabilities.sql

-- Add new columns to scanner_catalog table
ALTER TABLE scanner_catalog 
ADD COLUMN IF NOT EXISTS max_rarity VARCHAR(20) DEFAULT 'common',
ADD COLUMN IF NOT EXISTS detect_artifacts BOOLEAN DEFAULT TRUE,
ADD COLUMN IF NOT EXISTS detect_gear BOOLEAN DEFAULT TRUE;

-- Update existing EchoVane Mk.0 scanner with proper capabilities
UPDATE scanner_catalog 
SET 
    max_rarity = 'rare',
    detect_artifacts = TRUE,
    detect_gear = TRUE
WHERE code = 'echovane_mk0';

-- Add comments for documentation
COMMENT ON COLUMN scanner_catalog.max_rarity IS 'Maximum rarity level this scanner can detect (common, rare, epic, legendary)';
COMMENT ON COLUMN scanner_catalog.detect_artifacts IS 'Whether this scanner can detect artifacts';
COMMENT ON COLUMN scanner_catalog.detect_gear IS 'Whether this scanner can detect gear items';

-- Create index for efficient filtering
CREATE INDEX IF NOT EXISTS idx_scanner_catalog_capabilities 
ON scanner_catalog (max_rarity, detect_artifacts, detect_gear);

-- Log the migration
INSERT INTO schema_migrations (version, applied_at) 
VALUES ('003_add_scanner_capabilities', NOW())
ON CONFLICT (version) DO NOTHING;
