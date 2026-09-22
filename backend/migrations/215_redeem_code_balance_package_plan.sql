-- 兑换码新增「余额套餐」类型：code 绑定一个 balance_package_plans 档位，
-- 用户兑换后按该档位发放余额套餐（与购买页同一套到账/刷新逻辑）。
-- 列名/可空性必须与 backend/ent/schema/redeem_code.go 保持一致（ent 不做自动迁移）。
ALTER TABLE redeem_codes
    ADD COLUMN IF NOT EXISTS balance_package_plan_id BIGINT NULL
        REFERENCES balance_package_plans(id) ON DELETE SET NULL;

CREATE INDEX IF NOT EXISTS idx_redeem_codes_balance_package_plan_id
    ON redeem_codes (balance_package_plan_id);

COMMENT ON COLUMN redeem_codes.balance_package_plan_id IS '余额套餐兑换码绑定的套餐档位（仅 type = balance_package 时有效）';
