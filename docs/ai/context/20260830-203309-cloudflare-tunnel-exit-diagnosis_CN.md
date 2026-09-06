# Cloudflare Tunnel 异常退出诊断

## 结论

本次公网不可用不是 Cloudflare 边缘服务或 DNS 托管故障，而是本机 Tunnel 连接器 `cloudflared.exe` 进程退出。应用容器、本地 Nginx 和 Docker 栈保持正常；Tunnel 进程消失后 Cloudflare 无可用 connector，因此域名返回 `530`。

最可能的本次直接触发因素是 Windows 在同一时间更新 `OpenAI.Codex` AppX 并强制终止相关应用进程。由于 cloudflared 是从 Codex 执行链路中以子进程方式启动的，更新导致其退出具有高度时间相关性，但系统没有记录足够信息证明 Windows 直接向 cloudflared 发送了终止信号，故保留为高概率判断而非绝对结论。

长期根因是 Tunnel 没有独立守护：现有脚本只通过 `Start-Process` 启动隐藏进程，进程退出后不会自动拉起。

## 证据时间线

- 2026-08-30 `08:36:44`（+09）：通过既有脚本启动 cloudflared，成功建立 4 条 QUIC 连接。
- 2026-08-30 `20:11:32`（+09）：Windows AppX 部署日志开始处理 `OpenAI.Codex_26.825.6671.0`，执行卸载/置换操作。
- 2026-08-30 `20:11:56`（+09）：`cloudflared-aaccx-20260830-083644.err.log` 最后一行；内容是请求流被远端取消，没有 `graceful shutdown`、`no more connections active and exiting`、panic 或 fatal 记录。
- 2026-08-30 `20:12:23`（+09）：AppX 部署日志记录 `ForceTargetApplicationShutdownOption` 和 `TerminateApplications` 成功。
- 2026-08-30 `20:12:24`（+09）：`OpenAI.Codex` 新版本安装完成。
- 2026-08-30 `20:17:40`（+09）：重新启动 cloudflared，4 条 Tunnel 连接重新注册，公网恢复。

同一窗口内没有发现 System/Application/Security 日志报告 cloudflared 崩溃、Windows 重启、网络适配器重置或计划任务停止它。Docker 应用容器在整个过程中保持运行，因此这次不是 Docker、PostgreSQL、Redis 或 Nginx 引起的公网中断。

## 历史交叉验证

- 2026-08-30 凌晨的上一次 Tunnel 结束于 `15:29:41Z`（+09 `00:29:41`），日志明确写入 `Initiating graceful shutdown due to signal terminated`；Windows 随后在 `00:35:38` 因计划内操作系统升级重启，`LastBootUpTime` 为 `00:36:03`。这是宿主机重启导致的正常退出。
- 更早的 2026-07-27 日志曾出现所有连接终止、`no more connections active and exiting`，说明当所有 QUIC 连接因网络问题丢失时，cloudflared 会自行结束。
- 2026-08-30 白天日志反复出现 `Failed to refresh DNS local resolver`，解析器为 WLAN 网关 `192.168.1.1:53`，错误为 `server misbehaving`；同时有大量远端取消的请求流。当前使用默认解析器、Cloudflare 公共 DNS 和 TCP `443` 测试均已成功，说明这是间歇性网络/DNS 隐患，不足以单独证明本次进程被它杀死。

## 启动方式缺陷

`D:\CodeWorkSpace\sub2api\deploy\start-cloudflared-windows-aaccx.ps1` 第 70-76 行仅执行隐藏的 `Start-Process`，没有服务管理器、计划任务重启策略或 watchdog；第 59-63 行还固定使用 `--no-autoupdate`。本机没有发现 cloudflared Windows 服务或包含该程序的计划任务。因此，任何系统更新、会话/进程树清理、网络异常导致的进程退出都会直接暴露为公网 `530`。

## 建议

1. 将 cloudflared 安装为独立 Windows 服务，设置自动启动和失败自动重启；或者使用独立计划任务配置“无论用户是否登录都运行”和失败重启。
2. Tunnel 进程不要挂在 Codex、PowerShell 或其他开发工具的执行链路下，避免桌面应用更新/重启影响公网组件。
3. 为实际出网的 WLAN 连接配置稳定的备用 DNS，或确保路由器 `192.168.1.1` 的 DNS 转发稳定；同时保留现有多连接 QUIC 配置。
4. 增加本地 watchdog，至少监测 `cloudflared.exe` 存活和 `https://aaccx.pw/health`，异常时自动重启并保留退出码、事件时间和日志。

## 本次边界

本次仅读取日志、Windows 事件、进程/服务/计划任务和网络状态；未修改 Cloudflare 配置、DNS、Windows 服务、计划任务、Docker、代码或业务数据。
