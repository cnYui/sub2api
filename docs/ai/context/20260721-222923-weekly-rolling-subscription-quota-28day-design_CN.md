# 2026-07-21 周滚动订阅额度与 28 天周期设计

## 目标

公共 Codex 订阅统一采用“28 天有效期 + 每 7 天滚动刷新额度”。订阅额度不再按自然日或自然周计算；刷新时间由订阅自身窗口决定。

| 售价 | 周额度 | 28 天总额度 |
|---:|---:|---:|
| 29 元 | 72 USD | 288 USD |
| 39 元 | 97 USD | 388 USD |
| 59 元 | 148 USD | 592 USD |
| 79 元 | 198 USD | 792 USD |
| 99 元 | 248 USD | 992 USD |
| 149 元 | 374 USD | 1496 USD |
| 199 元 | 500 USD | 2000 USD |

## 额度窗口规则

### 新购与续费

- 新购订阅：`starts_at` 是周窗口锚点，`expires_at = starts_at + 28 天`。
- 同分组续费：新权益段从当前 `expires_at` 开始，增加 28 天；原订阅周窗口节奏保持不变。
- 新购的 28 天恰好是 4 个完整 7 天窗口，每个完整窗口发放一次完整周额度。

### 存量迁移

- 存量 active 订阅保留现有 `expires_at`；迁移不统一加减有效期。
- 迁移时刻 `migration_at` 是存量订阅的首个周窗口锚点，必须持久化到 `weekly_anchor_at`。
- 每条存量订阅的剩余时长以 `max(expires_at - migration_at, 0)` 单独计算，不假设其历史套餐有效期为 28 天，也不按用户所属套餐写死额度。
- 存量首窗为 `[migration_at, min(migration_at + 7 天, expires_at))`。剩余时长中每个完整 7 天窗口发放该用户权益快照中的完整周额度，最后不足 7 天的窗口按比例发放；例如剩余 13 天时为一个完整 7 天窗口加一个 6 天尾窗。
- 历史日额度用量不并入新的首个周窗口；周额度开始前的超额需要通过独立、可审计的有效期扣减处理。

### 两个历史超额账号的强制处理

周额度代码、前端和数据库迁移完成后，开始整体测试前，必须优先处理当前两个完全超过日额度的 `codex-pool-19-usd` 订阅。处理使用 29 元套餐的 72 USD/周作为抵扣基准：

`daily_equivalent_usd = 72 / 7`

`deduction_days = floor(overage_usd / daily_equivalent_usd)`

`estimated_weeks = ceil(overage_usd / 72)` 仅用于说明需要多少周消化，不用于把扣减天数强制凑成 7 的倍数。实际扣减始终按日均额度计算并向下取整。

当前已核对的基线为：

| 用户 | 超额 USD | 预计周数 | 原始计算天数 | 实际扣减天数 |
|---|---:|---:|---:|---:|
| `xunskyler@gmail.com` | 234.1998836 | 4 | 22.7694331278 | 22 |
| `luzhiyuan2026@163.com` | 189.9496876 | 3 | 18.4673307389 | 18 |

执行要求：

- 在清理旧日窗口用量前，先锁定并保存两人的 `overage_usd`、原到期时间和计算参数；否则切换周额度后会失去债务依据。
- 周额度迁移完成后，在同一事务中扣减 `user_subscriptions.expires_at`，并同步截断覆盖新到期时间的 active entitlement periods；新到期时间不晚于执行时刻的订阅直接设为 expired。
- 扣掉的有效期属于历史超额抵扣，不得在后续退款中重新计入可退款权益。
- 新增幂等审计事实，唯一键至少包含 `weekly_quota_cutover_overage:<subscription_id>`；重复运行必须返回已处理，禁止再次扣减。
- 本地开发库已经按上述基线手动扣减过 22 天和 18 天。后续本地迁移必须把这两条写成 `already_applied` 的审计记录，不能再次扣减；生产执行时重新读取并锁定生产事实，不直接照抄本地到期时间。
- 两条调整完成、订阅与权益段一致、缓存失效且额度状态回到正常范围后，才允许开始购买、续费、尾窗、退款和 Dashboard 的整体测试。

