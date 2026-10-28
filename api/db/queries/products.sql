-- name: CreateProduct :exec
INSERT INTO products (id, account_id, name, description, price_cents, currency, attributes, active, created_at, updated_at, source_id, source_page)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12);

-- name: GetProductByID :one
SELECT * FROM products WHERE id = $1 LIMIT 1;

-- name: ListProductsByAccountID :many
SELECT * FROM products WHERE account_id = $1 ORDER BY created_at DESC;

-- name: UpdateProduct :exec
UPDATE products
SET name = $2, description = $3, price_cents = $4, currency = $5, attributes = $6, active = $7, updated_at = NOW()
WHERE id = $1;

-- name: DeleteProduct :exec
DELETE FROM products WHERE id = $1;

-- name: UpsertProductSource :one
-- Re-importing the same brochure must reuse its row, not pile up copies that
-- each claim to be the origin. The no-op DO UPDATE is required: DO NOTHING
-- returns zero rows on conflict, and the caller needs the id either way.
INSERT INTO product_sources (id, account_id, filename, object_path, content_hash, pages, byte_size)
VALUES ($1, $2, $3, $4, $5, $6, $7)
ON CONFLICT (account_id, content_hash)
DO UPDATE SET filename = EXCLUDED.filename
RETURNING id;

-- name: GetProductSource :one
SELECT * FROM product_sources WHERE id = $1 AND account_id = $2 LIMIT 1;

-- name: InsertProductSourceImage :exec
-- The same artwork twice (a mark repeated across pages, a re-imported file)
-- is one row. DO NOTHING is right here, unlike the source upsert: the caller
-- does not need the id back, it needs the picture stored once.
INSERT INTO product_source_images (
    id, account_id, source_id, page, object_path, content_type,
    width, height, byte_size, content_hash
) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
ON CONFLICT (source_id, content_hash) DO NOTHING;

-- name: ListProductSourceImages :many
-- Page 0 means "the whole brochure": a product whose page is unknown still
-- deserves the catalogue's pictures rather than none.
SELECT * FROM product_source_images
WHERE source_id = $1 AND account_id = $2
  AND (sqlc.narg(page)::int IS NULL OR page = sqlc.narg(page)::int)
ORDER BY page, created_at;
