-- 上架 7 元购买 30 USD 的全渠道流量卡，沿用统一手续费和购买流程。
INSERT INTO traffic_packs (code, name, description, price, credit_usd, validity_days, platform, for_sale, sort_order)
VALUES (
    'traffic_30usd_7cny',
    '流量包 30 刀',
    '购买后获得全渠道可用的流量卡额度；普通余额不足时自动切换扣费。',
    7.00,
    30.0000000000,
    28,
    'all',
    TRUE,
    40
)
ON CONFLICT (code) DO UPDATE SET
    name = EXCLUDED.name,
    description = EXCLUDED.description,
    price = EXCLUDED.price,
    credit_usd = EXCLUDED.credit_usd,
    validity_days = EXCLUDED.validity_days,
    platform = EXCLUDED.platform,
    for_sale = EXCLUDED.for_sale,
    sort_order = EXCLUDED.sort_order,
    updated_at = NOW();
