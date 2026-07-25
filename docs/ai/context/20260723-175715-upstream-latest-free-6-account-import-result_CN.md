# 内层 latest 6 个 free OpenAI OAuth 账号导入结果

## 范围

- 操作对象：本地内层 latest Sub2API，`http://127.0.0.1:18086`。
- 附件：
  - `codex-josemcneil61436mwt@outlook.com-free.sub2api.json`
  - `codex-lawrencekelly71527j0@outlook.com-free.sub2api.json`
  - `codex-michelestewart50176c@outlook.com-free.sub2api.json`
  - `codex-michellewilliamson23548k@outlook.com-free.sub2api.json`
  - `codex-chelseacraig9826@outlook.com-free.sub2api.json`
  - `codex-deborahmahoney30954c@outlook.com-free.sub2api.json`
- 账号分组：`groups.id=2 / internal-openai-upstream`。
- 未触碰公网 Nginx、Cloudflare、公网容器、外层定制版 Sub2API 用户/套餐/流量卡/计费事实。

## 备份

- 导入前已备份内层 latest 的 `accounts/account_groups/proxies`：
  - `backups/20260723-175715-upstream-latest-free-6-preimport.sql`

## 导入

- 6 个附件均为单账号文件。
- 导入结果：
  - `account_created=1` × 6
  - `account_failed=0` × 6
  - `proxy_created=0` × 6
  - `proxy_reused=0` × 6
  - `proxy_failed=0` × 6
- 新增账号：
  - `id=80` `josemcneil61436mwt@outlook.com`
  - `id=81` `lawrencekelly71527j0@outlook.com`
  - `id=82` `michelestewart50176c@outlook.com`
  - `id=83` `michellewilliamson23548k@outlook.com`
  - `id=84` `chelseacraig9826@outlook.com`
  - `id=85` `deborahmahoney30954c@outlook.com`

## 分组与状态

- 已通过 `POST /api/v1/admin/accounts/bulk-update` 统一绑定到 `internal-openai-upstream`（`groups.id=2`），并设为 `active`、`schedulable=true`。
- 批量更新结果：`success=6`、`failed=0`。
- SQL 核对新增账号：
  - `id=80..85`
  - `platform=openai`
  - `type=oauth`
  - `status=active`
  - `schedulable=true`
  - `group_ids={2}`

## gpt-5.4 验证

- 已显式用 `model_id=gpt-5.4` 调用管理测试接口 `POST /api/v1/admin/accounts/80..85/test`。
- 管理测试接口 HTTP 200，但 SSE 中均返回错误。
- 错误摘要：`API returned 400: {"detail":"The 'gpt-5.4' model is not supported when using Codex with a ChatGPT account."}`
- 结论：这 6 个账号已成功入池并保持 active/schedulable，但不能按 `gpt-5.4` 可用账号看待。

## 当前内层账号统计

- OpenAI OAuth 账号总数：85。
- 当前 `active/schedulable`：49。
- 当前 `error / false`：36。
