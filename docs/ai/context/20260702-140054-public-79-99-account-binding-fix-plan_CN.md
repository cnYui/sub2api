# 公网 79/99 元套餐账号绑定修复计划

- 时间：2026-07-02 14:00 JST
- 背景：公网真实请求发现随机 19 元组用户请求 `https://api.aaccx.pw/v1/responses` 成功，但 99 元组用户 `2864533153@qq.com` 返回 503。

## 根因证据

- 99 元用户 API Key 鉴权成功，请求进入 `codex-pool-89-usd`。
- 应用日志显示 `openai.account_select_failed`，错误为 `no available accounts`。
- 当前唯一可用上游账号 `cliproxy-local-openai` 只绑定：
  - `codex-pool-19-usd`
  - `codex-pool-29-usd`
  - `codex-pool-49-usd`
  - `codex-pool-local-unlimited`
- 新增的 `codex-pool-69-usd` 与 `codex-pool-89-usd` 没有绑定上游账号。

## 修复方案

1. 先备份公网 PostgreSQL。
2. 在 `account_groups` 中幂等新增：
   - `account_id=1 -> group_id=9`（79 元 / `codex-pool-69-usd`）
   - `account_id=1 -> group_id=8`（99 元 / `codex-pool-89-usd`）
3. 不修改用户、API Key、订阅、套餐价格、镜像、nginx 或 Redis。
4. 修复后用 99 元用户 Key 对公网 `/v1/responses` 重新发真实请求。

## 风险与取舍

- 该修复会让 79/99 元套餐使用与 19/29/49 元套餐相同的上游账号池；这是当前架构下的预期运行方式。
- 由于当前只有一个 OpenAI 上游账号，新增绑定不会增加上游容量，只是补齐新套餐可调度范围。
