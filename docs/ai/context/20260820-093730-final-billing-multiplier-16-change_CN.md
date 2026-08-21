# 18082 隐藏最终计费倍率调整为 16 倍

## 变更

- 按管理员要求，将生产 `sub2api-official-18082` 的隐藏最终计费倍率由 `18x` 调整为 `16x`。
- 唯一配置变更为 `deploy/docker-compose.18082.yml` 中的 `BILLING_FINAL_MULTIPLIER=16`。
- 仅影响配置生效后的新请求；历史用量、用户余额、订单、模型分组倍率和账户统计倍率不重算、不修改。

## 发布边界

- 只替换应用容器 `sub2api-official-18082`。
- PostgreSQL、Redis、Nginx、Cloudflare Tunnel 和数据卷保持不变。

## 验证

- 应用容器 `sub2api-official-18082` 已替换为容器 ID `2a898172d3ea25ecbbcc11f3a28757149013f76b34d61ba9fa1a17506687d672`，使用当前工作区 `main` 构建的镜像 `sha256:b2a892da1f52985e411cd31951e659c6685dbbe939c0f9c7723c155ef3f76132`。
- 运行态环境变量已回读为 `BILLING_FINAL_MULTIPLIER=16`。
- 应用容器状态为 `running (healthy)`。
- `127.0.0.1:18082/health`、本地 Nginx `127.0.0.1:8080/health` 及 `aaccx.pw`、`www.aaccx.pw`、`api.aaccx.pw` 的 `/health` 均返回 200。
- PostgreSQL、Redis、Nginx、Cloudflare Tunnel 和数据卷未重建。
