# 本地 Sub2API 与 CLIProxyAPI 启动前备份设计

## 背景

用户要求先不写代码，先写设计文档：本地需要完整启动当前 Sub2API 项目，并重新启动 CLIProxyAPI。启动前必须备份 Sub2API 当前关联的 Redis 与 PostgreSQL 内容，避免启动、重启或容器切换造成数据丢失。

本文中的 “Subql API” 按当前工作区 `/Users/wujianxiang/CodeSpace/sub2api` 的 Sub2API 项目理解。

## 当前只读观察

- 当前 Git 分支：`main`，HEAD 为 `a575f28c1 chore: clean up golangci lint issues`。
- 当前运行中的 Sub2API 相关容器：
  - `sub2api-candidate`：监听 `127.0.0.1:18084->8080/tcp`，健康检查通过。
  - `sub2api-candidate-postgres`：`postgres:18-alpine`，`pg_isready` 通过。
  - `sub2api-candidate-redis`：`redis:8-alpine`，`redis-cli ping` 返回 `PONG`。
- 当前运行态数据目录不是当前 `main/deploy/candidate`，而是旧 worktree：
  - PostgreSQL bind 源：`/Users/wujianxiang/CodeSpace/sub2api/.worktrees/codex-sub2api-candidate-rehearsal-20260626/deploy/candidate/postgres_data`
  - Redis bind 源：`/Users/wujianxiang/CodeSpace/sub2api/.worktrees/codex-sub2api-candidate-rehearsal-20260626/deploy/candidate/redis_data`
  - App data bind 源：`/Users/wujianxiang/CodeSpace/sub2api/.worktrees/codex-sub2api-candidate-rehearsal-20260626/deploy/candidate/data`
- 当前 candidate Redis 启动参数为 `--save "" --appendonly no`，没有周期 RDB，也没有 AOF；重启前必须主动导出当前内存态。
- 当前 CLIProxyAPI 不是 Docker 容器在跑；端口 `8317` 由宿主机进程监听：
  - PID：`24104`
  - cwd：`/Users/wujianxiang/CodeSpace/CLIProxyAPI`
  - 启动参数：`--config config.yaml`
  - stdout/stderr 指向 `/private/tmp/cli-proxy-http.log`

## 目标

- 在不写业务代码的前提下，形成一套可执行的本地启动/重启流程。
- 在任何启动或重启动作之前，先备份当前运行态 PostgreSQL 与 Redis。
- 备份文件必须可验证、权限收紧、路径明确，且不能打印或提交密钥、JWT、API Key、OAuth token。
- 尽量保持现有运行态数据层不被重建：优先重启应用进程/容器，不重建 PostgreSQL 与 Redis。

## 非目标

- 不修改 Go、TS、Vue、Dockerfile 或 Compose 代码。
- 不切换公网 nginx 或 Cloudflare Tunnel。
- 不删除、覆盖、重命名历史 `docs/ai/context/` 文档。
- 不用当前 `main/deploy/candidate` 目录盲目替代正在运行的旧 worktree 数据目录。
- 不执行 `docker compose down -v`、`rm -rf postgres_data redis_data`、`docker volume prune` 等破坏性命令。

## 方案取舍

### 方案 A：按当前运行态精确保守重启

先从 `docker inspect` 读取真实容器、真实挂载、真实端口，再备份 `sub2api-candidate-postgres` 和 `sub2api-candidate-redis`。备份通过后，只重启需要重启的应用层：Sub2API 应用容器与 CLIProxyAPI 宿主机进程。PostgreSQL 与 Redis 保持原容器与原数据目录。

优点：最贴近当前运行态，最不容易误伤数据；符合近期 18084 候选环境一直采用的“只替换/重启应用，保留数据层”原则。

缺点：如果目标其实是新建一套完全独立的本地 dev 栈，这个方案不会自动创建 `sub2api-dev`。

### 方案 B：使用当前 `main/deploy/docker-compose.dev.yml` 新建 dev 栈

从当前源码构建并启动 `sub2api-dev`、`sub2api-postgres-dev`、`sub2api-redis-dev`。启动前仍备份当前 candidate 数据层，但不把 candidate 数据自动导入 dev 栈。

优点：适合纯开发验证，不影响 18084 candidate。

缺点：与“当前内容”不是同一套数据；如果误以为 dev 栈继承了 candidate 数据，会出现账号、订阅、Key 不一致。

### 方案 C：迁移/克隆 candidate 数据到当前 main 的 local/dev 栈

先备份 candidate Postgres/Redis，再把备份恢复到当前 `main/deploy/postgres_data`、`main/deploy/redis_data` 或 dev 栈。

优点：可以得到一套当前 main 目录下独立可反复启动的本地环境。

缺点：风险最高，涉及恢复、迁移、端口和数据一致性判断；若没有明确需要，不应作为第一步。

## 推荐设计

采用方案 A：先备份当前运行态数据，再保守重启当前链路。

原因：

- 用户强调“当前内容进行备份，防止启动之后数据丢失”，说明第一优先级是保护现有数据，而不是重新建一套空 dev 环境。
- 当前 Sub2API 已有运行中的 candidate 容器与真实数据目录；直接从运行态容器导出备份比从文件路径猜测更可靠。
- CLIProxyAPI 当前是宿主机进程，不是容器；重启应按实际进程处理。

## 备份设计

### 备份目录

统一使用当前仓库的 `deploy/backups/`：

