# 停止本地 CPA 并切到内层 latest Sub2API 计划

时间：2026-07-22 19:27

## 目标

- 外层本地定制版 Sub2API 保持用户 Key、套餐、流量卡、`usage_facts` 和扣费事实源。
- 内层 latest Sub2API 容器替代 CPA，负责 GPT 凭证、账号调度和上游请求。
- 本地 CPA 容器 `cliproxyapi-local-dev` 暂停，CPA 数据库账号记录保留，方便回滚。

## 本地拓扑

- 外层定制版：`sub2api-dev`，入口 `http://127.0.0.1:18080`
- 内层 latest：`sub2api-upstream-latest`，入口 `http://127.0.0.1:18086`
- CPA：`cliproxyapi-local-dev`，入口 `https://127.0.0.1:8317`

## 步骤

1. 在外层本地创建或复用 `sub2api-latest-openai-upstream` OpenAI APIKey 账号。
2. 账号指向 `http://host.docker.internal:18086/v1`，API Key 使用内层 latest 生成的内部转发 Key。
3. 绑定外层所有 OpenAI 分组，启用 pool mode，保留 401/403/429/502/503/504 同账号重试。
4. 将旧 CPA 账号 `cliproxy-local-openai` 设为不可调度，保留账号记录和分组绑定。
5. 停止本地 CPA 容器 `cliproxyapi-local-dev`。
6. 用外层测试 Key 验证 `/v1/models` 和 `/v1/responses`，核对请求走到内层 latest，外层继续写本地计费事实。

## 回滚

- 启动 `cliproxyapi-local-dev`。
- 将 `cliproxy-local-openai` 恢复为可调度。
- 将 `sub2api-latest-openai-upstream` 设为不可调度或删除其分组绑定。

## 边界

- 不触碰公网 Nginx、Cloudflare、公网数据库或公网容器。
- 不在文档记录完整 GPT 凭证、JWT、内部 API Key。
