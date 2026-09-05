# 生产 VPS 远程操作手册（SSH → 容器 → 数据库）

- 时间：2026-09-05 17:31（+09）
- 性质：**可复用的操作方法**，不是一次性执行记录
- 首次成型于 `users.id=565` 周额度提前刷新，执行记录见 `20260905-172724-*`

> 本仓库是公开仓库。下文一律用 `${变量名}` 占位，真值只在 `deploy/ops.env`（已 gitignore）。

---

## 1. 先解决「连不上」

### 密钥不叫 `id_*`

部署密钥**按机器命名**，文件名记在 `deploy/ops.env` 的 `OPS_SSH_KEY_FILE`。

**不要用 `ls ~/.ssh/*.pem ~/.ssh/id_*` 这类过滤去找私钥**——会漏掉它，
而默认的 `id_ed25519` 确实会被服务器拒绝：

```
debug1: Offering public key: ~/.ssh/id_ed25519 ED25519 SHA256:...
debug1: Authentications that can continue: publickey
Permission denied (publickey).
```

于是很容易得出「本机没有 SSH 访问权限」这个**看起来有依据的错误结论**。
本次会话就因此白白卡住了三件事。

**正确做法：`ls ~/.ssh/` 全量看一遍。**

### 连接

```bash
set -a && . deploy/ops.env && set +a
ssh -i "${OPS_SSH_KEY_FILE}" -o BatchMode=yes root@"${OPS_VPS_HOST}" 'hostname; docker ps --format "{{.Names}}\t{{.Status}}"'
```

`BatchMode=yes` 让认证失败立刻退出而不是挂着等输入。诊断认证问题加 `-v`，
看 `Offering public key` 与 `Authentications that can continue` 两行即可定位。

生产三个容器：

```
sub2api            应用
sub2api-postgres   数据库
sub2api-redis      缓存
```

---

## 2. 执行数据库命令

### 只读查询

```bash
ssh -i "${OPS_SSH_KEY_FILE}" root@"${OPS_VPS_HOST}" \
  'docker exec sub2api-postgres psql -U sub2api -d sub2api -c "SELECT ..."'
```

常用 psql 开关：

| 开关 | 用途 |
| --- | --- |
| `-x` | 纵向展开输出，宽表必用 |
| `-tAc` | 无表头无对齐，适合取单值喂给 shell |
| `-c` | 可重复，多条语句一次执行 |
| `\d 表名` | 看表结构与约束（配合 `-c` 用） |

### 嵌套引号是主要障碍

SQL 里的单引号要穿过 shell → ssh → docker exec 三层。字面量用
`'"'"'` 转义可以工作但极易写错：

```bash
# 能用但难读，一旦 SQL 变长就失控
... -c "SELECT * FROM t WHERE k='"'"'v'"'"';"
```

**写复杂 SQL 一律改走 stdin，彻底绕开引号问题：**

```bash
ssh -i "${OPS_SSH_KEY_FILE}" root@"${OPS_VPS_HOST}" \
  'docker exec -i sub2api-postgres psql -U sub2api -d sub2api -v ON_ERROR_STOP=1' < local.sql
```

注意 `docker exec` 要加 **`-i`**，否则 stdin 不会转发进容器。

### Redis

```bash
ssh -i "${OPS_SSH_KEY_FILE}" root@"${OPS_VPS_HOST}" \
  'docker exec sub2api-redis redis-cli EXISTS billing:balance:<user_id>'
```

---

## 3. 写操作的事务模板

生产写入一律用**单个 `SERIALIZABLE` 事务 + `DO` 块**，把前置校验写成
`RAISE EXCEPTION`，任一不符自动回滚。骨架：

```sql
\set ON_ERROR_STOP on
BEGIN ISOLATION LEVEL SERIALIZABLE;

DO $do$
DECLARE
  u users%ROWTYPE;
  p user_balance_packages%ROWTYPE;
BEGIN
  -- 1. 固定顺序加锁，避免与其它事务死锁
  SELECT * INTO u FROM users WHERE id = ... FOR UPDATE;
  IF NOT FOUND THEN RAISE EXCEPTION '...'; END IF;
  SELECT * INTO p FROM user_balance_packages WHERE id = ... FOR UPDATE;
  ...

  -- 2. 逐项校验前置，包括幂等
  IF p.status <> 'active' THEN RAISE EXCEPTION '状态不符：%', p.status; END IF;
  IF EXISTS (SELECT 1 FROM payment_audit_logs WHERE order_id = ... AND action = ...) THEN
    RAISE EXCEPTION '幂等冲突';
  END IF;

  -- 3. 用锁内实时值计算，不要用事务外读到的快照
  ...

  -- 4. 写入 + 审计 + 缓存失效
  ...
  RAISE NOTICE '执行成功 ...';  -- 把关键中间值打出来，便于事后核对
END $do$;

COMMIT;
```

