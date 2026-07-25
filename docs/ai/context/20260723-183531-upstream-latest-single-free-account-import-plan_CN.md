# 内层 latest 单个 free OpenAI OAuth 账号导入计划

## 目标

- 将附件 `timothytorres43800l@outlook.sub2api.2026-07-23_18-34-39.json` 导入本地内层 latest Sub2API。
- 绑定到 `groups.id=2 / internal-openai-upstream`。
- 用 `gpt-5.4` 做真实管理测试，确认是否可用。

## 范围

- 仅操作本地内层 latest：`http://127.0.0.1:18086`。
- 不碰公网链路和外层计费。

## 步骤

1. 备份内层 `accounts/account_groups/proxies`。
2. 导入单账号文件。
3. 绑定分组并设为 `active/schedulable`。
4. 逐个测试 `gpt-5.4`。
5. 汇总结果并更新上下文。
