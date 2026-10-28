-- The brochure a product came from, kept and reachable.
--
-- The PDF was already being archived to GCS on every import — and immediately
-- orphaned: the object was named with a fresh ULID and nothing recorded it, so
-- nothing could ever find it again. A file written and lost in the same breath
-- is not an archive, it is storage spend. This is the record that makes it a
-- source: which file, where, how many pages, and which products came out of it.
CREATE TABLE IF NOT EXISTS product_sources (
    id           CHAR(26)    PRIMARY KEY,
    account_id   CHAR(26)    NOT NULL REFERENCES accounts(id) ON DELETE CASCADE,
    -- The name the person recognises ("Incubadora Line Duo G.pdf"), not the
    -- object path — a UI that shows the storage key is showing our plumbing.
    filename     TEXT        NOT NULL,
    object_path  TEXT        NOT NULL,
    -- sha256 of the bytes. Re-importing the same brochure must reuse this row
    -- instead of piling up copies that all claim to be the origin.
    content_hash CHAR(64)    NOT NULL,
    pages        INTEGER     NOT NULL DEFAULT 0,
    byte_size    BIGINT      NOT NULL DEFAULT 0,
    created_at   TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE UNIQUE INDEX IF NOT EXISTS uniq_product_sources_account_hash
    ON product_sources (account_id, content_hash);
CREATE INDEX IF NOT EXISTS idx_product_sources_account_created
    ON product_sources (account_id, created_at DESC);

-- source_page rides on the product itself rather than only inside attributes:
-- "open the brochure at the page this came from" is a lookup, not a detail.
ALTER TABLE products ADD COLUMN IF NOT EXISTS source_id CHAR(26)
    REFERENCES product_sources(id) ON DELETE SET NULL;
ALTER TABLE products ADD COLUMN IF NOT EXISTS source_page INTEGER NOT NULL DEFAULT 0;

CREATE INDEX IF NOT EXISTS idx_products_source ON products (source_id)
    WHERE source_id IS NOT NULL;
