# 内层 latest 附件 3 个 OpenAI OAuth 账号导入计划

时间：2026-07-25 11:45:33

## 范围

- 操作对象：本地内层 latest Sub2API，`http://127.0.0.1:18086`。
- 来源文件：`C:/Users/yui/.codex/attachments/8d08f530-1d41-48c3-9799-d5e015c0a3b9/pasted-text.txt`。
- 账号分组：`groups.id=2 / internal-openai-upstream`。
- 不触碰外层 `18080` 的计费、套餐、用户、流量卡和历史用量。

## 已完成检查

- 附件是 3 行完整导出 JSONL，每行包含 1 个账号。
- 源文件账号数：3。
- 源文件代理数：0。
- 文件内 `name` 无重复。
- 文件内凭据邮箱无重复。
- 账号均为 `platform=openai`、`type=oauth`。
- 内层 latest DB 中按 `name` 与凭据邮箱均无匹配记录。
- 18086 当前健康，容器为 `sub2api-upstream-latest`。

## 执行步骤

1. 备份内层 latest 的 `accounts/account_groups/proxies`。
2. 将 3 行 JSONL 合并为单个导入 JSON。
3. 使用正式管理导入接口 `POST /api/v1/admin/accounts/data` 导入，`skip_default_group_bind=true`。
4. 使用正式管理接口 `POST /api/v1/admin/accounts/bulk-update` 绑定 `group_ids=[2]`。
5. 将新增账号全部设为 `status=active`、`schedulable=true`。
6. 本轮不跑模型测试，只核对导入、分组绑定和启用状态。