### 尾窗折算

任何不足 7 天的窗口都使用订阅自身权益快照的精确比例额度：

`effective_weekly_limit_usd = entitlement.weekly_limit_usd × (window_end - window_start) / 7 天`

其中 `window_end = min(window_start + 7 天, expires_at)`。`entitlement.weekly_limit_usd` 是该订单、兑换或管理员分配时写入的权益快照，不从当前 group 配置硬编码或回读。请求前校验、成功结算、缓存、Dashboard 和用户订阅展示都必须使用 `effective_weekly_limit_usd`，不能直接使用完整周额度。

因此，不同套餐、历史 30 天订阅、管理员手动加减天数和退款后的订阅，都会按各自 `expires_at` 与权益快照计算实际可用总额；代码中不能为“剩 1 天”“剩 6 天”或某个套餐写死固定额度。

订阅到期即停止；尾窗不能因为不足一周而继续有效，也不能获得完整一周额度。

## 精度与展示契约

- 金额事实使用高精度数值：权益快照、限额判断、预授权、成功结算和进度百分比都保留精确值。
- 用户可见的周额度、当前窗口额度和剩余额度统一四舍五入为整数 USD，不显示小数点。
- 展示整数不参与任何计费比较；用户看到的是 `round(effective_weekly_limit_usd)`，实际可用上限始终是精确计算结果。
- 购买页、用户订阅页、Dashboard、顶部订阅进度和 Key 用量页必须复用同一个 formatter。
- 上述整数展示规则至少在周额度上线后保留一个月。

## 数据模型与迁移

新增前向迁移，不修改历史迁移。

1. 公共 Codex groups：清空 `daily_limit_usd`、`monthly_limit_usd`，写入对应 `weekly_limit_usd`，并将默认有效期设为 28 天。
2. `subscription_plans`：有效期设为 28 天；名称、商品名、描述和 features 统一为“周额度、28 天、购买时间起每 7 天刷新”。
3. `user_subscriptions`：增加 `weekly_anchor_at`；新购写入 `starts_at`，存量迁移写入统一 `migration_at`。`weekly_window_start` 记录当前实际窗口起点，`weekly_usage_usd` 只记录当前窗口用量。
4. `payment_orders`：增加版本化 `subscription_snapshot`，在订单创建时写入套餐名称、group、28 天、周额度、28 天总额度和窗口规则。支付履约只消费该快照，不能在付款后重新读取可变的 group 或 plan；订单 `amount` 继续作为不含手续费的购买本金，`pay_amount` 仅记录实际支付金额。
5. `subscription_entitlement_periods`：增加 `weekly_limit_usd`、`period_total_quota_usd`、`quota_window_unit`、`quota_window_days`，并从订单快照复制，成为订单/兑换/管理员分配的不可变权益事实。
6. `usage_facts`：增加可空 `entitlement_period_id`。请求前选择订阅计费来源时确定当前权益段，成功事实持久化该 ID；退款按此 ID 汇总，不用时间范围或当前订阅窗口猜测用量。历史 facts 仅在权益段无重叠时回填；无法无歧义归属的记录标记为 legacy-unallocated，相关退款进入人工审核。
7. `payment_orders` 增加不可变 `refund_basis`：保存退款时使用的权益段、周期总额度、已用额度、使用比例、购买本金、手续费和计算时间，支持重试、审计和客服复核。
8. 新增 `subscription_quota_debt_adjustments`：保存订阅、用户、group、来源唯一键、超额金额、周额度、日均额度、原始计算天数、实际扣减天数、原到期时间、新到期时间、应用状态和时间；用于两个历史超额账号及未来同类人工审计，禁止只在日志或文档中记录。
9. 存量迁移仅处理公共 Codex 付费组：保留到期时间，按每条订阅的 `expires_at - migration_at` 计算剩余时长，写入存量锚点与首窗起点，将首窗周用量置零；本地 dry-run 必须逐条输出权益快照周额度、剩余时长、首窗结束时间和有效周额度。
10. 迁移完成后按用户和分组删除 Redis `billing:sub:*`，并清除应用订阅 L1 缓存。

