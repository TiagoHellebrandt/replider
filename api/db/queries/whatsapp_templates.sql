-- name: UpsertWhatsAppTemplate :one
INSERT INTO whatsapp_templates (
    id, account_id, channel_id, meta_template_id, name, language,
    category, meta_status, body, components, variables, rejected_reason
) VALUES (
    $1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12
)
ON CONFLICT (channel_id, name, language) DO UPDATE SET
    meta_template_id = EXCLUDED.meta_template_id,
    category         = EXCLUDED.category,
    meta_status      = EXCLUDED.meta_status,
    body             = EXCLUDED.body,
    components       = EXCLUDED.components,
    variables        = EXCLUDED.variables,
    rejected_reason  = EXCLUDED.rejected_reason,
    updated_at       = NOW()
RETURNING *;

-- name: ListWhatsAppTemplatesByChannel :many
SELECT * FROM whatsapp_templates
WHERE channel_id = $1
ORDER BY name, language;
