# 内层 latest sub2api-account 100 个 free OpenAI OAuth 账号导入结果

## 范围

- 操作对象：本地内层 latest Sub2API，`http://127.0.0.1:18086`。
- 来源文件：`C:/Users/yui/Downloads/sub2api-account.json`。
- 账号分组：`groups.id=2 / internal-openai-upstream`。
- 未触碰公网 Nginx、Cloudflare、公网容器、外层 Sub2API 用户/套餐/流量卡/计费事实。

## 备份

- 导入前已备份内层 latest 的 `accounts/account_groups/proxies`：
  - `backups/20260725-081548-upstream-latest-sub2api-account-free-100-preimport.sql`
- 该备份包含账号凭据原文，不应提交。

## 导入前检查

- 源文件账号数：100。
- 源文件代理数：0。
- 文件内 `name` 无重复。
- 文件内凭据邮箱无重复。
- 内层 latest DB 中按 `name` 与凭据邮箱均无匹配记录。
- 100 个账号均为：
  - `platform=openai`
  - `type=oauth`
  - `plan_type=free`

## 导入与启用

- 导入接口：`POST /api/v1/admin/accounts/data`，`skip_default_group_bind=true`。
- 导入结果：
  - `account_created=100`
  - `account_failed=0`
  - `proxy_created=0`
  - `proxy_reused=0`
  - `proxy_failed=0`
- 新增账号：`id=195..294`。
- 已通过 `POST /api/v1/admin/accounts/bulk-update` 统一绑定到 `internal-openai-upstream`（`groups.id=2`），并设为：
  - `status=active`
  - `schedulable=true`
- 批量更新结果：
  - `success=100`
  - `failed=0`

## 验证

- 按用户此前要求，本轮不跑模型测试。
- SQL 核对新增账号 `id=195..294`：
  - `active=100`
  - `schedulable=100`
  - `active/schedulable=100`
  - `group_id=2` 绑定数：100
- 内层 latest OpenAI OAuth 账号总数：294。
- 当前 `active/schedulable`：236。
- 当前非 `active/schedulable`：58。