迁移前必须先做一致性分类：付款完成但没有订阅链接的订单、没有权益段的完成订单、重叠权益段、退款中订单和待支付订单分别输出清单。不能用一条批量 SQL 假设所有历史订单都可以自动映射。

## 后端实现

### 单一窗口计算器

在订阅领域提供唯一的滚动周窗口计算器，输入 `weekly_anchor_at`、`weekly_window_start`、`expires_at` 和 `now`，输出：

- `window_start`
- `window_end`
- `effective_weekly_limit_usd`
- `resets_at`
- 窗口是否已过期

禁止订阅代码继续使用 `StartOfWeek`、自然周一或 `weekly_window_start + 7 天` 的前端自行推导。窗口跨过时，新的窗口起点是上一个 `window_end`；最后一个窗口的 `resets_at` 就是 `expires_at`。

### 计费与结算

- 请求前预授权以当前窗口精确有效额度判断：`weekly_usage_usd + estimated_cost <= effective_weekly_limit_usd`。
- 成功结算以同一窗口计算器决定是否归零、写入哪个 `weekly_window_start` 和如何累加用量。
- Redis 订阅缓存必须携带周窗口所需字段；缓存命中、DB fallback 和持久化 SQL 使用同一计算结果。
- 周额度耗尽错误返回 `WEEKLY_LIMIT_EXCEEDED` 与精确 `window_resets_at`，Gateway 将其映射为 HTTP 429 和正确的 `Retry-After`。
- 日额度超额顺延逻辑不适用于已切换到周额度的公共 Codex 订阅组。

### 权益、退款与续费

- `GrantSubscriptionEntitlement`、兑换、管理员分配、管理员延长和支付履约都写入周额度快照，禁止以后续 group 配置覆盖历史权益。
- 退款按订单对应的 28 天 entitlement period 计算，不按整个订阅的历史 `starts_at`、当前 group 配置或剩余天数推算；尾窗和多次续费必须读取该订单的不可变快照。
- 28 天总额度为该权益周期的完整额度总和，即 `entitlement.weekly_limit_usd × 4`。订单周期内的已用额度从可审计的 `usage_facts` 汇总，`usage_logs` 仅作历史兜底；不能使用只代表当前窗口的 `weekly_usage_usd`。
- 使用比例为 `usage_ratio = clamp(used_quota_usd / period_total_quota_usd, 0, 1)`；已使用价值为 `purchase_base_amount × usage_ratio`；可退款金额为 `max(purchase_base_amount - 已使用价值, 0)`。
- `purchase_base_amount` 是用户购买套餐的本金（订单 `amount`），不包含支付手续费、通道手续费或其他附加费；退款计算不得使用含手续费的 `pay_amount`。手续费不因套餐未用完而计入可退款金额。
- 多次续费必须按各自订单权益段分别汇总使用量和计算退款，不能把其它 28 天周期的用量混入当前订单。
- 退款只撤销对应 entitlement period，并从剩余 active 权益段重新计算订阅 `expires_at`、状态和当前窗口；禁止调用“撤销整个 subscription”的旧路径，否则会误删后续续费权益。
- 用户退款前提供只读 refund quote：购买本金、不可退手续费、周期总额度、已用额度、使用比例和预计可退款金额。提交退款时在事务内锁订单、权益段和用量事实后重新计算 quote，防止报价后新增用量造成少扣。
- 管理员退款分为“按规则计算”和“强制人工退款”；强制退款必须填写原因、写入审计，不复用自动退款的额度比例结果。
- 管理员缩短订阅、退款撤销和超额扣减导致新的不足 7 天窗口时，必须重新计算有效周额度并失效缓存。

### 购买与支付履约

