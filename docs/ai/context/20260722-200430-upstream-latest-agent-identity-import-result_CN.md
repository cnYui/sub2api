# 内层 latest 追加 agent identity 账号结果

## 结果

- 已将附件 `sub2api-agentIdentity-alive (1).json` 中 5 个 OpenAI agent identity 账号追加到内层 latest Sub2API。
- 导入目标：`sub2api-upstream-latest` / `http://127.0.0.1:18086`。
- 导入接口：`POST /api/v1/admin/accounts/data`，`skip_default_group_bind=true`。
- 导入返回：
  - `account_created=5`
  - `account_failed=0`
  - `proxy_created=0`
  - `proxy_reused=0`
  - `proxy_failed=0`
- 新账号 ID：`6,7,8,9,10`。
- 已通过正式批量更新接口将新账号绑定到内层 OpenAI 分组 `internal-openai-upstream`（`groups.id=2`）。

## 当前账号池

- 内层当前 OpenAI OAuth 账号总数：10。
- active 且 schedulable 的 OpenAI OAuth 账号：10。
- 绑定 `groups.id=2` 的 OpenAI OAuth 账号：10。
- 新增账号 `6-10` 均为：
  - `platform=openai`
  - `type=oauth`
  - `status=active`
  - `schedulable=true`
  - `group_ids={2}`
  - `priority=50`
  - `rate_multiplier=1.0`

## 验证

- 管理测试接口 `POST /api/v1/admin/accounts/:id/test` 对新增账号 `6,7,8,9,10` 均返回 HTTP 200。
- SSE 响应均包含成功信号，未检测到错误信号。
- 本轮未展开或记录完整 access token、refresh token、agent private key、JWT、内部转发 Key。

## 备份

- 导入前备份：`backups/20260722-194856-upstream-latest-agent-identity-preimport.sql`
- 大小：约 14 KB。
- 行数：48。
- 验证：文件可读、非空。

## 边界

- 本轮只改内层 latest 本地运行态。
- 未修改外层定制版 Sub2API 用户、套餐、流量卡和计费事实。
- 未触碰公网 Nginx、Cloudflare、公网容器或远程数据库。
