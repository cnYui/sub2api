-- 记录余额套餐被同档续费的次数：续费会重置到账周期（回到第 1 期、重新计时刷新、
-- 有效期在原到期基础上顺延），renewal_count 用于前端展示"续费×N"并支持运营统计。
-- 历史行默认 0，续费时 +1。该字段不参与计费或退款金额计算。
ALTER TABLE user_balance_packages
    ADD COLUMN IF NOT EXISTS renewal_count INTEGER NOT NULL DEFAULT 0;

ALTER TABLE user_balance_packages
    DROP CONSTRAINT IF EXISTS user_balance_packages_renewal_count_check;
ALTER TABLE user_balance_packages
    ADD CONSTRAINT user_balance_packages_renewal_count_check
    CHECK (renewal_count >= 0);
