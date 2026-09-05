# 公网服务切换至云端 VPS、安全加固、异地备份与容量实测

## 概述

本轮把 `aaccx.pw` / `www.aaccx.pw` / `api.aaccx.pw` 三个域名的服务，从跑在 Windows 笔记本上的 `sub2api-official-18082` 完整迁移到 DediOne 洛杉矶 VPS（`${OPS_VPS_HOST}`），随后完成安全加固、云端看门狗、Cloudflare R2 异地备份，并针对即将到来的活动（60 人 / 3 天 / 约 60 请求每分钟）做了容量与瓶颈实测。

两次停机窗口合计约 9 分钟（`21:00:38`–`21:05:52` 预演，`21:09:44`–`21:13:08` 正式切换）。笔记本侧全部容器与 Cloudflared 服务已停止，数据卷原样保留可回滚。

## 一、域名切换

### 策略

**复用同一个 tunnel ID `${OPS_TUNNEL_ID}`**，把笔记本 `C:\Users\yui\.cloudflared\<id>.json` 凭证拷到 VPS `/etc/cloudflared/`。这样 **DNS 一个字都不用改**，停笔记本连接器 + 起 VPS 连接器，流量自动跟过去；回滚即反向操作，无传播延迟。

**代价：两端连接器绝不能同时在线** —— Cloudflare 会在多个连接器间轮询，请求会随机落到两个数据库上。

原公网链路是三层：cloudflared → `sub2api-public-nginx-local`（占 `127.0.0.1:8080`，配置 `sub2api/deploy/nginx-public-local-18080.conf`）→ `host.docker.internal:18082`。云端不需要 nginx 这层，应用本身监听 8080。

### 切换前发现并修复的两个致命问题

1. **`BILLING_FINAL_MULTIPLIER=18` 没有进入云端容器。**
   `deploy/docker-compose.yml` 没有 `env_file:`，`environment` 是逐条列举的，凡是没列进去的 `.env` 变量一律静默失效退回代码默认值。该变量在 `.env` 第 26 行写着 18，容器里根本没有，viper 默认 `1.0` —— **直接切过去会只收应收的 1/18**。已在 `docker-compose.vps.yml` 补上并用 `${VAR:?}` 语法让缺失时直接启动失败。
   排查方法：把 `.env` 的键集合与 `docker exec X env` 的键集合做差集比对。

2. **两端 JWT secret 不一致，会导致所有用户网页会话失效。**
   笔记本生效的密钥来自 `/app/data/config.yaml` 的 `jwt.secret`（容器里 `JWT_SECRET` 为空，base compose 写的是 `${JWT_SECRET:-}`，viper 回落到配置文件），云端 `.env` 里却是另一个值。已把云端 `.env` 对齐到笔记本的值，两端指纹同为 `fd01ac75bff3d377`，用户无感切换。
   TOTP 密钥此前已对齐（`6a73e07eda6f911d`），凭证加密密钥指纹 `e1e4135f9f093074` 一致。

### 数据迁移

**整库替换而非合并**：`DROP DATABASE` + `CREATE DATABASE` + `pg_restore -j 4 --no-owner --no-acl`，规避上一轮 `--clean --if-exists` 的外键报错。1.9GB 库实测：导出 11 秒、传输 19 秒（105.8MB，约 5.5MB/s）、恢复 62 秒。脚本 `/opt/sub2api/restore-cutover.sh`。

**恢复后必须重新施加 DB 层硬化**：`settings` 表随 dump 覆盖，`api_key_acl_trust_forwarded_ip` 会被笔记本的值（非 false）盖回去。脚本已包含该步骤，两次恢复均返回 `UPDATE 1`，证实必要。

### 验证

- 三个域名 `/health` 均 200（205–420ms）
- 数据一致：199 用户 / 733 订单 / 370 API Key / 15 上游账号，最新订单 `2026-09-04 17:31:18`
- **凭证解密验证**：切换后新增 170 次上游监控检查，160 次 `operational`
- 真实 API 调用返回正常内容
- 隧道注册 4 条连接（lax07 / sjc11）

### 计费实测

用真实 API Key 调用 `gpt-5.4-mini` 验证隐藏倍率：

```
输入 858 tok 0.0006435 + 缓存读 7488 tok 0.0005616 + 输出 14 tok 0.0000630
= total_cost 0.0012681（不含任何倍率）
× 分组倍率 0.16 × 最终倍率 18 = ×2.88
= actual_cost 0.0036521280
账户余额扣减 0.00365213 ✓
```

