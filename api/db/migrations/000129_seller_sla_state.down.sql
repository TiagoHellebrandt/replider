DROP TABLE IF EXISTS seller_sla_state;

DELETE FROM platform_settings WHERE key IN (
    'sla_first_response_minutes',
    'sla_breach_pct',
    'sla_business_start_hour',
    'sla_business_end_hour',
    'sla_timezone'
);
