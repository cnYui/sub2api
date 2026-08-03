-- 余额套餐与模型分组订阅完全独立：用户支付人民币后，按固定周期获得美元 API 余额。
-- 订单快照保证套餐被下架或改价后，已支付订单仍按购买时承诺发放。

CREATE TABLE IF NOT EXISTS balance_package_plans (
    id BIGSERIAL PRIMARY KEY,
    code VARCHAR(64) NOT NULL UNIQUE,
    name VARCHAR(100) NOT NULL,
    price_cny DECIMAL(20,2) NOT NULL,
    weekly_credit_usd DECIMAL(20,8) NOT NULL,
    validity_days INTEGER NOT NULL DEFAULT 28,
    refresh_count INTEGER NOT NULL DEFAULT 4,
    refresh_interval_days INTEGER NOT NULL DEFAULT 7,
    for_sale BOOLEAN NOT NULL DEFAULT TRUE,
    sort_order INTEGER NOT NULL DEFAULT 0,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CHECK (price_cny > 0),
    CHECK (weekly_credit_usd > 0),
    CHECK (validity_days > 0),
    CHECK (refresh_count > 0),
    CHECK (refresh_interval_days > 0)
);

CREATE INDEX IF NOT EXISTS balancepackageplan_for_sale_sort_order
    ON balance_package_plans (for_sale, sort_order);

CREATE TABLE IF NOT EXISTS user_balance_packages (
    id BIGSERIAL PRIMARY KEY,
    user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    plan_id BIGINT NOT NULL,
    payment_order_id BIGINT NOT NULL UNIQUE,
    weekly_credit_usd DECIMAL(20,8) NOT NULL,
    credited_count INTEGER NOT NULL DEFAULT 0,
    refresh_count INTEGER NOT NULL,
    refresh_interval_days INTEGER NOT NULL,
    starts_at TIMESTAMPTZ NOT NULL,
    next_credit_at TIMESTAMPTZ NULL,
    expires_at TIMESTAMPTZ NOT NULL,
    status VARCHAR(20) NOT NULL DEFAULT 'active',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CHECK (weekly_credit_usd > 0),
    CHECK (credited_count >= 0),
    CHECK (refresh_count > 0),
    CHECK (refresh_interval_days > 0)
);

CREATE INDEX IF NOT EXISTS userbalancepackage_user_id_status
    ON user_balance_packages (user_id, status);
CREATE INDEX IF NOT EXISTS userbalancepackage_status_next_credit_at
    ON user_balance_packages (status, next_credit_at);

ALTER TABLE payment_orders
    ADD COLUMN IF NOT EXISTS balance_package_plan_id BIGINT NULL,
    ADD COLUMN IF NOT EXISTS balance_package_weekly_credit_usd DECIMAL(20,8) NULL,
    ADD COLUMN IF NOT EXISTS balance_package_refresh_count INTEGER NULL,
    ADD COLUMN IF NOT EXISTS balance_package_refresh_interval_days INTEGER NULL,
    ADD COLUMN IF NOT EXISTS balance_package_validity_days INTEGER NULL;

CREATE INDEX IF NOT EXISTS paymentorder_balance_package_plan_id
    ON payment_orders (balance_package_plan_id);

INSERT INTO balance_package_plans
    (code, name, price_cny, weekly_credit_usd, validity_days, refresh_count, refresh_interval_days, for_sale, sort_order)
VALUES
    ('balance-29',  '余额套餐 ¥29',  29,  76, 28, 4, 7, TRUE, 10),
    ('balance-39',  '余额套餐 ¥39',  39, 102, 28, 4, 7, TRUE, 20),
    ('balance-49',  '余额套餐 ¥49',  49, 128, 28, 4, 7, TRUE, 30),
    ('balance-59',  '余额套餐 ¥59',  59, 154, 28, 4, 7, TRUE, 40),
    ('balance-79',  '余额套餐 ¥79',  79, 206, 28, 4, 7, TRUE, 50),
    ('balance-99',  '余额套餐 ¥99',  99, 258, 28, 4, 7, TRUE, 60),
    ('balance-149', '余额套餐 ¥149', 149, 389, 28, 4, 7, TRUE, 70),
    ('balance-199', '余额套餐 ¥199', 199, 520, 28, 4, 7, TRUE, 80),
    ('balance-249', '余额套餐 ¥249', 249, 651, 28, 4, 7, TRUE, 90),
    ('balance-299', '余额套餐 ¥299', 299, 781, 28, 4, 7, TRUE, 100)
ON CONFLICT (code) DO NOTHING;
