# Windows 承接公网 Cloudflare Tunnel 迁移计划

## 目标

- 不再依赖 macOS 上的 `cloudflared`、Homebrew nginx、yui.web 运行态。
- 在当前 Windows 本地承接完整公网链路：

```text
aaccx.pw / api.aaccx.pw
  -> Cloudflare DNS / Cloudflare Edge
  -> Cloudflare Tunnel connector on Windows
  -> Windows nginx 127.0.0.1:8080
  -> Sub2API 127.0.0.1:18080
  -> CLIProxyAPI 127.0.0.1:8317
  -> 上游账号
```

## 当前 Windows 本地事实

- `sub2api-dev` 已映射 `127.0.0.1:18080->8080`。
- `cliproxyapi-local-dev` 已映射 `127.0.0.1:8317->8317`，并已加载 4 个 Plus auth entries。
- `sub2api-public-nginx-local` 已映射 `127.0.0.1:8080->8080`，将全部请求反代到 `18080`。
- `http://127.0.0.1:8080/health` 和 `http://127.0.0.1:18080/health` 均返回正常。

## 必要条件

- Windows 需要运行 `cloudflared` connector。
- 若继续使用原 Cloudflare Tunnel：
  - 需要原 tunnel 的 connector token；或
  - 需要原 tunnel credentials JSON（`7f5fafd9-8a59-4013-ba42-3116dfc29463.json`）和对应 `config.yml`。
- 若创建新 Tunnel：
  - 需要 Cloudflare 登录/Zero Trust 权限，用于创建 tunnel 并把 `aaccx.pw/www/api` 的 public hostname 指到新 tunnel。

## 执行策略

1. 在 Windows 查找是否已经存在原 tunnel credentials/token。
2. 若无 `cloudflared`，安装或使用 Docker 镜像运行。
3. 以原 tunnel token/credentials 启动 Windows connector，服务指向 `http://127.0.0.1:8080`。
4. 停止或暂时下线 macOS connector，避免两个不同本地入口同时承接同一 tunnel。
5. 验证：
   - `https://api.aaccx.pw/health`
   - `https://aaccx.pw/shop`
   - `https://aaccx.pw/purchase`
   - `https://api.aaccx.pw/v1/models`（需要用户 API Key 才能完整验证）

## 回滚

- 停止 Windows `cloudflared` connector。
- 恢复 macOS connector。
- 若改过 Cloudflare public hostname，恢复原 tunnel 配置。

## 不做事项

- 不把 tunnel token、credentials JSON、API Key 写入文档或 Git。
- 不改生产 DB、用户额度或支付配置。
- 不直接删除 macOS 运行态，直到 Windows 公网验证完成。
