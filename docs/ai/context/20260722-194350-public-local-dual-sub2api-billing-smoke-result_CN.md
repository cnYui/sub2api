# 公网/本地双 Sub2API 扣费链路验证结果

时间：2026-07-22 19:43

## 前置状态

- 本地 CPA 容器 `cliproxyapi-local-dev` 已确认停止。
- 外层定制版 Sub2API `sub2api-dev` 健康运行，入口 `127.0.0.1:18080`。
- 内层 latest Sub2API `sub2api-upstream-latest` 健康运行，入口 `127.0.0.1:18086`。
- 外层 CPA 上游账号 `cliproxy-local-openai` 保留但 `schedulable=false`。
- 外层 latest 上游账号 `sub2api-latest-openai-upstream` 为 `schedulable=true`，base_url 为 `http://host.docker.internal:18086/v1`。

## 追加账号

- 已将附件 `sub2api-agentIdentity-alive.json` 中 4 个 OpenAI agent identity 账号导入内层 latest。
- 导入接口返回：
  - `account_created=4`
  - `account_failed=0`
- 已通过正式批量更新接口将新账号绑定到内层 OpenAI 分组 `internal-openai-upstream`（`groups.id=2`）。
- 内层当前 OpenAI OAuth 账号共 5 个，均为 `active` 且 `schedulable=true`，均绑定分组 `2`。
- 管理测试接口已对新增账号 `2/3/4/5` 返回 200。

## 公网请求结果

- 使用用户提供的本地 Sub2API 用户 Key，从公网入口 `https://api.aaccx.pw/v1/responses` 发起低成本真实请求。
- 请求返回 200。
- 返回模型：`gpt-5.4-mini-2026-03-17`。
- 响应包含 usage 字段。

## 外层扣费事实

- 外层新增 `usage_facts.id=14923`：
  - `api_key_id=32`
  - `user_id=13`
  - `account_id=2`
  - `billing_status=settled`
- 外层新增 `usage_logs.id=172539`：
  - `api_key_id=32`
  - `user_id=13`
  - `account_id=2`
  - `group_id=5`
  - `requested_model=gpt-5.4-mini`
  - `inbound_endpoint=/v1/responses`
  - `upstream_endpoint=/v1/responses`
  - `actual_cost=0.0007372500`

## 内层实际使用

- 内层同一时间新增 `usage_logs.id=32`：
  - `api_key_id=1`
  - `user_id=1`
  - `account_id=3`
  - `group_id=2`
  - `requested_model=gpt-5.4-mini`
  - `inbound_endpoint=/v1/responses`
  - `upstream_endpoint=/v1/responses`
  - `actual_cost=0.0007372500`

## 结论

- 当前请求链路已验证为：公网入口 -> 外层定制版 Sub2API 计费 -> 内层 latest Sub2API 账号池 -> OpenAI 上游。
- 本次请求没有切换到 CPA：CPA 容器处于停止状态，外层 CPA 账号不可调度，外层使用的上游账号为 `account_id=2`。
- 本轮未记录完整 API Key、内部转发 Key、JWT 或 agent identity 凭证。
