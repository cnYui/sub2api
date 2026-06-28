-- 修正 79 元订阅套餐基础价，避免用户购买页在 1% 手续费上二次加费。

UPDATE subscription_plans AS sp
SET
    price = 79.00,
    updated_at = NOW()
FROM groups AS g
WHERE sp.group_id = g.id
  AND g.name = 'codex-pool-69-usd'
  AND g.deleted_at IS NULL
  AND (
      sp.name = '79 元订阅池'
      OR sp.product_name = '79 元订阅池'
  )
  AND sp.price = 79.79;
