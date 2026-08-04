# 2026-08-04 aaccx.pw 公网链路切换记录

## 目标

让当前 18082 Docker 实例通过既有公网链路提供服务：

`aaccx.pw -> Cloudflare -> Cloudflare Tunnel -> Nginx -> 127.0.0.1:18082`

## 根因

域名 DNS 和 Cloudflare Tunnel 均正常，但宿主机挂载到 `sub2api-public-nginx-local` 的 Nginx 配置仍使用：

`host.docker.internal:18080`

18080 已停止，因此公网请求全部由 Nginx 返回 502。

## 变更

- 先备份旧配置：
  `D:\CodeWorkSpace\sub2api\deploy\backups\nginx-public-local-18080-before-18082-20260804-112316.conf`
- 修改宿主机配置：
  `D:\CodeWorkSpace\sub2api\deploy\nginx-public-local-18080.conf`
- upstream 从 `host.docker.internal:18080` 改为 `host.docker.internal:18082`。
- Nginx 配置测试通过后执行容器内 reload，未重建或更换 Cloudflare Tunnel。

## 验证

- `http://127.0.0.1:18082/health`：200，应用健康。
- `http://127.0.0.1:8080/health`：200，Nginx 反向代理健康。
- `https://aaccx.pw/health`：200。
- `https://www.aaccx.pw/health`：200。
- `https://api.aaccx.pw/health`：200。
- `https://aaccx.pw/`：200，返回当前前端页面。
- `https://api.aaccx.pw/v1/models`：403，为应用鉴权响应，说明请求已到达 18082。
- `https://aaccx.pw/api/v1/payment/checkout-info`：401，为应用未认证响应，说明请求已到达 18082。

## 运行维护

Cloudflare Tunnel 当前由本机 `cloudflared.exe` 进程运行，配置文件为 `D:\CodeWorkSpace\sub2api\deploy\cloudflared-windows-aaccx.yml`。后续若 18082 端口变化，只需备份并更新 Nginx upstream，执行 `nginx -t` 后 reload，再复核三个域名的 `/health`。
