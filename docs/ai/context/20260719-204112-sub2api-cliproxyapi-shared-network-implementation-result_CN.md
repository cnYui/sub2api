# Sub2API 与 CLIProxyAPI 本地共享网络实施结果

## 实施范围

- 仅修改本地 `sub2api-dev:8080` 与 `cliproxyapi-local-dev:8317` 开发链路。
- 未修改公网候选环境、Nginx、Cloudflare Tunnel、18084 容器或生产 CLIProxyAPI。
- PostgreSQL、Redis 未重建、未迁移，继续只连接原数据网络。
- 未提交、未推送、未改写 Git 历史。

## 文件改动

### Sub2API

- `Dockerfile`
  - 将原有 CLIProxyAPI 旧证书注册为标准 CA 信任锚，避免运行时执行 `update-ca-certificates` 时丢失旧信任。
- `deploy/docker-entrypoint.sh`
  - 新增可选 `CLIPROXY_CA_CERT_FILE` 注入；未设置时保持原启动路径。
  - 文件不可读或不是 PEM 证书时启动失败，CA 在降权前导入。
- `deploy/docker-compose.cliproxy-local.yml`
  - 只让 `sub2api` 额外加入外部网络 `sub2api-cliproxy-local`，别名为 `sub2api`。
  - 只读挂载 CLIProxyAPI CA 公钥，并集中配置本地 usage event、OPS 与 URL allowlist 环境变量。
- `deploy/docker-entrypoint.test.sh`
  - 覆盖未配置 CA、成功导入 CA、缺失/错误 CA 失败和旧 CA 标准安装方式。

### CLIProxyAPI-private

- `docker-compose.sub2api-local.yml`
  - 固定 Compose project 为 `cliproxyapi-sub2api-local`，容器名为 `cliproxyapi-local-dev`。
  - 本地源码构建 `cliproxyapi-local:dev`，只发布 `127.0.0.1:8317`。
  - 加入外部网络 `sub2api-cliproxy-local`，别名为 `cliproxyapi`。
  - 配置、证书只读挂载；账号与日志目录可写；未发布 OAuth 回调端口。
  - usage event 回调使用 `http://sub2api:8080/api/internal/usage-events`。
- `generate-sub2api-local-tls.ps1`
  - 自动定位 PATH 或 Git for Windows 的 OpenSSL。
  - 生成 RSA 3072/SHA-256 内部 CA 和 serverAuth 叶子证书。
  - 叶子 SAN 包含 `cliproxyapi`、`localhost`、`host.docker.internal`、`127.0.0.1`。
  - 默认拒绝覆盖已有证书，只有 `-Force` 允许轮换。
- `.gitignore`、`.dockerignore`、`.env.example`
  - 忽略 CA 私钥和运行时证书目录，排除 Docker 构建上下文，补充本地集成变量示例。
- 删除工作区中的旧跟踪文件 `certs/tls.crt`、`certs/tls.key`；未改写历史。

## 备份与回滚保护

- Sub2API 备份目录：`deploy/backups/shared-network-20260719-195935/`
  - PostgreSQL custom dump：约 100.6 MB，`pg_restore --list` 通过。
  - Redis RDB：`redis-check-rdb` 校验通过，共 112 个键。
  - 容器元数据 JSON 可读。
- CLIProxyAPI 备份目录：`D:\CodeWorkSpace\CLIProxyAPI-private\backups\20260719-195935-before-shared-network\`
  - 运行文件 tar 可列出，包含 `.env`、`config.yaml`、`auths/` 和旧 `certs/`。
- Sub2API 回滚镜像：`sub2api-localdev-sub2api:before-cliproxy-network-20260719-195935`。
- 旧 CLIProxyAPI 容器保留为 `cliproxyapi-local-dev-before-shared-network-20260719-195935`，当前为停止状态。

## 最终运行态

```text
sub2api-localdev_sub2api-network:
  sub2api-dev + sub2api-postgres-dev + sub2api-redis-dev

sub2api-cliproxy-local:
  sub2api-dev(alias: sub2api) + cliproxyapi-local-dev(alias: cliproxyapi)
