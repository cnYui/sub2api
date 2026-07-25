# 内层 latest vitas plus Codex 账号导入计划

## 目标

- 将附件 `codex-vitas-antigen.0t@icloud.com-plus.json` 导入本地内层 latest Sub2API。
- 使用已有 `import-codex-session` 管理接口解析 Codex session 凭据。
- 绑定到 `groups.id=2 / internal-openai-upstream`。
- 用 `gpt-5.4` 做真实管理测试，确认是否可用。

## 范围

- 仅操作本地内层 latest：`http://127.0.0.1:18086`。
- 不触碰公网链路，不改外层 Sub2API 计费、用户或套餐。

## 步骤

1. 备份内层 `accounts/account_groups/proxies`。
2. 调用 `POST /api/v1/admin/accounts/import/codex-session` 导入。
3. 核对 DB 账号、分组和调度状态。
4. 用 `gpt-5.4` 调用管理测试接口。
5. 记录导入结果和最终可用性。
