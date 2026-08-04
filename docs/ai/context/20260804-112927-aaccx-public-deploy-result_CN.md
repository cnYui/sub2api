# 2026-08-04 aaccx.pw 公网发布完成记录

## 发布动作

- 使用当前源码构建 `deploy-sub2api` 镜像，前端生产构建和 Go 后端构建均完成。
- 因旧容器与当前 Compose 项目标签冲突，先精确移除旧的 `sub2api-official-18082` 应用容器，再用新镜像启动同名容器。
- PostgreSQL、Redis、`data-18082` 数据目录和 Nginx/Tunnel 未被删除或重置。
- 旧 `sub2api-dev` 保持退出状态；宿主机无 18080 监听。

## 当前链路

`aaccx.pw -> Cloudflare Tunnel -> sub2api-public-nginx-local:8080 -> host.docker.internal:18082 -> sub2api-official-18082`

## 发布后验证

- `sub2api-official-18082`：running/healthy。
- `http://127.0.0.1:18082/health`：200。
- `http://127.0.0.1:8080/health`：200。
- `https://aaccx.pw/health`：200。
- `https://www.aaccx.pw/health`：200。
- `https://api.aaccx.pw/health`：200。
- `https://aaccx.pw/`：200，返回当前前端构建。
- `https://api.aaccx.pw/v1/models`：403，为应用鉴权响应，说明请求已经过公网链路到达当前 18082。
- `https://aaccx.pw/api/v1/payment/checkout-info`：401，为应用未认证响应，说明支付 API 也已到达当前 18082。
