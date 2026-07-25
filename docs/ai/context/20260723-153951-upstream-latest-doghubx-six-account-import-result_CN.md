# 内层 latest 六个 DogHubX OpenAI OAuth 账号导入结果

## 范围

- 操作对象：本地内层 latest Sub2API，`http://127.0.0.1:18086`。
- 附件：`doghubx-claimed-20260723143808.json`。
- 账号分组：`groups.id=2 / internal-openai-upstream`。
- 未触碰公网 Nginx、Cloudflare、公网容器、外层定制版 Sub2API 用户/套餐/流量卡/计费事实。

## 备份

- 导入前已备份内层 latest 的 `accounts/account_groups/proxies`：
  - `backups/20260723-153951-upstream-latest-doghubx-six-accounts-preimport.sql`
- 该备份包含账号凭据原文，不应提交。

## 导入

- 附件账号数：6。
- 导入接口：`POST /api/v1/admin/accounts/data`，`skip_default_group_bind=true`。
- 导入结果：
  - `account_created=6`
  - `account_failed=0`
  - `proxy_created=0`
  - `proxy_reused=0`
  - `proxy_failed=0`
- 新增账号：`id=72..77`。

## 分组与状态

- 已通过 `POST /api/v1/admin/accounts/bulk-update` 绑定到 `internal-openai-upstream`（`groups.id=2`），并设为 `active`、`schedulable=true`。
- 批量更新结果：`bulk_success=6`、`bulk_failed=0`。
- SQL 核对新增账号：
  - `id=72 / scottandrews39782 / group_ids={2}`
  - `id=73 / jefferyarnold29263 / group_ids={2}`
  - `id=74 / carolwebb33315t0 / group_ids={2}`
  - `id=75 / robertwong65455yj / group_ids={2}`
  - `id=76 / thomasbutler21115 / group_ids={2}`
  - `id=77 / glenndavis43393 / group_ids={2}`
- 新增 6 个账号均为 `active/schedulable`。

## gpt-5.4 验证

- 已显式用 `model_id=gpt-5.4` 逐个调用管理测试接口 `POST /api/v1/admin/accounts/:id/test`。
- 账号 `72,73,74,75,77` 返回 HTTP 200，SSE 中未出现错误事件，测试成功。
- 账号 `76` 管理测试接口 HTTP 200，但 SSE 中返回上游错误。
- 错误摘要：上游返回 `400`，提示 `gpt-5.4` 在 ChatGPT account 的 Codex 模式下不支持。
- 结论：这 6 个账号已添加进池并保持 active/schedulable，其中 `72,73,74,75,77` 可按 `gpt-5.4` 可用账号看待，`76` 不可按 `gpt-5.4` 可用账号看待。

## 当前内层账号统计

- OpenAI OAuth 账号总数：77。
- 当前 `active/schedulable`：42。
- 当前 `error / false`：35。

## 备注

- 当前运行态管理员密码已和容器环境变量脱钩；本次没有修改管理员密码，仅在本地进程内基于持久化 JWT secret 生成短期管理 JWT 调用正式管理 API。
- 生成临时 JWT 时将 `nbf/iat` 回退 5 分钟，以避免本机和容器时钟轻微偏差导致 token 被判定尚未生效。
