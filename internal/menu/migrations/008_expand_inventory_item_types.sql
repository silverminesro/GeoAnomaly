-- 008_expand_inventory_item_types.sql
-- Purpose: Allow 'scanner' items to be stored in inventory_items
-- Reason: Market purchase inserts item_type='scanner' and violates check_item_type

BEGIN;

-- Drop old constraint if exists
ALTER TABLE inventory_items
  DROP CONSTRAINT IF EXISTS check_item_type;

-- Recreate with extended list (add 'scanner')
-- Zachovať existujúce typy ('artifact','gear','consumable','cosmetic') a pridať 'scanner'
ALTER TABLE inventory_items
  ADD CONSTRAINT check_item_type
  CHECK (item_type::text = ANY (ARRAY['artifact','gear','consumable','cosmetic','scanner']::text[]));

COMMIT;


