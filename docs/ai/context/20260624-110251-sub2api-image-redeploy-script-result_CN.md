# Sub2API 镜像替换发布脚本实现结果

## 变更

- 新增 `deploy/redeploy-sub2api-image.sh`
  - 构建新镜像 `weishaw/sub2api:latest`。
  - 使用 `deploy/docker-compose.local.yml` 和 `deploy/.env.scheme-a.local` 重建 `sub2api` 服务。
  - 只重建 Sub2API，不重启 Postgres、Redis、CLIProxyAPI、nginx 或 Cloudflare Tunnel。
  - 默认 detached 后台执行，日志写入 `deploy/logs/redeploy-sub2api-*.log`，避免 `https://api.aaccx.pw/v1/*` 重启导致当前 Codex 会话断开后流程中断。
  - 前台调试可用 `--foreground`，演练可用 `--dry-run`。
- 新增 `deploy/redeploy-sub2api-image.test.sh`
  - 使用 dry-run 和 mock Docker/curl 验证构建、重建和健康检查命令。
  - 不触碰真实 Docker，不替换镜像，不重启容器。

## 常用命令

先演练：

```bash
./deploy/redeploy-sub2api-image.sh --dry-run --yes
```

正式一键替换镜像并重建 Sub2API 容器：

```bash
./deploy/redeploy-sub2api-image.sh --yes
```

前台执行，便于观察完整输出：

```bash
./deploy/redeploy-sub2api-image.sh --yes --foreground
```

如果 Docker CLI 不在 PATH，可显式指定 Docker Desktop 路径：

```bash
./deploy/redeploy-sub2api-image.sh --yes --docker-bin /Applications/Docker.app/Contents/Resources/bin/docker
```

## 验证

已执行：

```bash
bash -n deploy/redeploy-sub2api-image.sh
bash -n deploy/redeploy-sub2api-image.test.sh
bash deploy/redeploy-sub2api-image.test.sh
bash deploy/redeploy-sub2api-image.sh --dry-run --yes --context . --docker-bin docker
```

结果：

- shell 语法检查通过。
- 测试输出 `redeploy-sub2api-image tests passed`。
- dry-run 展示 detached 后台命令、`docker build`、`docker compose ... up -d --no-deps --force-recreate sub2api` 和健康检查命令。
- 本次未执行真实镜像构建、未替换容器、未重启服务。

## 注意

真实执行 `./deploy/redeploy-sub2api-image.sh --yes` 会构建镜像并重建 `sub2api` 容器，期间 `https://api.aaccx.pw/v1/*` 会短暂中断。脚本默认后台 detached 继续执行，断联后可用日志查看进度：

```bash
tail -f deploy/logs/redeploy-sub2api-*.log
```
