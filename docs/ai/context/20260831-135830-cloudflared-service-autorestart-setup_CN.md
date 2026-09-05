# Cloudflared 自动重启改造（服务化 + 看门狗）

## 背景与问题

用户反馈「cloudflare 进程掉线不会自己重启」，并担心「sub2api 跑在 codex 沙箱里，codex 进程一退出服务就掉」。

本次排查结论：

- **Docker 容器其实不受 codex 影响，已经是安全的。** `sub2api-official-18082`（及 redis / postgres / nginx）均由 Docker Desktop 守护进程（`com.docker.backend`）托管，`restart=unless-stopped`，以 `docker compose up -d` 后台方式启动；Docker Desktop 已在 HKCU Run 键注册为登录自启。codex 退出不影响这些容器。
- **真正跟 codex 一起死、且死后不自愈的是 cloudflared。** 检查时 cloudflared（PID 37684，当日 13:04 启动）虽在运行、公网 `aaccx.pw` / `api.aaccx.pw` / `/health` 均 200，但它的父进程（PID 23004，codex/终端 shell）已消失 —— 是一个**孤儿进程**，没有服务管理器或看门狗，未注册为 Windows 服务。
- 与既往诊断 [20260830-203309-cloudflare-tunnel-exit-diagnosis](20260830-203309-cloudflare-tunnel-exit-diagnosis_CN.md) 结论一致：上次公网 530 的根因就是 cloudflared 作为 codex 子进程被 AppX 更新强制终止，且无独立守护。

## 架构（公网链路）

```
Internet -> Cloudflare edge -> [cloudflared 孤儿进程·无监管] 
  -> 127.0.0.1:8080 (nginx 容器 sub2api-public-nginx-local)
  -> sub2api-official-18082 (127.0.0.1:18082 -> 容器 8080)  + redis / postgres
```

除 cloudflared 外均为 Docker 守护进程托管、`restart=unless-stopped`，与 codex 无关。

## 本次改动（仅新增文件，未执行、未触碰任何在跑服务）

在 `D:\CodeWorkSpace\sub2api\deploy`（与 cloudflared 配置同目录）新增两个宿主机层脚本：

1. `setup-cloudflared-service.ps1` —— 一次性安装脚本（需管理员）。
   - 通过 `cloudflared service install` 注册 Windows 服务，交由系统 SCM 托管。
   - `sc.exe config binPath=` 显式写死启动命令为 `"<exe>" tunnel --config "<cfg>" --no-autoupdate run <tunnelId>`，消除 LocalSystem 默认 config 路径歧义。
   - `sc.exe config start= auto`（开机自启）+ `sc.exe failure ... restart/5000`（崩溃 5 秒后无限重启）+ `sc.exe failureflag 1`（即使 exit 0 正常退出也触发恢复）。
   - 注册看门狗计划任务（SYSTEM，默认每 2 分钟，登录与否都跑）。
2. `cloudflared-watchdog.ps1` —— 看门狗本体，由计划任务调用。
   - 公网 `/health` = 200 时**直接退出、绝不干预**（不打扰当前在服务的进程）。
   - 公网不通但本地 `127.0.0.1:8080/health` 也不通 -> 判为下游 nginx/容器问题，**不重启 cloudflared**，仅记日志（不与 docker 抢修）。
   - 公网不通、本地正常 -> 判为隧道层假死，`Start/Restart` cloudflared 服务并复检。

固定参数：
- cloudflared: `C:\Program Files (x86)\cloudflared\cloudflared.exe`
- 配置: `D:\CodeWorkSpace\sub2api\deploy\cloudflared-windows-aaccx.yml`
- tunnelId: `${OPS_TUNNEL_ID}`
- 凭证: `C:\Users\yui\.cloudflared\${OPS_TUNNEL_ID}.json`
- 健康检查: 公网 `https://aaccx.pw/health`，本地 `http://127.0.0.1:8080/health`
- 日志: `D:\CodeWorkSpace\sub2api\deploy\logs\cloudflared-watchdog.log`

## 自动重启原理（两层）

- **第一层 · Windows 服务（SCM）**：把“保活”责任交给永不退出的系统组件 SCM。开机时 `start=auto` 拉起；进程崩溃/被杀时 failure actions + failureflag 在 5 秒后重启。SCM 只感知“进程在不在”。
- **第二层 · 看门狗计划任务**：覆盖 SCM 看不到的“进程活着但隧道不通”（如 QUIC 连接全掉、DNS 抖动），以真实请求 `aaccx.pw/health` 为准，异常才动作，并区分下游故障避免误伤。

覆盖场景：被 codex 杀掉 / 崩溃 / 开机重启 / 隧道假死。

## 安全设计（不影响本地在跑服务）

- 本次仅创建脚本文件，**未运行、未改动 cloudflared 进程、未改动 docker**。
- 脚本**默认执行**（无参数）：只安装/配置服务 + 注册看门狗，**服务保持停止但就绪**；当前孤儿进程继续服务。因看门狗“健康时不动作”，孤儿死后才由看门狗拉起服务，自然收敛到服务托管，无需人工打断。
- **`-Cutover`**：可选的零停机切换（服务连接器与旧进程并存 -> 验证公网 200 -> 停旧孤儿 -> 复检；失败自动停服务并重启独立进程回滚，保证不劣化）。

