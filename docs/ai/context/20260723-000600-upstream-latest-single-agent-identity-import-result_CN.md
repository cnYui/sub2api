# 内层 latest 追加单个 agent identity 账号结果

时间：2026-07-23 00:06 JST

## 结果

- 已将 `C:\Users\yui\Downloads\hectordenigris42356@outlook.com.json` 中的 1 个 OpenAI agent identity 账号追加到内层 latest Sub2API。
- 导入目标：`sub2api-upstream-latest` / `http://127.0.0.1:18086`。
- 导入接口：`POST /api/v1/admin/accounts/data`，`skip_default_group_bind=true`。
- 新账号 ID：`22`。
- 新账号名：`hectordenigris42356@outlook.com`。
- 已通过 `POST /api/v1/admin/accounts/bulk-update` 绑定到 `internal-openai-upstream`（`groups.id=2`）。
- 按用户要求，本轮未重启 Docker。

## 当前账号池

- 内层当前 OpenAI OAuth 账号总数：22。
- active 且 schedulable 的 OpenAI OAuth 账号：22。
- 新账号状态：
  - `platform=openai`
  - `type=oauth`
  - `status=active`
  - `schedulable=true`
  - `concurrency=10`
  - `priority=1`
  - `group_ids={2}`
  - `group_names={internal-openai-upstream}`

## 验证

- 管理批量更新返回：`success=1`、`failed=0`、`success_ids=[22]`。
- 已显式使用 `model_id=gpt-5.4` 调用管理测试接口 `POST /api/v1/admin/accounts/22/test`。
- 测试接口返回 HTTP 200，SSE 内容检测到完成信号，未检测到错误信号。
- Redis active 调度池 `sched:2:openai:single:v446` 中账号 `22` 的 `ZSCORE=0`，说明新账号已进入当前内层 OpenAI 单账号调度池。

## 备份

- 导入前管理导出快照：`backups/20260723-000458-upstream-latest-single-agent-preimport-data.json`
- 注意：管理导出快照包含账号凭据原文，仅本地留存，不应提交。

## 边界

- 本轮只改内层 latest 本地运行态账号池。
- 未修改外层定制版 Sub2API 用户、套餐、流量卡和计费事实。
- 未触碰公网 Nginx、Cloudflare、公网容器或远程数据库。
- 未重启 Docker。
- 未在文档或回复中记录完整 access token、refresh token、agent private key、JWT、内部转发 Key。
