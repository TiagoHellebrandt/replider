-- Best-effort rollback: the pre-migration model of each row is not stored,
-- so everything lands on the previous plan-tier default (flash-lite).
UPDATE plans
SET ai_model = 'gemini-3.1-flash-lite'
WHERE ai_provider IN ('google', 'gemini')
  AND ai_model = 'gemini-3.7-flash';

UPDATE agents
SET model = 'gemini-3.1-flash-lite'
WHERE provider IN ('google', 'gemini')
  AND model = 'gemini-3.7-flash';

ALTER TABLE agents ALTER COLUMN model SET DEFAULT 'gemini-3.1-flash-live-preview';

DELETE FROM provider_costs
WHERE id IN ('01K0000000000000000PC0ST05', '01K0000000000000000PC0ST06');
