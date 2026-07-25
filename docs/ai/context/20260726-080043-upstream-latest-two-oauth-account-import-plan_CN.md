# 内层 latest 2 个 OpenAI OAuth 账号导入计划

## 背景

- 附件：`C:/Users/yui/Downloads/karltautges03146+gvx85ii@outlook.sub2api.2026-07-26_07-59-04.json`
- 目标：仅导入本地内层 latest Sub2API（`127.0.0.1:18086`），绑定 `internal-openai-upstream`（`groups.id=2`），并启用调度。
- 约束：不触碰公网链路、不跑模型测试、不记录完整 token/key/secret。

## 导入前检查

- 文件结构：标准 Sub2API 导出，`accounts=2`、`proxies=0`。
- 账号类型：全部 `platform=openai`、`type=oauth`。
- 来源文件未声明 `plan_type`。
- 文件内 `name` 无重复。
- 文件内 `chatgpt_account_id` 有 1 组重复；两个账号的 refresh/access token 指纹不同。
- DB 去重：按 `name` 与 `chatgpt_account_id` 均无命中。

## 执行步骤

1. 备份内层 latest 的 `accounts/account_groups/proxies`。
2. 通过正式管理接口 `POST /api/v1/admin/accounts/data` 导入，避免绕过应用层解析。
3. 对新增账号批量绑定 `group_ids=[2]`，设置 `status=active`、`schedulable=true`。
4. 用 SQL 核对新增 ID 范围、启用状态、分组绑定和全量统计。
