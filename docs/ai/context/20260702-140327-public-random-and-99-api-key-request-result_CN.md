# 公网随机用户与 99 元用户 API Key 真实请求结果

- 时间：2026-07-02 14:03 JST
- 公网入口：`https://api.aaccx.pw/v1/responses`
- 请求模型：`gpt-5.5`
- 请求内容：最小文本请求，要求只回复 `ok`
- 安全处理：仅记录 API Key 尾号，不记录完整 Key。

## 初始检查

- 当前运行容器：
  - `sub2api`：`weishaw/sub2api:latest`，`127.0.0.1:18080->8080`，healthy
  - `sub2api-postgres`：healthy
  - `sub2api-redis`：healthy
- 健康检查：
  - `https://api.aaccx.pw/health`：200
  - `http://127.0.0.1:8080/health`：200
  - `http://127.0.0.1:18080/health`：200

## 随机用户请求

修复前随机抽样的普通用户请求结果：

- `3056163754@qq.com`，`codex-pool-19-usd`，Key 尾号 `b16284`：HTTP 200，返回 `ok`
- `milesyang987@gmail.com`，`codex-pool-19-usd`，Key 尾号 `1aad57`：HTTP 200，返回 `ok`
- `luzhiyuan2026@163.com`，`codex-pool-19-usd`，Key 尾号 `9789b3`：HTTP 200，返回 `ok`
- `1915474749@qq.com`，`codex-pool-19-usd`，Key 尾号 `0jDDr8`：HTTP 200，返回 `ok`

修复后再次随机抽样：

- `amarsimoss@gmail.com`，`codex-pool-19-usd`，Key 尾号 `956mss`：HTTP 200，返回 `ok`
- `3418186387@qq.com`，`codex-pool-19-usd`，Key 尾号 `a0969d`：HTTP 200，返回 `ok`
- `2466439791@qq.com`，`codex-pool-19-usd`，Key 尾号 `ec751d`：HTTP 200，返回 `ok`

## 99 元用户请求与修复

- 99 元 active 用户：`2864533153@qq.com`
- 用户 ID：`54`
- API Key ID：`58`
- API Key 尾号：`f8258e`
- 套餐组：`codex-pool-89-usd`

初始真实请求结果：

- HTTP 503
- 错误：`Service temporarily unavailable`
- 应用日志：`openai.account_select_failed`，原因 `no available accounts`

根因：

- 新增的 99 元组 `codex-pool-89-usd` 和 79 元组 `codex-pool-69-usd` 没有绑定到唯一可用上游账号 `cliproxy-local-openai`。
- 原账号只绑定了 `codex-pool-19-usd`、`codex-pool-29-usd`、`codex-pool-49-usd` 和 `codex-pool-local-unlimited`。

已执行修复：

- 修复前备份公网库：`deploy/backups/20260702-140054-sub2api-before-79-99-account-binding.dump`
- 幂等新增 `account_groups`：
  - `account_id=1 -> group_id=8`（99 元 / `codex-pool-89-usd`）
  - `account_id=1 -> group_id=9`（79 元 / `codex-pool-69-usd`）
- 未修改用户、API Key、订阅、套餐价格、镜像、nginx 或 Redis。

修复后 99 元用户真实请求结果：

- HTTP 200
- 返回 `ok`
- 响应 ID：`resp_09f23b50638ecdbb016a45f0bfec908191aadd47590da54e5f`
- `usage_logs` 已产生成功记录：
  - `user_id=54`
  - `api_key_id=58`
  - `account_id=1`
  - `group_id=8`
  - `model=gpt-5.5`
  - `inbound_endpoint=/v1/responses`

## 79 元说明

当前公网没有 active 的 79 元订阅用户，因此没有用户 API Key 可直接做 79 元真实请求。运行态绑定层已补齐：`cliproxy-local-openai` 当前绑定 `codex-pool-69-usd` 与 `codex-pool-89-usd`，后续 79 元用户会进入同一上游账号池。