```

- `sub2api-dev`：healthy，绑定 `127.0.0.1:8080`。
- `cliproxyapi-local-dev`：healthy，绑定 HTTPS `127.0.0.1:8317`。
- 共享网络成员严格为上述两个应用容器。
- PostgreSQL、Redis 容器 ID 在两次应用重建中保持不变。
- 账号 `cliproxy-local-openai` 的数据库与 Redis 调度快照均为：
  - `https://cliproxyapi:8317/v1`
- 上游 URL 通过已有本地管理员页面调用正式管理 API 更新，没有直接写 PostgreSQL 或 Redis。

## 验证结果

- Compose 渲染：
  - Sub2API 同时连接数据网络与共享网络。
  - PostgreSQL、Redis 只连接数据网络。
  - CLIProxyAPI 只连接共享网络，只发布回环 8317。
  - CA、配置挂载为只读。
- DNS 与 TLS：
  - Sub2API 可解析 `cliproxyapi`，CLIProxyAPI 可解析 `sub2api`。
  - Sub2API 容器通过系统 CA bundle 访问 `https://cliproxyapi:8317`，TLS 校验通过后得到预期 401。
  - 宿主机 OpenSSL 使用 CA 校验 `127.0.0.1:8317`，TLS 1.3 与主机名 `localhost` 验证通过。
- 证书：
  - CA/叶子关系校验通过，CA 与叶子私钥均可读。
  - CA 有效期 10 年；叶子有效期 825 天。
  - RSA 3072、serverAuth 和四项 SAN 均通过检查。
  - 默认重复运行生成脚本会拒绝覆盖。
- 业务：
  - `/health` 返回 200。
  - `/v1/models` 返回 200，共 13 个模型。
  - 普通与流式最小 `/v1/responses` 请求均经新服务名到达 CLIProxyAPI，并因空账号池返回预期 502。
  - 账号仍为 active/schedulable，没有被 502 标记错误。
- usage event：
  - 从 CLIProxyAPI 容器通过 `sub2api` 服务名发送签名正确的失败事件，返回 200、`skipped=true`。
  - Sub2API 日志确认来源 IP 为共享网络中的 CLIProxyAPI 容器。
  - 探针没有创建 `usage_facts`，没有产生扣费。
- 重建：
  - 单独重建 Sub2API 后，数据库/Redis、DNS、CA、TLS 和回调正常。
  - 单独重建 CLIProxyAPI 后，Sub2API/数据库/Redis 未被替换，DNS、TLS、业务和回调正常。
- 自动化与静态检查：
  - `deploy/docker-entrypoint.test.sh` 通过。
  - CLIProxyAPI `go test ./internal/usage` 通过。
  - CLIProxyAPI 最终 `go test ./...` 全量通过；首次全量运行中 `TestHandleEventConfigChangeSchedulesReload` 因固定睡眠时序偶发失败，随后单测连续 20 次和第二次全量运行均通过，未修改无关 watcher 代码。
  - CLIProxyAPI `go build -o test-output ./cmd/server` 通过，临时产物已删除。
  - 两仓库 `git diff --check` 通过。
  - 新增/修改的 10 个实施文件敏感信息扫描无发现。
  - 新 CA 私钥、叶子私钥和运行时证书均被 Git 忽略，且不在 Docker 构建上下文。

## 已知边界

- CLIProxyAPI 本地 `auths/` 当前为空，因此无法验证成功模型响应及成功 usage event 的完整计费链路；本次只验证网络、TLS、失败请求和无副作用回调。
- 成功 usage event 路径当前仍使用独立 `cliproxy:` 请求 ID 并固定余额计费来源，存在与 Sub2API 主计费链重复或错来源的架构风险；该问题不属于本次网络改造，后续应按计费来源统一设计单独处理。
- Windows `curl --cacert` 使用 Schannel 时会因本地吊销状态不可用失败；OpenSSL 和容器 CA 验证均通过，证书链本身正常。

## 回滚

1. 通过管理页面/API 将账号 URL 恢复为 `https://host.docker.internal:8317/v1`。
2. 停止新 CLIProxyAPI Compose 容器，恢复备份配置、账号目录和旧证书，启动保留的旧容器。
3. 如 Sub2API 入口脚本异常，使用预留回滚镜像重建 `sub2api-dev`。
4. PostgreSQL/Redis 备份只在确认数据损坏时人工恢复，不用于普通回滚。
