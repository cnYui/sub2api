# 内层 latest accountflow 60 个 free OpenAI OAuth 账号导入结果

## 范围

- 操作对象：本地内层 latest Sub2API，`http://127.0.0.1:18086`。
- 来源目录：`sub2_json-60-20260724T023051Z`。
- 来源文件：`accountflow-redeem-sub2.json`。
- 账号分组：`groups.id=2 / internal-openai-upstream`。
- 未触碰公网 Nginx、Cloudflare、公网容器、外层 Sub2API 用户/套餐/流量卡/计费事实。

## 备份

- 导入前已备份内层 latest 的 `accounts/account_groups/proxies`：
  - `backups/20260724-113342-upstream-latest-accountflow-free-60-preimport.sql`
- 该备份包含账号凭据原文，不应提交。

## 导入前检查

- 源文件账号数：60。
- 文件内邮箱无重复。
- 文件内 ChatGPT 身份无重复。
- 内层 latest DB 中无匹配邮箱或 ChatGPT 身份记录。
- 60 个账号均为：
  - `platform=openai`
  - `type=oauth`
  - `plan_type=free`

## 导入

- 导入接口：`POST /api/v1/admin/accounts/data`，`skip_default_group_bind=true`。
- 导入结果：
  - `account_created=60`
  - `account_failed=0`
  - `proxy_created=0`
  - `proxy_reused=0`
  - `proxy_failed=0`
- 新增账号：`id=92..151`。

## 分组与状态

- 已通过 `POST /api/v1/admin/accounts/bulk-update` 统一绑定到 `internal-openai-upstream`（`groups.id=2`）。
- 初始批量更新结果：`success=60`、`failed=0`。
- `gpt-5.4` 测试后，为避免污染可调度池，已将 60 个新增账号统一设为：
  - `status=active`
  - `schedulable=false`
- 最终 SQL 核对：
  - `id=92..151`
  - `active=60`
  - `schedulable=0`
  - `active_unschedulable=60`
  - `group_ids={2}`

## gpt-5.4 验证

- 已显式用 `model_id=gpt-5.4` 调用管理测试接口 `POST /api/v1/admin/accounts/92..151/test`。
- 管理测试接口 HTTP 200，但 60 个账号的 SSE 均返回错误。
- 错误摘要：`gpt-5.4` 在 ChatGPT account 的 Codex 模式下不支持。
- 结论：这 60 个账号已成功入库并绑定分组，但不能按 `gpt-5.4` 可用账号看待。

## 当前内层账号统计

- OpenAI OAuth 账号总数：151。
- 当前 `active/schedulable`：33。
- 当前 `error / false`：118。
