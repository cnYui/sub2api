# .gitignore 移除 docs 与 AGENTS 忽略规则计划

## 目标

- 让当前项目的 `docs/` 目录内容不再被 `.gitignore` 忽略。
- 让根目录 `AGENTS.md` 不再被 `.gitignore` 忽略。

## 当前判断

- `.gitignore` 中存在 `AGENTS.md` 规则，会导致协作入口文件不进入版本控制。
- `.gitignore` 中存在 `docs/*`，并额外保留了若干 `!docs/...` 白名单。若移除 `docs/*`，这些白名单已无必要，应一起删除，避免规则表达变复杂。
- 工作区已有其他未提交改动，本次只修改 `.gitignore` 并新增本记录，不触碰其他文件。

## 执行计划

1. 从 `.gitignore` 删除 `AGENTS.md`。
2. 从 `.gitignore` 删除 `docs/*` 以及对应的 `!docs/...` 白名单规则。
3. 使用 `git check-ignore` 验证 `AGENTS.md` 与 `docs/ai/context/` 新文件不再被忽略。
4. 使用 `git diff -- .gitignore` 核对实际变更范围。