- `GET /payment/plans` 与 `GET /payment/checkout-info` 返回的周额度、28 天和商品文案必须来自同一份套餐快照结构，购买页不得拼接或硬编码“日限额 / 24 点刷新”。
- 创建订阅订单时，在同一事务中锁定 plan/group、校验同组续费限制，并写入 `subscription_snapshot` 与本金 `amount`；支付完成、补单、迟到回调和余额支付都只能从该快照创建权益。
- 待支付订单跨越切换时间时不能静默套用新套餐：发布前停止公共套餐售卖并清理/取消旧待支付订单，或将其明确标记为 legacy 并按创建时快照履约。
- 管理员套餐编辑、默认注册订阅、按认证来源赠送订阅、兑换码和管理员手动分配都必须显式传入有效期与权益快照；公共 Codex 默认值统一为 28 天，不能保留隐式 30 天。

## API 与前端

### API 契约

订阅详情、订阅进度、Key 用量和 Dashboard 统一提供或消费以下精确字段：

- `weekly_window_start`
- `weekly_window_resets_at`
- `effective_weekly_limit_usd`
- `weekly_usage_usd`
- `weekly_remaining_usd`

Dashboard 的读模型改为通用窗口字段：`quota_window_unit`、`window_usage_usd`、`window_limit_usd`、`window_limit_unlimited`、`window_starts_at`、`window_resets_at`。公共 Codex 周订阅显示“本周额度”，不再显示“今日额度”。

退款增加 `GET /payment/orders/:id/refund-quote`。该接口只返回当前可退款资格与精确 quote，不改变订单；退款提交接口内部必须重新计算，而不是信任前端金额。

### 页面范围

- 购买页与商品卡：使用后端套餐名称，展示“周额度 / 每 7 天刷新 / 28 天有效期”，删除“日限额 / 24 点刷新”。
- 用户订阅页、订阅进度组件、顶部迷你进度：展示后端返回的有效周额度、精确刷新时间和整数格式化额度。
- 用户 Dashboard：展示当前周窗口用量与额度，不再按 `daily_limit_usd × period_days` 计算。
- Key 用量页：使用有效周额度和窗口重置时间，而非 group 的完整周额度。
- 用户订单页：退款弹窗展示购买本金、手续费不退说明、28 天总额度、已用额度、使用比例和预计退款；不得只显示原订单金额后直接提交。
- 管理端订单与退款弹窗：展示订单快照、权益段、退款依据和强制退款审计；管理员不能手工填写一个绕过额度使用比例的“普通退款金额”。
- 管理端分组、订阅、套餐管理、系统默认订阅和认证来源默认订阅：突出周额度和 28 天；管理员手动分配默认 28 天；重置配额只重置当前滚动周窗口。
- 中文、英文文案与相应测试同步更新。

## 验收与上线

### 自动化验证

- 新购订阅依次产生 4 个完整 7 天窗口。
- 存量剩余 1、2、3、4、5、6、13 天均验证窗口额度、到期时间和用量归零。
- 对每个套餐额度和 1 至 6 天短窗，验证展示值等于精确有效额度的四舍五入，且预授权拒绝超过精确限额的请求。
- 缓存命中、缓存失效、DB fallback、成功结算和 Dashboard 对同一窗口返回一致结果。
- 周额度耗尽返回 HTTP 429、`WEEKLY_LIMIT_EXCEEDED` 和正确 `Retry-After`。
- 续费、退款、管理员加减天数和超额扣减后，尾窗仍按比例计算。
- 两个历史超额账号分别只扣减 22 天和 18 天；重复运行不再次扣减，订阅到期时间、权益段、审计事实和缓存保持一致。
- 支付期间修改 plan/group、余额支付、混合支付、支付回调重试和迟到回调均验证最终权益仍等于订单快照。
- 单次购买、连续续费、历史重叠权益段和无法归属历史用量分别验证退款：只撤销目标权益段，后续权益不受影响；无法归属的历史退款转人工审核。
- 用户退款 quote 与提交事务内重新计算的退款金额一致；手续费不进入退款本金或网关退款金额。

### 发布步骤

