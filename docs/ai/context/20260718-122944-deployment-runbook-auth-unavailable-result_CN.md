# Sub2API + CLIProxyAPI 部署 Runbook 文档结果

## 完成内容

- 新增正式部署文档：`docs/SUB2API_CLIPROXYAPI_DEPLOYMENT_RUNBOOK_CN.md`。
- 更新部署目录入口：`deploy/README.md` 增加中文 Runbook 链接。
- 更新长期记忆：`AGENTS.md` 记录 8317 当前是 HTTPS/TLS，以及历史 `auth_unavailable` / 502 根因。
- 更新计划上下文：`docs/ai/context/20260718-122226-deployment-runbook-auth-unavailable-plan_CN.md`。

## Runbook 覆盖范围

- 当前公网链路和职责边界：
  `Cloudflare Tunnel -> nginx 127.0.0.1:8080 -> sub2api-candidate 127.0.0.1:18084 -> CLIProxyAPI 127.0.0.1:8317`。
- Sub2API 与 CLIProxyAPI 边界：
  Sub2API 是公网入口、用户 Key、计费和用量事实源；CLIProxyAPI 是内网账号池、OAuth、协议转换和调度上游。
- 部署前检查：
  git 状态、容器/project/volume/端口、Nginx 指向、Postgres/Redis 备份和备份可读验证。
- 配置要求：
  Sub2API 上游账号指向 `https://host.docker.internal:8317/v1`；8317 当前是 TLS；Sub2API 用户 Key 不能直接打 CLIProxyAPI；内部转发密钥只做脱敏说明。
- 发布流程：
  构建 `sub2api-candidate:*` 镜像，dry-run `promote-sub2api-candidate.sh`，再只替换 18084 应用容器。
- 验收流程：
  18084、8080、公网 health，真实公网 `/v1/chat/completions`，`usage_facts.billing_status=settled`。
- `auth_unavailable` / 502 排障：
  历史根因是 Sub2API account 1 被临时失败状态/Redis 调度快照排除，日志 `excluded_account_count=1`；不是 CLIProxyAPI 调度器坏。恢复动作写成先备份、先确认、优先管理接口清临时状态、重启应用触发调度快照重建，不让新人直接删除业务数据或泛删 Redis。

## 验证结果

- `git diff --check`：通过，无输出。
- `rg -n "''base_url''|''api_key''|''pool_mode''|''usage_log''|''effects''" docs/SUB2API_CLIPROXYAPI_DEPLOYMENT_RUNBOOK_CN.md`：无匹配，SQL 示例中的错误 shell 引号已修正。
- 敏感信息粗扫：
  `rg -n "sk-|HMAC|secret|password|token|api key|API Key|Bearer [A-Za-z0-9_\\-]{20,}" ...` 只命中通用说明、占位符和既有 README 示例；未发现完整真实密钥。

## 运行态影响

- 未部署。
- 未构建镜像。
- 未重启容器。
- 未修改 PostgreSQL、Redis、Nginx、Cloudflare Tunnel 或 CLIProxyAPI 配置。

## 工作区提醒

本轮只新增/修改部署文档相关文件。工作区中已有的后端代码改动和以下未跟踪上下文不是本轮引入，未处理：

- `docs/ai/context/20260717-111648-delete-using-superpowers-plan_CN.md`
- `docs/ai/context/20260717-111743-delete-using-superpowers-result_CN.md`
- `docs/ai/context/20260718-122941-refund-failure-investigation-result_CN.md`
- `docs/ai/context/20260718-cliproxyapi-usage-event-billing-design-plan_CN.md`
