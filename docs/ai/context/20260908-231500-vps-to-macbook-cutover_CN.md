# 生产从 VPS 迁到日本 MacBook 的切换记录（2026-09-08）

<!-- prune:keep -->

> 一次性执行流水（迁移 + 切换）。**当前生效的拓扑事实已写进 `AGENTS.md` 第二节**，本文件只留完整流水、踩到的坑、验证与回滚步骤。
> 本仓库公开，敏感值一律 `${变量}` 占位（实际值只在 `deploy/ops.env`）；密码、密钥、真实 IP/Tunnel ID 一律不写入本文件。

## 一、背景与决策

- 管理员人在**日本**，要求把生产从 DediOne 洛杉矶 VPS `${OPS_VPS_HOST}` 整体迁到手边的 **MacBook（Apple M1 / 8GB / macOS 15.5，tailscale `${OPS_MAC_HOST}`，登录用户 `${OPS_MAC_USER}`）**,在公网继续跑。
- **数据处理**：整库全量迁移（用户余额、流水、加密凭证、会话都要保留,用户无感）。
- **公网入口**：复用同一 Cloudflare Tunnel `${OPS_TUNNEL_ID}`，DNS 不动（约定「不要重建 Tunnel」）。
- **VPS**：只停容器、数据卷全保留,作冷备回滚。
- 网络前置结论：日本网络**直连所有上游**（OpenAI / Anthropic / Google 原生 + 各中转站实测 401/404/200 全部可达），原生 Claude/Gemini/OpenAI 分组照常可用,**不需要代理**（据此删了 ClashX）。这与「国内需代理」的担忧相反——先确认所在网络再判断。

## 二、目标拓扑

```
aaccx.pw / www.aaccx.pw / api.aaccx.pw
  → Cloudflare Tunnel ${OPS_TUNNEL_ID}
  → MacBook ${OPS_MAC_HOST}
  → OrbStack（arm64 运行时）
  → 应用容器 sub2api（仅 127.0.0.1:8080）
```

- **运行时选 OrbStack**：Docker Desktop 在这台 8GB 机器上后台进程起来了但引擎 API 死活不响应（冷启动卡死），改用 OrbStack（约 9 秒就绪，arm64，更轻）。Docker Desktop 已卸载（连同 `~/Library/Containers/com.docker.docker` 虚拟磁盘）。docker CLI 在 `~/.orbstack/bin/docker`。
- 应用镜像 `ghcr.io/cnyui/sub2api:sha-dc343af…`（amd64）**经 Rosetta 模拟**运行；`postgres:18-alpine` / `redis:8-alpine` 用 **arm64 原生**镜像（性能更好）。

## 三、迁移前准备（全程不停机，VPS 照常服务）

1. **装 OrbStack**：官网 `orbstack.dev/download/stable/latest/arm64` 下 dmg（约 463MB）→ `hdiutil attach` → 拷进 `/Applications` → `open -a OrbStack`。
2. **装 cloudflared**：GitHub release `cloudflared-darwin-arm64.tgz`（日本网络可直下）→ 解包到 `~/.local/bin/cloudflared`（`/usr/local/bin` 无写权限）。
3. **拉镜像**：`postgres:18-alpine`、`redis:8-alpine` 直接 `docker pull --platform linux/arm64`；应用镜像从 ghcr 直接 `pull --platform linux/amd64`（该 package 是公开的，日本可达，**无需从 VPS 传**）。
   - 坑：卸载 Docker Desktop 后 `~/.docker/config.json` 残留 `credsStore: osxkeychain`，helper 不在导致**任何 pull 都报 `docker-credential-osxkeychain not found`**。删掉该键即可（公共镜像本不需要凭证 helper）。
4. **配置与密钥**（VPS→本机中转→Mac，都用 tar/流式管道）：
   - `docker-compose.yml` + `docker-compose.vps.yml` + `.env` → `~/sub2api/`。
   - `.env` 改 `IMAGE_TAG` 对齐**生产实际在跑的镜像**（`docker ps` 看到的 sha,不是 `.env` 里可能超前的值）。
   - **账号凭证 AES 密钥**：VPS `/etc/sub2api/account_credentials_encryption_key` → Mac `~/sub2api/secrets/`（`/etc` 要 sudo,放用户目录）。**sha256 两端校验一致后才算数**——这把密钥错一位,全部 `accounts.credentials` 解不开、所有分组失效（坑 9）。
   - 生成 `docker-compose.mac.yml`：`cp docker-compose.vps.yml docker-compose.mac.yml` 后只改两处 → secret `file:` 指 `./secrets/…`；`SERVER_TRUSTED_PROXIES` 改成 OrbStack 网段（见坑）。