与迁移前笔记本时期的记录对照，不同分组倍率下「剥离分组倍率后」恒等于 18，行为完全一致。

## 二、笔记本侧下线

已 `docker stop`（重启策略均为 `unless-stopped`，Docker 重启后不会自己回来）：`sub2api-official-18082` 全栈、`sub2api-public-nginx-local`、`cliproxyapi-local-dev`、`sub2api-redis-dev`、`sub2api-postgres-dev`、`pkb-backend`、`pkb-postgres`、`supabase_db_SW`。本机已无任何运行中的容器。

**Cloudflared 服务需管理员权限才能停**，普通会话下 `Stop-Service` / `sc.exe stop` / `taskkill` 全部 Access denied。已由用户以管理员执行 `Stop-Service Cloudflared -Force` + `Set-Service -StartupType Manual`（服务保留不删，供回滚），并 `schtasks /delete /tn cloudflared-watchdog /f` 删除了看门狗计划任务。

**云端零依赖笔记本**：15 个上游账号 `proxy_id` 全为 NULL，`extra` 字段中匹配本机/内网地址的记录数为 0。

## 三、安全加固

- `SECURITY_TRUST_FORWARDED_IP_FOR_API_KEY_ACL=false`、`SERVER_TRUSTED_PROXIES=172.18.0.0/16`（已核实 docker 子网确为 `172.18.0.0/16`、网关 `172.18.0.1`）
- 8080 只绑 `127.0.0.1`，UFW 仅放行 22
- 凭证密钥移至 `/etc/sub2api/`，`-r--------` 1000:1000
- Redis `maxmemory 256mb` + `noeviction`
- **`totp_enabled` 此前是 `false`** —— 管理员「没开 2FA」其实是开不了，不是没去开。已置 true（接口实时生效，无需重启）。启用流程需邮箱验证码授权（`/totp/send-code`），依赖 SMTP；云端已配 Gmail（`smtp.gmail.com:587`）。用户已完成 2FA 绑定（用户 448，`2026-09-04 20:51:19`）。
- **`step_up_enabled` 已开启** —— 导出数据、创建/下载备份、改 S3 配置、提升管理员等敏感操作需要 TOTP 二次验证。该开关随系统设置页底部「保存设置」一起提交，单独切换不会持久化。
- **默认管理员 `admin@sub2api.local`（id 436）已硬删除。**
  删除顺序有坑：必须**先删 users**（CASCADE 带走 `user_traffic_credits`），**再删 payment_orders**；反过来会被 `user_traffic_credits.order_id` 的 RESTRICT 外键拦住。`payment_orders` 对 users 没有外键，不删会留孤儿行。该账号 4 笔订单均为零金额批量赠送。快照表 `deleted_user_436_backup` / `_orders_backup` / `_credits_backup` 保留在库中。
  `AUTO_SETUP=true` 不会让它复活 —— `NeedsSetup()` 只要 `/app/data/config.yaml` 存在就返回 false。

**应用没有「强制所有用户 2FA」的功能**，只有 `step_up_enabled`。且强制 2FA 对保护 API 调用毫无作用，因为 API key 调用根本不走登录流程。

## 四、云端看门狗

`/opt/sub2api/watchdog.sh` + `sub2api-watchdog.timer`（每 2 分钟，`OnBootSec=3min`），复刻并强化了笔记本那套已删除的逻辑：

```
公网探测 3 次（间隔 5 秒）
├─ 任意一次 200 → 健康，静默退出
└─ 全失败 → 探本地 127.0.0.1:8080
   ├─ 本地正常   → 隧道层故障 → 重启 cloudflared
   └─ 本地也不通 → 应用层故障
      ├─ 容器自报 healthy → 判为瞬时过载，不动它（避免掐断在途 SSE 流）
      └─ 容器 unhealthy/已退出 → 重建容器
```

一小时滑动窗口内最多 4 次恢复动作，超了只记日志，防无限重启循环。日志 `/var/log/sub2api-watchdog.log`（超 5MB 轮转），状态目录 `/var/lib/sub2api-watchdog`。cloudflared 单元的 `Restart=on-failure` 已改 `Restart=always` + `StartLimitIntervalSec=300` / `StartLimitBurst=10`。

**已做故障注入实测**：`systemctl stop cloudflared` 后触发，**22.5 秒**完成检测 → 重启 → 验证回 200。

**盲区**：看门狗跑在同一台 VPS 上，机器本身宕机时无人恢复，需要外部拨测。**尚未配置。**

## 五、Cloudflare R2 异地备份

