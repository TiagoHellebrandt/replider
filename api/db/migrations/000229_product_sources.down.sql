DROP INDEX IF EXISTS idx_products_source;
ALTER TABLE products DROP COLUMN IF EXISTS source_page;
ALTER TABLE products DROP COLUMN IF EXISTS source_id;
DROP INDEX IF EXISTS idx_product_sources_account_created;
DROP INDEX IF EXISTS uniq_product_sources_account_hash;
DROP TABLE IF EXISTS product_sources;
