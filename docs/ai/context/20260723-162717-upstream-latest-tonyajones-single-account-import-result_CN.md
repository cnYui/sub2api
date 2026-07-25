# 内层 latest 单个 TonyJones OpenAI OAuth 账号导入结果

## 范围

- 操作对象：本地内层 latest Sub2API，`http://127.0.0.1:18086`。
- 附件：`tonyajones71651ih@outlook.sub2api.2026-07-23_16-26-13.json`。
- 账号分组：`groups.id=2 / internal-openai-upstream`。
- 未触碰公网 Nginx、Cloudflare、公网容器、外层定制版 Sub2API 用户/套餐/流量卡/计费事实。

## 备份

- 导入前已备份内层 latest 的 `accounts/account_groups/proxies`：
  - `backups/20260723-162717-upstream-latest-tonyajones-single-account-preimport.sql`
- 该备份包含账号凭据原文，不应提交。

## 导入

- 附件账号数：1。
- 账号名：`TonyaJones71651iH@outlook.com`。
- 导入接口：`POST /api/v1/admin/accounts/data`，`skip_default_group_bind=true`。
- 新增账号：`id=78`。

## 分组与状态

- 已通过 `POST /api/v1/admin/accounts/bulk-update` 绑定到 `internal-openai-upstream`（`groups.id=2`），并设为 `active`、`schedulable=true`。
- 批量更新结果：`bulk_success=1`、`bulk_failed=0`。
- SQL 核对新增账号：
  - `id=78`
  - `name=TonyaJones71651iH@outlook.com`
  - `platform=openai`
  - `type=oauth`
  - `status=active`
  - `schedulable=true`
  - `group_ids={2}`

## gpt-5.4 验证

- 已显式用 `model_id=gpt-5.4` 调用管理测试接口 `POST /api/v1/admin/accounts/78/test`。
- 结果：HTTP 200，SSE 中未出现错误事件，测试成功。
- 结论：该账号已入池，并可按 `gpt-5.4` 可用账号看待。

## 当前内层账号统计

- OpenAI OAuth 账号总数：78。
- 当前 `active/schedulable`：43。
- 当前 `error / false`：35。

## 备注

- 首次批量绑定请求因 PowerShell 将单元素 `account_ids` 数组序列化成数字而被后台 400 拒绝；随后使用明确 JSON 数组重试成功，未重复导入账号。
- 当前运行态管理员密码已和容器环境变量脱钩；本次没有修改管理员密码，仅在本地进程内基于持久化 JWT secret 生成短期管理 JWT 调用正式管理 API。
- 生成临时 JWT 时将 `nbf/iat` 回退 5 分钟，以避免本机和容器时钟轻微偏差导致 token 被判定尚未生效。
