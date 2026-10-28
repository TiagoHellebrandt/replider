DROP TABLE IF EXISTS reorder_nudges;
ALTER TABLE products
    DROP COLUMN IF EXISTS consumption_cycle_days,
    DROP COLUMN IF EXISTS shelf_life_days;
