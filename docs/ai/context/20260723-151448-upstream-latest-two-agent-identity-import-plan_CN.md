# 内层 latest 两个 OpenAI OAuth 账号导入计划

## 范围

- 操作对象：本地内层 latest Sub2API，`http://127.0.0.1:18086`。
- 附件：`sub2api-agentIdentity-alive (2).json`。
- 账号数量：2。
- 目标分组：`groups.id=2 / internal-openai-upstream`。
- 不触碰公网 Nginx、Cloudflare、公网容器、外层定制版 Sub2API 用户/套餐/流量卡/计费事实。

## 步骤

1. 备份内层 latest 的 `accounts/account_groups/proxies`。
2. 用正式管理 API `POST /api/v1/admin/accounts/data` 导入，`skip_default_group_bind=true`。
3. 查询新增账号 ID。
4. 用 `POST /api/v1/admin/accounts/bulk-update` 绑定 `group_ids=[2]`，并设为 `active/schedulable`。
5. 用管理测试接口显式 `model_id=gpt-5.4` 测试新增账号。
6. SQL 核对总数、分组绑定和状态，并写 result 上下文。

## 回滚边界

- 导入前备份保存到 `backups/`，含账号凭据原文，不提交。
- 若导入或绑定失败，只记录失败账号和接口错误，不删除已有账号。
