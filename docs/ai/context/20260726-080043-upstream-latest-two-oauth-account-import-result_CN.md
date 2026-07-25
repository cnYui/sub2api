# 内层 latest 2 个 OpenAI OAuth 账号导入结果

时间：2026-07-26 08:00:43

## 范围

- 操作对象：本地内层 latest Sub2API，`http://127.0.0.1:18086`。
- 来源文件：`C:/Users/yui/Downloads/karltautges03146+gvx85ii@outlook.sub2api.2026-07-26_07-59-04.json`。
- 账号分组：`groups.id=2 / internal-openai-upstream`。
- 未触碰外层 `18080` 的计费、套餐、用户、流量卡和历史用量。

## 备份

- 导入前已备份内层 latest 的 `accounts/account_groups/proxies`：
  - `backups/20260726-080043-upstream-latest-two-oauth-account-preimport.sql`
- 该备份包含账号凭据原文，不应提交。

## 导入前检查

- 文件结构：标准 Sub2API 导出。
- 源文件账号数：2。
- 源文件代理数：0。
- 文件内 `name` 无重复。
- 文件内 `chatgpt_account_id` 有 1 组重复。
- 两个账号的 refresh/access token 指纹不同。
- 内层 latest DB 中按 `name` 与 `chatgpt_account_id` 均无匹配记录。
- 2 个账号均为：
  - `platform=openai`
  - `type=oauth`
- 来源文件未声明 `plan_type`。

## 导入与绑定

- 导入接口：`POST /api/v1/admin/accounts/data`，`skip_default_group_bind=true`。
- 导入结果：
  - `account_created=2`
  - `account_failed=0`
  - `proxy_created=0`
  - `proxy_reused=0`
  - `proxy_failed=0`
- 新增账号：`id=308..309`。
- 已通过 `POST /api/v1/admin/accounts/bulk-update` 绑定到 `internal-openai-upstream`（`groups.id=2`）。
- 批量更新接口返回：
  - `success=2`
  - `failed=0`

## 最终状态

- 本轮没有主动跑模型测试。
- 两个账号刚进入 `active/schedulable` 后，被内层现有请求立即调度命中，上游返回 `402 deactivated_workspace`。
- 服务自动将新增账号置为：
  - `id=308 / karltautges03146+gvx85ii@outlook.com`：`status=error`、`schedulable=false`
  - `id=309 / selenaherlin1192+ccw028g@outlook.com`：`status=error`、`schedulable=false`
- 错误信息：`Workspace deactivated (402): workspace has been deactivated`。
- `privacy_mode`：`training_set_failed`。

## 验证

- SQL 核对新增账号 `id=308..309`：
  - 新增记录数：2
  - `group_id=2` 绑定数：2
  - `active/schedulable`：0
  - `deleted_at is null`：2
  - distinct `chatgpt_account_id`：1
- `http://127.0.0.1:18086/health` 返回 `{"status":"ok"}`。
- 内层 latest OpenAI OAuth 全量账号数：309。
- 当前全量 `active/schedulable`：245。
- 当前全量非 `active/schedulable`：64。
- 当前未删除 OpenAI OAuth：242，其中 `active/schedulable`：215，非 `active/schedulable`：27。
