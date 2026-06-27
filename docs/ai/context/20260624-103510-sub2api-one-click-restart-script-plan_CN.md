# Sub2API 一键重启脚本实现计划

## 目标

新增 `deploy/restart-sub2api.sh`，让维护者能一键重启 Sub2API，同时明确提示 `https://api.aaccx.pw/v1/*` 会短暂中断，避免误重启 CLIProxyAPI、Postgres、Redis、nginx 或 Cloudflare Tunnel。

## 设计

- 默认后端为 `auto`，优先检测 Docker Compose，其次检测同名 Docker 容器，最后检测 systemd。
- 默认只重启 Sub2API 本体：
  - Compose：`docker compose -f <file> restart sub2api`
  - Compose + `--build`：`docker compose -f <file> up -d --build --no-deps sub2api`
  - Container：`docker restart sub2api`
  - systemd：`systemctl restart sub2api`
- 默认需要交互确认；`--yes` 可跳过确认；`--dry-run` 只打印将执行的命令。
- 重启后默认轮询 `http://127.0.0.1:18080/health`，超时则提示查看日志，不自动重启其它服务。
- 支持参数覆盖：
  - `--compose-file`
  - `--service`
  - `--container`
  - `--unit`
  - `--health-url`
  - `--timeout`
  - `--interval`
  - `--backend`
  - `--build`
  - `--no-health-check`

## 测试策略

先写 `deploy/restart-sub2api.test.sh`，只用 Bash、临时目录和 mock 命令验证：

- `--help` 输出断联风险说明。
- Compose dry-run 只重启 `sub2api`，不包含 Postgres/Redis/CLIProxyAPI/nginx。
- Compose `--build` dry-run 使用 `up -d --build --no-deps sub2api`。
- systemd dry-run 输出 `systemctl restart sub2api`。
- mock Docker + mock curl 的真实路径测试只调用 mock，不触碰真实服务。

## 验证命令

```bash
bash deploy/restart-sub2api.test.sh
bash -n deploy/restart-sub2api.sh
bash deploy/restart-sub2api.sh --dry-run --yes --backend compose --compose-file deploy/docker-compose.local.yml
```

以上命令不执行真实重启。
