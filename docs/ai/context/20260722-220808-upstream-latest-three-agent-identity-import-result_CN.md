# 内层 latest 追加 3 个 agent identity 账号结果

## 结果

- 已将附件 `C:\Users\yui\Downloads\20260722-210541-sub2-API.json` 中 3 个 OpenAI agent identity 账号追加到内层 latest Sub2API。
- 导入目标：`sub2api-upstream-latest` / `http://127.0.0.1:18086`。
- 导入接口：`POST /api/v1/admin/accounts/data`，`skip_default_group_bind=true`。
- 导入返回：
  - `account_created=3`
  - `account_failed=0`
  - `proxy_created=0`
  - `proxy_failed=0`
  - `errors=1`，但没有账号或代理失败，属于非阻塞 warning
- 新账号 ID：`19,20,21`。
- 已通过 `POST /api/v1/admin/accounts/bulk-update` 绑定到 `internal-openai-upstream`（`groups.id=2`），`bulk_success=3`、`bulk_failed=0`。

## 当前账号池

- 内层当前 OpenAI OAuth 账号总数：21。
- active 且 schedulable 的 OpenAI OAuth 账号：21。
- 新增 3 个账号均为：
  - `platform=openai`
  - `type=oauth`
  - `status=active`
  - `schedulable=true`
  - `group_ids={2}`

## gpt-5.4 验证

- 已显式用 `model_id=gpt-5.4` 调用管理测试接口 `POST /api/v1/admin/accounts/:id/test`。
- 账号 `19,20,21` 均返回 HTTP 200。
- SSE 均包含成功信号，未检测到错误信号。

## 备份

- 导入前管理导出快照：`backups/20260722-220808-upstream-latest-agent-identity-preimport-data.json`
- 注意：管理导出快照包含账号凭据原文，仅本地留存，不应提交。

## 边界

- 本轮只改内层 latest 本地运行态。
- 未修改外层定制版 Sub2API 用户、套餐、流量卡和计费事实。
- 未触碰公网 Nginx、Cloudflare、公网容器或远程数据库。
- 未在文档或输出中记录完整 access token、refresh token、agent private key、JWT、内部转发 Key。
