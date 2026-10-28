-- The pictures inside the brochure.
--
-- "Manda uma foto do equipamento" is a question a distributor already has the
-- answer to: it is on page 4 of the PDF they handed us. The file was kept
-- (000229); this keeps what is INSIDE it, so a picture can be attached to an
-- offer without anyone hunting through a forty-page catalogue.
--
-- Keyed to the SOURCE, not to the product: one page can describe several
-- products, and a picture belongs to the page it was printed on. Products
-- reach their images through source_id + source_page.
CREATE TABLE IF NOT EXISTS product_source_images (
    id           CHAR(26)    PRIMARY KEY,
    account_id   CHAR(26)    NOT NULL REFERENCES accounts(id) ON DELETE CASCADE,
    source_id    CHAR(26)    NOT NULL REFERENCES product_sources(id) ON DELETE CASCADE,
    page         INTEGER     NOT NULL,
    object_path  TEXT        NOT NULL,
    content_type TEXT        NOT NULL,
    width        INTEGER     NOT NULL DEFAULT 0,
    height       INTEGER     NOT NULL DEFAULT 0,
    byte_size    BIGINT      NOT NULL DEFAULT 0,
    -- sha256 of the bytes. The same brand mark repeats across pages and the
    -- same brochure gets re-imported; neither should multiply rows.
    content_hash CHAR(64)    NOT NULL,
    created_at   TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE UNIQUE INDEX IF NOT EXISTS uniq_product_source_images_hash
    ON product_source_images (source_id, content_hash);
CREATE INDEX IF NOT EXISTS idx_product_source_images_page
    ON product_source_images (source_id, page);