5. **卷数据**（`docker save`/`tar` 流式，用 `docker run --rm -i -v <vol>:/dest postgres:18-alpine tar xf -` 灌入命名卷；卷名 `sub2api_sub2api_data` / `sub2api_redis_data` 与 VPS 一致,因为项目名都是 `sub2api`）：
   - `sub2api_data`（config.yaml 等）、`redis_data` 原样拷。
   - **postgres 用 `pg_dump -Fc` 逻辑迁移,不用裸拷卷**（跨 amd64→arm64 更稳）。
6. **config.yaml 密码对齐**：迁来的 `config.yaml` 里 `database.password` 与 `.env` 的 `POSTGRES_PASSWORD` **不一致**（历史遗留）。实测 env 覆盖 config.yaml（redis 密码/`mode` 同样是 env 生效),但为双保险把 config.yaml 的 DB 密码 sed 成与 `.env` 一致,免得应用读哪边都连得上库。
7. **dry-run**：本地 `docker compose ... up -d` 起全栈 → `/health` ok、model-plaza 出数据、行数与 VPS 基本一致（只差在线写入的几条）、日志无解密错误、上游全通。**这一步 VPS 仍在对外服务,零影响。**

## 四、正式切换（有停机；本次实际约 15 分钟）

顺序很重要,踩了坑（见第五节）后修正为：

1. **先掐断 VPS 的「复活 + 进流量」**：`systemctl stop cloudflared && systemctl disable cloudflared`；`systemctl stop sub2api-watchdog.timer sub2api-watchdog.service && systemctl disable sub2api-watchdog.timer`。
2. `docker stop -t 20 sub2api`（VPS）。连查两次 `usage_logs` 行数确认**已静止**。
3. Mac：停应用 → drop/create 空库（`recreate.sql`：先 `pg_terminate_backend` 再 `DROP/CREATE DATABASE`）。
4. **最终一致 dump→restore**：`ssh VPS 'docker exec sub2api-postgres pg_dump -U … -Fc -Z6' | (Mac) docker exec -i sub2api-postgres pg_restore -U … -d … --no-owner --no-privileges`。1.78GB 约 4–5 分钟。
5. Mac：`redis-cli FLUSHALL`（强制用新库重建调度快照,避免坑 9 的陈旧凭证）→ `docker start sub2api` → 等 healthy。
6. **行数精确核对**：`usage_logs 384141 / users 204 / accounts 16 / groups 17 / schema_migrations 285` **两端完全一致**,确认零丢单。
7. **切隧道**：Mac 装 launchd agent `com.sub2api.cloudflared` 并 `launchctl bootstrap gui/$(id -u) …` → cloudflared 注册 4 条连接到 Cloudflare 东京边缘（nrt）。此前 VPS 连接器已停,无 split-brain。
8. **验证公网**（见第六节）。

## 五、关键坑：看门狗把 VPS「复活」，导致差 4 条流水

- 第一次只 `docker stop sub2api`、没停 `sub2api-watchdog.timer`、也没停 VPS cloudflared → **看门狗每 2 分钟把应用拉起来,cloudflared 还在喂公网流量** → dump 之后 VPS 又写了几条,Mac 比 VPS 少 4 条 `usage_logs`。
- **教训**：停 VPS 应用前,必须**先 `disable` 看门狗 timer + stop/disable cloudflared**,`docker stop` 才真的「停住」。`restart: unless-stopped` 不会自己重启手动 stop 的容器,但**看门狗脚本会**（它做 `docker start`/`compose up`,绕过 restart 策略）。
- 修正后重取一次最终 dump,行数才精确一致。**切换必须以「行数逐表相等」为验收,不能只看 `/health`。**

## 六、验证（全绿）

