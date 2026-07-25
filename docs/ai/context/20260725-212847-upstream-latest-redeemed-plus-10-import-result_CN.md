# 内层 latest redeemed plus 10 账号导入结果

时间：2026-07-25 21:28:47

## 范围

- 操作对象：本地内层 latest Sub2API，`http://127.0.0.1:18086`。
- 来源文件：`C:/Users/yui/Downloads/redeemed_sub2api_20260725113032/sub2api/sub2api.json`。
- 账号分组：`groups.id=2 / internal-openai-upstream`。
- 未触碰外层 `18080` 的计费、套餐、用户、流量卡和历史用量。

## 备份

- 导入前已备份内层 latest 的 `accounts/account_groups/proxies`：
  - `backups/20260725-212847-upstream-latest-redeemed-plus-10-preimport.sql`
- 该备份包含账号凭据原文，不应提交。

## 导入前检查

- 文件结构：标准 Sub2API 导出。
- 源文件账号数：10。
- 源文件代理数：0。
- 文件内 `name` 无重复。
- 文件内 `chatgpt_account_id` 无重复。
- 内层 latest DB 中按 `name` 与 `chatgpt_account_id` 均无匹配记录。
- 10 个账号均为：
  - `platform=openai`
  - `type=oauth`
  - `plan_type=plus`

## 导入与启用

- 导入接口：`POST /api/v1/admin/accounts/data`，`skip_default_group_bind=true`。
- 导入结果：
  - `account_created=10`
  - `account_failed=0`
  - `proxy_created=0`
  - `proxy_reused=0`
  - `proxy_failed=0`
- 新增账号：`id=298..307`。
- 已通过 `POST /api/v1/admin/accounts/bulk-update` 统一绑定到 `internal-openai-upstream`（`groups.id=2`），并设为：
  - `status=active`
  - `schedulable=true`
- 批量更新结果：
  - `success=10`
  - `failed=0`

## 验证

- 本轮不跑模型测试。
- SQL 核对新增账号 `id=298..307`：
  - 新增记录数：10
  - `active/schedulable`：10
  - `deleted_at is null`：10
  - `group_id=2` 绑定数：10
- `http://127.0.0.1:18086/health` 返回 `{"status":"ok"}`。
- 内层 latest OpenAI OAuth 全量账号数：307。
- 当前全量 `active/schedulable`：245。
- 当前全量非 `active/schedulable`：62。
- 当前未删除 OpenAI OAuth：240，其中 `active/schedulable`：215，非 `active/schedulable`：25。
