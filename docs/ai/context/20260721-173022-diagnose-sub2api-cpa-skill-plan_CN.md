# diagnose-sub2api-cpa skill 创建计划

## 背景

用户希望把 Sub2API 每次新代码部署和版本更新后的重复排查沉淀为个人 Codex skill。目标覆盖 Sub2API 公网入口、CLIProxyAPI/CPA 上游、用户面板扣费展示、TLS/证书、CPA 凭证失效和常见 `auth_unavailable`/502/429 问题。

## 必须保留的边界

- 不把完整 Sub2API 用户 API Key、CLIProxyAPI 内部转发密钥、HMAC、SMTP、支付密钥写进 skill、项目文档、日志或命令历史。
- 公网请求真实消耗额度，skill 必须要求先记录请求前基线、使用唯一测试标记，并在请求后核对 `usage_facts`、`usage_logs` 和用户面板。
- 图片生成成本和耗时高于文本请求，脚本需显式开关，skill 在完整验收时要求执行但不能让普通 smoke 默认批量跑图。
- 排查默认只读；修改 DB、Redis、容器、Nginx、Cloudflare 或 CPA auth 文件前必须另写计划、备份并获得明确授权。

## 技术入口

- Sub2API 当前公开模型 API 只接受 `/v1/*`：`/v1/models`、`/v1/responses`、`/v1/chat/completions`、`/v1/images/generations`。
- 裸 `/models`、`/responses`、`/chat/completions`、`/images/generations` 会被 Sub2API 明确返回 `INVALID_BASE_URL`。
- 用户面板路由为 `/dashboard`，使用 `/api/v1/usage/dashboard/stats` 和 `/api/v1/usage/dashboard/quota`；明细页为 `/usage`。
- 当前公网运行链路仍按 AGENTS 记忆：`Cloudflare Tunnel -> nginx 127.0.0.1:8080 -> sub2api-candidate 127.0.0.1:18084 -> CLIProxyAPI 127.0.0.1:8317`。
- CPA 仓库为 `D:\CodeWorkSpace\CLIProxyAPI-private`，其 `/v1` 路由覆盖 models、responses、chat completions、images generations；8317 当前是 HTTPS/TLS。

## 交付

- 创建个人 skill：`C:\Users\yui\.codex\skills\diagnose-sub2api-cpa`。
- `SKILL.md` 写核心排查流程、停止条件、分段定位和危险动作禁令。
- `references/runtime-map.md` 写 Sub2API/CPA 现有链路、路由、错误契约、SQL 和日志查询参考。
- `scripts/public_smoke.ps1` 提供只从环境变量读取 Key 的公网 API smoke 工具，隐藏密钥，输出唯一 marker 和响应摘要。
- 完成后运行 skill quick validate 和脚本帮助验证，不执行真实公网请求。
