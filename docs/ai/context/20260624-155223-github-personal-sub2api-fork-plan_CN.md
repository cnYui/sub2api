# GitHub 个人 Sub2API 仓库计划

## 目标

在 GitHub 登录账号 `cnYui` 下创建可用于保存和提交当前 Sub2API 工作的个人仓库，并把本地仓库接入该远端。

## 当前事实

- 本地路径：`/Users/wujianxiang/CodeSpace/sub2api`
- 当前分支：`codex/gpt-traffic-pack-20260624`
- 当前 `origin`：`https://github.com/Wei-Shaw/sub2api.git`
- GitHub 登录账号：`cnYui`
- `cnYui/sub2api` 当前不存在
- 工作区已有未提交改动，不能在未确认范围前重置或丢弃

## 方案

1. 在 GitHub 创建 `cnYui/sub2api`，优先作为 `Wei-Shaw/sub2api` 的 fork，保留来源关系。
2. 本地新增独立远端 `personal` 指向 `https://github.com/cnYui/sub2api.git`，不修改 `origin`。
3. 推送当前分支到 `personal`，让当前提交历史先有一份个人仓库备份。
4. 检查远端与分支状态，再记录结果。

## 取舍

- 保留 `origin` 指向上游，避免以后需要同步上游时关系混乱。
- 使用 `personal` 作为个人写入远端，后续提交可显式 `git push personal <branch>`。
- 本次不自动重置、不删除、不覆盖历史；未提交改动是否提交另行按实际状态处理。
