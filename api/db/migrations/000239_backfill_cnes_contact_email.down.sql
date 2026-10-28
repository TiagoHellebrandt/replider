-- No-op on purpose: this migration copied a public registry address onto
-- contacts that had none, and there is no record of which rows were blank
-- before. Clearing them on a rollback would delete addresses a person may
-- have typed since.
SELECT 1;
