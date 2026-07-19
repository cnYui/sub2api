-- 流量卡扣费曾沿用余额类型；以扣款 ledger 为事实来源回填审计分类。
UPDATE usage_logs AS usage_log
SET billing_type = 2
WHERE usage_log.billing_type = 0
  AND EXISTS (
      SELECT 1
      FROM traffic_credit_ledger AS ledger
      WHERE ledger.user_id = usage_log.user_id
        AND ledger.request_id = usage_log.request_id
        AND ledger.entry_type = 'deduction'
  );
