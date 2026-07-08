# personal/main 同步计划

> 2026-07-08 21:34 JST 制定。

## 目标

- 将当前本地 `main` 的代码与项目上下文同步到远端 `personal/main`。
- 保持同步为常规 fast-forward 推送，不改写远端历史。

## 当前状态

- 当前分支：`main`
- 远端：`personal https://github.com/cnYui/sub2api.git`
- `git fetch personal main` 后差异：`personal/main...main = 0 23`，说明远端没有本地缺失提交，本地领先 23 个提交。
- 工作区还有上下文改动：
  - `AGENTS.md`
  - `docs/ai/context/20260708-211958-public-18084-model-path-whitelist-redeploy-result_CN.md`
  - `docs/ai/context/20260708-212514-3876129758-v1-models-public-retest-result_CN.md`
  - `docs/ai/context/20260708-212652-xinlise-v1-models-public-retest-result_CN.md`

## 决策

- 先提交上述上下文改动，避免远端只同步代码提交而丢失最新项目记忆。
- 推送目标为 `personal main`。
- 不使用 force push；若远端在推送前新增提交，则停止并重新核对。

## 验证

- 推送前运行 `git diff --check`。
- 推送后运行 `git fetch personal main`、`git rev-list --left-right --count personal/main...main`、`git status --short --branch`。
