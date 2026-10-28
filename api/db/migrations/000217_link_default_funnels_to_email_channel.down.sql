-- Data backfill only; intentionally a no-op. Reversing would unbind every
-- default funnel from its email channel and re-break approvals
-- (LEAD_FUNNEL_NO_CHANNEL) — the pre-migration state was the bug, not a
-- schema to restore. FunnelChannelLinker keeps new accounts linked at
-- channel creation, so nothing depends on the rows this touched.
SELECT 1;
