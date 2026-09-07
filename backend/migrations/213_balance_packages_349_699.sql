-- 扩充余额套餐档位：在既有 ¥29~¥299 阶梯之上新增 ¥349~¥699 八档。
-- 每周到账沿用既有阶梯的边际口径 weekly = price×2.6 + 3.6（与 ¥249/¥299 完全一致），
-- 保证阶梯连续、不出现更高价位反而单价更差。周期 28 天、每 7 天到账、共 4 期，与其它档位一致。
-- 1% 充值手续费是全局 RechargeFeeRate，下单时统一叠加，不在套餐记录里单列。
-- ON CONFLICT DO NOTHING：生产已有前 10 档，重复应用安全；仅补齐缺失的新档。
INSERT INTO balance_package_plans
    (code, name, price_cny, weekly_credit_usd, validity_days, refresh_count, refresh_interval_days, for_sale, sort_order)
VALUES
    ('balance-349', '余额套餐 ¥349', 349,  911, 28, 4, 7, TRUE, 110),
    ('balance-399', '余额套餐 ¥399', 399, 1041, 28, 4, 7, TRUE, 120),
    ('balance-449', '余额套餐 ¥449', 449, 1171, 28, 4, 7, TRUE, 130),
    ('balance-499', '余额套餐 ¥499', 499, 1301, 28, 4, 7, TRUE, 140),
    ('balance-549', '余额套餐 ¥549', 549, 1431, 28, 4, 7, TRUE, 150),
    ('balance-599', '余额套餐 ¥599', 599, 1561, 28, 4, 7, TRUE, 160),
    ('balance-649', '余额套餐 ¥649', 649, 1691, 28, 4, 7, TRUE, 170),
    ('balance-699', '余额套餐 ¥699', 699, 1821, 28, 4, 7, TRUE, 180)
ON CONFLICT (code) DO NOTHING;
