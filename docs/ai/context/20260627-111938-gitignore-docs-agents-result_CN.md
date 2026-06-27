# .gitignore 移除 docs 与 AGENTS 忽略规则结果

## 实际改动

- 已从 `.gitignore` 删除 `AGENTS.md` 忽略规则。
- 已从 `.gitignore` 删除 `docs/*` 忽略规则。
- 已同步删除 `!docs/PAYMENT.md`、`!docs/PAYMENT_CN.md`、`!docs/ADMIN_PAYMENT_INTEGRATION_API.md`、`!docs/legal/`、`!docs/legal/*.md`，因为 `docs/*` 移除后这些反向白名单不再需要。

## 验证

- 执行 `git check-ignore -v -- AGENTS.md docs/ai/context/20260627-111856-gitignore-docs-agents-plan_CN.md`，命令退出码为 1 且无输出，表示目标文件不再被忽略。
- 执行 `git diff -- .gitignore`，确认 diff 仅删除 `AGENTS.md` 与 `docs` 相关忽略规则。
- 执行 `git status --short -- .gitignore AGENTS.md docs/ai/context/20260627-111856-gitignore-docs-agents-plan_CN.md`，确认 `.gitignore` 已修改，新增计划记录已可被 Git 看到。
