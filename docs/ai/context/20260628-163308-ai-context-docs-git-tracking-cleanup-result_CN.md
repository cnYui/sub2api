# AI 上下文文档 Git 跟踪整理结果

## 背景

`docs/ai/context/` 已不再被 `.gitignore` 忽略，但此前多个上下文文件仍显示为未跟踪。根因是：移除 ignore 只会让 Git 看见文件，不会自动把文件加入提交历史；未执行 `git add` / `git commit` 的文档仍会保持 `??`。

## 本次整理

- 确认 `.gitignore` 和 `.git/info/exclude` 没有忽略 `docs/ai/context/`。
- 使用 `git check-ignore` 验证未跟踪上下文文档没有被 ignore。
- 敏感词扫描未发现完整 API Key、内部 token、SMTP 密码或 HMAC secret；命中的 `smtp_password=[CONFIGURED]` 均为脱敏状态说明。
- 将此前遗留的上下文文档统一纳入文档归档提交。
- 在 `AGENTS.md` 中新增合并收尾规则：每次合并/提交前必须检查并提交 `docs/ai/context` 下的上下文文档，或说明暂不提交原因。
- `.gitignore` 保留 `deploy/backups/`，避免数据库备份文件误入版本库。

## 后续规则

以后完成实现、部署、排查或合并前，必须把相关 design/plan/result 文档一起纳入提交。若某次变更需要拆成代码提交和文档提交，也可以独立提交，但不能让上下文文档长期停留在未跟踪状态。
