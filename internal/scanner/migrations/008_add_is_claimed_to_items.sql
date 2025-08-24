-- Migration: Add is_claimed column to artifacts and gear tables
-- Date: 2024-01-21

-- Add is_claimed column to artifacts table
ALTER TABLE artifacts 
ADD COLUMN is_claimed BOOLEAN DEFAULT FALSE;

-- Add is_claimed column to gear table
ALTER TABLE gear 
ADD COLUMN is_claimed BOOLEAN DEFAULT FALSE;

-- Create index on is_claimed for better query performance
CREATE INDEX idx_artifacts_is_claimed ON artifacts(is_claimed);
CREATE INDEX idx_gear_is_claimed ON gear(is_claimed);

-- Update existing items to have is_claimed = false
UPDATE artifacts SET is_claimed = FALSE WHERE is_claimed IS NULL;
UPDATE gear SET is_claimed = FALSE WHERE is_claimed IS NULL;
