# 内层 latest card withdraw 10 个 free OpenAI OAuth 账号导入计划

## 范围

- 操作对象：本地内层 latest Sub2API，`http://127.0.0.1:18086`。
- 来源 txt：`C:/Users/yui/Downloads/card-withdraw-history-20260724211304.txt`。
- 可导入 JSON：`C:/Users/yui/Downloads/sub2api-account-20260724131644.json`。
- 账号分组：`groups.id=2 / internal-openai-upstream`。
- 不触碰公网 Nginx、Cloudflare、公网容器、外层 Sub2API 用户/套餐/流量卡/计费事实。

## 已完成检查

- JSON 与 txt 的 10 个邮箱完全一致。
- JSON 为标准 Sub2API 导出结构，包含 `exported_at/proxies/accounts`。
- 源文件账号数：10。
- 账号均为 `platform=openai`、`type=oauth`、`plan_type=free`。
- 文件内 `name` 无重复。
- 内层 latest DB 中按 `name` 与凭据邮箱均无匹配记录。

## 执行步骤

1. 备份内层 latest 的 `accounts/account_groups/proxies`。
2. 使用正式管理导入接口 `POST /api/v1/admin/accounts/data` 导入，`skip_default_group_bind=true`。
3. 使用正式管理接口 `POST /api/v1/admin/accounts/bulk-update` 绑定 `group_ids=[2]`。
4. 将新增账号全部设为 `status=active`、`schedulable=true`。
5. 按用户要求不跑模型测试。
6. 用 SQL 核对新增账号数量、ID 范围、分组绑定和启用状态。

