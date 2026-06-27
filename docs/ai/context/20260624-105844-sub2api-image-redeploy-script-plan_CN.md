# Sub2API 镜像替换发布脚本实现计划

## 目标

新增 `deploy/redeploy-sub2api-image.sh`，把“构建新镜像、替换 Sub2API 容器、等待健康检查”合成一次性操作。由于替换容器会短暂影响 `https://api.aaccx.pw/v1/*`，脚本默认以后台 detached 方式执行并写日志，避免当前 Codex 会话断联导致发布流程中断。

## 设计

- 默认命令：`./deploy/redeploy-sub2api-image.sh --yes`
- 默认行为：
  - 自动寻找 Docker CLI；macOS Docker Desktop 路径 `/Applications/Docker.app/Contents/Resources/bin/docker` 作为 fallback。
  - 构建镜像 `weishaw/sub2api:latest`，使用仓库根目录 `Dockerfile`。
  - 使用 `deploy/docker-compose.local.yml` 重建 `sub2api` 服务。
  - 默认读取 `deploy/.env`；若不存在则读取 `deploy/.env.scheme-a.local`，解决 Compose 环境变量缺失问题。
  - 只执行 `up -d --no-deps --force-recreate sub2api`，不重启 Postgres、Redis、CLIProxyAPI、nginx 或 Cloudflare Tunnel。
  - 轮询 `http://127.0.0.1:18080/health`。
- 默认 detached：
  - 前台进程确认参数后，用 `nohup` 启动自身 `--foreground --yes`。
  - 日志写入 `deploy/logs/redeploy-sub2api-YYYYMMDD-HHMMSS.log`。
  - 用户断联后后台流程继续完成。
- 支持参数：
  - `--foreground`
  - `--dry-run`
  - `--yes`
  - `--image`
  - `--compose-file`
  - `--env-file`
  - `--service`
  - `--dockerfile`
  - `--context`
  - `--health-url`
  - `--timeout`
  - `--interval`

## 测试策略

新增 `deploy/redeploy-sub2api-image.test.sh`：

- `--help` 必须说明断联风险、detached 行为和 dry-run。
- dry-run 必须打印 `docker build` 和 `docker compose ... up -d --no-deps --force-recreate sub2api`。
- dry-run 不得包含 postgres、redis、cli-proxy、nginx。
- foreground mock 执行只调用 mock Docker 和 mock curl。
- 后台模式 dry-run 必须提示日志路径，不实际启动后台任务。

## 验证命令

```bash
bash -n deploy/redeploy-sub2api-image.sh
bash -n deploy/redeploy-sub2api-image.test.sh
bash deploy/redeploy-sub2api-image.test.sh
bash deploy/redeploy-sub2api-image.sh --dry-run --yes
```

以上命令不替换镜像、不重启容器。
