# billing detached 工作树排除勘误

## 核对结果

- 当前本地 `main` 的所有具名分支均已合并，`git branch --no-merged main` 无输出；`main` 工作区无未跟踪文件。
- `D:\CodeWorkSpace\sub2api-billing-loss-fix-20260805` 是 detached 工作树，仍有未提交的 `backend/internal/repository/usage_billing_repo.go` 差异和未跟踪的 `deploy/docker-compose.production-billing-fix.yml`。

## 不合并原因

- billing 工作树基于旧版本，结算差异对应的修复已经存在于当前 `main`；直接移植会覆盖当前主线已有的批量生图套餐来源逻辑。
- 该 Compose 文件固定 `BILLING_FINAL_MULTIPLIER=15`，与当前 18082 生产配置的 `16` 冲突，且不是当前部署链路的正式 Compose 文件。
- 因此两项均保留在原 detached 工作树，不删除、不纳入 `main`，主工作区通过 Git/Docker 排除规则避免误提交或进入镜像构建上下文。

