-- US-178 follow-up: server-side NPS prompt cadence. Replaces the
-- client-only FlutterSecureStorage "handled this quarter" flag so the
-- "once per quarter" gate is durable, per-account, and survives a
-- storage clear / device switch. Stamped on both submit and dismiss.
-- Write-only column: NOT mapped back by rowToAccount.
ALTER TABLE accounts ADD COLUMN IF NOT EXISTS nps_prompt_last_quarter TEXT;
