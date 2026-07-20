# Sub2API 与 CLIProxyAPI Docker 网络调查结果

## 结论

当前本地 CLIProxyAPI 已经运行在 Docker 容器中，问题不是“是否容器化”，而是两个容器没有共享用户定义网络：

- `sub2api-dev` 位于 `sub2api-localdev_sub2api-network`。
- `cliproxyapi-local-dev` 位于 Docker 默认 `bridge`。
- Sub2API 通过宿主机发布端口和 `host.docker.internal` 访问 CLIProxyAPI。
- 当前实际端口是 `8080` 和 `8317`，不是 `8137`。

对当前单机架构，专用用户定义 bridge 网络优于现状。推荐保持两个 Compose 项目独立，只新增一张只连接 Sub2API 与 CLIProxyAPI 的共享外部网络，通过稳定服务别名直连；不要把 CLIProxyAPI 加入 PostgreSQL/Redis 所在网络。

Docker 网络与 TLS 是两个独立问题。推荐生产环境采用“共享专用网络 + TLS”，本地开发可以继续使用同一策略以保持一致；如果只追求本地简化，也可以在专用网络内使用 HTTP，但不应把这个选择直接推广到生产。

## 当前事实

### 容器和网络

| 容器 | 当前网络 | 宿主端口 |
|---|---|---|
| `sub2api-dev` | `sub2api-localdev_sub2api-network` | `127.0.0.1:8080 -> 8080` |
| `cliproxyapi-local-dev` | 默认 `bridge` | `127.0.0.1:8317 -> 8317` |

当前双向通信：

```text
Sub2API -> https://host.docker.internal:8317/v1 -> CLIProxyAPI
CLIProxyAPI -> http://host.docker.internal:8080/api/internal/usage-events -> Sub2API
```

Sub2API 数据库中的 CLIProxyAPI 上游仍为 `https://host.docker.internal:8317/v1`，池模式已启用。CLIProxyAPI usage event 回调仍指向宿主机 `8080`。

### CLIProxyAPI 当前并非由仓库 Compose 管理

- `cliproxyapi-local-dev` 没有 Compose 标签，是手工创建的容器。
- 当前容器额外挂载了 `certs/`，仓库现有 `docker-compose.yml` 没有该挂载。
- 仓库现有 Compose 将 `8317` 和多个 OAuth 回调端口发布到所有宿主机接口；直接使用会比当前仅绑定 `127.0.0.1` 的运行方式暴露更大。
- 因此不能直接执行现有 CLIProxyAPI Compose 作为迁移方案，必须先修正网络、证书挂载和端口绑定。

### 当前 TLS 耦合

- CLIProxyAPI 证书 SAN 只覆盖 `localhost`、`host.docker.internal`、`127.0.0.1` 和少量 Docker Desktop IP。
- 证书不覆盖推荐服务名 `cliproxyapi`，直接改成 `https://cliproxyapi:8317` 会导致主机名校验失败。
- Sub2API 镜像构建时写入的证书与 CLIProxyAPI 当前证书哈希一致。
- 当前证书是自签名端点证书，Subject 与 Issuer 相同；它同时承担服务证书和信任锚角色，轮换时需要同步重建 Sub2API 镜像。
- 证书私钥当前被跟踪在 CLIProxyAPI 私有仓库。虽然不是公开仓库，仍不适合作为长期生产密钥分发方式。

## Docker 官方依据

Docker 官方文档明确说明：

- 用户定义 bridge 优于默认 `bridge`，提供容器名/别名 DNS、范围更小的网络隔离，并支持运行时连接和断开。
- 同一用户定义网络内的容器可以直接访问容器端口，不需要发布宿主端口。
- Compose 跨项目通信的正式方式是预先创建外部网络，并在两个 Compose 项目中声明 `external: true`。
- 服务间通信应使用容器端口和稳定服务名，不应依赖动态容器 IP 或宿主端口。
- 发布端口会扩大访问面；如果服务只供同网络容器使用，就不需要发布。

参考：

- https://docs.docker.com/engine/network/drivers/bridge/
- https://docs.docker.com/compose/how-tos/networking/
- https://docs.docker.com/engine/network/port-publishing/

## 方案对比

