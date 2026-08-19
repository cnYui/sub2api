# DeepSeek 基础价格 Docker 重建与公网替换

## 目标

按管理员要求，将工作区中已同步的 DeepSeek V4 基础价构建进生产镜像，并替换公网应用服务。

## 发布范围

- 应用源码：`backend/internal/service/billing_service.go` 中 DeepSeek V4 Pro/Flash 基础价。
- 生产分组：`groups.id=8` 已为 `【国产】DeepSeek（5折）`、`rate_multiplier=3.5`，本次不重复修改。
- 仅重建并替换 `sub2api-official-18082` 应用容器。
- PostgreSQL、Redis、Nginx、Cloudflare Tunnel 和数据卷不重建。

## 价格口径

| 模型 | 输入 | 输出 | 缓存读取 |
| --- | ---: | ---: | ---: |
| `deepseek-v4-pro` | `$0.66/M` | `$1.98/M` | `$0.022/M` |
| `deepseek-v4-flash` | `$0.22/M` | `$0.66/M` | `$0.007/M` |

以上是基础价；模型广场实付价还会乘 DeepSeek 分组倍率 `3.5x`。余额最终扣费继续受现有 `BILLING_FINAL_MULTIPLIER=18` 影响。

## 执行记录

## 执行结果

- Docker 构建成功：`deploy-sub2api:latest`，镜像摘要 `sha256:7a1403b62fa12132a757aa405a797166a032002b47f1685bd68d8a11982f172a`。构建过程中的 Vite Browserslist 和大 chunk 信息为既有警告，不影响构建成功。
- 通过 `docker compose --env-file deploy/.env -f deploy/docker-compose.dev.yml -f deploy/docker-compose.18082.yml up -d --no-deps --force-recreate --no-build sub2api` 仅替换 `sub2api-official-18082`。
- 应用容器状态为 `healthy`；PostgreSQL、Redis、Nginx 均保持原容器运行，未重建。
- 本地 `/api/v1/model-plaza` 与公网 `https://api.aaccx.pw/api/v1/model-plaza` 均返回分组 `【国产】DeepSeek（5折）`、倍率 `3.5x`：
  - `deepseek-v4-flash`：基础价 `$0.22 / $0.66 / $0.007`（输入/输出/缓存读取，每百万 token）。
  - `deepseek-v4-pro`：基础价 `$0.66 / $1.98 / $0.022`（输入/输出/缓存读取，每百万 token）。
- `127.0.0.1:18082`、本地 Nginx、`aaccx.pw`、`www.aaccx.pw`、`api.aaccx.pw` 的 `/health` 均返回 HTTP 200。