1. 在本地附件数据库运行 dry-run 和迁移演练。
2. 备份 PostgreSQL 与 Redis，并验证备份可读。
3. 部署代码与前向迁移，执行存量迁移和缓存失效。
4. 优先完成两个历史超额账号的幂等有效期抵扣并复核，未完成时禁止开始整体测试。
5. 用新购、存量尾窗、续费和退款样本进行 API 与页面验收。
6. 全程不修改公网 Nginx、Cloudflare 或 CLIProxyAPI 账号池配置。

## 实施顺序、备份与回滚

### 实施顺序

1. 先完成 schema、订单快照、权益归属和退款 quote 的向后兼容代码；旧日额度订阅继续按旧逻辑运行。
2. 在本地附件库执行迁移 dry-run，输出历史订单、权益段和 usage facts 的分类清单；先修复或隔离重叠权益段和无法归属用量。
3. 关闭公共 Codex 套餐售卖，处理旧待支付订单，再对生产事实源做完整备份。
4. 部署兼容代码与前向迁移，执行存量周窗口迁移和缓存失效；此阶段不允许订阅购买、退款、管理员调整或公共 Codex 模型请求并发写入。
5. 写入或导入两个超额账号的债务审计事实，应用有效期扣减，复核订阅、权益段和缓存；这是整体测试的硬门禁。
6. 通过购买、续费、尾窗、退款 quote、支付回调和用量结算验收后，开放新套餐售卖。

### 生产备份

执行前必须先确认当前 Nginx 指向、Compose project、容器名、volume 和端口；当前候选环境名称只能作为核对起点，不能不经确认直接执行。备份完整 PostgreSQL 与 Redis，而不是只导出订阅相关表。

```bash
TS="$(date +%Y%m%d-%H%M%S)"
BACKUP_DIR="deploy/backups/weekly-quota-${TS}"
PG_CONTAINER="sub2api-candidate-postgres" # 先按实际运行态确认
REDIS_CONTAINER="sub2api-candidate-redis" # 先按实际运行态确认
mkdir -p "$BACKUP_DIR"

docker exec "$PG_CONTAINER" sh -lc \
  'pg_dump -U "$POSTGRES_USER" -d "$POSTGRES_DB" --format=custom --no-owner --no-privileges --file=/tmp/weekly-quota-before.dump'
docker cp "$PG_CONTAINER":/tmp/weekly-quota-before.dump "$BACKUP_DIR/postgres.dump"
docker exec "$PG_CONTAINER" sh -lc 'pg_restore --list /tmp/weekly-quota-before.dump >/dev/null && rm -f /tmp/weekly-quota-before.dump'
sha256sum "$BACKUP_DIR/postgres.dump" > "$BACKUP_DIR/postgres.dump.sha256"

docker exec "$REDIS_CONTAINER" sh -lc \
  'redis-cli SAVE >/dev/null && cp /data/dump.rdb /tmp/weekly-quota-before.rdb && redis-check-rdb /tmp/weekly-quota-before.rdb >/dev/null'
docker cp "$REDIS_CONTAINER":/tmp/weekly-quota-before.rdb "$BACKUP_DIR/redis.rdb"
docker exec "$REDIS_CONTAINER" rm -f /tmp/weekly-quota-before.rdb
sha256sum "$BACKUP_DIR/redis.rdb" > "$BACKUP_DIR/redis.rdb.sha256"
```

只记录备份路径、时间、大小和校验值，不打印或提交 dump 内容、环境变量或密钥。备份至少异机保存一份。

### 回滚边界

- 数据库迁移是前向的。常规回滚使用随版本交付的补偿 SQL：恢复公共套餐配置、撤销新增窗口字段的业务使用、恢复迁移前的订阅窗口和缓存；不能在仍有新订单、新 usage facts 写入时直接整库恢复。
- 完整 PostgreSQL/Redis restore 只用于确认停止写入后的灾难恢复；整库恢复会丢失备份时间之后的订单、退款和用量事实。
- 应用镜像可回滚到前一版本，但仅在新 schema 保持向后兼容时执行；每一步都先在本地附件库演练并验证恢复路径。
