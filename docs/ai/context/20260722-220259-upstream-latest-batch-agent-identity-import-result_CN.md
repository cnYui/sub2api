# 内层 latest 批量追加 agent identity 账号结果

## 结果

- 已将两份附件中的 8 个 OpenAI agent identity 账号追加到内层 latest Sub2API。
- 附件：
  - `C:\Users\yui\Downloads\20260722-204504-sub2-API.json`
  - `C:\Users\yui\Downloads\20260722-205243-sub2-API.json`
- 导入目标：`sub2api-upstream-latest` / `http://127.0.0.1:18086`。
- 导入接口：`POST /api/v1/admin/accounts/data`，`skip_default_group_bind=true`。
- 导入返回：
  - 第一份：`account_created=3`、`account_failed=0`、`proxy_created=0`、`proxy_failed=0`
  - 第二份：`account_created=5`、`account_failed=0`、`proxy_created=0`、`proxy_failed=0`
- 新账号 ID：`11,12,13,14,15,16,17,18`。
- 已通过 `POST /api/v1/admin/accounts/bulk-update` 绑定到 `internal-openai-upstream`（`groups.id=2`），`bulk_success=8`、`bulk_failed=0`。

## 当前账号池

- 内层当前 OpenAI OAuth 账号总数：18。
- active 且 schedulable 的 OpenAI OAuth 账号：18。
- 新增 8 个账号均为：
  - `platform=openai`
  - `type=oauth`
  - `status=active`
  - `schedulable=true`
  - `group_ids={2}`

## gpt-5.4 验证

- 已显式用 `model_id=gpt-5.4` 调用管理测试接口 `POST /api/v1/admin/accounts/:id/test`。
- 账号 `11,12,13,14,15,16,17,18` 均返回 HTTP 200。
- SSE 均包含成功信号，未检测到错误信号。

## 备份

- 导入前账号列表快照：`backups/20260722-215541-upstream-latest-agent-identity-preimport-list.json`
- 导入前管理导出快照：`backups/20260722-215541-upstream-latest-agent-identity-preimport-data.json`
- 注意：管理导出快照包含账号凭据原文，仅本地留存，不应提交。

## 边界

- 本轮只改内层 latest 本地运行态。
- 未修改外层定制版 Sub2API 用户、套餐、流量卡和计费事实。
- 未触碰公网 Nginx、Cloudflare、公网容器或远程数据库。
- 未在文档或输出中记录完整 access token、refresh token、agent private key、JWT、内部转发 Key。
