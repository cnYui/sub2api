# 内层 latest accountflow 10 个 free OpenAI OAuth 账号导入计划

## 范围

- 操作对象：本地内层 latest Sub2API，`http://127.0.0.1:18086`。
- 来源文件：`C:/Users/yui/AppData/Local/Temp/accountflow-redeem-sub2.json`。
- 账号分组：`groups.id=2 / internal-openai-upstream`。
- 不触碰公网 Nginx、Cloudflare、公网容器、外层 Sub2API 用户/套餐/流量卡/计费事实。

## 已完成检查

- 源文件账号数：10。
- 文件内 `name` 无重复。
- 文件内 `chatgpt_account_id` 无重复。
- 账号均为 `platform=openai`、`type=oauth`、`plan_type=free`。

## 执行步骤

1. 先做内层 latest 的 `accounts/account_groups/proxies` 备份。
2. 使用正式管理导入接口 `POST /api/v1/admin/accounts/data` 导入，`skip_default_group_bind=true`。
3. 使用正式管理接口 `POST /api/v1/admin/accounts/bulk-update` 绑定 `group_ids=[2]`。
4. 将新增账号全部设为 `status=active`、`schedulable=true`。
5. 使用管理测试接口显式指定 `model_id=gpt-5.4` 逐个测试新增账号。
6. 用 SQL 核对新增账号数量、ID 范围、分组绑定和启用状态。