### 四条硬性要求

**① 金额必须用锁内实时值算，不能用事务外的快照。**
活跃用户的余额随时在变。本次执行时目标用户 20 分钟前还在调 API。

**② 幂等前置必须查。** `payment_audit_logs` 对 `(order_id, action)` 有唯一索引，
重复执行会在写审计时冲突——但那时前面的 `UPDATE` 已经执行过了，
靠约束兜底不如提前 `RAISE`。

**③ 写审计是硬规则。** 项目约定：生产数据变更必须写 `payment_audit_logs`。
操作者统一用 `admin:authorized_manual_settlement`。

**④ 用 `RAISE NOTICE` 打出关键中间值。** 事务提交后这些值就是唯一的
执行凭据，比事后回读更能说明「当时算的是什么」。

---

## 4. 已知的表结构坑

这几条都是本次踩到的，写 SQL 前先确认：

| 坑 | 事实 |
| --- | --- |
| `payment_audit_logs.order_id` | 是 **`varchar(64)`** 不是整型。写 `WHERE order_id=763` 会报 `operator does not exist: character varying = integer`，要写 `'763'` |
| `payment_audit_logs` 没有 `actor` 列 | 是 **`operator`** |
| `balance_debt_ledger` | 有 `CHECK (amount_usd > 0)`。**无欠费时不能插零额度行**，要用 `IF v_debt > 0` 跳过 |
| `api_keys` 存的是原文 | 列名就是 `key`。缓存失效函数 `enqueue_auth_cache_invalidation(raw_key text)` 要原文——**用子查询传递，不要 SELECT 出来打印** |

查结构：`-c "\d 表名"`，一次看全列、索引和约束。

---

## 5. 缓存失效

改余额后必须处理两处缓存，否则旧快照会继续生效（负余额场景下会**继续拦截用户**）。

### 认证缓存（API Key）

事务内调用，Key 明文经子查询传递，全程不输出：

```sql
PERFORM enqueue_auth_cache_invalidation(k.key)
   FROM api_keys k WHERE k.user_id = <uid> AND k.deleted_at IS NULL;
```

写入 `auth_cache_invalidation_outbox` 后由 worker 异步处理。等待排空：

```bash
ssh ... 'for i in $(seq 1 10); do
  N=$(docker exec sub2api-postgres psql -U sub2api -d sub2api -tAc \
      "SELECT COUNT(*) FROM auth_cache_invalidation_outbox;")
  echo "待处理=$N"; [ "$N" = "0" ] && break; sleep 6
done'
```

相关函数还有 `enqueue_user_auth_cache_invalidation()`、
`enqueue_group_auth_cache_invalidation()`、`enqueue_allowed_group_auth_cache_invalidation()`。

### 余额缓存（Redis）

```bash
docker exec sub2api-redis redis-cli EXISTS billing:balance:<user_id>
```

返回 `0` 即不存在，下次请求会按新余额重算，无需额外操作。存在则 `DEL`。

---

## 6. 执行后核验清单

改完一律从数据库回读，不靠「应该成功了」：

- [ ] 目标表的每个被改字段，逐项对比预期
- [ ] **未被改的字段确认没动**（本次要确认 `starts_at` / `expires_at` / `status` 未变）
- [ ] 关联表状态未受影响（订单状态、退款金额）
- [ ] 审计行已写入，`detail` 内容正确
- [ ] 条件写入的表按预期写了或没写（如无欠费时不应有账本行）
- [ ] `auth_cache_invalidation_outbox` 待处理数归零
- [ ] Redis 相关键状态符合预期

---

## 7. 什么时候该走这条路，什么时候不该

**该走**：操作在管理 API 和后台 UI 里都没有对应能力。
先确认这一点——全仓路由枚举：

```bash
grep -rhoE '\.(GET|POST|PUT|PATCH|DELETE)\("[^"]*"' backend/internal/server/routes/*.go
```

**不该走**：管理 API 已经有对应端点。走 API 能自动获得事务、审计、
缓存失效和业务校验，手写 SQL 每一环都要自己保证，漏一个就是事故。

**尤其不要**：用一个语义不同的 API「凑合」。例如用
`POST /admin/users/:id/balance` 顶替周额度刷新——它只改余额数字，
会导致 `next_credit_at` 不推进、定时任务重复发放，且不写审计。
**比不做更糟。**

---

## 8. 这类操作值得固化

仓库里同类的「周额度提前刷新」执行记录已有 **9 份以上**，每次都要人工写 SQL 事务。
幂等键、锁顺序、`creditDueBalance` 口径、账本的正数约束、缓存失效——
每一环漏掉都会出问题。

**值得加一个管理端点把它固化进代码。** 手写事务的正确性依赖执行者每次都记得
全部细节，而这些细节散落在 9 份文档里。
