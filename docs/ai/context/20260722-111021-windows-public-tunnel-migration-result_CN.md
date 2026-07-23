# Windows 公网 Tunnel 迁移阶段结果

## 已完成

- 当前 Windows 本地链路已就绪：
  - `sub2api-public-nginx-local`：`127.0.0.1:8080->8080`
  - `sub2api-dev`：`127.0.0.1:18080->8080`
  - `cliproxyapi-local-dev`：`127.0.0.1:8317->8317`
  - `cliproxyapi-local-dev` 已加载 4 个 Plus auth entries。
- 本地 health：
  - `http://127.0.0.1:8080/health` 返回 `{"status":"ok"}`
  - `http://127.0.0.1:18080/health` 返回 `{"status":"ok"}`
- 已在 Windows 安装 cloudflared：
  - 路径：`C:\Program Files (x86)\cloudflared\cloudflared.exe`
  - 版本：`2026.7.2`
- 已新增 Windows cloudflared ingress 配置模板：
  - `deploy/cloudflared-windows-aaccx.example.yml`
  - 指向 `aaccx.pw`、`www.aaccx.pw`、`api.aaccx.pw` -> `http://127.0.0.1:8080`
  - `cloudflared tunnel --config ... ingress validate` 通过。
- 已新增 Windows 启动脚本：
  - `deploy/start-cloudflared-windows-aaccx.ps1`
  - 优先支持原 tunnel credentials JSON。
  - 也支持通过 `TUNNEL_TOKEN` 环境变量启动，避免 token 出现在命令行参数。

## 当前阻塞

Windows 上没有原 Cloudflare Tunnel 的 connector 凭证：

- 缺少 credentials JSON：
  - `C:\Users\yui\.cloudflared\7f5fafd9-8a59-4013-ba42-3116dfc29463.json`
- 也没有 `cert.pem` / origin cert。
- `cloudflared tunnel list` 因缺 origin cert 无法列出 tunnel。
- 启动脚本预检停在：
  - 缺少 `TUNNEL_TOKEN` 环境变量或 credentials JSON。

## 需要的外部材料

二选一即可：

1. 从原 macOS 复制原 tunnel credentials JSON：
   - 源：`/Users/wujianxiang/.cloudflared/7f5fafd9-8a59-4013-ba42-3116dfc29463.json`
   - 目标：`C:\Users\yui\.cloudflared\7f5fafd9-8a59-4013-ba42-3116dfc29463.json`
2. 提供同一个 tunnel 的 connector token，并在当前 PowerShell 会话设置为环境变量 `TUNNEL_TOKEN`。

## 凭证到位后的启动命令

```powershell
powershell -ExecutionPolicy Bypass -File D:\CodeWorkSpace\sub2api\deploy\start-cloudflared-windows-aaccx.ps1
```

启动后验证：

```powershell
Invoke-WebRequest -UseBasicParsing https://api.aaccx.pw/health
Invoke-WebRequest -UseBasicParsing https://aaccx.pw/shop
Invoke-WebRequest -UseBasicParsing https://aaccx.pw/purchase
```

## macOS 下线建议

Windows connector 验证成功前，不直接删除 macOS 配置。

验证成功后再停止 macOS LaunchAgent，避免同一 tunnel 两个不同 origin 混跑：

```bash
launchctl unload ~/Library/LaunchAgents/com.wjx.cloudflared.cliproxy.plist
```

如果 Windows connector 出问题，再恢复 macOS connector。