- `curl https://api.aaccx.pw/health` 与 `https://aaccx.pw/health` → `{"status":"ok"}`（经 Cloudflare 边缘回到 Mac）。
- 应用日志见真实用户流量全 200：`/api/v1/auth/me`（**JWT 会话有效 = 密钥迁移正确,用户没掉登录**）、`/v1/models`、`/api/v1/payment/checkout-info`、`/subscriptions/active`。
- 计费口径：`BILLING_FINAL_MULTIPLIER=18` 由 `docker-compose.mac.yml` 的 `:?` 强校验（缺了直接起不来）；时区保持 `Asia/Shanghai`。
- **`SERVER_TRUSTED_PROXIES` 坑**：初切后日志 `client_ip` 全是 `192.168.97.1`（OrbStack 网关）。原因：override 里还是 VPS 的 `172.18.0.0/16`,与 OrbStack 网段不符 → 应用不信任转发头 → 限流把所有用户当同一 IP、日志丢真实 IP。改成 `192.168.97.0/24`（`docker network inspect sub2api_sub2api-network` 得到）后,`client_ip` 恢复真实公网 IP（含 IPv6）。
- 防睡眠：`pmset -g` 显示 `sleep 0 (prevented by caffeinate)`。

## 七、回滚步骤（VPS 数据已冻结在切换时刻）

⚠️ **回滚越晚,丢的 Mac 新数据越多**——VPS 库停在 2026-09-08 切换那一刻,之后 Mac 上的所有扣费/订单都不在 VPS。

1. 停 Mac 连接器：`launchctl bootout gui/$(id -u)/com.sub2api.cloudflared`（确认 4 条连接断开）。
2. VPS：`systemctl start cloudflared` + `docker start sub2api`（必要时 `systemctl enable --now sub2api-watchdog.timer` 恢复看门狗）。
3. 验证 `https://api.aaccx.pw/health` 回到 VPS。
   - 若要保「Mac 期间的新数据」,回滚前需反向 dump（Mac→VPS）,否则丢弃。

## 八、遗留 / 待办（重要）

1. ✅ **CI 自动部署已改指生产 Mac**（2026-09-08 稍后实现，见 AGENTS.md 第二节 / `build.yml` 的 `deploy-mac` job）：`push main → build → 云端 runner 经 Tailscale 组网 → SSH（部署密钥 `command=…deploy-mac.sh,restrict` 锁死）到 Mac 跑 `deploy-mac.sh`。VPS 的 deploy 已降为**仅手动触发**，push main 不再动 VPS。需 secrets `MAC_HOST/MAC_USER/MAC_SSH_KEY/TS_AUTHKEY`（前三个已设，`TS_AUTHKEY` 待用户在 Tailscale 后台生成；缺它该 job 优雅跳过）。手动/回滚：`~/sub2api/deploy-mac.sh sha-<commit>`。
2. ⚠️ **R2 异地备份大概率失效**：应用内置备份仍按 `0 2 * * *` 跑,但 R2 令牌限 IP `${OPS_VPS_HOST}`,Mac 出口 IP 不同会被拒。需换令牌或在 R2 侧放行 Mac 出口 IP,并验证一次备份成功。
3. ⚠️ **durability 是 GUI 会话级**：cloudflared/caffeinate 是 GUI 域 LaunchAgent,OrbStack 也需登录自启。**重启后无人登录 GUI → 全断**。真·headless 要把它们改成 LaunchDaemon（`/Library/LaunchDaemons`,需 sudo），OrbStack 也要设开机自启。
4. **合盖/断电即断**：`caffeinate` 只挡空闲睡眠。须保持接电源;合盖免睡用 Amphetamine（已保留）或 `sudo pmset -a disablesleep 1`。
5. **外部拨测仍未配置**——笔记本 + 家用网络当服务器,宕机无人知晓的风险比 VPS 时更高。
6. **单点、无冗余**：从「一台 VPS」变成「一台家用笔记本」,可靠性是降级,VPS 是唯一后路。
7. `AGENTS.md` 旧的 VPS 侧手册（防火墙 `docker-user-firewall`、容量结论、DB 运维手册的连接方式）现在**只在回滚到 VPS 时才相关**;当前生效的生产库在 Mac（`~/.orbstack/bin/docker exec sub2api-postgres psql …`）。

## 九、敏感值位置

`${OPS_VPS_HOST}`、`${OPS_MAC_HOST}`、`${OPS_MAC_USER}`、`${OPS_TUNNEL_ID}`、`${OPS_R2_BUCKET}`、`${OPS_DEPLOY_DIR}` 实际值只在 `deploy/ops.env`（gitignore）。Mac 登录密码、JWT/TOTP/DB/凭证密钥**不落任何被跟踪文件**。Mac SSH 目前用密码认证走 tailscale,后续可换 key。
