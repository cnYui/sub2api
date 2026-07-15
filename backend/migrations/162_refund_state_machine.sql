ALTER TABLE payment_orders ADD COLUMN IF NOT EXISTS subscription_id BIGINT;
ALTER TABLE payment_orders ADD COLUMN IF NOT EXISTS refund_request_id VARCHAR(128);
ALTER TABLE payment_orders ADD COLUMN IF NOT EXISTS refund_gateway_status VARCHAR(20) NOT NULL DEFAULT 'NOT_STARTED';
ALTER TABLE payment_orders ADD COLUMN IF NOT EXISTS refund_entitlement_status VARCHAR(20) NOT NULL DEFAULT 'NOT_STARTED';
ALTER TABLE payment_orders ADD COLUMN IF NOT EXISTS refund_provider_ref VARCHAR(128);

DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1
        FROM pg_constraint
        WHERE conname = 'payment_orders_subscription_id_fkey'
    ) THEN
        ALTER TABLE payment_orders
            ADD CONSTRAINT payment_orders_subscription_id_fkey
            FOREIGN KEY (subscription_id)
            REFERENCES user_subscriptions(id)
            ON DELETE SET NULL;
    END IF;
END $$;

CREATE INDEX IF NOT EXISTS idx_payment_orders_subscription_id
    ON payment_orders(subscription_id);

WITH candidates AS (
    SELECT
        po.id AS order_id,
        MIN(us.id) AS subscription_id,
        COUNT(*) AS matched
    FROM payment_orders po
    JOIN user_subscriptions us
      ON us.user_id = po.user_id
     AND us.group_id = po.subscription_group_id
     AND us.notes ~ ('(^|\n)payment order ' || po.id::text || '($|\n)')
    WHERE po.order_type = 'subscription'
      AND po.subscription_id IS NULL
    GROUP BY po.id
)
UPDATE payment_orders po
SET subscription_id = candidates.subscription_id
FROM candidates
WHERE po.id = candidates.order_id
  AND candidates.matched = 1;
