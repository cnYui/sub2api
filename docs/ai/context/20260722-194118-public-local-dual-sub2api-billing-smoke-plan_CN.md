# 公网/本地双 Sub2API 扣费链路验证计划

时间：2026-07-22 19:41

## 目标

- 停止本地 CPA 容器，避免测试请求落回 CPA。
- 使用用户提供的本地 Sub2API 用户 Key 发起真实 OpenAI 请求。
- 验证外层定制版 Sub2API `18080` 产生扣费事实。
- 验证内层 latest Sub2API `18086` 实际请求上游模型。

## 验证点

- CPA 容器保持 `Exited`。
- 外层账号调度使用 `sub2api-latest-openai-upstream`，不使用 `cliproxy-local-openai`。
- 外层 `usage_facts` 新增并 settled。
- 外层 `usage_logs` 记录 `actual_cost`。
- 内层 `sub2api-upstream-latest` 日志出现对应 `/v1/responses` 成功请求和 OpenAI account_id。

## 安全边界

- API Key 只放环境变量使用，不写入文档。
- 不输出完整 Key、内部转发 Key、JWT 或 agent identity 凭证。
- 不触碰公网 Nginx、Cloudflare、公网数据库、公网容器。
