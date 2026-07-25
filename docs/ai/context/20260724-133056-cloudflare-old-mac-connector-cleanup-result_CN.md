# Cloudflare 旧 Mac 连接器清理结果

时间：2026-07-24 13:30:56

## 执行结果

已调用精确清理命令，目标为旧 Mac 连接器 `62e87b01-9be8-419f-a261-a590c9ea0463`。命令正常返回。

复核发现旧 Mac 连接器在约 8 秒内自动重新连接，因此未能永久移除。原因是 Mac 上仍有 `cloudflared` 进程运行，并持有同一 Tunnel 凭据；Cloudflare 的 connector cleanup 只能断开当前连接，不能阻止持有凭据的进程再次连接。

Windows 连接器 `230df29a-1dc0-410c-84b1-e92b5b52f656` 始终保持在线，未受影响。

## 当前验证

- Tunnel 两个连接器均接入东京 NRT 边缘节点。
- 本机 `http://127.0.0.1:18080/health` 返回 HTTP `200`。
- 公网 `https://api.aaccx.pw/health` 的响应头为 `CF-RAY: ...-NRT`。

## 后续选择

1. 在旧 Mac 上停止对应的 `cloudflared` 进程或服务。这是无中断、最小影响的永久移除方式。
2. 若无法访问旧 Mac，则需要新建或轮换 Tunnel 凭据，并将 Windows 切换到新凭据。这样可阻断旧 Mac，但会影响公网链路，需要单独计划和授权。

## 备份

变更前 Tunnel 连接快照已保存并验证可读：`C:\tmp\20260724-132916-cloudflare-tunnel-pre-old-mac-cleanup.txt`。
