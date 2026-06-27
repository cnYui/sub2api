# 释放占用 `main` 的 worktree 计划

## 背景

- 当前仓库存在两个 worktree。
- `/Users/wujianxiang/CodeSpace/sub2api-mobile-touch-overlay-fix-20260623` 当前签出的是本地 `main`。
- 用户希望后续能够在另一个工作目录中切换到本地 `main`，因此需要先释放这个 worktree 对 `main` 的占用。

## 当前状态

- 占用 `main` 的 worktree 路径：`/Users/wujianxiang/CodeSpace/sub2api-mobile-touch-overlay-fix-20260623`
- 当前分支：`main`
- 当前提交：`25ebb8a9`
- `git status --short --branch` 显示该 worktree 当前没有未提交改动。

## 方案

- 在占用 `main` 的 worktree 中创建并切换到新的临时分支，保留当前文件与提交不变。
- 推荐分支名：`codex/release-main-worktree-20260624`

## 预期结果

- `main` 不再被 `/Users/wujianxiang/CodeSpace/sub2api-mobile-touch-overlay-fix-20260623` 占用。
- 该 worktree 仍保留当前全部代码内容，只是绑定到新的临时分支。
- 后续其他 worktree 可以切换到本地 `main`，前提是自身工作区状态允许切换。
