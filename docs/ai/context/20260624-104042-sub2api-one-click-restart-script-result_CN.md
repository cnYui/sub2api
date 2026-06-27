# Sub2API 一键重启脚本实现结果

## 变更

- 新增 `deploy/restart-sub2api.sh`
  - 默认提示 `https://api.aaccx.pw/v1/*` 会短暂中断。
  - 支持 `--dry-run`、`--yes`、`--build`、`--backend auto|compose|container|systemd`。
  - 默认只重启 Sub2API 本体，不重启 Postgres、Redis、CLIProxyAPI、nginx 或 Cloudflare Tunnel。
  - 默认重启后轮询 `http://127.0.0.1:18080/health`。
- 新增 `deploy/restart-sub2api.test.sh`
  - 使用 dry-run 和 mock 命令验证脚本行为，不触碰真实服务。

## 常用命令

先看将执行什么，不重启：

```bash
./deploy/restart-sub2api.sh --dry-run --yes
```

直接重启当前 Sub2API：

```bash
./deploy/restart-sub2api.sh --yes
```

发布新前端或后端镜像时，重建并重启 Sub2API：

```bash
./deploy/restart-sub2api.sh --yes --build --compose-file deploy/docker-compose.local.yml
```

注意：`--build` 只有在 Compose 服务包含 `build:` 配置时才会从源码重建；如果 Compose 文件只有 `image:`，需要先用现有镜像发布流程构建并打好相同镜像标签，再运行脚本重启。

如果部署不是 Docker Compose，可以显式指定：

```bash
./deploy/restart-sub2api.sh --yes --backend container --container sub2api
./deploy/restart-sub2api.sh --yes --backend systemd --unit sub2api
```

## 验证

已执行：

```bash
bash -n deploy/restart-sub2api.sh
bash -n deploy/restart-sub2api.test.sh
bash deploy/restart-sub2api.test.sh
bash deploy/restart-sub2api.sh --dry-run --yes --backend compose --compose-file deploy/docker-compose.local.yml
bash deploy/restart-sub2api.sh --dry-run --yes --backend compose --compose-file deploy/docker-compose.local.yml --build
```

结果：

- shell 语法检查通过。
- 测试输出 `restart-sub2api tests passed`。
- dry-run 只打印命令，没有执行真实重启。

## 注意

真实执行 `./deploy/restart-sub2api.sh --yes` 会短暂影响 `https://api.aaccx.pw/v1/*`，正在进行的 Codex 流式请求可能断开。建议先运行 `--dry-run`，确认命令和 Compose 文件正确后再执行。
