# 内层 latest 追加 free 10 账号结果

## 结果

- 已将附件 `C:\Users\yui\Downloads\sub2-free-10.json` 中 10 个 OpenAI OAuth 账号追加到内层 latest Sub2API。
- 导入目标：`sub2api-upstream-latest` / `http://127.0.0.1:18086`。
- 导入接口：`POST /api/v1/admin/accounts/data`，`skip_default_group_bind=true`。
- 导入返回：
  - `account_created=10`
  - `account_failed=0`
  - `proxy_created=0`
  - `proxy_failed=0`
  - `errors=1`，但没有账号或代理失败，属于非阻塞 warning
- 新账号 ID：`54..63`。
- 已通过 `POST /api/v1/admin/accounts/bulk-update` 绑定到 `internal-openai-upstream`（`groups.id=2`），并设为 `active`、`schedulable=true`，`bulk_success=10`、`bulk_failed=0`。

## 当前账号池

- 数据库核对时，内层 OpenAI OAuth 账号总数：63。
- 当前 `status=active` 且 `schedulable=true` 的 OpenAI OAuth 账号：31。
- 当前 `status=error` 且 `schedulable=false` 的 OpenAI OAuth 账号：32。
- 新增账号 `54..63` 均为：
  - `platform=openai`
  - `type=oauth`
  - `status=active`
  - `schedulable=true`
  - `group_ids={2}`

## gpt-5.4 验证

- 已显式用 `model_id=gpt-5.4` 逐个调用管理测试接口 `POST /api/v1/admin/accounts/:id/test`。
- 账号 `54..63` 的管理接口均返回 HTTP 200，但 SSE 内部均返回错误信号。
- 错误摘要：上游返回 `400`，提示 `gpt-5.4` 不支持 Free account。
- 结论：这 10 个账号已添加进池并保持 active/schedulable，但不能按 `gpt-5.4` 可用账号看待。

## 备份

- 导入前 SQL 备份：`backups/20260723-104809-upstream-latest-free-10-preimport.sql`
- 注意：该备份包含账号凭据原文，仅本地留存，不应提交。

## 边界

- 本轮只改内层 latest 本地运行态。
- 未修改外层定制版 Sub2API 用户、套餐、流量卡和计费事实。
- 未触碰公网 Nginx、Cloudflare、公网容器或远程数据库。
- 未在文档或输出中记录完整 access token、refresh token、agent private key、JWT、内部转发 Key。
