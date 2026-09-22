# 用户自助「提前刷新」周额度（用户端 /subscriptions）

日期：2026-09-22
状态：**代码已改完、测试通过，尚未部署**

## 起因

管理员反馈「很多用户来提提前刷新」。此前提前发放只有管理端端点
`POST /api/v1/admin/payment/balance-packages/{id}/credit-next`（AGENTS.md 坑 22），
前端无按钮、后台也没有只读接口返回 `package_id`，每次都得查库，人工成本落在管理员身上。

管理员拍板：**做成用户自助**，门槛是**本周额度花光才能点**。

## 位置选型

做了本地 HTML 演示对比三个位置（演示文件在 `tmp/refresh-btn-demo/`，`tmp/` 已 gitignore，
样式用项目自己的 `tailwind.config.js` + `src/style.css` 编译，与真实页面一致）：

- **A1（采用）**：`/subscriptions` 余额套餐卡片里「下次刷新」那一行的右侧。
  按钮紧贴 `下次刷新 2026-09-25`，语义自解释；视觉重量小，不抢右上角「再次购买」。
- A2：卡片底部整宽按钮。太重，会把用户从复购引向「先把剩的榨出来」；且「刷新」二字离时间远，
  容易被当成刷新页面。
- A3：与「再次购买」并排。手机上会把卡片标题挤变形；点错代价（作废本周未用额度）不对称。

管理端加按钮（方案 B）被否：用户还是得来找人，而且订单列表只回
`can_cancel_balance_package` 布尔、拿不到 `package_id`，反而要先改后端。

## 关键事实（决定了按钮怎么设计）

1. **提前刷新是「窗口替换」不是「累加」**。
   `balance_package_service.go` 的 `balanceDelta = weekly_credit_usd - remaining_usd`，
   本周没花完的部分会被直接抹掉。这是「必须花光才能点」的技术原因，不只是产品偏好。
2. **用户端接口已经返回 package id**（`UserBalancePackageView.ID` → `/payment/balance-packages`），
   所以用户端加按钮前端不缺数据；管理端反而缺。
3. 管理端端点**不校验套餐归属**（坑 22 已警告），所以自助路径必须自己补这一条。

## 实现

### 后端

- `BalancePackageService.CreditNextEarly` 的函数体抽成 `creditNextEarly(ctx, packageID, actor, now)`，
  `actor`（`earlyCreditActor`）带三样东西：审计 operator、`requireUserID`、`requireDrained`。
  管理端行为完全不变，只是变成 `actor{operator: "admin:<id>"}`。
- 新增 `CreditNextEarlySelf(ctx, packageID, userID, now)`：
  - 归属校验放在**加锁之前**（避免拿别人的套餐 ID 去锁别人的用户行），不匹配回
    `BALANCE_PACKAGE_NOT_FOUND`（不泄露套餐是否存在）；
  - 锁内复核 `RemainingUsd <= 0.01`，否则回 `BALANCE_PACKAGE_WEEKLY_QUOTA_REMAINING`；
  - 审计 operator 写 `user:<id>`，与管理员的 `admin:<id>` 区分。
    审计 action 仍是 `BALANCE_PACKAGE_EARLY_WEEKLY_CREDIT_<n>`，
    所以 `(order_id, action)` 唯一索引在自助与管理员之间同样生效。
- 容差 `balancePackageSelfCreditRemainingEpsilon = 0.01`：不留容差的话，用户会被几厘钱
  永久卡住（花不掉也刷不了），只能回来找人工——正是这次要消灭的场景。
- `evaluateSelfEarlyCredit(item, now)` 把准入条件集中一处，`ListUserPackages` 用它填
  `can_credit_next_early` / `early_credit_block_reason`，前端不再自己推导规则。
  原因码：`not_active` / `fully_credited` / `expired` / `no_schedule` / `weekly_quota_remaining`。
- 新路由（用户 JWT + 现有 `panelRateLimiter.Global()`）：
  `POST /api/v1/payment/balance-packages/:id/credit-next`。

### 前端

- `SubscriptionsView.vue`：「下次刷新」行右侧加按钮，禁用态下方一行琥珀色提示
  「本周还有 $X 未用完，用完后即可提前刷新下一期。」
  期数发完 / 已过期 / 无排期时按钮**不出现**（不给必然失败的入口）。
- 确认弹窗复用 `ConfirmDialog`，列出「本周剩余 → 新额度」「到账进度 n/N → n+1/N」，
  并说明总额度不变、只是把后面的期数挪到现在。
- 成功后同时 `loadSubscriptions()` 和 `authStore.refreshUser()`——额度直接进 `users.balance`，
  顶栏余额不刷新用户会以为没到账。
- i18n zh/en 均已补 `userSubscriptions.earlyRefresh*`。

### 踩到的坑

**插入提示段落时把 `v-if` / `v-else-if` 链打断了**：原来是
`<div v-if="next_credit_at">` → `<div v-else-if="status==='completed'">`，
我把新的 `<p v-if>` 插在两者之间，`v-else-if` 就改挂到新元素上，
导致「本周期已完成，不再刷新」不再显示。已有测试直接抓到。提示段落要放在整条链之后。

## 测试

- 后端 `internal/service`：新增 4 个用例（自助成功且审计 operator 为 `user:<id>`、
  拒绝他人套餐且套餐与余额都不变、未花光时拒绝但管理员仍可强发、
  `evaluateSelfEarlyCredit` 的各分支含容差）。`go test -tags=unit ./internal/service/` 全量通过（146s）。
- 前端：`SubscriptionsView.spec.ts` 新增 3 个用例（禁用态与原因、可用态点击到调用成功并刷新、
  期数发完不露出入口）。全量 `vitest run` 204 文件 / 1426 用例通过。
- `go test -tags=unit -run '^$' ./...` 全量编译通过；`vue-tsc --noEmit` 通过。

## 已知边界（**待管理员决定是否收紧**）

**欠费用户可以连续点，把剩余期数一次性拉完。**
当负余额 ≥ 一期额度时，这期额度会被欠费全额吃掉、`remaining_usd` 落回 0，
于是门槛再次放行、按钮又亮起来，可以继续点到欠费还清或期数用尽。

- 判断：这与门槛的意图不冲突——用户没有囤积任何可用额度，钱直接抵了已经产生的消费，
  总额度仍以 `refresh_count` 封顶，平台不多付。
- 触发门槛不低：¥29 套餐要欠满 $76 才够。
- 若仍想堵死，改法是在自助路径额外限制「整单最多提前 N 次」。

## 后续

- **部署后要更新 AGENTS.md 坑 22**：那条现在写着「前端仍无按钮」，届时应改为
  「用户端 /subscriptions 已有自助按钮（门槛：本周额度花光），管理端仍无按钮、仍需查库拿 package_id」。
  在部署前不要改，避免 AGENTS.md 记录未生效的事实。
- 演示文件 `tmp/refresh-btn-demo/` 用完可删（gitignore，不影响仓库）。
