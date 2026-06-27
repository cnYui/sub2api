# 本地 main 工作区修改保存计划

## 背景

用户要求将当前工作区所有修改保存，并合并进本地 `main` 分支。当前工作区已经位于 `main`，因此本次操作的目标是把现有工作区修改、已暂存修改和未跟踪上下文文档统一纳入一次本地提交，不推送远端。

## 范围

- 保存当前已修改源码、测试、`.gitignore` 与 `AGENTS.md`。
- 保存 `docs/ai/context/` 下当前未跟踪的历史上下文文档。
- 包含本轮 SMTP 18085 配置相关上下文，但只记录脱敏状态，不记录 Gmail App Password 明文。

## 风险控制

- 提交前扫描最终 staged diff，确认不包含 SMTP 密码、完整 API Key、内部 token、HMAC secret、JWT secret 或其他敏感明文。
- `AGENTS.md` 当前带有 `skip-worktree` 标记，需要显式取消后再暂存，避免长期上下文未被保存。
- 本次只做本地提交，不执行 push，不重启公网链路。

## 验证计划

- 运行后端迁移 checksum 相关测试。
- 运行前端 `PaymentView` 相关测试。
- 检查 `git status`，确认修改已进入本地 `main` 提交。
