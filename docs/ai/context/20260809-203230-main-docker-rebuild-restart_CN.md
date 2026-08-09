# main 合并后 Docker 重建与公网重启结果

## Git 状态

- 当前 `main` 最新提交为 `ae452d2d6`，工作区干净。
- `git branch --no-merged main` 无输出，`git ls-files --others --exclude-standard` 无输出。
- detached billing 工作树仍按 `20260809-202934-billing-worktree-exclusion-correction_CN.md` 保留，未纳入主线。

## Docker 发布

- 使用当前 `main` 重新构建 `deploy-sub2api:latest`。
- 通过 `docker compose -f deploy/docker-compose.dev.yml -f deploy/docker-compose.18082.yml up -d --no-deps --force-recreate --no-build sub2api` 替换 `sub2api-official-18082`，随后重启该应用容器。
- 当前镜像摘要为 `sha256:607eccf01f8d88483b540ea3eb014ebaa7d36ec4e3164c09782a62d336e5a0c0`，容器状态为 `running/healthy`。
- 运行时 `BILLING_FINAL_MULTIPLIER=16`，账号凭证 Secret 仍以只读挂载使用。
- PostgreSQL、Redis、数据卷、Nginx 和 Cloudflare Tunnel 未重建或修改。

## 链路验收

| 目标 | 结果 |
| --- | --- |
| `http://127.0.0.1:18082/health` | HTTP 200 |
| `http://127.0.0.1:8080/health` | HTTP 200 |
| `https://aaccx.pw/health` | HTTP 200 |
| `https://www.aaccx.pw/health` | HTTP 200 |
| `https://api.aaccx.pw/health` | HTTP 200 |
| `sub2api-public-nginx-local nginx -t` | 通过 |

