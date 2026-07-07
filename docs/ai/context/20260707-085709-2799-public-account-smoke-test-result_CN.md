# 2799523972@qq.com 公网账号验收结果

时间：2026-07-07 08:57 JST

## 范围

- 使用测试账号 `2799523972@qq.com` 登录公网控制台。
- 检查当前公网 18084 栈、购买页、API Key 创建、真实模型请求、计费落账和测试 Key 清理。
- 不记录账号密码、完整 API Key、内部 token 或任何支付密钥。

## 当前运行态

- 当前公网应用容器：`sub2api-candidate`
- 当前镜像：`sub2api-candidate:20260707-084458-74e5a4bb0-subscription-window-refresh`
- 端口：`127.0.0.1:18084->8080`
- `sub2api-candidate-postgres` 与 `sub2api-candidate-redis` 继续复用候选数据层。
- `http://127.0.0.1:18084/health` 与 `https://api.aaccx.pw/health` 均返回 200。

## 账号初始状态

- 用户状态：active
- 用户余额：0
- active OpenAI 套餐：2 个
  - `user_subscriptions.id=49`，`codex-pool-19-usd`，日用量 `0/19`
  - `user_subscriptions.id=71`，`codex-pool-69-usd`，日用量 `0/69`
- OpenAI/GPT 流量卡余额：`24.995929 USD`
- 可见 API Key 数量：1

## 购买页验收

- `/purchase` 可正常加载。
- 可见商品包含 29/39/59/79/99 元套餐，以及 5/10/20 刀 GPT 流量卡。
- 所有商品按钮可用。
- 点击 10 刀 GPT 流量卡后进入确认页：
  - 商品：`GPT 流量包 10 刀`
  - 充值金额：`¥3.00`
  - 手续费：`¥0.03`
  - 实付金额：`¥3.03`
  - 支付方式：支付宝
  - `确认支付 ¥3.03` 按钮可用
- 未点击确认支付，未创建真实支付订单。

## API Key 创建验收

- `/keys` 页面当前公网版本已显示自动 Key 语义：
  - 现有 Key 分组显示为“自动分组”
  - 文案为“按当前套餐或 GPT 流量包自动使用”
- 创建 Key 弹窗不再出现“分组”字段。
- 通过前端真实创建临时 Key：
  - 名称：`codex-public-test-20260706235406`
  - DB id：`85`
  - `group_id=NULL`
  - 页面显示“自动分组”
- 页面只展示脱敏 Key，未记录完整 Key。

## 真实模型请求验收

- 使用临时 Key 请求公网 `https://api.aaccx.pw/v1/responses`
- 请求模型：`gpt-5.5`
- HTTP 状态：200
- 返回模型：`gpt-5.5`
- 返回文本：`OK`
- 生成 `usage_logs.id=55398`

## 计费落账

请求前：

- `codex-pool-19-usd`：`0/19`
- `codex-pool-69-usd`：`0/69`
- OpenAI/GPT 流量卡余额：`24.995929 USD`
- 该用户历史流量卡 deduction 数：1

请求后：

- 临时 Key `api_keys.id=85` 仍为 `group_id=NULL`，请求时解析到 effective group。
- `usage_logs.id=55398`
  - `group_id=9`
  - `subscription_id=71`
  - `billing_type=1`
  - `actual_cost=0.004031`
  - `model=gpt-5.5`
- `codex-pool-69-usd` 日用量从 `0` 增至 `0.004031`
- `codex-pool-19-usd` 日用量仍为 `0`
- OpenAI/GPT 流量卡余额仍为 `24.995929 USD`
- 流量卡 deduction 数仍为 1，未新增 deduction

结论：该账号有套餐时，自动 Key 真实请求优先扣套餐额度，未扣流量卡，符合“有套餐优先扣套餐”的规则。

## 清理

- 已通过前端删除临时 Key `codex-public-test-20260706235406`。
- DB 复核：`api_keys.id=85` 已软删除，用户可见 Key 数回到 1。
- 使用记录保留，用于真实计费审计。

## 未覆盖场景

- 本账号有 active 套餐，因此本次不覆盖“只有流量卡、无套餐”的路径。
- 本次不覆盖“套餐额度耗尽后扣流量卡”的路径，因为 `codex-pool-69-usd` 当前日额度充足。
- 本次不覆盖“套餐与流量卡都没有时提示今天额度已经使用完”的路径。
