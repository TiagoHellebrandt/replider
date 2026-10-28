-- Manual ordering inside a Kanban column (Clientes → Pipeline).
--
-- Leads had no persisted position: columns were sorted automatically
-- ("mais parados primeiro") and dragging a card within its own column did
-- nothing. board_pos is a fractional rank written ONLY by an explicit drag
-- (FunnelStageRepository.MoveLead): a card dropped between two placed
-- neighbors gets the midpoint of their positions, so a reorder is one write
-- and never renumbers the column.
--
-- NULL is meaningful: "never manually placed". Within a column, placed
-- cards sort first by board_pos ASC; the never-placed keep the automatic
-- most-stalled-first order after them. Resetting a column's manual order is
-- just clearing the column's board_pos back to NULL.
ALTER TABLE leads
    ADD COLUMN IF NOT EXISTS board_pos DOUBLE PRECISION;
