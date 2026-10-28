UPDATE channels SET is_active = true, updated_at = NOW()
WHERE id LIKE '01DEMO_CHANNEL_%';
