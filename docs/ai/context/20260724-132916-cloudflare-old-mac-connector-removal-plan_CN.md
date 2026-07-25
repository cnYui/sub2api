# Cloudflare 旧 Mac 连接器移除计划

时间：2026-07-24 13:29:16

## 目标

从 Tunnel `cliproxyapi`（`7f5fafd9-8a59-4013-ba42-3116dfc29463`）移除旧 Mac 连接器，避免 Cloudflare 在 Windows 与旧 Mac 的不同本地源站之间分流。

## 当前事实

- Windows 连接器：`230df29a-1dc0-410c-84b1-e92b5b52f656`，必须保留。
- 旧 Mac 连接器：`62e87b01-9be8-419f-a261-a590c9ea0463`，目标移除。
- 两台连接器目前都接入东京 NRT 边缘节点；Tunnel 不是香港接入。
- 三个域名均经 Tunnel 转发至当前 Windows 本机 `http://127.0.0.1:8080`。

## 变更步骤

1. 已保存并验证 Tunnel 连接快照：`C:\tmp\20260724-132916-cloudflare-tunnel-pre-old-mac-cleanup.txt`。
2. 调用 `cloudflared tunnel cleanup --connector-id`，仅终止旧 Mac 连接器。
3. 重新查询 Tunnel，确认 Windows 连接器仍存在并且旧 Mac 连接器消失。
4. 从本机和公网执行健康检查，确认公网仍可达。

## 风险与回滚

- 精确 cleanup 不会删除 Tunnel、DNS 路由、Windows 配置或 Windows 连接器。
- 若旧 Mac 上的 `cloudflared` 仍在运行，它可能自动重新连接；这时必须在 Mac 上停止该进程或服务。单独删除 Tunnel/轮换全部凭据会中断 Windows，未经额外确认不执行。
- 若 Windows 连接器异常，可由现有 Windows 进程使用原配置重新连接；快照用于核对恢复状态。
