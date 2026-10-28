-- name: CreateCalendar :exec
INSERT INTO calendars (
    id, agent_id, name, available_days, time_slots,
    slot_duration_minutes, google_calendar_id, is_active, created_at, updated_at
) VALUES (
    $1, $2, $3, $4, $5, $6, $7, $8, $9, $10
);

-- name: GetCalendarByID :one
SELECT * FROM calendars WHERE id = $1 LIMIT 1;

-- name: ListCalendarsByAgentID :many
SELECT * FROM calendars WHERE agent_id = $1 ORDER BY created_at ASC;

-- name: UpdateCalendar :exec
UPDATE calendars
SET name = $2,
    available_days = $3,
    time_slots = $4,
    slot_duration_minutes = $5,
    google_calendar_id = $6,
    is_active = $7,
    updated_at = $8
WHERE id = $1;

-- name: DeleteCalendar :exec
DELETE FROM calendars WHERE id = $1;
