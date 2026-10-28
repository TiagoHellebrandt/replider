DROP INDEX IF EXISTS idx_leads_stage;
ALTER TABLE leads DROP COLUMN IF EXISTS stage_id;
DROP INDEX IF EXISTS idx_funnel_stages_funnel_name;
DROP INDEX IF EXISTS idx_funnel_stages_funnel;
DROP TABLE IF EXISTS funnel_stages;
