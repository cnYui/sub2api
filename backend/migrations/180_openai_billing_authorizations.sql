-- 将流量卡专用 reservation 原地演进为通用 authorization，保留历史记录和主键。
DO $$
BEGIN
    IF to_regclass('public.billing_authorizations') IS NULL
       AND to_regclass('public.traffic_credit_reservations') IS NOT NULL THEN
        ALTER TABLE traffic_credit_reservations RENAME TO billing_authorizations;
    END IF;

    IF to_regclass('public.billing_authorization_traffic_credit_items') IS NULL
       AND to_regclass('public.traffic_credit_reservation_items') IS NOT NULL THEN
        ALTER TABLE traffic_credit_reservation_items RENAME TO billing_authorization_traffic_credit_items;
    END IF;
END $$;

ALTER TABLE billing_authorizations
    ADD COLUMN IF NOT EXISTS billing_source VARCHAR(20) NOT NULL DEFAULT 'traffic_credit',
    ADD COLUMN IF NOT EXISTS subscription_id BIGINT,
    ADD COLUMN IF NOT EXISTS entitlement_period_id BIGINT,
    ADD COLUMN IF NOT EXISTS estimate_breakdown JSONB NOT NULL DEFAULT '{}'::jsonb,
    ADD COLUMN IF NOT EXISTS estimator_version VARCHAR(64) NOT NULL DEFAULT 'legacy-traffic-credit-v1',
    ADD COLUMN IF NOT EXISTS suspense_usd DECIMAL(20,10) NOT NULL DEFAULT 0,
    ADD COLUMN IF NOT EXISTS dispatched_at TIMESTAMPTZ,
    ADD COLUMN IF NOT EXISTS settled_at TIMESTAMPTZ,
    ADD COLUMN IF NOT EXISTS reconciled_at TIMESTAMPTZ;

DO $$
BEGIN
    ALTER TABLE billing_authorizations
        DROP CONSTRAINT IF EXISTS traffic_credit_reservations_status_check;
    ALTER TABLE billing_authorizations
        ADD CONSTRAINT billing_authorizations_status_check
        CHECK (status IN ('reserved', 'dispatched', 'unknown', 'settled', 'released', 'debt', 'suspense'));
EXCEPTION WHEN duplicate_object THEN NULL;
END $$;

DO $$
BEGIN
    ALTER TABLE billing_authorizations
        ADD CONSTRAINT billing_authorizations_source_check
        CHECK (billing_source IN ('subscription', 'traffic_credit'));
EXCEPTION WHEN duplicate_object THEN NULL;
END $$;

DO $$
BEGIN
    IF EXISTS (
        SELECT 1
        FROM information_schema.columns
        WHERE table_schema = 'public' AND table_name = 'usage_facts' AND column_name = 'reservation_id'
    ) AND NOT EXISTS (
        SELECT 1
        FROM information_schema.columns
        WHERE table_schema = 'public' AND table_name = 'usage_facts' AND column_name = 'authorization_id'
    ) THEN
        ALTER TABLE usage_facts RENAME COLUMN reservation_id TO authorization_id;
    END IF;

    IF to_regclass('public.idx_usage_facts_reservation_id') IS NOT NULL
       AND to_regclass('public.idx_usage_facts_authorization_id') IS NULL THEN
        ALTER INDEX idx_usage_facts_reservation_id RENAME TO idx_usage_facts_authorization_id;
    END IF;
END $$;

CREATE INDEX IF NOT EXISTS idx_usage_facts_authorization_id
    ON usage_facts (authorization_id)
    WHERE authorization_id IS NOT NULL;

CREATE INDEX IF NOT EXISTS idx_billing_authorizations_subscription_active
    ON billing_authorizations (subscription_id, entitlement_period_id, id)
    WHERE billing_source = 'subscription' AND status IN ('reserved', 'dispatched', 'unknown');

CREATE INDEX IF NOT EXISTS idx_billing_authorizations_unknown_reconcile
    ON billing_authorizations (updated_at, id)
    WHERE status = 'unknown';
