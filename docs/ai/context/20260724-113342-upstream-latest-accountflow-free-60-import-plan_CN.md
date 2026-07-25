# 内层 latest accountflow 60 个 free OpenAI OAuth 账号导入计划

## 目标

- 将目录 `sub2_json-60-20260724T023051Z` 内的 `accountflow-redeem-sub2.json` 导入本地内层 latest Sub2API。
- 文件包含 60 个 `openai/oauth` 账号，均为 `plan_type=free`。
- 绑定到 `groups.id=2 / internal-openai-upstream`。
- 用 `gpt-5.4` 逐个做真实管理测试，区分“已入库”和“可按 gpt-5.4 调度”。

## 范围

- 仅操作本地内层 latest：`http://127.0.0.1:18086`。
- 不触碰公网 Nginx、Cloudflare、公网容器、外层 Sub2API 用户/套餐/流量卡/计费事实。

## 已检查

- 源文件账号数：60。
- 文件内邮箱无重复。
- 文件内 ChatGPT 身份无重复。
- 内层 latest DB 中无匹配邮箱或 ChatGPT 身份记录。

## 步骤

1. 备份内层 `accounts/account_groups/proxies`。
2. 调用 `POST /api/v1/admin/accounts/data` 导入，跳过默认分组绑定。
3. 批量绑定 `groups.id=2`，先设为 `active/schedulable`。
4. 用 `gpt-5.4` 逐个调用管理测试接口。
5. 对测试失败且不适合作为 `gpt-5.4` 调度来源的账号设为不可调度。
6. 记录最终统计和结果。
