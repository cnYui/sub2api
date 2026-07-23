# 内层 latest 追加单个 agent identity 账号计划

时间：2026-07-23 00:00 JST

## 目标

- 将 `C:\Users\yui\Downloads\hectordenigris42356@outlook.com.json` 中的 1 个 OpenAI agent identity 账号追加到内层 latest Sub2API。
- 目标运行态为 `sub2api-upstream-latest` / `http://127.0.0.1:18086`。
- 18080 继续作为真实用户计费事实源，本轮不修改外层用户、套餐、流量卡和计费事实。
- 按用户要求，本轮不重启 Docker。

## 输入确认

- 文件类型：`sub2api-data`。
- 文件版本：`1`。
- 账号数量：`1`。
- 账号名：`hectordenigris42356@outlook.com`。
- 账号平台：`openai`。
- 账号类型：`oauth`。
- 文件包含凭据字段，但文档和回复不记录完整凭据内容。

## 执行方案

1. 使用 18086 管理导出接口备份导入前账号数据到 `backups/`。
2. 调用 18086 管理导入接口 `POST /api/v1/admin/accounts/data`，参数 `skip_default_group_bind=true`。
3. 找到新创建账号 ID，并通过批量更新接口绑定到 `internal-openai-upstream`（`groups.id=2`）。
4. 使用管理测试接口显式以 `gpt-5.4` 测试新账号。
5. 写入 result 上下文文档，记录账号 ID、测试状态和备份路径。

## 回滚边界

- 若导入后测试失败或需要撤销，使用管理接口禁用/删除新增账号，或使用导入前管理导出快照恢复。
- 回滚只影响内层 18086 账号池，不触碰 18080 计费事实。

## 安全边界

- 不在文档、日志和回复中记录完整 access token、refresh token、agent private key、JWT、内部转发 Key。
- 管理导出快照可能包含凭据原文，仅保存在本地 `backups/`，不提交。
