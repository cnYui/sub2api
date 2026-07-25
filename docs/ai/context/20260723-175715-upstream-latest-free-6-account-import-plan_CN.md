# 内层 latest 6 个 free OpenAI OAuth 账号导入计划

## 目标

- 将 6 个 `outlook.com-free.sub2api.json` 账号导入本地内层 latest Sub2API。
- 绑定到 `groups.id=2 / internal-openai-upstream`。
- 用 `gpt-5.4` 逐个做管理测试，确认真实可用性。

## 范围

- 仅操作本地内层 latest：`http://127.0.0.1:18086`。
- 不碰公网链路，不改外层计费。

## 步骤

1. 先备份内层 `accounts/account_groups/proxies`。
2. 导入 6 个附件账号。
3. 绑定账号池分组并设为可调度。
4. 用 `gpt-5.4` 做逐个测试。
5. 汇总结果并更新上下文。
