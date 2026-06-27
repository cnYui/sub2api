# 10 USD GPT 流量卡批量赠送设计与计划

## 背景

- 用户要求：给当前所有用户账户发放一张“10 美金，也就是 3 元”的 GPT 流量卡，并确认能否直接写数据库，以及公网旧版本是否会更新。
- 当前公网链路落到 Docker 容器 `sub2api`，端口映射为 `127.0.0.1:18080 -> 8080`，数据库容器为 `sub2api-postgres`。
- 生产库已存在流量卡相关表：`traffic_packs`、`user_traffic_credits`、`traffic_credit_ledger`。
- 目标流量卡为 `traffic_packs.code = 'gpt_traffic_10usd_3cny'`，当前生产库中 `id=2`，`credit_usd=10.0000000000`，`price=3.00`，有效期 365 天，平台为 `openai`。
- 当前用户表共 47 条，其中 `deleted_at IS NULL AND status='active'` 为 44 条。3 条软删除用户不应作为“当前用户”发放对象。
- 当前生产库尚无流量卡发放/扣减记录：`user_traffic_credits` 为 0，`traffic_credit_ledger` 为 0。

## 关键结论

- 不应直接执行 `UPDATE users SET balance = balance + 3`。流量卡不是用户余额，OpenAI 请求的流量卡扣费逻辑读取的是 `user_traffic_credits`，扣减时写 `traffic_credit_ledger`。
- 可以直接写数据库，但需要一次事务同时写入订单、额度、流量卡流水和支付审计日志，保持与现有业务模型一致。
- 如果公网旧版本连接同一个生产库，并且运行代码已经包含流量卡扣费逻辑，它会立即读取并消费这些额度；当前公网容器的数据库已经完成相关迁移，且代码存在流量卡扣费表依赖。从运行态看，直接写生产库会被公网 Sub2API 使用。
- 如果未来切换到不包含流量卡逻辑的更旧镜像，这些数据仍在库里，但旧代码不会消费它们。

## 发放范围

推荐范围：

- `users.deleted_at IS NULL`
- `users.status = 'active'`

这会覆盖 44 个当前有效用户，包含管理员账号。原因是用户说“当前所有用户账户”，而软删除用户不是当前账户；当前未删除用户全部为 active。

## 方案对比

### 方案 A：逐个模拟真实支付订单后走后端 fulfillment

- 优点：最贴近正常支付完成路径。
- 缺点：需要构造 44 个支付订单并调用内部服务，且赠送并不是实际支付，容易触发支付通道、通知、返利等非目标副作用。
- 结论：不推荐。

### 方案 B：直接 SQL 事务写业务事实表

- 写入 `payment_orders`：每个用户一条 `traffic_pack` 类型的完成订单，`payment_type='manual_grant'`，`amount=0`，`pay_amount=0`，不虚增收入、不占用真实支付通道额度。
- 写入 `user_traffic_credits`：每个用户一条 10 USD OpenAI 流量卡额度，`remaining_usd=initial_usd=10`，`expires_at=credited_at + 365 days`。
- 写入 `traffic_credit_ledger`：每个额度一条 `purchase` 流水，金额 10 USD。
- 写入 `payment_audit_logs`：每个订单一条 `TRAFFIC_PACK_MANUAL_GRANT` 审计日志，记录批次号和卡包信息。
- 使用固定批次号和按用户生成的 `out_trade_no` 做幂等防重。
- 结论：推荐。

### 方案 C：只写 `user_traffic_credits`

- 优点：最少写入。
- 缺点：`user_traffic_credits.order_id` 是必填且唯一外键，必须有对应 `payment_orders`；缺少流水和审计也不利于排查。
- 结论：不可取。

## 推荐 SQL 计划

批次号：

```text
manual_grant_gpt_10usd_20260626
```

执行前只读确认：

```sql
SELECT id, code, price, credit_usd, validity_days, platform
FROM traffic_packs
WHERE code = 'gpt_traffic_10usd_3cny';

SELECT COUNT(*)
FROM users
WHERE deleted_at IS NULL AND status = 'active';
```

执行事务：

