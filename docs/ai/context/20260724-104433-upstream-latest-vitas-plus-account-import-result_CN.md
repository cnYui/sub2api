# 内层 latest vitas plus Codex 账号导入结果

## 范围

- 操作对象：本地内层 latest Sub2API，`http://127.0.0.1:18086`。
- 附件：`codex-vitas-antigen.0t@icloud.com-plus.json`。
- 账号分组：`groups.id=2 / internal-openai-upstream`。
- 未触碰公网 Nginx、Cloudflare、公网容器、外层定制版 Sub2API 用户/套餐/流量卡/计费事实。

## 备份

- 导入前已备份内层 latest 的 `accounts/account_groups/proxies`：
  - `backups/20260724-104253-upstream-latest-vitas-plus-preimport.sql`
- 该备份包含账号凭据原文，不应提交。

## 导入

- 附件账号数：1。
- 账号邮箱：`vitas-antigen.0t@icloud.com`。
- 导入接口：`POST /api/v1/admin/accounts/import/codex-session`。
- 导入参数：
  - `group_ids=[2]`
  - `skip_default_group_bind=true`
  - `confirm_mixed_channel_risk=true`
  - `concurrency=3`
  - `priority=50`
- 导入结果：
  - `total=1`
  - `created=1`
  - `updated=0`
  - `skipped=0`
  - `failed=0`
- 新增账号：`id=91`。

## 分组与状态

- SQL 核对新增账号：
  - `id=91`
  - `name=vitas-antigen.0t@icloud.com`
  - `platform=openai`
  - `type=oauth`
  - `status=active`
  - `schedulable=true`
  - `credentials.email=vitas-antigen.0t@icloud.com`
  - `credentials.plan_type=plus`
  - `group_ids={2}`

## gpt-5.4 验证

- 已显式用 `model_id=gpt-5.4` 调用管理测试接口 `POST /api/v1/admin/accounts/91/test`。
- 管理测试接口 HTTP 200，SSE 最终返回 `test_complete`，`success=true`。
- 结论：该账号已成功入池，且可按 `gpt-5.4` 可用账号看待。

## 当前内层账号统计

- OpenAI OAuth 账号总数：91。
- 当前 `active/schedulable`：33。
- 当前 `error / false`：58。