- PostgreSQL：`deploy/backups/YYYYMMDD-HHMMSS-sub2api-candidate-postgres-before-local-restart.dump`
- Redis：`deploy/backups/YYYYMMDD-HHMMSS-sub2api-candidate-redis-before-local-restart.rdb`
- 备份记录：`docs/ai/context/YYYYMMDD-HHMMSS-local-sub2api-cliproxyapi-restart-backup-result_CN.md`

备份文件权限设置为 `600`。备份结果文档只记录路径、大小、校验命令结果和健康检查结果，不记录 dump 内容、RDB 内容、环境变量、密码、API Key、JWT secret 或 OAuth token。

### PostgreSQL 备份

使用容器内 `pg_dump -Fc` 导出 custom format dump。导出后用 `pg_restore -l` 验证目录可读，并记录 `schema_migrations` 数量和最新 migration。

备份失败时立刻停止后续启动/重启动作。

### Redis 备份

因为当前 Redis 禁用了周期持久化与 AOF，不能依赖已有 `/data/dump.rdb`。备份应主动从当前 Redis 内存态导出 RDB，再用 `redis-check-rdb` 校验。

可选实现：

- 在容器内执行 `redis-cli --rdb /tmp/sub2api-candidate-redis.rdb`，再 `docker cp` 到 `deploy/backups/`。
- 或执行 `redis-cli SAVE` 后复制 `/data/dump.rdb`。若采用该方式，必须确认生成时间与本轮备份时间一致。

推荐使用 `redis-cli --rdb`，因为它不依赖 Redis 当前持久化配置，也不会误拿旧文件。

备份失败时立刻停止后续启动/重启动作。

## 启动/重启设计

### Sub2API

先确认目标是当前 candidate 链路：

- 容器名：`sub2api-candidate`
- 端口：`127.0.0.1:18084`
- 健康检查：`http://127.0.0.1:18084/health`

备份通过后，优先只重启应用容器：

```bash
docker restart sub2api-candidate
```

不重启、不删除、不重建：

- `sub2api-candidate-postgres`
- `sub2api-candidate-redis`
- nginx
- Cloudflare Tunnel

重启后检查：

- `docker ps --filter name=sub2api-candidate`
- `curl -fsS http://127.0.0.1:18084/health`
- 视需要再检查公网健康入口，但不改变 nginx。

### CLIProxyAPI

当前 CLIProxyAPI 是宿主机进程，cwd 为 `/Users/wujianxiang/CodeSpace/CLIProxyAPI`，配置为 `config.yaml`。重启流程应按宿主机进程处理，而不是 Docker。

推荐步骤：

1. 记录当前 PID、cwd、命令行、监听端口。
2. 先确认 8317 当前能响应核心端点，例如 `/v1/models`，而不是只请求 `/health`，因为当前 `/health` 返回 404。
3. 结束旧进程。
4. 从 `/Users/wujianxiang/CodeSpace/CLIProxyAPI` 重新启动：

```bash
nohup go run ./cmd/server --config config.yaml > /private/tmp/cli-proxy-http.log 2>&1 &
```

或复用已有本地二进制：

```bash
nohup ./cli-proxy-api --config config.yaml > /private/tmp/cli-proxy-http.log 2>&1 &
```

推荐优先复用已有二进制 `./cli-proxy-api`，因为当前进程来自 `go run` 的临时构建产物，重启时再次 `go run` 会重新构建并引入不必要变量；除非明确要运行最新源码。

重启后检查：

- `lsof -nP -iTCP:8317 -sTCP:LISTEN`
- `curl -fsS http://127.0.0.1:8317/v1/models`，只记录是否 200 与模型列表存在，不打印完整账号或 token。
- 必要时检查 `/private/tmp/cli-proxy-http.log` 最近错误，不打印敏感凭据。

## 回滚设计

### Sub2API 应用回滚

如果只是 `docker restart sub2api-candidate` 后不健康：

1. 查看 `docker logs --tail 200 sub2api-candidate`。
2. 再次重启同一容器。
3. 若仍失败，保留 PostgreSQL 与 Redis 不动，按最近一次成功镜像替换应用容器。

### PostgreSQL 数据回滚

只有确认启动/重启导致数据被破坏时才恢复 dump。恢复前必须再次备份当前坏态，避免覆盖排查证据。

恢复方向：

```bash
pg_restore --clean --if-exists --no-owner --dbname=sub2api <backup.dump>
```

实际执行前需要单独确认，因为这会覆盖数据库内容。

### Redis 数据回滚

只有确认 Redis 内存态丢失且需要恢复时才导入 RDB。恢复前必须停止写入方或暂停 Sub2API，避免恢复过程中产生新写入丢失。

Redis 回滚需要单独确认执行窗口，不作为普通启动流程的一部分。

## 验收标准

- 备份文件存在，权限为 `600`。
- PostgreSQL dump 可被 `pg_restore -l` 读取。
- Redis RDB 可被 `redis-check-rdb` 校验。
- 备份后、重启前，`sub2api-candidate-postgres` 与 `sub2api-candidate-redis` 仍健康。
- Sub2API 重启后 `http://127.0.0.1:18084/health` 返回 200。
- CLIProxyAPI 重启后 `8317` 有监听，核心模型端点可响应。
- 全程不打印、不提交密钥和 token。

## 待确认点

- 是否确认本次“完整启动当前 Sub2API 项目”指当前已运行的 `sub2api-candidate` / `18084` 链路，而不是新建一套 `sub2api-dev`。
- 是否允许我在你确认后先实际执行 PostgreSQL 与 Redis 备份，再执行 Sub2API 与 CLIProxyAPI 重启。
