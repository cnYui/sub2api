INSERT INTO settings (key, value, updated_at)
VALUES
  ('affiliate_rebate_rate', '8', NOW()),
  ('affiliate_rebate_freeze_hours', '24', NOW()),
  ('affiliate_rebate_duration_days', '365', NOW()),
  ('affiliate_rebate_per_invitee_cap', '100', NOW())
ON CONFLICT (key) DO UPDATE
SET value = EXCLUDED.value,
    updated_at = NOW()
WHERE settings.value IS DISTINCT FROM EXCLUDED.value;
