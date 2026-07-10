# 本地 main 最新代码重部署与 GPT 5.6 公网真实测试结果

## 时间

2026-07-10 13:09 (北京时间)

## 执行摘要

按用户要求：

1. 先暂停当前公网 18084 Sub2API 应用容器与 CLIProxyAPI 8317。
2. 备份公网候选数据层。
3. 核对 GPT 5.6 三个新模型的历史用量和计费规则。
4. 将本地最新 `main` 部署到 18084。
5. 恢复 CLIProxyAPI 并使用用户提供的真实 API Key 测试三款 GPT 5.6 模型。

## 暂停动作

- `sub2api-candidate` 已先停止。
- CLIProxyAPI 8317 监听进程已先停止。
- `sub2api-candidate-postgres` 与 `sub2api-candidate-redis` 未停止、未重建。

## 部署信息

- 本地分支：`main`
- 本地 HEAD：`a575f28c1`
- 新镜像：`sub2api-candidate:20260710-125927-a575f28c1-local-main-gpt56`
- 容器：`sub2api-candidate`
- 端口：`127.0.0.1:18084->8080`
- 状态：`healthy`

保留 HTTP 本机上游运行态环境：

- `SECURITY_URL_ALLOWLIST_ALLOW_INSECURE_HTTP=true`
- `SECURITY_URL_ALLOWLIST_ALLOW_PRIVATE_HOSTS=true`

## 备份

执行前已备份：

- Postgres dump：`deploy/backups/20260710-125036-sub2api-candidate-postgres-before-main-redeploy-gpt56-audit.dump`，约 45MB，权限 600
- Redis RDB：`deploy/backups/20260710-125036-sub2api-candidate-redis-before-main-redeploy-gpt56-audit.rdb`，约 156KB，权限 600

## GPT 5.6 历史计费核对

### 当前历史用量

重部署前核对到 GPT 5.6 已产生历史用量：

- `gpt-5.6-sol`：304 条
- `gpt-5.6-terra`：13 条
- `gpt-5.6-luna`：0 条
- 涉及用户数：6 个
- 历史总成本：约 `48.7976285 USD`

### 是否需要补扣

结论：**不需要补扣**。

原因：

1. 所有 317 条 GPT 5.6 历史 usage 均已有 `usage_billing_dedup`。
2. 所有记录都有 `subscription_id`，已进入订阅额度。
3. 按当前 main 的 GPT 5.6 新规则复算：
   - 包含 `gpt-5.6-sol/terra/luna` 基础价格
   - 包含 272K+ 长上下文输入/输出倍率
   - 复算结果与 `usage_logs.total_cost` 完全一致，delta=0
4. 因此如果再次补扣会造成重复扣费。

用户规则理解落实：

- 规则正式出现前的调用：如果未来存在未计费/未 dedup 记录，应不进入平台扣费。
- 规则正式出现后的调用：当前已全部按新规则进入平台 usage 和订阅额度，无需二次追扣。

## 公网真实 Key 测试

用户提供真实 API Key 已用于测试，文档不记录完整 Key。

Key 归属：

- `api_keys.id=65`
- `user_id=31`
- active 订阅：`subscription_id=71`
- group：`codex-pool-69-usd`

### `/v1/models`

公网 `https://api.aaccx.pw/v1/models` 可见：

- `gpt-5.6-sol`: true
- `gpt-5.6-terra`: true
- `gpt-5.6-luna`: true
- 裸 `gpt-5.6`: false

### `gpt-5.6-sol`

真实请求：

- URL：`POST https://api.aaccx.pw/v1/responses`
- model：`gpt-5.6-sol`
- 输入：`请只回复 ok`

结果：

- HTTP：200
- status：completed
- output：`ok`
- usage_logs 新增：`id=79364`
- total_cost：`0.0063000000`
- billing_type：1（订阅）
- subscription_id：71

### `gpt-5.6-terra`

真实请求：

- URL：`POST https://api.aaccx.pw/v1/responses`
- model：`gpt-5.6-terra`
- 输入：`请只回复 ok`

结果：

- HTTP：200
- status：completed
- output：`ok`
- usage_logs 新增：`id=79365`
- total_cost：`0.0019980000`
- billing_type：1（订阅）
- subscription_id：71

### `gpt-5.6-luna`

真实请求：

- URL：`POST https://api.aaccx.pw/v1/responses`
- model：`gpt-5.6-luna`
- 输入：`请只回复 ok`

结果：

- HTTP：502
- 未新增 usage_logs
- 未扣费

根因：

- Sub2API 已正确进入内容审核、账号选择和上游转发。
- CLIProxyAPI 已收到请求，并轮换多个 Codex OAuth 账号。
- 多个上游账号返回 `Model not found gpt-5.6-luna`，期间有一次上游 500。
- Sub2API 日志最终为上游 404/账号 failover 后无可用账号，返回 502。

判断：这是 CLIProxyAPI/上游账号池对 `gpt-5.6-luna` 实际不可用，不是 Sub2API 部署、鉴权或计费问题。

## 重部署后最终状态

### Health

- `http://127.0.0.1:18084/health`：200
- `http://127.0.0.1:8080/health`：200
- `https://api.aaccx.pw/health`：200
- `https://aaccx.pw/dashboard`：200
- `https://aaccx.pw/purchase`：200

### 容器

- `sub2api-candidate`：新镜像，healthy
- `sub2api-candidate-postgres`：未重建，healthy
- `sub2api-candidate-redis`：未重建，healthy
- CLIProxyAPI：8317 listening，`/healthz=200`

### GPT 5.6 当前累计

- `gpt-5.6-sol`：305 条，总成本 `48.0245650000`
- `gpt-5.6-terra`：14 条，总成本 `0.7813615000`
- `gpt-5.6-luna`：0 条

## 结论

- 本地最新 main 已完整部署到公网 18084。
- GPT 5.6 规则在 Sub2API 中有效。
- 历史 5.6 用量已按当前新规则计入订阅额度，无需补扣。
- 用户提供真实 Key 下：
  - `gpt-5.6-sol` 可真实使用并扣费。
  - `gpt-5.6-terra` 可真实使用并扣费。
  - `gpt-5.6-luna` 当前上游返回模型不存在，未扣费；需到 CLIProxyAPI/上游模型池继续修复。
