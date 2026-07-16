ALTER TABLE payment_orders ADD COLUMN IF NOT EXISTS funding_mode VARCHAR(20);
ALTER TABLE payment_orders ADD COLUMN IF NOT EXISTS balance_amount NUMERIC(20,2) NOT NULL DEFAULT 0;
ALTER TABLE payment_orders ADD COLUMN IF NOT EXISTS gateway_amount NUMERIC(20,2) NOT NULL DEFAULT 0;
ALTER TABLE payment_orders ADD COLUMN IF NOT EXISTS provider_init_status VARCHAR(20) NOT NULL DEFAULT 'NOT_STARTED';
ALTER TABLE payment_orders ADD COLUMN IF NOT EXISTS provider_init_attempted_at TIMESTAMPTZ;
ALTER TABLE payment_orders ADD COLUMN IF NOT EXISTS provider_init_lease_until TIMESTAMPTZ;
ALTER TABLE payment_orders ADD COLUMN IF NOT EXISTS payment_resolution_status VARCHAR(20) NOT NULL DEFAULT '';
ALTER TABLE payment_orders ADD COLUMN IF NOT EXISTS payment_resolution_deadline TIMESTAMPTZ;
ALTER TABLE payment_orders ADD COLUMN IF NOT EXISTS cancel_requested_at TIMESTAMPTZ;
ALTER TABLE payment_orders ADD COLUMN IF NOT EXISTS compensation_amount NUMERIC(20,2) NOT NULL DEFAULT 0;
ALTER TABLE payment_orders ADD COLUMN IF NOT EXISTS compensated_at TIMESTAMPTZ;
ALTER TABLE payment_orders ADD COLUMN IF NOT EXISTS refund_balance_amount NUMERIC(20,2) NOT NULL DEFAULT 0;
ALTER TABLE payment_orders ADD COLUMN IF NOT EXISTS refund_gateway_amount NUMERIC(20,2) NOT NULL DEFAULT 0;
ALTER TABLE payment_orders ADD COLUMN IF NOT EXISTS refund_balance_status VARCHAR(20) NOT NULL DEFAULT 'NOT_STARTED';

UPDATE payment_orders
SET funding_mode = CASE WHEN payment_type = 'balance' THEN 'balance' ELSE 'gateway' END,
    balance_amount = CASE WHEN payment_type = 'balance' THEN pay_amount ELSE 0 END,
    gateway_amount = CASE WHEN payment_type = 'balance' THEN 0 ELSE pay_amount END
WHERE funding_mode IS NULL
   OR funding_mode = ''
   OR (funding_mode IN ('gateway', 'balance') AND balance_amount = 0 AND gateway_amount = 0);

ALTER TABLE payment_orders ALTER COLUMN funding_mode SET DEFAULT 'gateway';
ALTER TABLE payment_orders ALTER COLUMN funding_mode SET NOT NULL;

CREATE INDEX IF NOT EXISTS paymentorder_funding_mode
    ON payment_orders(funding_mode);

CREATE INDEX IF NOT EXISTS paymentorder_provider_init_status_provider_init_lease_until
    ON payment_orders(provider_init_status, provider_init_lease_until);

CREATE INDEX IF NOT EXISTS paymentorder_payment_resolution_status_payment_resolution_deadline
    ON payment_orders(payment_resolution_status, payment_resolution_deadline);

CREATE TABLE IF NOT EXISTS payment_balance_holds (
    id BIGSERIAL PRIMARY KEY,
    amount NUMERIC(20,2) NOT NULL,
    status VARCHAR(20) NOT NULL DEFAULT 'RESERVED',
    expires_at TIMESTAMPTZ NOT NULL,
    captured_at TIMESTAMPTZ,
    released_at TIMESTAMPTZ,
    release_reason TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    order_id BIGINT NOT NULL,
    user_id BIGINT NOT NULL
);

DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM pg_constraint
        WHERE conname = 'payment_balance_holds_payment_orders_balance_hold'
    ) THEN
        ALTER TABLE payment_balance_holds
            ADD CONSTRAINT payment_balance_holds_payment_orders_balance_hold
            FOREIGN KEY (order_id)
            REFERENCES payment_orders(id)
            ON DELETE NO ACTION;
    END IF;

    IF NOT EXISTS (
        SELECT 1 FROM pg_constraint
        WHERE conname = 'payment_balance_holds_users_payment_balance_holds'
    ) THEN
        ALTER TABLE payment_balance_holds
            ADD CONSTRAINT payment_balance_holds_users_payment_balance_holds
            FOREIGN KEY (user_id)
            REFERENCES users(id)
            ON DELETE NO ACTION;
    END IF;
END $$;

CREATE UNIQUE INDEX IF NOT EXISTS paymentbalancehold_order_id
    ON payment_balance_holds(order_id);

CREATE INDEX IF NOT EXISTS paymentbalancehold_user_id
    ON payment_balance_holds(user_id);

CREATE INDEX IF NOT EXISTS paymentbalancehold_status
    ON payment_balance_holds(status);

CREATE INDEX IF NOT EXISTS paymentbalancehold_expires_at
    ON payment_balance_holds(expires_at);
