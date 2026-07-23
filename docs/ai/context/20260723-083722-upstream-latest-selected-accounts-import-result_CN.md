# 内层 latest 追加 selected accounts 结果

## 结果

- 已将附件 `D:\xwechat_files\wxid_4lkns2swsaad22_1df8\msg\file\2026-07\sub2api-selected-accounts-2026-07-22134906.json` 中 10 个 OpenAI agent identity 账号追加到内层 latest Sub2API。
- 导入目标：`sub2api-upstream-latest` / `http://127.0.0.1:18086`。
- 导入接口：`POST /api/v1/admin/accounts/data`，`skip_default_group_bind=true`。
- 导入返回：
  - `account_created=10`
  - `account_failed=0`
  - `proxy_created=0`
  - `proxy_failed=0`
  - `errors=1`，但没有账号或代理失败，属于非阻塞 warning
- 新账号 ID：`23,24,25,26,27,28,29,30,31,32`。
- 已通过 `POST /api/v1/admin/accounts/bulk-update` 绑定到 `internal-openai-upstream`（`groups.id=2`），`bulk_success=10`、`bulk_failed=0`。

## 当前账号池

- 数据库核对时，内层 OpenAI OAuth 账号总数：32。
- 当前 `status=active` 且 `schedulable=true` 的 OpenAI OAuth 账号：10。
- 新增 10 个账号均为：
  - `platform=openai`
  - `type=oauth`
  - `status=active`
  - `schedulable=true`
  - `group_ids={2}`

## gpt-5.4 验证

- 已显式用 `model_id=gpt-5.4` 调用管理测试接口 `POST /api/v1/admin/accounts/:id/test`。
- 账号 `23..32` 均为管理接口 HTTP 200，但 SSE 内部均返回错误信号。
- 错误摘要：上游返回 `402 deactivated_workspace`。
- 结论：这 10 个账号已添加进池，但本轮真实上游测试不可用，不应按可用账号看待。

## 备份

- 本轮导入前尝试保存管理导出快照：`backups/20260723-083722-upstream-latest-selected-accounts-preimport-data.json`
- 注意：管理导出快照可能包含账号凭据原文，仅本地留存，不应提交。

## 边界

- 本轮只改内层 latest 本地运行态。
- 未修改外层定制版 Sub2API 用户、套餐、流量卡和计费事实。
- 未触碰公网 Nginx、Cloudflare、公网容器或远程数据库。
- 未在文档或输出中记录完整 access token、refresh token、agent private key、JWT、内部转发 Key。