| 方案 | 优点 | 缺点 | 结论 |
|---|---|---|---|
| 保持 `host.docker.internal + HTTPS` | 当前已验证，改动最少 | 依赖 Docker Desktop/宿主端口/NAT，双向地址不便携，存在端口冲突和额外暴露面 | 可继续临时使用，不适合作为目标架构 |
| 专用 Docker 网络 + HTTP | 最简单，无证书 SAN 和信任链维护；不发布 8317 | 网络内明文传输 bearer key、请求和响应，只适合单机可信容器边界 | 可作为本地开发简化方案 |
| 专用 Docker 网络 + TLS | 同时获得 DNS、隔离、少宿主端口和传输加密 | 需要重做证书 SAN 和轮换方式 | 推荐生产目标，也可用于本地保持一致 |

## 推荐目标架构

```text
宿主机 / 浏览器
  -> 127.0.0.1:8080
  -> Sub2API
       -> 原有 Sub2API 网络 -> PostgreSQL / Redis
       -> 专用共享网络 -> https://cliproxyapi:8317/v1

CLIProxyAPI
  -> 专用共享网络 -> http://sub2api:8080/api/internal/usage-events
  -> 公网模型提供方
```

设计要求：

- 两个项目继续使用不同 Compose project，避免一个项目的 `down` 或重建误伤另一项目。
- 预创建按环境隔离的外部网络，例如本地与生产使用不同网络名。
- 共享网络只连接 Sub2API 和 CLIProxyAPI；PostgreSQL、Redis 不加入。
- Sub2API 使用稳定别名 `cliproxyapi`，CLIProxyAPI 使用稳定别名 `sub2api`。
- 生产不发布 CLIProxyAPI `8317`；本地若需要宿主机调试，只绑定 `127.0.0.1`，并放到独立调试覆盖或 profile。
- OAuth 回调端口只在执行对应登录流程时绑定 `127.0.0.1`，不要默认发布到所有接口。
- usage event 回调继续使用 token + HMAC；共享网络不会替代应用层鉴权。

## TLS 推荐

不要继续把同一张自签名端点证书同时当作服务证书和 CA。推荐：

1. 创建内部 CA，CA 公钥证书作为 Sub2API 信任锚；CA 私钥不进入代码仓库和运行容器。
2. 为 CLIProxyAPI 签发叶子证书，SAN 至少包含稳定 Docker DNS 名 `cliproxyapi`。
3. CLIProxyAPI 叶子证书和私钥以只读 secret 或 Git 忽略的只读挂载提供。
4. 叶子证书可独立轮换，不需要重建 Sub2API；只有内部 CA 轮换才需要更新信任锚。

如果本地开发决定使用 HTTP，应同时满足：共享网络只连接这两个服务、`8317` 不对外发布、API Key/HMAC 继续启用、生产配置仍使用 TLS。

## 迁移顺序

1. 先修正 CLIProxyAPI Compose，使其复现当前容器挂载、仅回环端口和运行参数。
2. 创建环境专用外部 bridge 网络，并让两个服务在保留原网络的同时加入共享网络。
3. 先保持旧宿主端口路径，验证容器 DNS、TLS 证书和双向回调。
4. 更新 Sub2API 上游地址与 CLIProxyAPI usage event URL。
5. 完成模型请求、流式请求、模型列表、usage event 和容器重建后的 DNS 恢复验证。
6. 确认稳定后再取消生产 `8317` 宿主端口发布；本地按调试需要保留回环绑定。

回滚只需要恢复两端 URL 并保留旧宿主端口路径，不涉及数据库事实、Redis 数据或账号池迁移。

## 次要设计问题

Sub2API 当前识别 CLIProxyAPI 账号时仍包含 `host.docker.internal:8317` / `127.0.0.1:8317` 字符串判断，并依赖账号名包含 `cliproxy` 作为兜底。共享网络迁移不会立即失效，但长期应改成显式账号能力/类型字段，避免网络地址参与业务身份判断。

## 本次边界

- 未停止、重建或修改任何容器、网络、数据库、Redis、Nginx 或公网链路。
- 未修改 CLIProxyAPI 配置、证书或账号池。
- 只新增调查计划和结果文档。
