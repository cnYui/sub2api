# 旧 Mac Cloudflare 自动重连与公网 502 验证

时间：2026-07-24 13:36:39

## 验证结果

用户在 Mac 上停止 Cloudflare 后，旧的 Mac 连接器 ID 已消失，但 Tunnel 随即出现新的 Mac 连接器：

- 新 Mac 连接器：`dd80c67d-75b3-4474-985f-131fd9a8050d`
- 架构：`darwin_arm64`
- 新建时间：`2026-07-24 13:35:17 +09`
- Windows 连接器仍在线：`230df29a-1dc0-410c-84b1-e92b5b52f656`

这证明 Mac 上仍存在自动启动机制，会在进程结束后重新拉起 `cloudflared`。

## 公网影响

- Windows 本机 `http://127.0.0.1:18080/health`：HTTP `200`。
- 对 `https://api.aaccx.pw/health` 连续 12 次无鉴权探测：全部 HTTP `502 Bad Gateway`。
- 12 次响应的 `CF-RAY` 后缀均为 `NRT`，证明当前边缘节点是东京，不是香港。

旧 Mac 连接器正在接收公网流量，但其本机 `127.0.0.1:8080` 不提供当前源站服务，因此造成公网 502。

## 必须的 Mac 侧操作

需要停止并卸载启动项，而不是只结束前台进程：

1. 查找：`pgrep -alf cloudflared`、`launchctl list | grep -i cloudflared`。
2. 若为用户 LaunchAgent，使用对应 plist 执行 `launchctl bootout gui/$(id -u) <plist路径>`。
3. 若为系统 LaunchDaemon，使用 `sudo launchctl bootout system <plist路径>`。
4. 最后执行 `pkill -f cloudflared`，并保持 Mac 进程列表中不再出现它。

完成后重新查询 Tunnel，只保留 Windows `windows_amd64` 连接器，再复测公网。
