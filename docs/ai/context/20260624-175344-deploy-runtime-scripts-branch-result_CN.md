# 部署脚本与运行时模板分支提交结果

## 处理结果

- 已将 `deploy/.env.scheme-a.runtime` 改为脱敏模板，不再包含真实 API Key 或测试密码。
- 已在 `.gitignore` 中加入 `deploy/.env.scheme-a.runtime`，避免后续误提交本地真实值。
- 已为本次提交创建独立分支：
  `codex/deploy-runtime-scripts-20260624`

## 纳入提交的文件

- `.gitignore`
- `deploy/.env.scheme-a.runtime`
- `deploy/redeploy-sub2api-image.sh`
- `deploy/redeploy-sub2api-image.test.sh`
- `deploy/restart-sub2api.sh`
- `deploy/restart-sub2api.test.sh`
- `docs/ai/context/20260624-175344-deploy-runtime-scripts-branch-plan_CN.md`
- `docs/ai/context/20260624-175344-deploy-runtime-scripts-branch-result_CN.md`

## 验证

- `bash deploy/redeploy-sub2api-image.test.sh`
- `bash deploy/restart-sub2api.test.sh`

两项测试均通过。
