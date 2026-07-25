# 内层 latest 单账号追加结果

## 结果

- 已将附件 `C:\Users\yui\Downloads\joshuawoodward36403m5f@outlook.sub2api.2026-07-23_09-39-00.json` 中的 1 个 OpenAI agent identity 账号追加到内层 latest Sub2API。
- 导入目标：`sub2api-upstream-latest` / `http://127.0.0.1:18086`。
- 导入接口：`POST /api/v1/admin/accounts/data`，`skip_default_group_bind=true`。
- 导入返回：
  - `account_created=1`
  - `account_failed=0`
  - `proxy_created=0`
  - `proxy_failed=0`
  - `errors=1`，但没有账号或代理失败，属于非阻塞 warning
- 新账号 ID：`33`。
- 已通过 `POST /api/v1/admin/accounts/bulk-update` 绑定到 `internal-openai-upstream`（`groups.id=2`），并设为 `active`、`schedulable=true`，`bulk_success=1`、`bulk_failed=0`。

## 当前账号池

- 数据库核对时，内层 OpenAI OAuth 账号总数：33。
- 当前 `status=active` 且 `schedulable=true` 的 OpenAI OAuth 账号：1。
- 其余 32 个 OpenAI OAuth 账号当前为 `error / false`。
- 新增账号 `33` 为：
  - `platform=openai`
  - `type=oauth`
  - `status=active`
  - `schedulable=true`
  - `group_ids={2}`

## gpt-5.4 验证

- 已显式用 `model_id=gpt-5.4` 调用管理测试接口 `POST /api/v1/admin/accounts/33/test`。
- 账号 `33` 返回 HTTP 200。
- SSE 包含成功信号，未检测到错误信号。

## 备份

- 导入前导出快照文件：`backups/20260723-094307-upstream-latest-single-account-preimport-data.json`
- 该导出实际内容是空壳 JSON：`exported_at`、`proxies: []`、`accounts: []`。
- 这份文件不能当作完整账号回滚快照使用；若要回滚本次新增账号，按 `id=33` 删除即可。

## 边界

- 本轮只改内层 latest 本地运行态。
- 未修改外层定制版 Sub2API 用户、套餐、流量卡和计费事实。
- 未触碰公网 Nginx、Cloudflare、公网容器或远程数据库。
- 未在文档或输出中记录完整 access token、refresh token、agent private key、JWT、内部转发 Key。
