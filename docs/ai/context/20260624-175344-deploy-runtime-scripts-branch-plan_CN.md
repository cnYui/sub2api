# 部署脚本与运行时模板分支提交计划

## 背景

- 用户要求将本地新增的部署相关文件单独提交到一个新分支。
- 待处理文件：
  - `deploy/.env.scheme-a.runtime`
  - `deploy/redeploy-sub2api-image.sh`
  - `deploy/redeploy-sub2api-image.test.sh`
  - `deploy/restart-sub2api.sh`
  - `deploy/restart-sub2api.test.sh`

## 风险识别

- `deploy/.env.scheme-a.runtime` 当前包含真实 `SUB2API_USER_KEY` 与测试用户密码。
- 若原样提交，会把敏感凭据写入 Git 历史，后续删除文件也无法彻底撤回历史暴露。

## 处理方案

- 将 `deploy/.env.scheme-a.runtime` 改为脱敏占位版，仅保留字段结构与用途说明。
- 在 `.gitignore` 中加入 `deploy/.env.scheme-a.runtime`，避免后续再次误提交本地真实值。
- 在新分支中仅提交部署脚本、测试脚本、脱敏后的运行时模板与本次上下文文档，不混入其他未跟踪临时文件。

## 目标分支

- `codex/deploy-runtime-scripts-20260624`

## 验证计划

- 运行：
  - `bash deploy/redeploy-sub2api-image.test.sh`
  - `bash deploy/restart-sub2api.test.sh`
- 检查 `git diff --cached --stat`，确认仅包含预期文件。
