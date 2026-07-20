# Sub2API 与 CLIProxyAPI 本地开发环境设计

## 目标

- 从当前 `sub2api` 与 `D:\CodeWorkSpace\CLIProxyAPI-private` 源码启动一套本地开发环境。
- 将附件中的 PostgreSQL 备份恢复为本地参考数据副本。
- 验证 Sub2API 到 CLIProxyAPI 的请求转发和 CLIProxyAPI 到 Sub2API 的 usage event 回传链路。
- 不连接、不复制、不挂载本机全局 CLIProxyAPI 账号池；仓库内账号池保持为空，后续由用户自行添加。

## 已确认条件

- PostgreSQL 备份为 `pg_dump -Fc` 自定义格式，文件约 100.5 MB。
- Redis 备份为 RDB，包含 941 个键，但 Redis 数据属于会话、限流、调度快照等可重建运行态。
- 两个项目都要求 Go 1.26，本机 Go 为 1.25.7，因此使用 Docker 内的 Go 1.26 构建环境。
- CLIProxyAPI TLS 证书包含 `localhost`、`host.docker.internal` 和 `127.0.0.1`，Sub2API 镜像已内置对应 CA。
- CLIProxyAPI 仓库当前没有 `config.yaml`、`.env` 和账号 JSON，符合本次空账号池要求。

## 本地架构

```text
浏览器
  -> http://127.0.0.1:8080
  -> Sub2API 本地源码镜像
       -> PostgreSQL 本地恢复副本
       -> 空 Redis
       -> https://host.docker.internal:8317/v1
       -> CLIProxyAPI 本地源码镜像

CLIProxyAPI
  -> http://host.docker.internal:8080/api/internal/usage-events
  -> Sub2API usage_facts
```

## 数据隔离

- 原始 ZIP 只读保留，不在原路径解压或修改。
- PostgreSQL 恢复到 `deploy/postgres_data` 对应的新本地容器，不覆盖任何已有数据库。
- Redis 空启动，不恢复生产 RDB，避免带入公网会话、锁、限流状态和历史账号调度快照。
- CLIProxyAPI 使用仓库内 Git 忽略的 `auths/`；不访问 `C:\Users\yui\.cli-proxy-api`。
- 本地密钥、HMAC secret、数据库密码和 CLIProxyAPI API Key 只写入 Git 忽略文件，不写入文档或日志摘要。

## 通信配置

- 从恢复数据库中读取 Sub2API 当前 CLIProxyAPI 上游账号的地址与 API Key，只用于生成本地 CLIProxyAPI `config.yaml`。
- 保持数据库中的 HTTPS 上游地址，CLIProxyAPI 在 `8317` 启用仓库证书。
- Sub2API 与 CLIProxyAPI 使用同一组本地 usage event token 和 HMAC secret。
- CLIProxyAPI 账号池为空时，服务和鉴权链路仍可启动；模型请求预期返回无可用账号，用户添加账号后再验证真实上游响应。

## 外部副作用控制

- 所有服务仅绑定 `127.0.0.1`，不启动 Nginx、Cloudflare Tunnel 或公网入口。
- 本地数据库副本中关闭订阅到期邮件等主动外发开关，避免向真实用户发送消息。
- 不修改公网数据库、Redis、容器、Nginx 或 CLIProxyAPI 运行态。

## 验证标准

- PostgreSQL dump 可列出 TOC，并能恢复到空库。
- Sub2API、PostgreSQL、Redis 和 CLIProxyAPI 容器处于运行状态。
- `http://127.0.0.1:8080/health` 返回成功。
- `https://127.0.0.1:8317` 可以完成 TLS 握手。
- Sub2API 恢复出的 CLIProxyAPI 上游地址、池模式和重试状态码符合主链路要求。
- 空账号池模型请求失败原因来自 CLIProxyAPI 无可用认证，而不是网络、TLS 或 Sub2API 上游配置错误。