```sql
BEGIN;

WITH params AS (
    SELECT
        'manual_grant_gpt_10usd_20260626'::text AS batch_id,
        NOW() AS ts
),
pack AS (
    SELECT id, code, name, price, credit_usd, validity_days, platform
    FROM traffic_packs
    WHERE code = 'gpt_traffic_10usd_3cny'
    FOR SHARE
),
eligible_users AS (
    SELECT id, email, username, notes
    FROM users
    WHERE deleted_at IS NULL
      AND status = 'active'
),
inserted_orders AS (
    INSERT INTO payment_orders (
        user_id,
        user_email,
        user_name,
        user_notes,
        amount,
        pay_amount,
        fee_rate,
        recharge_code,
        out_trade_no,
        payment_type,
        payment_trade_no,
        order_type,
        provider_snapshot,
        status,
        expires_at,
        paid_at,
        completed_at,
        client_ip,
        src_host,
        src_url,
        created_at,
        updated_at
    )
    SELECT
        u.id,
        u.email,
        COALESCE(u.username, ''),
        NULLIF(u.notes, ''),
        0.00,
        0.00,
        0,
        'GIFT-GPT10-20260626-' || u.id::text,
        'sub2_gift_gpt10_20260626_u' || u.id::text,
        'manual_grant',
        params.batch_id || '_u' || u.id::text,
        'traffic_pack',
        jsonb_build_object(
            'provider_key', 'manual_grant',
            'grant_batch', params.batch_id,
            'grant_reason', 'bulk grant 10 USD GPT traffic pack to current active users',
            'traffic_pack_id', pack.id,
            'traffic_pack_code', pack.code,
            'traffic_pack_name', pack.name,
            'traffic_pack_price', pack.price,
            'traffic_pack_credit_usd', pack.credit_usd,
            'traffic_pack_validity_days', pack.validity_days,
            'traffic_pack_platform', pack.platform
        ),
        'COMPLETED',
        params.ts,
        params.ts,
        params.ts,
        '127.0.0.1',
        'manual.local',
        '/manual-grants/traffic-pack/20260626',
        params.ts,
        params.ts
    FROM eligible_users u
    CROSS JOIN pack
    CROSS JOIN params
    WHERE NOT EXISTS (
        SELECT 1
        FROM payment_orders po
        WHERE po.out_trade_no = 'sub2_gift_gpt10_20260626_u' || u.id::text
    )
    RETURNING id, user_id
),
inserted_credits AS (
    INSERT INTO user_traffic_credits (
        user_id,
        order_id,
        pack_id,
        platform,
        initial_usd,
        remaining_usd,
        credited_at,
        expires_at,
        created_at,
        updated_at
    )
    SELECT
        o.user_id,
        o.id,
        pack.id,
        pack.platform,
        pack.credit_usd,
        pack.credit_usd,
        params.ts,
        params.ts + make_interval(days => pack.validity_days),
        params.ts,
        params.ts
    FROM inserted_orders o
    CROSS JOIN pack
    CROSS JOIN params
    ON CONFLICT (order_id) DO NOTHING
    RETURNING id, user_id, order_id, remaining_usd
),
inserted_ledger AS (
    INSERT INTO traffic_credit_ledger (
        user_id,
        credit_id,
        order_id,
        request_id,
        entry_type,
        amount_usd,
        balance_after_usd,
        created_at
    )
    SELECT
        c.user_id,
        c.id,
        c.order_id,
        '',
        'purchase',
        c.remaining_usd,
        c.remaining_usd,
        params.ts
    FROM inserted_credits c
    CROSS JOIN params
    RETURNING id
),
inserted_audit_logs AS (
    INSERT INTO payment_audit_logs (
        order_id,
        action,
        detail,
        operator,
        created_at
    )
    SELECT
        o.id::text,
        'TRAFFIC_PACK_MANUAL_GRANT',
        jsonb_build_object(
            'batch_id', params.batch_id,
            'credit_usd', pack.credit_usd,
            'pack_code', pack.code,
            'reason', 'bulk grant to current active users'
        )::text,
        'system',
        params.ts
    FROM inserted_orders o
    CROSS JOIN pack
    CROSS JOIN params
    RETURNING id
)
SELECT
    (SELECT COUNT(*) FROM inserted_orders) AS inserted_orders,
    (SELECT COUNT(*) FROM inserted_credits) AS inserted_credits,
    (SELECT COUNT(*) FROM inserted_ledger) AS inserted_ledger,
    (SELECT COUNT(*) FROM inserted_audit_logs) AS inserted_audit_logs;

COMMIT;
```

预期首次执行结果：

- `inserted_orders = 44`
- `inserted_credits = 44`
- `inserted_ledger = 44`
- `inserted_audit_logs = 44`

如重复执行，同一批次应为 0 新增，避免重复发放。

## 验证计划

执行后验证：

```sql
SELECT COUNT(*) AS credits,
       COALESCE(SUM(initial_usd), 0) AS initial_usd,
       COALESCE(SUM(remaining_usd), 0) AS remaining_usd
FROM user_traffic_credits utc
JOIN payment_orders po ON po.id = utc.order_id
WHERE po.out_trade_no LIKE 'sub2_gift_gpt10_20260626_u%';

SELECT COUNT(*) AS purchase_ledgers,
       COALESCE(SUM(amount_usd), 0) AS amount_usd
FROM traffic_credit_ledger tcl
JOIN payment_orders po ON po.id = tcl.order_id
WHERE po.out_trade_no LIKE 'sub2_gift_gpt10_20260626_u%'
  AND tcl.entry_type = 'purchase';

SELECT COUNT(*) AS manual_orders,
       COALESCE(SUM(amount), 0) AS amount,
       COALESCE(SUM(pay_amount), 0) AS pay_amount
FROM payment_orders
WHERE out_trade_no LIKE 'sub2_gift_gpt10_20260626_u%';
```

预期：

- 44 张额度。
- `initial_usd = 440`，`remaining_usd = 440`。
- 44 条 purchase 流水，`amount_usd = 440`。
- 44 条手工订单，`amount = 0`，`pay_amount = 0`。

## 风险与回滚

- 风险：若以后部署极旧版本，不会消费 `user_traffic_credits`；当前公网容器和库结构已支持。
- 风险：订单数会增加 44 条，但支付收入不增加，因为 `pay_amount=0`。
- 回滚：按 `out_trade_no LIKE 'sub2_gift_gpt10_20260626_u%'` 定位本批次，先删除 `traffic_credit_ledger`，再删除 `user_traffic_credits`，最后删除 `payment_audit_logs` 和 `payment_orders`。若已有用户消费本批流量，不应直接回滚，应先计算已消费额度并由业务决策处理。