应用**自带完整的数据库备份系统**（`service/backup_service.go` 1168 行 + `handler/admin/backup_handler.go`），支持 cron 定时、上传 S3 兼容存储、保留策略清理、一键恢复、生成下载链接，全部从 **系统设置 → 数据备份** 配置，设置键 `backup_s3_config`。前置条件是固定的 `TOTP_ENCRYPTION_KEY`（否则自动生成的密钥每次重启都变，存进去的 S3 secret 重启后解不开），该密钥迁移时已带过来。

注意区分：`image_storage.*` 那组 S3/R2 配置是给**异步图片任务结果**用的，与数据库备份无关。

### 配置

Cloudflare 账号 ID `98545bbd1b328d81cd05a93f61d930da`。**开通 R2 需要账号先绑定付款方式**，即使在免费额度内（10GB 存储 / 100 万次 A 类 / 1000 万次 B 类每月）也要求，该步骤必须用户本人完成。

- 桶 `${OPS_R2_BUCKET}`：位置提示 **WNAM 北美洲西部**（就近洛杉矶 VPS）、标准存储类（不频繁访问类有 30 天最短计费期，14 天保留反而更贵）、公开访问禁用
- Account API 令牌 `sub2api-backup`：对象读写、**仅限该桶**、永久有效、**客户端 IP 限制 `${OPS_VPS_HOST}`**（令牌泄露到别处无效；实测不影响应用，反证应用出口 IP 确为该地址）
- 定时策略 `{"enabled":true,"cron_expr":"0 2 * * *","retain_days":14,"retain_count":20}` —— 容器 TZ 是 Asia/Shanghai 所以是北京时间凌晨 2 点，与 VPS 本机 cron 备份（UTC 4:00 = 北京 12:00）不撞车。`retain_count` 设 20 而非默认 10，因为手动备份也占份额。

### 验证

测试连接 `connection successful` → 手动备份完成 → 在 R2 中确认对象 `backups/2026/09/04/sub2api_20260904_210904.sql.gz`，`application/gzip`，**105.11 MB**（= 100.24 MiB，与后台报告的 100.2 MB 精确吻合，证明上传未截断）。

## 六、容量与瓶颈实测

### 结论：瓶颈是 CPU，带宽完全不沾边

三组压测，系统 CPU 全部顶到 98–100%：

| 端点 | 并发 50 时 RPS | app CPU | postgres CPU | 出站带宽 |
|---|---|---|---|---|
| `/health`（纯应用） | 6,920 | 107% | 2.5% | 微不足道 |
| `/settings/public`（查库） | 552 | 65% | **94%** | 9 Mbps |
| `/` 首页（3.2KB） | 2,556 | 88% | — | **67 Mbps** |

带宽最高只跑到 67 Mbps 时 CPU 已 99%，链路是 200 Mbps。**真实生产流量实测：入站 0.752 Mbps / 出站 0.653 Mbps，即 200 Mbps 的 0.3%。**

近 30 天 109,020 次请求中含图片的仅 38 次（0.035%），95% 是纯文本流式，因此图片不构成带宽因素。

### 不是 IO，是纯计算

压 DB 端点期间的 CPU 时间归属：用户态 63.9% / 内核态 35.1% / **iowait 0.0%** / 空闲 0.9%。shared_buffers 命中率 89.5%，数据基本在内存里，压根没读盘。

开销来自：每次查询重新解析与规划、PostgreSQL 一连接一进程的上下文切换、MVCC 可见性检查、行格式到网络协议的转换、系统调用。

### 真实 API 请求的 CPU 成本（关键数据）

15 次真实流式调用实测：

```
平均墙钟时长      2.9 秒
平均响应字节      19,889 字节
每请求净 CPU      99.6 ms（应用）+ 21 ms（cloudflared）≈ 121 ms
CPU 占用率        3.42%（96.6% 时间在等上游）
```

据此推算容量上限：

- **吞吐上限**：2 核 ÷ 121 ms ≈ **每秒 16.5 请求 ≈ 每分钟 990**
- **并发流上限**：16.5 × 25.8 秒平均时长 ≈ **425 条并发流**
- 内存侧上限更高（100 并发连接仅耗 32MB，约可支撑 3000 条），故 CPU 先约束

吞吐与并发不是两个独立指标，而是同一约束的两种表述，由「每请求时长」连接：并发 = 速率 × 时长。

### 两处结论修正（重要）