## 待用户执行（需管理员 PowerShell）

```
# 影响为零的默认安装
powershell -ExecutionPolicy Bypass -File "D:\CodeWorkSpace\sub2api\deploy\setup-cloudflared-service.ps1"

# 择机无缝切换到服务托管
powershell -ExecutionPolicy Bypass -File "D:\CodeWorkSpace\sub2api\deploy\setup-cloudflared-service.ps1" -Cutover
```

## 未决/后续

- 服务注册、failureflag、看门狗任务均为**改系统配置**，按约定由用户本人以管理员执行，AI 未代为运行。
- 既往诊断提到的 WLAN 网关 DNS（`192.168.1.1` `server misbehaving`）间歇隐患仍在，可另配稳定备用 DNS 增强抗抖动。
- 若日后要“无人登录也整栈自启”，需评估用 WSL2 + Docker Engine（systemd 常驻）替代 Docker Desktop（Docker Desktop 需用户登录）。

## 本次边界（初稿时）

初稿仅读取状态并新增两个脚本，未改系统。**后续经用户明确授权，已实际执行切换（见下）。**

---

## 执行记录（2026-08-31，用户授权后实际落地）

用户授权“把 cloudflare service 切换成脚本来重新启动”后，以管理员（UAC 提权）实际执行。过程中发现并修正了脚本 bug，最终成功切换。

### 遇到的问题与修正
1. **`sc.exe config binPath=` 引号 bug**：PowerShell 向 sc.exe 传含空格/引号的 binPath 时被截断，导致服务启动命令只剩 exe、丢了 `tunnel --config ... run <id>` 参数（会空转崩溃）。
   - 修正：改用**注册表直接写入** `HKLM\SYSTEM\CurrentControlSet\Services\Cloudflared\ImagePath`（REG_EXPAND_SZ）。
2. **`Start-Service` 长时间阻塞 + 优雅停机 30s 等待**：cloudflared 服务模式向 SCM 汇报 RUNNING 较晚，`Start-Service` 阻塞；停止时默认约 30s 优雅排空产生“Waiting for service to stop”刷屏（一度被误当异常而 Ctrl+C 中断了首次提权运行）。
   - 修正：切换改用 `sc start` + 轮询公网 + 对旧进程**强杀**，避免阻塞与长等待。
3. **计划任务“消失”实为误判**：以 `SYSTEM/最高权限` 注册的任务，**非管理员会话查询会返回 `Access is denied`**、`Get-ScheduledTask` 也不列出；任务其实一直存在并运行过（14:37 曾自动恢复一次）。
   - 修正：任务注册改用 `schtasks /sc minute /mo 2 /ru SYSTEM /rl HIGHEST /f`（语言无关、最稳定）。
4. **看门狗误触发**：14:37 因公网单次瞬时探测失败触发了一次不必要的服务重启。
   - 修正：看门狗改为**单次巡检内连续 3 次（间隔 5s）都失败**才判定故障。

### 切换效果
- 停掉旧孤儿进程 37684 时，公网 `t+0s` 有约 1–2 秒短暂空档，`t+2s` 即恢复 200，由服务连接器独立承载。与预估“0～数秒”一致。

### 最终已验证状态
- 服务 `Cloudflared`：`Running` / `Automatic`，进程 PID 38324，父进程 = `services.exe`（SCM 托管）。
- `ImagePath = "C:\Program Files (x86)\cloudflared\cloudflared.exe" tunnel --config "D:\CodeWorkSpace\sub2api\deploy\cloudflared-windows-aaccx.yml" --no-autoupdate run ${OPS_TUNNEL_ID}`
- 失败恢复：`restart/5000 ×3`，`reset=0`，failureflag 已设。
- 看门狗任务 `cloudflared-watchdog`：SYSTEM，每 2 分钟；已实测会运行并恢复。
- 只剩 1 个 cloudflared 进程（服务），旧孤儿已清除。
- `aaccx.pw` / `www.aaccx.pw` / `api.aaccx.pw` 的 `/health` 与本地 `127.0.0.1:8080/health` 全部 200。

### 运维备忘
- 查看/管理服务与任务需**管理员**会话（非管理员查任务会 Access denied，属正常，不代表任务丢失）。
- 停服务会有约 30s 优雅停机等待属正常；如需快速停止可强杀服务进程，SCM 会按恢复策略拉起。
- 看门狗日志：`D:\CodeWorkSpace\sub2api\deploy\logs\cloudflared-watchdog.log`（健康时静默，仅故障/恢复时记录）。
- 正式脚本 `setup-cloudflared-service.ps1` 已更新为上述有效方法，可幂等复现（默认安装，`-Cutover` 零停机切换）。

## 后续
- 既往诊断的 WLAN 网关 DNS（`192.168.1.1` `server misbehaving`）间歇隐患仍在，建议另配稳定备用 DNS 降低看门狗误触发概率。
- 若要“无人登录也整栈自启”，需评估 WSL2 + Docker Engine（systemd 常驻）替代 Docker Desktop。
