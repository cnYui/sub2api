# 本地 main 整合与公网发布结果

## Git 整合

- 本地 `main` 提交为 `1db97d589`（`feat: 汇总本地监控与 API 页面改动`）。
- 所有具名本地分支在整合前均已是 `main` 祖先，没有额外分支合并动作。
- 主工作区可纳入的 83 个文件已提交；本地账号恢复二进制/工具及过期 billing Compose 已由 Git/Docker 排除并保留在原工作区。

## 构建与替换

- 从提交后的本地 `main` 构建 `deploy-sub2api:latest` 成功。
- 新镜像摘要：`sha256:8d17820336d7af0796816b6f8c56f5228fb353777192bce48013c9d623f6a58c`。
- 通过 `docker compose -f deploy/docker-compose.dev.yml -f deploy/docker-compose.18082.yml up -d --no-deps --force-recreate --no-build sub2api` 仅替换 `sub2api-official-18082`。
- 新容器 ID：`6a69f48484c2f547a0256089d16c0cb65964a302bb9d873bb525373e797aa0b1`，状态为 `healthy`。
- 运行时 `BILLING_FINAL_MULTIPLIER=16`，账号凭证 Secret 仍按既有只读挂载使用。

## 线上核验

| 目标 | 结果 |
| --- | --- |
| `http://127.0.0.1:18082/health` | HTTP 200 |
| `http://127.0.0.1:8080/health` | HTTP 200 |
| `https://aaccx.pw/health` | HTTP 200 |
| `https://www.aaccx.pw/health` | HTTP 200 |
| `https://api.aaccx.pw/health` | HTTP 200 |
| `sub2api-public-nginx-local nginx -t` | 通过 |
| 监控调度器 | 加载 14 条任务 |

PostgreSQL、Redis、Nginx、Cloudflare Tunnel 和数据卷未重建。构建仅保留项目既有的 Browserslist、动态导入和大 chunk 警告，无构建错误。