1. **曾误报「余量 800 倍」。** 该数字用 `/settings/public`（每请求约 3 ms CPU）外推，而真实转发请求每次 121 ms，**贵 40 倍**。修正后余量约 **16 倍**；若按生产真实平均时长 25.8 秒（测试用例仅 2.9 秒）保守折算到每请求 300 ms，余量约 **6.7 倍**。
2. **曾误报「`settings/public` 3 小时被调用 33,020 次」。** 该数字是压测自身产生的（wrk 打的正是该端点）。真实生产为 **10 分钟 1 次**。因此「给该接口加缓存」这项优化**在当前规模下价值接近于零，已决定不做**。网关热路径的设置读取本来就有 TTL 缓存（`GetClaudeCodeVersionBounds`、`GetGatewayForwardingSettings` 均走 `atomic.Value`）。

### 已实施的优化

`DATABASE_MAX_OPEN_CONNS` 由 50 降为 **25**。每个 postgres 后端约占 5–10MB，50 个连接在 1.9GB 机器上会挤爆内存触发换页（Swap 已用 269MB 即为佐证）；同时减少极端情况下的进程数与上下文切换。实测 100 并发时仅用 7 个连接，25 是充足余量。容器重建耗时 9 秒，期间零在途请求，计费倍率与密钥指纹均未受影响。

### 100 并发验证

100 并发连接持续 30 秒：7,528 RPS、app 内存 56→88MB、DB 连接 7/25、Redis 1.81M/256M、系统可用内存 1,103MB、**错误数 0**、P99 51.83 ms。

### 即将到来的活动评估

60 人 / 3 天 / 约 60 请求每分钟（用户估计），60 人尚未注册。

- 请求速率 60/分钟 ÷ 上限 990/分钟 = **6%**
- 并发流约 26 条 ÷ 上限 425 条 = **6%**
- 上游账号并发总和 1,400（14 个 active × 100），每用户并发上限 20，用户 RPM 限制 0（不限）
- 历史峰值：最忙的一小时 1,123 次请求，推算并发 7.3 条

**硬件与配置均不构成风险。** 但需留意：时长比速率更容易让人误判 —— 若黑客松场景下请求时长从 25.8 秒涨到 90 秒，并发会从 26 涨到 90（3.5 倍），而人数和请求数并未变化。

## 七、遗留问题

1. **单点故障（最高优先级）** —— 3 天活动跑在一台机器上，无故障转移。看门狗跑在同一台机器上，救不了机器本身宕机。建议加一台备机做数据库流复制 + 隧道备用连接器。
2. **无外部拨测** —— 机器宕机时无人知晓，只能等用户投诉。**尚未配置。**
3. **129 / 198 个用户余额 ≤ 0，会被直接拒绝** —— 活动前须确认参与者账号与余额情况。
4. **上游 429** —— 14 个 active 账号中有 8 个历史上被限流过（最近 `2026-09-03` 的 Codex 两条路径），恢复窗口很短（2 分钟 / 5 秒），同平台有多账号可调度切换。3 倍负载下会更频繁。
5. `api_base_url` 仍为空 —— 影响微信 OAuth 回调地址与「导入到 CC Switch」功能，建议填 `https://api.aaccx.pw`。
6. 管理员登录有限流（`auth-login` 20 次/分钟，fail-close）但**无失败锁定**；现已有 2FA 缓解。

### 此前审计结论的修正

- **请求体不是「256MB 全缓冲」**：有 `RequestBodyLimit` 中间件用 `MaxBytesReader` 流式截断，文本类上限 32MB（`gateway.text_max_body_size`），256MB 只用于图片类；代码中无 `io.ReadAll(c.Request.Body)`。残余风险低，且需有效 API key 才能触达。
- **不是「无读写超时」**：`ReadHeaderTimeout` 与 `IdleTimeout` 均已设置，不设 `ReadTimeout` / `WriteTimeout` 是有意为之并有注释说明（流式响应可持续十几分钟）。Slowloris 打的正是请求头阶段，已被 `ReadHeaderTimeout` 覆盖；且流量现全部经由 Cloudflare。

## 八、回滚方法

VPS `systemctl stop cloudflared` → 笔记本 `docker start` 那四个容器 → `Set-Service Cloudflared -StartupType Automatic; Start-Service Cloudflared`。笔记本数据库原样保留在磁盘上未被改动（全程只读导出）。watchdog 需用 `setup-cloudflared-service.ps1` 重新注册。

## 附：本轮踩到的工具类坑

**Git Bash 调用 docker.exe 时容器内路径会被 MSYS 改写**（`/tmp/x` → `C:/Users/.../Temp/x`，报 `could not open output file`），切换脚本第一次即栽于此。加 `MSYS_NO_PATHCONV=1` 前缀，或直接用 PowerShell 执行 docker 命令。
