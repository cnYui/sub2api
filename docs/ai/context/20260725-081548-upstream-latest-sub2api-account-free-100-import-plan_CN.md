# 内层 latest sub2api-account 100 个 free OpenAI OAuth 账号导入计划

## 范围

- 操作对象：本地内层 latest Sub2API，`http://127.0.0.1:18086`。
- 来源文件：`C:/Users/yui/Downloads/sub2api-account.json`。
- 账号分组：`groups.id=2 / internal-openai-upstream`。
- 不触碰公网 Nginx、Cloudflare、公网容器、外层 Sub2API 用户/套餐/流量卡/计费事实。

## 已完成检查

- 源文件账号数：100。
- 源文件代理数：0。
- 文件内 `name` 无重复。
- 文件内凭据邮箱无重复。
- 账号均为 `platform=openai`、`type=oauth`、`plan_type=free`。
- 内层 latest DB 中按 `name` 与凭据邮箱均无匹配记录。

## 执行步骤

1. 备份内层 latest 的 `accounts/account_groups/proxies`。
2. 使用正式管理导入接口 `POST /api/v1/admin/accounts/data` 导入，`skip_default_group_bind=true`。
3. 使用正式管理接口 `POST /api/v1/admin/accounts/bulk-update` 绑定 `group_ids=[2]`。
4. 将新增账号全部设为 `status=active`、`schedulable=true`。
5. 按用户此前要求不跑模型测试。
6. 用 SQL 核对新增账号数量、ID 范围、分组绑定和启用状态。

