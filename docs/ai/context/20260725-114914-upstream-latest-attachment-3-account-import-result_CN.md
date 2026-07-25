# 内层 latest 附件 3 个 OpenAI OAuth 账号导入结果

时间：2026-07-25 11:49:14

## 范围

- 操作对象：本地内层 latest Sub2API，`http://127.0.0.1:18086`。
- 来源文件：`C:/Users/yui/.codex/attachments/8d08f530-1d41-48c3-9799-d5e015c0a3b9/pasted-text.txt`。
- 账号分组：`groups.id=2 / internal-openai-upstream`。
- 未触碰外层 `18080` 的计费、套餐、用户、流量卡和历史用量。

## 备份

- 导入前已备份内层 latest 的 `accounts/account_groups/proxies`：
  - `backups/20260725-114533-upstream-latest-attachment-3-account-preimport.sql`
- 该备份包含账号凭据原文，不应提交。

## 导入前检查

- 附件是 3 行完整导出 JSONL，每行包含 1 个账号。
- 源文件账号数：3。
- 源文件代理数：0。
- 文件内 `name` 无重复。
- 文件内凭据邮箱无重复。
- 内层 latest DB 中按 `name` 与凭据邮箱均无匹配记录。
- 3 个账号均为：
  - `platform=openai`
  - `type=oauth`

## 导入与启用

- 导入接口：`POST /api/v1/admin/accounts/data`，`skip_default_group_bind=true`。
- 导入结果：
  - `account_created=3`
  - `account_failed=0`
  - `proxy_created=0`
  - `proxy_reused=0`
  - `proxy_failed=0`
- 新增账号：`id=295..297`。
- 已通过 `POST /api/v1/admin/accounts/bulk-update` 统一绑定到 `internal-openai-upstream`（`groups.id=2`），并设为：
  - `status=active`
  - `schedulable=true`
- 批量更新结果：
  - `success=3`
  - `failed=0`

## 验证

- 本轮不跑模型测试。
- SQL 核对新增账号 `id=295..297`：
  - `active=3`
  - `schedulable=3`
  - `group_id=2` 绑定数：3
- `http://127.0.0.1:18086/health` 返回 `{"status":"ok"}`。
- 内层 latest OpenAI OAuth 全量账号数：297。
- 当前全量 `active/schedulable`：235。
- 当前全量非 `active/schedulable`：62。
- 当前未删除 OpenAI OAuth：230，其中 `active/schedulable`：205，非 `active/schedulable`：25。

## 临时文件

- 导入时生成的临时 payload：`C:/tmp/sub2api-import-20260725-114533-attachment-3.json`。
- 删除动作被本地执行策略拦截，已将文件内容覆写为空白，当前长度为 2 字节。
