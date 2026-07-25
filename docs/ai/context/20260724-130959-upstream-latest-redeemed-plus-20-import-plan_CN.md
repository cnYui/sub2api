# 内层 latest redeemed zip 20 个 plus OpenAI OAuth 账号导入计划

## 范围

- 操作对象：本地内层 latest Sub2API，`http://127.0.0.1:18086`。
- 来源 zip：`redeemed_sub2api_20260724040423(1).zip`。
- 解压目录：`tmp/redeemed_sub2api_20260724040423_1/`。
- 来源文件：`sub2api/sub2api.json`。
- 目标分组：`groups.id=2 / internal-openai-upstream`。
- 不触碰公网 Nginx、Cloudflare、公网容器、外层 Sub2API 用户/套餐/流量卡/计费事实。

## 已完成检查

- zip 内 manifest 显示账号数 20。
- 来源文件账号数 20，均为 `platform=openai`、`type=oauth`、`plan_type=plus`。
- 文件内 `name` 无重复，`chatgpt_account_id` 无重复。
- 内层 latest DB 中按 `name` 与 `chatgpt_account_id` 均无重复记录。

## 执行步骤

1. 备份内层 latest 的 `accounts/account_groups/proxies`。
2. 使用正式管理导入接口 `POST /api/v1/admin/accounts/data` 导入，`skip_default_group_bind=true`。
3. 通过正式管理接口 `POST /api/v1/admin/accounts/bulk-update` 将新增账号绑定到 `group_ids=[2]`。
4. 将新增账号全部设为 `status=active`、`schedulable=true`。
5. 使用管理测试接口显式指定 `model_id=gpt-5.4` 逐个测试新增账号，记录成功/失败摘要。
6. 用 SQL 核对新增账号数量、ID 范围、分组绑定和启用状态。

