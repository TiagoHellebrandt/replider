DROP INDEX IF EXISTS idx_ai_costs_account_operation;

ALTER TABLE plans
    DROP COLUMN IF EXISTS max_ai_drafts_per_month,
    DROP COLUMN IF EXISTS max_document_extractions_per_month;
