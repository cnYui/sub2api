# 内层 latest redeemed zip 20 个 plus OpenAI OAuth 账号导入结果

## 范围

- 操作对象：本地内层 latest Sub2API，`http://127.0.0.1:18086`。
- 来源 zip：`redeemed_sub2api_20260724040423(1).zip`。
- 解压目录：`tmp/redeemed_sub2api_20260724040423_1/`。
- 来源文件：`sub2api/sub2api.json`。
- 账号分组：`groups.id=2 / internal-openai-upstream`。
- 未触碰公网 Nginx、Cloudflare、公网容器、外层 Sub2API 用户/套餐/流量卡/计费事实。

## 备份

- 导入前已备份内层 latest 的 `accounts/account_groups/proxies`：
  - `backups/20260724-130959-upstream-latest-redeemed-plus-20-preimport.sql`
- 该备份包含账号凭据原文，不应提交。

## 导入前检查

- manifest 账号数：20。
- 源文件账号数：20。
- 文件内 `name` 无重复。
- 文件内 `chatgpt_account_id` 无重复。
- 内层 latest DB 中按 `name` 与 `chatgpt_account_id` 均无匹配记录。
- 20 个账号均为：
  - `platform=openai`
  - `type=oauth`
  - `plan_type=plus`

## 导入与启用

- 导入接口：`POST /api/v1/admin/accounts/data`，`skip_default_group_bind=true`。
- 导入结果：
  - `account_created=20`
  - `account_failed=0`
  - `proxy_created=0`
  - `proxy_reused=0`
  - `proxy_failed=0`
- 新增账号：`id=152..171`。
- 已通过 `POST /api/v1/admin/accounts/bulk-update` 统一绑定到 `internal-openai-upstream`（`groups.id=2`），并设为：
  - `status=active`
  - `schedulable=true`
- 批量更新结果：
  - `success=20`
  - `failed=0`

## gpt-5.4 验证

- 已显式用 `model_id=gpt-5.4` 调用管理测试接口 `POST /api/v1/admin/accounts/152..171/test`。
- 管理测试接口 HTTP 200，但 20 个账号的 SSE 均返回错误。
- 错误摘要：`gpt-5.4` 在 ChatGPT account 的 Codex 模式下不支持。
- 按用户本轮要求，测试后没有禁用这批账号，仍保持 `active/schedulable=true`。

## 最终核对

- 新增账号 `id=152..171`：
  - `active=20`
  - `schedulable=20`
  - `active/schedulable=20`
  - `group_id=2` 绑定数：20
- 内层 latest OpenAI OAuth 账号总数：171。
- 当前 `active/schedulable`：113。
- 当前非 `active/schedulable`：58。

