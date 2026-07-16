CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_usage_facts_dashboard_user_completed
    ON usage_facts (user_id, completed_at)
    INCLUDE (request_id, api_key_id)
    WHERE billing_status IN ('pending', 'settling', 'settled', 'debt');
