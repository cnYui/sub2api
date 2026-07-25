# 内层 latest 单个 free OpenAI OAuth 账号导入结果

## 范围

- 操作对象：本地内层 latest Sub2API，`http://127.0.0.1:18086`。
- 附件：`timothytorres43800l@outlook.sub2api.2026-07-23_18-34-39.json`。
- 账号分组：`groups.id=2 / internal-openai-upstream`。
- 未触碰公网 Nginx、Cloudflare、公网容器、外层定制版 Sub2API 用户/套餐/流量卡/计费事实。

## 备份

- 导入前已备份内层 latest 的 `accounts/account_groups/proxies`：
  - `backups/20260723-183531-upstream-latest-single-free-account-preimport.sql`

## 导入

- 附件账号数：1。
- 账号名：`TimothyTorres43800l@outlook.com`。
- 导入接口：`POST /api/v1/admin/accounts/data`，`skip_default_group_bind=true`。
- 导入结果：
  - `account_created=1`
  - `account_failed=0`
  - `proxy_created=0`
  - `proxy_reused=0`
  - `proxy_failed=0`
- 新增账号：`id=86`。

## 分组与状态

- 已通过 `POST /api/v1/admin/accounts/bulk-update` 绑定到 `internal-openai-upstream`（`groups.id=2`），并设为 `active`、`schedulable=true`。
- 批量更新结果：`success=1`、`failed=0`。
- SQL 核对新增账号：
  - `id=86`
  - `name=TimothyTorres43800l@outlook.com`
  - `platform=openai`
  - `type=oauth`
  - `status=active`
  - `schedulable=true`
  - `group_ids={2}`

## gpt-5.4 验证

- 已显式用 `model_id=gpt-5.4` 调用管理测试接口 `POST /api/v1/admin/accounts/86/test`。
- 管理测试接口 HTTP 200，SSE 最终返回 `test_complete`，`success=true`。
- 结论：该账号已成功入池，且可按 `gpt-5.4` 可用账号看待。

## 当前内层账号统计

- OpenAI OAuth 账号总数：86。
- 当前 `active/schedulable`：50。
- 当前 `error / false`：36。
