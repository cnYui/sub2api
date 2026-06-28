# 合并本地未提交内容与未合并分支计划

## 目标

- 当前工作区位于 `main`，将本地 `main` 上的未提交修改和未跟踪文件提交进 `main`。
- 将本地尚未合并进 `main` 的分支逐个 merge 到 `main`。
- 不处理远端同步关系，不以 `origin/main` 作为判断依据；`origin` 当前是上游仓库，用户个人远端为 `personal`。

## 边界

- 不执行 `git reset --hard`、`git checkout --` 等会丢弃用户改动的命令。
- 不推送远端，不开 PR。
- 不删除本地分支或 worktree，除非用户另行明确要求。
- 不提交 `deploy/backups/` 下的运行态 dump 或任何密钥、密码、token。

## 步骤

1. 盘点当前 `main` 工作区：
   - 已修改文件
   - 未跟踪文件
   - 暂存区状态
   - 冲突状态
2. 盘点本地未合并分支：
   - `git branch --no-merged main`
   - `git worktree list --porcelain`
   - 每个相关 worktree 的 `git status --short`
3. 自查当前工作区 diff，确认没有敏感信息和明显临时文件。
4. 运行与当前工作区改动直接相关的验证。
5. stage 当前应纳入 `main` 的文件并提交。
6. 逐个 merge 本地未合并分支到 `main`：
   - 使用非交互 merge 命令。
   - 如发生冲突，停止、查看冲突并解决，不丢弃已有改动。
7. 合并全部完成后运行验证：
   - `git diff --check`
   - 后端相关测试
   - 前端相关测试或构建视合并范围决定
8. 新增结果文档并更新 `AGENTS.md`。

## 风险点

- 未合并分支可能包含较早实验内容，合并时可能与当前 `main` 产生冲突。
- 标记 `+` 的本地分支正在其他 worktree 签出，合并其已提交内容可以进行，但不能擅自清理对应 worktree。
- 如果分支 worktree 里有未提交改动，分支 tip 不包含这些内容；需要单独确认是否也要提交。
