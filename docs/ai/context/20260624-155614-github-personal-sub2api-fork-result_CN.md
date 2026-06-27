# GitHub 个人 Sub2API 仓库结果

## 完成事项

- 已在 GitHub 登录账号 `cnYui` 下创建个人 fork：`https://github.com/cnYui/sub2api`
- 本地新增远端：`personal -> https://github.com/cnYui/sub2api.git`
- 保留原远端：`origin -> https://github.com/Wei-Shaw/sub2api.git`
- 已把分支 `codex/gpt-traffic-pack-20260624` 推送到 `personal`

## 后续使用

- 保存当前分支：`git push personal <branch>`
- 首次推送新分支并设置跟踪：`git push -u personal <branch>`
- 同步上游仍可通过 `origin` 读取，但不要把个人改动直接推到 `origin`

## 注意

- 本次未提交未跟踪的 `deploy/*.sh` 发布脚本，避免把历史未跟踪文件混入 GitHub 远端接入提交。
- `docs/ai/context/` 受 `.gitignore` 的 `docs/*` 规则影响，结果文档只作为本地上下文保存。
