# Sub2API 与 CLIProxyAPI Docker 网络调查计划

## 目标

- 核对当前本地与公网链路中 Sub2API、CLIProxyAPI 的容器化和端口事实。
- 比较 `host.docker.internal + HTTPS` 与用户定义 Docker bridge 网络直连的可靠性、安全性、可运维性和迁移成本。
- 明确 TLS 证书在单机容器网络中的必要边界，避免把传输加密、身份认证和网络隔离混为一谈。
- 给出适合当前项目的目标架构和分阶段迁移建议，不修改当前运行态。

## 调查范围

- `sub2api` 的 Compose、部署 Runbook、上游 `base_url` 与 usage event 回调配置。
- `CLIProxyAPI-private` 的 Dockerfile、Compose、监听端口、TLS、证书 SAN、账号与密钥挂载方式。
- 当前 Docker 容器、端口发布、网络归属和容器间解析能力。
- Docker 官方关于用户定义 bridge 网络、端口发布、DNS 服务发现和网络隔离的建议。

## 约束

- 只做只读运行态检查，不停止、重建或接入任何容器网络。
- 不打印或记录完整 API Key、HMAC secret、OAuth 凭据和证书私钥。
- 不把本地开发结论直接套用到公网环境；公网迁移需要单独计划、备份、回滚和验证。

## 输出

- 当前事实与端口纠正。
- 方案对比与推荐结论。
- 推荐拓扑、TLS 策略、Compose/network 归属和迁移步骤。
- 风险、回滚边界与后续实施条件。
