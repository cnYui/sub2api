# 项目协作约定

> 本文件每个会话完整载入上下文。只写**当前生效的事实、规则和坑**。
>
> 一次性执行流水（发放、刷新、取消、重启、镜像替换、单用户核查）不进本文件，只写
> `docs/ai/context/`。压缩前的 392 行全文与逐条取舍判据归档在
> `docs/ai/context/20260905-112834-agents-md-compression_CN.md`。
>
> 新增条目前先回答一句：**下一个会话读不到这条，会做错什么？** 答不上来就只写 context 文档。

## 一、协作约定

- **本仓库是公开仓库**（`cnYui/sub2api`，`Wei-Shaw/sub2api` 的 fork）。任何 IP、桶名、
  Tunnel ID、账号地址一律写 `${变量名}`，真实值只进 `deploy/ops.env`（已 gitignore）。
  历史上多处文档误称「私有」，据此做的判断都要重查。
- 默认使用中文；代码注释只说明原因，不复述代码。
- 支付订单的金额、退款金额和订单状态**以服务端为准**，前端金额只用于展示。
- 退款必须绑定创建订单时的支付服务商实例，并保留可审计的订单状态变化。
- 设计与实现上下文写入 `docs/ai/context/`，**历史文档只新增不覆盖**。
- **`docs/ai/context/` 不是永久存储：本机 Windows 计划任务 `Prune AI Context Docs` 每天 06:00 自动删除超过 15 天的文档**（跑 `scripts/prune-ai-context.ps1`）。它**只删本地文件，不 commit 也不 push**，删完留在工作区。
  所以**长期生效的结论必须写进本文件**，只写 context 文档等于 15 天后丢失。
  两条护栏：被 AGENTS.md 或 `docs/` 下其它 Markdown **引用到的文档不会被删**（上面那些手册链接因此安全）；正文带 `<!-- prune:keep -->` 的也不会被删。预演用 `-DryRun`，日志在 `logs/prune-ai-context.log`。**这是纯本地任务、不走云端 routine**；换机器或任务丢失时跑 `pwsh -File scripts/install-prune-task.ps1` 重建（幂等），任务快照见 `deploy/prune-ai-context.task.xml`。
- 生产数据变更必须写 `payment_audit_logs` 审计，并处理认证/余额缓存失效。
- **SSH 私钥不叫 `id_*`**，部署密钥按机器命名（文件名见 `deploy/ops.env` 的 `OPS_SSH_KEY_FILE`）。用 `ls ~/.ssh/*.pem ~/.ssh/id_*` 过滤会漏掉它、误判为「无 SSH 访问」——**要 `ls ~/.ssh/` 全量看**。默认 `id_ed25519` 确实被服务器拒绝，容易据此得出确定的错误结论。
- **数据库运维手册**：`docs/ai/context/20260905-173123-vps-ssh-db-operations-runbook_CN.md`（psql 用法、写操作事务模板、表结构坑、缓存失效、核验清单）。**动生产数据库前先读它。** 注意：**当前生效的生产库在 Mac 上**（`~/.orbstack/bin/docker exec sub2api-postgres psql …`），手册的 psql/事务模板通用，只是连接方式换成 Mac；手册里的 VPS 连接方式仅在回滚后才相关。
- **部署/换镜像**：生产已迁到 Mac（见第二节）。换版本在 `~/sub2api` 改 `.env` 的 `IMAGE_TAG`（或 `~/.orbstack/bin/docker compose … pull`）再 `docker compose -f docker-compose.yml -f docker-compose.mac.yml up -d sub2api`。**重启前先 `docker compose config` 渲染检查** image / 端口绑定 / `BILLING_FINAL_MULTIPLIER` / secrets 四项。只 `prune -f` 不要 `prune -a`，否则丢回滚镜像。VPS 侧 GHCR 首切文档 `docs/ai/context/20260905-200812-first-ghcr-image-deploy_CN.md`（用 `docker-compose.vps.yml`）仅回滚时参考。
- 改公网 Nginx 必须先 `nginx -t` 通过再 `reload`；**不要重建 Cloudflare Tunnel**。
- 数据库迁移已应用后内容不可改（有 checksum 保护），只能新增迁移号。当前最大迁移号 `216`。

## 二、当前部署拓扑（2026-09-08：已从 VPS 迁到日本 MacBook）

> **本仓库是公开仓库。** 运维敏感值一律用 `${变量名}` 占位，实际值只写在
> `deploy/ops.env`（已 gitignore，模板见 `deploy/ops.env.example`）。
> 不要把 IP、桶名、Tunnel ID 写回任何被跟踪的文件。

```
aaccx.pw / www.aaccx.pw / api.aaccx.pw
  → Cloudflare Tunnel ${OPS_TUNNEL_ID}（复用同一隧道，DNS 未改）
  → 日本 MacBook ${OPS_MAC_HOST}（tailscale 内网；登录用户 ${OPS_MAC_USER}）
  → OrbStack（arm64 容器运行时）
  → 应用容器 sub2api（仅 127.0.0.1:8080）
```

- **运行时是 OrbStack，不是 Docker Desktop**（后者在这台 8GB 机器上引擎起不来，已卸载）。docker CLI 在 `~/.orbstack/bin/docker`（PATH 里可能没有，用全路径）。应用镜像 `ghcr.io/cnyui/sub2api`（amd64）**经 Rosetta 模拟**跑；`postgres:18` / `redis:8` 用 arm64 原生镜像。
- **部署目录 `~/sub2api/`**，命令 `docker compose -f docker-compose.yml -f docker-compose.mac.yml …`（**不是** vps.yml）。`docker-compose.mac.yml` 由 `docker-compose.vps.yml` 派生，只改两处：① secret 文件指 `./secrets/account_credentials_encryption_key`（Mac 上 `/etc` 要 sudo，故放用户目录，与 VPS `/etc/sub2api/…` sha256 一致）；② `SERVER_TRUSTED_PROXIES=192.168.97.0/24` 对齐 OrbStack 网段——**用 VPS 的 `172.18.0.0/16` 会让日志/限流把所有用户当同一网关 IP**。
- 公网入口：cloudflared 以 **launchd agent `com.sub2api.cloudflared`** 常驻（`~/.local/bin/cloudflared` + `~/.cloudflared/config.yml`，ingress→`127.0.0.1:8080`）。**同一 Tunnel 同一时刻只能一端连**（切换时已停 VPS 连接器）。
- postgres / redis 只在 OrbStack 内网、不发布端口；应用 `BIND_HOST=127.0.0.1`，公网只经 cloudflared 到 8080。`DATABASE_MAX_OPEN_CONNS=25`。已开 `totp_enabled`/`step_up_enabled`，默认管理员 `admin@sub2api.local` 已硬删除。`BILLING_FINAL_MULTIPLIER` 由 mac override 的 `:?` 强校验，缺了起不来；值写在 `~/sub2api/.env`，当前值见第三节。
- **防睡眠**：launchd agent `com.sub2api.caffeinate`（`caffeinate -dimsu`）。⚠️ 只挡空闲睡眠；**合盖/断电仍会睡＝服务断**——须保持接电源，合盖免睡用 Amphetamine（已保留）或 `sudo pmset -a disablesleep 1`。
- ⚠️ **durability 是 GUI 会话级**：cloudflared/caffeinate 都是 GUI 域 LaunchAgent，OrbStack 也需登录自启。**重启后必须有人登录 GUI 才会自起，无人值守重启会全断**。真·headless 需改 LaunchDaemon（`/Library/LaunchDaemons`，要 sudo）。
- **CI 自动部署到生产 Mac**（`build.yml` 的 `deploy-mac` job）：`push main → build（GHCR sha 镜像）→ 云端 runner 经 Tailscale 组网 → SSH 到 Mac 跑 `~/sub2api/deploy-mac.sh`（用 `docker-compose.mac.yml`，含健康检查 + 倍率自检）。需 secrets `MAC_HOST/MAC_USER/MAC_SSH_KEY/TS_AUTHKEY`；`TS_AUTHKEY` 是 Tailscale 后台生成的 auth key（reusable+ephemeral），**缺它该 job 优雅跳过**。部署密钥在 Mac `authorized_keys` 用 `command="…deploy-mac.sh",restrict` 锁死（只能部署合法 sha、无自由 shell）。**VPS 的 deploy 已降为仅手动触发**（`workflow_dispatch + deploy`），push main 不再动 VPS。手动部署/回滚 Mac：`~/sub2api/deploy-mac.sh sha-<commit>`。
- ⚠️ **R2 异地备份大概率已失效**：应用内置备份仍按 `0 2 * * *` 跑，但 R2 令牌限 IP `${OPS_VPS_HOST}`，从 Mac 上传会被拒。待换令牌或放行 Mac 出口 IP 并验证一次。

**VPS `${OPS_VPS_HOST}` 现为冷备回滚**：应用容器 stopped、`cloudflared` stopped+disabled、`sub2api-watchdog.timer` disabled。数据卷原样保留但**从 2026-09-08 切换时刻起冻结**——回滚越晚，丢的 Mac 新数据越多。**回滚**：先停 Mac 连接器 `launchctl bootout gui/$(id -u)/com.sub2api.cloudflared`，再 VPS `systemctl start cloudflared` + `docker start sub2api`（必要时恢复看门狗）。VPS 侧防火墙（`docker-user-firewall`）、容量结论（CPU 瓶颈、约 990 req/min）只在回滚时才相关。

完整切换流水、踩坑与回滚见 `docs/ai/context/20260908-231500-vps-to-macbook-cutover_CN.md`。VPS 时期文档：`20260905-100322-vps-cutover-hardening-and-capacity-audit_CN.md`、`20260905-110342-docker-user-firewall-hardening_CN.md`。

## 三、计费口径

**唯一公式**：`actual_cost = total_cost(标准成本) × 分组倍率 × BILLING_FINAL_MULTIPLIER`

- 当前 `BILLING_FINAL_MULTIPLIER=17`，**隐藏倍率**，以运行态容器环境变量为准。**2026-09-17 14:42:20（+08）由 18 改为 17**（管理员指示）。
  - 值写在生产 Mac `~/sub2api/.env`，由 `docker-compose.mac.yml` 注入。`deploy-mac.sh` 只打印这个值、不校验，所以 CI 部署会沿用。
  - 改动只走「改 `.env` + 重建应用容器」。改前先用 `docker compose … config --hash sub2api` 与运行中容器的 `com.docker.compose.config-hash` 标签比对，相等才说明重建只会带进这次改动。只重建应用：`up -d --no-deps sub2api`。
  - **`usage_logs` 不记录最终倍率**：核对历史扣费要按 `created_at` 分段，14:42:20 之前乘 18，之后乘 17（坑 25）。
  - 记录与回滚见 `docs/ai/context/20260917-155102-final-multiplier-18-to-17_CN.md`。
- 模型广场**刻意不展示、不叠加**最终倍率，只显示 `基础单价 × 生效倍率`。
- 账号统计倍率（`accounts` 上的）**不参与用户扣费**，只做渠道统计。
- 分组倍率以数据库 `groups.rate_multiplier` 为准。截至 2026-09-17（名称为当天统一后的写法）：

  | 分组名称 | id | 倍率 | 备注 |
  | --- | --- | --- | --- |
  | Claude0.2倍率（日常二） | 73 | 0.2 | 账号 #1167（huoshenai.net）；**`status=inactive`，已不在广场**（不是 2026-09-22 那轮下架改的，之前就停了，改停时间未考） |
  | Claude0.5倍率（日常一） | 5 | 0.5 | 原名 Claude Kiro（0.45），后改名为 `Claude0.5倍率(日常1)`，09-05 由 0.4 调到 0.5。**没有绑定账号**，见第七节 |
  | Claude0.78倍率 | 71 | 0.78 | 原生；账号 #1166（火神）。**2026-09-23 白名单收紧到 6 个**（去掉 11 个 404 的旧 Claude），但这 6 个当天三轮实测也全挂（502 后秒回 `No available accounts`，火神号池问题，近两周偶尔能用）。**2026-09-23 按管理员指示停用（`status=inactive`）**，停用时 9 个有效 key / 7 人，现会 403；白名单与账号绑定保留，恢复前先用「测试连接」确认能调通，备注原文见第七节引用的 context 文档。**2026-09-24 起 #1166 同时绑定 84、已改名 `Claude0.75倍率`、白名单改为 84 那 8 个**——恢复 71 时它会与 84 共用这个账号（同一把 key），别再给 71 另建账号。**2026-09-24 管理员决定：71 保持停用、不恢复、不改备注**（它的 9 个 Key / 7 人自 09-23 08:44 起无请求；用户只在自己的 Key 列表里还看得到这个分组名和备注） |
  | Claude0.75倍率 | 84 | 0.75 | 2026-09-24 新增，账号 #1166（**就是 71 那把 key**：管理员发来的 key 的上游逐日用量与 #1166 完全吻合，所以直接复用账号、没有重新填 key），无渠道。白名单 = 当天逐个实测可调通的 8 个：`claude-fable-5-1`、`claude-fable-5`、`claude-opus-5`、`claude-sonnet-5`、`claude-opus-4-8`、`claude-opus-4-7`、`claude-opus-4-6`、`claude-sonnet-4-6`（上游 `/v1/models` 恰好也是这 8 个；`claude-haiku-4-5` 上游 404，未开）。上游分组倍率 0.75，我方 1:1 镜像，收入/成本约 1.63～1.79 倍（充值 1:1～1:1.1），保本分组倍率约 0.42～0.46。价格走远端目录（取价第 ② 级），**`claude-fable-5-1` 缓存读是 $0.25、不是 fable-5 的 $1**；两个 fable 已补进内嵌兜底目录（PR #58）。上游不挑 UA，#1166 无需 `user_agent`。监控 27 自动生成。上线记录 `docs/ai/context/20260924-110500-add-claude075-group84_CN.md` |
  | Claude1.5倍率（优质） | 4 | 1.5 | 原 Claude Max；账号 #2。2026-09-23 实测 7 个全通，去掉了 404 的 `claude-opus-4-5-20251101` |
  | GPT0.15倍率（日常一） | 9 | 0.15 | 账号 #1128；**2026-09-22 已下架（`status=inactive`）**，上游 `api.ai-genesis.app` 对这个 key 持续返回 CF 502，全模型不可用。停用不是删除，`account_groups` 绑定保留，恢复只需改回 `active` |
  | GPT0.16倍率（日常二） | 13 | 0.16 | 唯一账号 #1132 停调度中；**2026-09-22 已下架（`status=inactive`）**，实测上游 429 限流，09-14 起零成功。见第七节 |
  | GPT0.16倍率（日常三） | 69 | 0.16 | 账号 #1164；**2026-09-22 已下架（`status=inactive`）**，管理员要求把 GPT 收敛到只剩 0.28 与 0.35。账号本身是好的（白名单与上游完全匹配、实测可调通），恢复只需改回 `active` |
  | GPT0.28倍率 | 74 | 0.28 | 账号 #1168（huoshenai.net，与 13/69 同主机、不同 key）。**2026-09-22 白名单收紧到 2 个：`gpt-5.6-sol`、`gpt-6-astra`**——这个 key 的上游总共只有 3 个模型（另一个是 `codex-auto-review`，缺定价未上）。Responses 路由 auto，实际走 `/v1/responses` |
  | GPT0.35倍率（优质） | 10 | 0.35 | 账号 #1129（api.ai-genesis.app）。**2026-09-23 白名单再收紧到 4 个：`gpt-5.5`、`gpt-5.6-sol`、`gpt-5.6-terra`、`gpt-6-astra`**（`gpt-5.6` 上游目录里有，但三次实测 overloaded / 502 / 429，09-17 后零成功，已剔除）。⚠️ **账号已锁 `extra.openai_responses_mode=force_chat_completions`**，因为它自 09-05 起就走 Chat Completions，别改回 auto（原因见坑 20） |
  | GPT1倍率 | 80 | 1.0 | 2026-09-22 新增，账号 1174、渠道 9。**只有 `gpt-6-astra` 一个模型**——上游那把 key 的 `/v1/models` 就只返回它，白名单与之完全一致。上游是 `api2.ai-genesis.app` 分组 57「Codex（Astra）」，**上游倍率 1.0（满价组，不是 0.15/0.35 那种折扣组）**，所以我方也取 1.0 才能拿到和其它 GPT 分组一样的 1.63 倍毛利，保本线是 0.615。渠道 9 登记了区间定价 $10/$1/$12.5/$50、>272K 档 $20/$2/$25/$75，与上游逐项一致；`restrict_models=true`。Responses 流式实测正常，**不要**照坑 31 给它加 `force_chat_completions`。区间边界写 `272000/272000` 是因为 `FindMatchingInterval` 的语义是 `(Min, Max]`（`channel.go:157`），恰好等于「严格大于 272000 才涨价」——**渠道 4 的 Grok 写 `199999` 是另一种口径（≥200000 涨价），别照抄**。上线记录见 `docs/ai/context/20260922-204700-add-gpt1-group80-channel9-astra_CN.md` |
  | GPT生图1倍率 | 12 | 1.0 | 原名 `Image-2生图`；生图渠道单独隔离，见坑 6 |
  | Grok0.15倍率 | 75 | 0.15 | 账号 #1169（huoshenai.net）；**2026-09-22 加开 `grok-4.7`**，与 4.6/4.5 同在白名单、三个实测均可调通。渠道 4 按 xAI 官方价登记，含 200K 长上下文档（4.7/4.6 缓存读 `$0.50`、4.5 `$0.30`），**长上下文靠渠道区间定价生效，与账号那个 `openai_long_context_billing_enabled=false` 无关，别去"修"它**。远端价格目录至今没有这三个键，**官方兜底价已于 2026-09-22 补登记在 `billing_service.go`（PR #49）**：4.7/4.6 `$2/$0.50/$6`、4.5 `$2/$0.30/$6`，≥200K 一律 2x 输入/2x 缓存/**2x 输出**（不是 GPT 的 1.5x），阈值写 `199999` 是因为判定为严格大于。Grok 4 系列同时进了 `usesCalibratedFallbackPricing`——**广场「官方」列只有走这一分支才取得到值，只加 `fallbackPrices` 等于没改** |
  | Gemini1倍率 | 70 | 1.0 | 原生；**`status=inactive`，已不在广场**（之前就停了，非 09-22 那轮） |
  | DeepSeek0.4倍率（日常一） | 77 | 2.8 | 2026-09-23 由 `DeepSeek0.4倍率` 改名。**2026-09-23 已停用（`status=inactive`），火神国模线路断了，见第七节。**2026-09-17 新增，账号 1170、渠道 6。只开 `deepseek-v4-flash`、`deepseek-v4.1-flash`，登记价 `$0.30/$1.20/读$0.006`，与火神标准价、DeepSeek 官方高峰价相同 |
  | DeepSeek1倍率 | 8 | 7.0 | 原名 `【国产】DeepSeek`；2026-09-16 由 `4.9` 改为原价。**`status=inactive`，已不在广场**（之前就停了，非 09-22 那轮） |
  | GLM0.4倍率（日常一） | 78 | 2.8 | 2026-09-23 由 `GLM0.4倍率（无GLM5.3）` 改名。**2026-09-23 已停用（`status=inactive`），火神国模线路断了，见第七节。**2026-09-17 新增，账号 1171、渠道 7。只开 `glm-5.2`（火神没有 glm-5.3），登记价 `$1.40/$4.40/读$0.26` |
  | GLM0.6倍率 | 76 | 4.2 | 原名 `GLM模型`，2026-09-16 新增，**2026-09-17 接上账号 1173 后可用**。渠道 5 登记 z.ai 官方 USD 价：glm-5.3 `$1.40/$4.40/读$0.26`、glm-5.3-flash `$0.15/$0.50/读$0.03`，缓存写 0（官方存储限时免费），`restrict_models=true`。上游 `api2.ai-genesis.app` 分组 52（倍率同为 4.2），账号必须强制走 Chat Completions，见坑 31。利润薄：按 ¥29 套餐额度用满、ai-genesis 按 1:1 充值算，收入约为成本的 1.62 倍 |
  | Kimi0.4倍率（日常一） | 79 | 2.8 | 2026-09-23 由 `Kimi0.4倍率` 改名。**2026-09-23 已停用（`status=inactive`），火神国模线路断了，见第七节。**2026-09-17 新增，账号 1172、渠道 8。只开 `kimi-k2.6`、`kimi-k2.7-code`、`kimi-k3`，按 Kimi 官方价登记：k2.6 `$0.95/$4/读$0.16`，k2.7-code `$0.95/$4/读$0.19`，k3 `$3/$15/读$0.30`。上线记录见 `docs/ai/context/20260917-121814-huoshenai-guomo-groups-rollout_CN.md` |
  | Kimi1倍率 | 7 | 7.0 | 账号 #5（ai-genesis 上游分组 51「月之暗面 Kimi」，上游倍率 4.9）。**2026-09-23 管理员指示由 `Kimi0.7倍率` / 4.9 改为 `Kimi1倍率` / 7.0**，收入/成本从约 1.62 倍升到约 2.32 倍（7×17÷10.48 = ¥11.36 对 4.9×¥1）。77～79 停用后**这是唯一能用 kimi-k3 的在售分组**，备注里已单独写明；账号白名单 2026-09-23 已收紧为只有 `kimi-k3`（k2.5、k2.6 上游 404）。记录见 `docs/ai/context/20260923-093500-group7-kimi1-rate-7_CN.md` |
  | DeepSeek0.4倍率（日常二） / GLM0.4倍率（日常二） / Kimi0.4倍率（日常二） | 81 / 82 / 83 | 2.8 | 2026-09-23 新增，账号 1175/1176/1177、渠道 10/11/12，各只开一个模型：`deepseek-v4.1-flash` / `glm-5.3-flash` / `kimi-k2.6`，登记价照抄渠道 6/5/8。上游是 `api2.ai-genesis.app` 分组 58「国产模型特惠」（4x）：**三把 key 在同一上游分组、同一钱包，上游都能调全部 5 个模型，隔离全靠我方白名单**；该上游分组的 glm-5.3 恒返回 400，未开放。**利润薄**：kimi-k2.6、glm-5.3-flash 我方登记价 = 上游标准价，收入约为成本 1.14 倍（2.8×17÷10.48 = ¥4.54 对 4×¥1），保本分组倍率约 2.47；deepseek 上游 08:52 实测 `$0.15/$0.60`、09:04 起变成 `$0.30/$1.20`（疑似分时段价），按后者同样约 1.14 倍；上游缓存读价与我方一致（ds 0.006、kimi 0.16）。上线记录见 `docs/ai/context/20260923-093000-add-daily2-domestic-groups-81-83_CN.md` |

  **分组名称不能作为倍率依据**，只查数据库。**这张表也不能当作「在售清单」**——截至 2026-09-23 13:00，5 和 73 已于当天 09:06 软删，8、9、13、69、70、71、77、78、79 是 `status=inactive`（广场看不到、现有 Key 403），真正 active 的只有 4、7、10、12、74、75、76、80、81、82、83 共 11 个（2026-09-24 新增 84 后为 12 个）。判断在售要查 `SELECT id,name,status FROM groups WHERE deleted_at IS NULL`，别照着这张表答。
- **分组命名与备注规范**（2026-09-17 管理员确定，18 个分组已按此改完，记录见 `docs/ai/context/20260917-125401-group-names-descriptions-unify_CN.md`）：
  - **名称** = 系列 + 倍率 + `倍率`，不加空格，如 `DeepSeek0.4倍率`。
    - 国外分组直接写分组倍率。
    - 国产分组写「分组倍率 ÷ 7」：2.8→0.4、4.2→0.6、4.9→0.7、7.0→1。
    - 同系列同倍率必须加后缀区分。后缀用全角括号、中文数字，如 `（日常一）`、`（优质）`。
  - **备注只写实际能调通的模型**：取白名单、`sync-upstream` 清单和近期成功请求三者的交集，不能照抄白名单。暂不可用的分组要写明，并指引用户改选其他分组。
  - **备注格式**：
    - 用 `\n` 换行，最多 3 行，每行约 55 个半角字符以内；
    - 用户下拉框（`GroupOptionItem`）按最长一行撑宽，且只显示 3 行；
    - 下拉框的搜索会匹配备注文字，所以要写完整的模型 ID。
  - **备注不写「官方价 X 折 / 原价」**，因为国产分组的这个折扣和用户实付对不上：
    - 在售套餐每 1 元约得 $10.45 余额，扣费时再乘 17，所以广场上每 $1 实际要花约 1.63 元（隐藏倍率为 18 时约 1.72 元）；
    - 国产官网人民币价约为美元价的 6.8 倍；
    - 两者相乘，国产分组实付 ≈ 官网价 × 分组倍率 × 0.24，比如 2.8 约为 6.7 折，7.0 约为官网价的 1.67 倍；
    - 国外分组实付远低于官网价，写折扣不算夸大，但为了统一，也不写。
- **Grok（分组 3 / 账号 1）、GLM（分组 6 / 账号 4）、GPT 0.1（分组 11 / 账号 1130）已于 2026-09-05 硬删除（软删 `deleted_at`）**，模型广场与 `/groups/available` 均已无。删除是软删，`usage_logs` 历史完整保留，回滚需直连数据库置回 `deleted_at=NULL` 并重插 `account_groups`（硬删的关联）。详见 `docs/ai/context/20260905-224500-delete-grok-glm-gpt01-groups-channels_CN.md`。**同日已把 `billing_service.go` 里 glm-5.x / grok 聊天模型的 `fallbackPrices` 与 match 分支一并清理**（grok-imagine 媒体价、Grok 平台代码、kimi/deepseek/gpt 共享价均保留）。**该代码改动仅在工作区，需出镜像部署才生效；生产当前无这些组/账号，删不删都是死代码。若日后重新上架 glm/grok，必须先补回定价再开放（坑 12：缺价按零成本放行）。** **Grok 已于 2026-09-22 按这条补回**：`grok-4.5/4.6/4.7` 的 `fallbackPrices` 与 match 分支已重新登记（PR #49），见第三节分组 75 那行；**GLM 仍是空的**，分组 76/78 靠渠道级定价撑着，别以为 grok 补了 glm 也补了。

- 汇率展示：国外模型按 `1 USD = 1 CNY`，国产模型按 `1 USD = 7 CNY`，模型广场标题下有固定说明。`BALANCE_RECHARGE_MULTIPLIER` 是"每支付 1 CNY 获得多少 USD"，**不要把汇率写进模型扣费倍率**。
- **国产分组「几折」的基准是 `7.0` 不是 `1.0`**：国产模型入库的基础价就是各家官方 **USD** 价（见 `billing_service.go` 国产兜底价注释），分组倍率里的 `×7` 是上面那条汇率换算、**不是加价**。所以 `3.5 = 5折`、`4.9 = 7折`、**`7.0 = 原价/不打折`**。广场「实付」列因此恒为「官方」列的 7 倍，用户端靠 `exchangeRateHint`（"请除以 7"）解释。**别把国产分组的 `7.0` 误读成 7 倍加价去下调**——那是把价格砍掉 85.7%。国外分组（Gemini 70 = `1.0`）才是以 `1.0` 为原价。注意：这里的「几折」只是命名和广场上的算法，**不是用户实付相对官网的折扣**，原因见上面的「分组命名与备注规范」。
- **登记一个新 OpenAI 模型的价格，必须同时改两处**（见坑 10、11）：`pricing_service.go` 的 `matchOpenAIModel`（加显式前缀分支 + 静态价，**这是生产实际走的路径**）和 `billing_service.go` 的 `fallbackPrices`（目录失效时的兜底）。只改后者等于没改。
- `gpt-6-astra` 两处均已登记：$10/$1/$12.50/$50 每百万 token，>272K 输入转 2x 输入与缓存、1.5x 输出（复用 `openAIGPT54LongContext*` 常量）。别名 `gpt-6`、`gpt6`、带 effort/日期后缀的写法在我方都解析到同一份价格，回归测试见 `billing_service_gpt6_test.go`。**但上游只认精确串 `gpt-6-astra`，别名一律 404**，所以坑 11 的别名少收在这条链路触发不了。（2026-09-22 起 #1129 裸 `gpt-6` 也能调通，见坑 16；我方别名都按 astra 价计，仍不会少收。）
- `gpt-6-astra` 已开放并实测计费正确，在售分组里 74、10、80 有它（13、69 的白名单也有，但两组已于 2026-09-22 下架）。**`#1128`（0.15）与 `#1130`（0.1，已下架）未加，也不要照搬着加**——同一 `base_url` 不同 key 的模型清单不同，见坑 24。2026-09-17 复查时，`#1128` 的 `sync-upstream` 清单里已经有 `gpt-6-astra`，但要不要加进分组 9，由管理员决定，加之前先用 `/test` 验证能调通。

## 四、业务规则

### 负余额与流量卡

- 余额 `>= 0`：不切流量卡。
- 余额 `< 0`：下一次请求**不再扣普通余额**，必须使用用户级全渠道流量卡。
- 流量卡净额度必须**严格 `> 0`** 才允许本次调用；用尽或形成流量卡欠费后，下一次请求拒绝。
- 历史的 `BILLING_MINIMUM_BALANCE_RESERVE=0.01` 保底阈值**已删除**，不要再引用。
- 负余额准入以 PostgreSQL `users.balance` 为最终事实，订阅 / 流量卡 / SimpleMode / Gemini / Live / WebSocket 均不能绕过。

### 余额套餐

- 标准周期 28 天，每 7 天到账一次，共 4 期。每用户**最多一个有效套餐**；购买其他档位需先退款，服务端在用户行锁内二次校验。
- `remaining_usd` 只表示**本周**未用额度，刷新是按窗口替换而非累加。
- **续费 = 周期重置**：立即发放新周期第 1 期、`credited_count` 回到 `1`、`next_credit_at` 重新计时、`status` 恢复 `active`、有效期在原到期基础上 `+28` 天、旧周期未发放期数顺延进新周期总期数、`renewal_count + 1`。
- 首期与每次周刷新**先抵扣负余额**，不足则继续为负，只有偿还后的剩余部分进入 `remaining_usd`。

### 欠费连续结算

- 余额套餐**不因欠费暂停**，下一周额度优先抵消余额欠费。历史 `debt_paused` 数据只做兼容展示。
- 普通余额已欠费后，所有渠道统一扣**用户级全渠道流量卡**，不足部分记入**流量卡欠费账本 `traffic_credit_debt_ledger`**（`entry_type='debt'`、`source_type='usage_billing'`，`repository.recordTrafficCreditDebt`），**不再扣 `users.balance`**。`balance_debt_ledger` 只记套餐到账抵消负余额时的 `repayment`，负余额本身就体现在 `users.balance` 上（2026-09-17 查代码更正，此处原先写错了账本名）。
- 流量卡充值优先抵消流量卡欠费；**余额套餐到账（首期、定时周刷新、提前刷新）只抵负余额，不抵流量卡欠费**。余额和流量卡均无净可用额度时拒绝请求。核对「余额变化 ≠ 用量合计」时先查 `traffic_credit_debt_ledger`，别误报少收（2026-09-17 user 525 的 $0.84 差额就记在这里）。

### 退款

- **准入**：仅**真实支付的余额套餐订单**可退，需同时具备实付金额、支付完成时间、支付平台交易号。管理员发放、兑换码、零金额、流量卡、旧普通余额订单一律不可退，前端四处入口同步隐藏。
- **公式**：`Max(已用时间比例, 周期用量比例)`，实现在 `backend/internal/service/payment_balance_package_refund.go`。
- 用量账本按 `created_at >= starts_at` 限定**当前周期**，且只汇总**余额套餐实际承担**的扣款——流量卡、普通余额和流量卡欠费不参与退款计算。
- 历史套餐因缺少资金来源归因，由迁移 `211` 标记为 `legacy_unattributed` 转人工审核，**不伪造历史归因**。
- **退款成功会连本周未用额度一起收回**（`balance -= remaining_usd`，与到期回收同口径，2026-09-18 上线，见坑 32）。管理员取消套餐时留给用户的额度，在之后的退款报价里算作已用。
- ZPay 易支付订单优先用 `out_trade_no` 发起退款；成功统一写 `REFUNDED`，只撤销对应套餐后续到账，不跨订单扣减其它订单带来的余额；网关失败进 `REFUND_FAILED`。**用户和管理员重试都会按当时的时间和用量重新报价。**
- `payment_audit_logs(order_id, action)` 有唯一索引，**重复失败会导致 `REFUND_FAILED` 审计写入冲突**。

### 其他

- 邀请返利默认开启、比例 8%，直接增加 `users.balance`，`frozen_until` 记 24 小时冻结；**冻结不限制模型使用**。
- **兑换码只给兑换人本人加普通余额，不触发邀请返利**（含后台 `create-and-redeem`），2026-09-11 管理员明确。上游 `Wei-Shaw/sub2api` 的 `RedeemService` 带这段返利，同步上游时别带回来。后台手动加余额是否返利由设置 `affiliate_admin_recharge_enabled` 单独控制。
- **兑换码类型 `balance_package`（2026-09-22 新增，迁移 `215`）**：码上绑 `redeem_codes.balance_package_plan_id`，兑换时按该档位发放余额套餐，走的就是购买页那套 `creditInitialBalance`（首期立即到账、28 天 4 期、先抵负余额）。
  - 生成时校验档位存在且配置合法；**兑换时不要求档位仍在售**——码发出去就是承诺，下架档位不该让码作废。`value` 恒为 0，别把它当美元额度。
  - 会建一笔零金额订单，`payment_type=redeem_code`。它不在 `validateRealPaidBalancePackageOrder` 的支付方式白名单里，所以**天然不可退款**，审计写 `REDEEM_BALANCE_PACKAGE_GRANTED`。
  - **到账邮件复用购买页那封**「余额套餐已生效」（事件 `balance_package_credited`），类型标成「兑换码兑换」、实付留空（码可能是用户在别处买的，写「赠送」是替对方下结论）。普通余额兑换码仍走 `NotifyRedeemBalance` 那封。
  - **用户已有有效套餐时一律拒绝**（`BALANCE_PACKAGE_ACTIVE`），不走同档续费：续费会把套餐改绑到这笔零金额订单，用户原来那笔真实支付的订单就再也退不了款了。拒绝时整个事务回滚，**兑换码保持未使用**，用户可在本期套餐失效后重试。同一条闸门也管着后台手动发放。
- 充值手续费 `RECHARGE_FEE_RATE=1%`，只增加订单 `pay_amount`，**不改变套餐到账额度或流量卡额度**；服务端始终用商品服务端价格重算。
- `/monitor` 全部渠道统一为**每次一个带鉴权的 `GET /v1/models`** 目录探测，间隔 1800 秒，不发真实推理请求，不做额外 HEAD。生图渠道禁止周期性生图探测。
  **监控按分组自动同步**（迁移 216，`channel_monitor_group_sync.go`）：每个启用分组 × 每个可用 API Key 账号一条监控，名称 = 分组名，模型 = 账号白名单（渠道开了 `restrict_models` 时再与渠道定价取交集），直接用账号的上游地址、key 和 `user_agent` 探测；调度器每 10 分钟同步一次，后台「从分组同步」按钮可立即同步。分组停用 / 账号解绑后对应监控自动停用。**同步监控的名称、地址、key、模型、启用状态都会被下次同步覆盖**，要改模型就改账号白名单，只有主模型（仍在白名单内时）、间隔、抖动的手改会保留。`source_group_id` 为空的是手工监控，同步不碰。
- 购买页余额套餐与流量卡**必须复用** `frontend/src/components/payment/PurchaseProductCard.vue`，禁止新增平行卡片样式。
- 商品当前只有余额套餐和流量卡；普通余额 / 旧订阅后端不再兼容，历史字段仅保留只读查询。

### 购买/到账成功邮件通知（2026-09-22 实现）

- **注册和忘记密码不是 SMTP 的全部**：`notification_email_service.go` 是一套通用通知邮件框架，
  后台「设置 → 邮件模板」可改中英文案并预览（实际只发中文，见下一节），事件 `Optional: true` 的还带退订链接和退订记录。
  加新通知应该**往这个框架里加事件**，不要另起一套发信逻辑。
- 三个到账事件：`payment.balance_package_credited`（余额套餐首期，新购/续费/管理员发放共用）、
  `payment.traffic_pack_credited`（流量卡）、`redeem.balance_credited`（兑换码加普通余额）。
  实现在 `purchase_notify_service.go`（`PurchaseNotifyService`，`PaymentService` 与 `RedeemService` 各持一份）。
- **挂载点是 `markCompleted`（`payment_fulfillment.go`）**，两种订单在这里汇合，
  且 lease + `recharging → completed` 乐观锁保证只赢一次，所以天然防重复发信；
  `updated == 0 && 已 completed` 的早返回分支不发。管理员发放不走这里，单独挂在
  `GrantBalancePackage` 提交之后；兑换码挂在 `Redeem` 提交之后。
- 全局开关 `purchase_notify_enabled`，**缺省即开启**（`!isFalseSettingValue`）。
  每周额度到账**刻意不发**（一个周期会发 3 封，管理员判定为骚扰）。
- **负数兑换码不发信**：后台手工补扣写的是负数 `admin_balance` 兑换码（坑 30），
  `amountUSD <= 0` 直接返回，否则用户会收到「成功兑换 $-33.5」。
- 续费与新购共用 `creditInitialBalance`，调用方看不出区别：靠套餐行 `renewal_count > 0` 判续费、
  `PaymentType == admin_grant` 判发放。续费会把 `payment_order_id` 改绑新订单，
  所以按订单 id 反查套餐行对两种情况都成立。
- 实现记录见 `docs/ai/context/20260922-124500-purchase-credited-email-notifications_CN.md`。

### 通知邮件外观与语言（2026-09-23）

- **全部用中文发信**（站长要求）：`ResolveRecipientLocale` 恒返回中文，`Send` 不再看 `input.Locale`
  和记住的浏览器语言。英文模板还能在后台预览、编辑，但不会发出去。业务方生成变量文字（「新购」「赠送」）
  必须用 `ResolveRecipientLocale` 取语言，自己判断会在中文模板里夹英文。
- 外观是站长的兑换卡白色版：`notification_email_layout.go` 管渲染，`notification_email_templates.go` 放 15 个事件的
  中英文官方模板。**改外观改 layout，改文案改 templates**，别在单个模板里手写样式。邮件客户端的兼容约束
  （表格布局、样式内联、有底色的单元格要写 `bgcolor`、禁用 flex/grid/position/rgba、官方模板 ≤ 24000 字节）
  由 `TestOfficialEmailTemplatesStayEmailClientSafe` 钉死。Gmail 不支持 `text-shadow` / `box-shadow`，青/品红错位只是锦上添花。
- 头像、字标和两张二维码放在 `frontend/public/email/`，模板用公共占位符 `{{site_url}}` 拼绝对地址
  （先取 `frontend_url`，再取 `api_base_url`；生产是 `https://aaccx.pw`）。**微信群二维码大约 7 天过期**：
  替换 `qr-wechat.png` 后推 main 自动部署即可，邮件引用的是线上地址，已发出的旧邮件也会显示新码。
  字标 PNG 外圈带白边，是为了 Gmail iOS / Windows 版 Outlook 深色模式把白底强制翻黑时仍然看得见，换图时要保留。
- 按钮跳转：余额套餐 `/subscriptions`，流量卡 `/orders`（「我的订阅」页不列流量卡），兑换码 `/redeem`。
  这几条路径是否真在前端路由表里，由 `TestPurchaseNoticeDashboardPathsExistInFrontendRouter` 检查。
- 后台自定义的模板读不出来或渲染失败时，`renderForSend` 自动退回官方模板；各服务里旧的英文/双语兜底正文实际已走不到。
  2026-09-23 核对生产没有任何自定义模板，新官方模板上线即生效。
- 记录见 `docs/ai/context/20260923-202300-notification-email-card-style-chinese_CN.md`。

### 报销/开票申请（2026-09-15 上线，PR #34）

- 用户侧 `/reimbursement`，管理侧 `/admin/reimbursements`；表 `reimbursement_requests`（迁移 `214`）。状态只有 `pending`（用户侧显示「审核中」、管理侧「待处理」）和 `completed`。六字段：`company_name / tax_id / bank_account / bank_name / address / amount`。
- 解析走 DeepSeek 官方 API，模型 ID 是 **`deepseek-flash`**（`GET https://api.deepseek.com/models` 只返回它和 `deepseek-v4-pro`，没有「flash-4」这种写法）。代码在 `service/reimbursement_llm.go`，**裸 net/http 直连、不经网关计费路径**（不扣用户余额、不写 `usage_logs`）。system prompt 在同文件常量里，已用三个真实案例验证；改 prompt 要同步跑 `TestReimbursementParser_Live`。
- **API key 不在仓库、不在 compose 的真实值里**：三级取值 = settings 表 `reimbursement_llm_config`（后台「报销开票管理 → 解析设置」弹窗可改、无需重启）> env `REIMBURSEMENT_LLM_API_KEY / _BASE_URL / _MODEL / _TIMEOUT_MS` > 内置默认。生产 key 已于上线当天写入 settings（后台「解析设置」可换）；若 settings 与 env 都为空， `/reimbursement/parse` 返回 `503 REIMBURSEMENT_LLM_NOT_CONFIGURED`（提交、列表、下载不受影响）。key 明文存 settings（与 `content_moderation_config`、SMTP 密码同口径），读回只给尾 4 位掩码。
- 六字段齐全才入库（服务端二次校验，缺项 `400 REIMBURSEMENT_INCOMPLETE`）；缺失时前端只存浏览器 `localStorage` 键 `reimbursement_draft`。补充信息 = 「previous 字段 + 新文本」再喂 LLM，服务端再做防御性合并（LLM 返回 null 而 previous 有值则沿用）。用户不能手工改字段，只能用文字补充。
- PDF 落在 `/app/data/reimbursement/<id>.pdf`（`sub2api_data` 卷，可用 `REIMBURSEMENT_PDF_DIR` 覆盖），DB 只存相对路径 + sha256 + 大小。**R2 异地备份已失效，所以发票 PDF 目前没有异地备份。** 上传是 multipart 字段 `file`，只认 `.pdf` 扩展名 + `%PDF-` 魔数、≤20MB；已完成的记录可重新上传（覆盖文件，`completed_at` 保留首次值）。
- 上传成功后邮件通知是 best-effort（事件 `reimbursement.completed`，SMTP 未配置静默跳过），失败不回滚状态；用户侧靠列表状态与「下载 PDF」按钮。
- 管理端列表默认 `created_at asc`（需求：按提交时间从早到晚），用户端列表 desc。`parse` 端点挂了 `panelRateLimiter.Heavy()`。
- 管理端 `/api/v1/admin/reimbursement/*` 同样接受 admin `x-api-key`，不挂 step-up。

## 五、坑

1. **`BIND_HOST`**：基础 compose 写的是 `"${BIND_HOST:-0.0.0.0}:..."`。漏掉 `-f docker-compose.vps.yml` 就会真的绕过 UFW 把端口暴露到公网。现有三层防护：`.env` 改为 `127.0.0.1`、vps override、`DOCKER-USER` 兜底。
2. **`DOCKER-USER` 第一条规则绝不能删**：`-i eth0 ESTABLISHED,RELATED -j RETURN` 必须在 `-i eth0 -j DROP` 之前。容器访问上游的返回包也从 eth0 进入，只写 DROP 会掐断**全部上游调用**，症状是"所有上游超时"，极易把排查方向带偏。
3. **基础 compose 没有 `env_file:`**：`.env` 里的变量不会自动进容器。VPS 切换时 `BILLING_FINAL_MULTIPLIER=18` 就因此没生效，差点按应收的 1/18 收费。
4. **JWT secret 实际生效值来自 `/app/data/config.yaml`，不是 `.env`**。两端不一致会让全部用户网页会话失效。
5. **`6379/tcp` 与 `0.0.0.0:6379->6379/tcp` 是两回事**：前者只是 `EXPOSE`，后者才会插 DNAT 绕过 UFW。这是判断"有没有真的暴露"的唯一依据。
6. **GPT 低价渠道必须排除图像模型**：上游虽列出图像模型，但若映射进 `0.1x` 渠道，生图请求会按 `0.1x` 结算并绕过独立生图渠道。
7. **Vite 公共依赖分包不能叫 `vendor-*`**：Cloudflare 对该路径静态资源返回 403，导致 `/login` 白屏。现用 `lib-*`。
8. **迁移 checksum 保护**：已应用的迁移改内容会导致启动失败，只能新增迁移号（207 曾踩，用新增 208 解决）。
9. **Redis `sched:acc:*` 保存调度快照**：轮换上游凭证必须同时同步数据库和缓存，否则旧凭证继续被调度。凭证在 `accounts.credentials` 以 AES-256-GCM 服务端加密存储。
10. **取价有三级，`fallbackPrices` 是最后一级，登记它往往不解决问题**。顺序是 **① 渠道/分组定价（`model_pricing_resolver.go`）→ ② `PricingService` 远端价格目录 → ③ 硬编码 `fallbackPrices`**。远端目录由 `pricing.remote_url` 拉取，收录了大量模型，所以第 ② 级几乎总会命中，第 ③ 级只在目录失效时才走到。**只改 `fallbackPrices` 而没改 `PricingService`，等于什么都没改。**
11. **`matchOpenAIModel` 末尾有个 `DefaultTestModel`(= `gpt-5.4`) 兜底，会静默少收费**。任何以 `gpt-` 开头、又没被前面分支拦住的模型都掉进去，按 `$2.5/$15` 计价。新 OpenAI 模型必须在该函数里加显式前缀分支（照抄 `gpt-5.6-sol/terra/luna` 的写法），**否则别名写法（裸族名、缺连字符、effort/日期后缀）会按 gpt-5.4 价结算**。GPT-6 Astra 实际 `$10/$50`，掉兜底就是输入少收 4 倍、输出少收 3.33 倍。
12. **缺定价不会拒绝请求，会记零成本放行**。`openai_gateway_usage.go` 在取不到价时打 `pricing_missing_record_zero_cost` 日志后按 0 计费；通用网关 `gateway_usage_billing.go` 同样吞错返回 `ActualCost: 0`。计费发生在响应转发**之后**，认证层不看模型名——**所以「开放模型」必须在「定价确认生效」之后**，顺序反了中间窗口的少收无法追回（`usage_logs.actual_cost` 记下的就是错值）。
13. **远端目录的长上下文字段解析不到**。目录用 `*_above_272k_tokens` 表达，而解析器只认 `long_context_*`，命中数为 0。`>272K` 档位只能靠静态 fallback 价或 `applyModelSpecificPricingPolicy` 补，且后者还要求账号 `extra.openai_long_context_billing_enabled=true`（默认 false）。
14. **`newTestBillingService()` 传的 `pricingService` 是 `nil`**，只覆盖第 ③ 级。用它写的定价测试**测不到生产实际走的路径**，会给出假阳性。测生产行为要用 `&PricingService{pricingData: ...}` 构造非 nil 的，参见 `billing_service_gpt6_test.go`。
15. **改 `accounts.credentials` 必须先 GET 再整体 PUT**。`MergePreservingSensitiveCreds` 以 incoming 为基底，**非敏感键没传就是删除**——只传 `model_mapping` 会把 `base_url` 一起删掉、直接废掉账号。敏感键（`api_key` 等 14 个）在 GET 时被**整个移除**而非返回掩码，所以 PUT 回去不带它们会自动保留原加密值。`Name`/`Status` 有判空、`GroupIDs`/`Concurrency`/`Extra` 判 nil，只传 `credentials` 不影响其它字段。
16. **测模型可用性要用精确 ID，别用族名**。2026-09-05 用裸 `gpt-6` 测出 404，据此误判"上游没有 GPT-6"；实际上游只认 `gpt-6-astra`，一直是通的。**测错字符串比没测更误导**——它给出一个看起来有依据的错误结论。（**2026-09-22 更正**：ai-genesis 的账号 #1129 现在裸 `gpt-6` 也能调通，`sync-upstream` 清单里也列了它。「别名一律 404」这条对该上游已经不成立——**结论会过期，重要的仍是用精确 ID 当场测**。）
17. **判断生产是否加载了远端价格目录，不需要 SSH**：模型广场 `/api/v1/model-plaza` 是公开端点且暴露价格。挑一个「只在远端目录（2026-09-24 为 254 键）、不在内嵌目录（200 键）、且源码无硬编码兜底」的模型（如 `claude-sonnet-5`：远端 `$2/$10`，缺目录时会按家族兜底落到 sonnet-4 的 `$3/$15`；或 `gemini-3.5-flash-lite`），生产能报出精确价格就说明目录已同步。**`claude-fable-5`/`-5-1` 已于 2026-09-24 补进内嵌目录，不能再拿来判断。** 也可以 SSH 直接看容器里 `/app/data/model_pricing.json` 的大小和键。
18. **`Dockerfile` 的 `GOPROXY`/`GOSUMDB` 默认是国内镜像**（`goproxy.cn`/`sum.golang.google.cn`）。在 GitHub Actions 等海外 runner 上构建必须显式覆盖为官方源。
19. **`internal/service` 的 `unit` 标签测试套件可以编译，也能全部跑通**（2026-09-17 实测 `go test -tags=unit ./internal/service/` 约 150 秒）。以前这里写的是「编译不了、只能跑定向用例」，已经过时，改动后应该跑全量。`balance_package_usage_ledger` 这类由迁移建的表不在 ent schema 里，SQLite 测试库要手工建表，写法见 `payment_balance_package_refund_reclaim_test.go`。
20. **上游模型白名单存在 `accounts.credentials.model_mapping`**（恒等映射），不是单独的表或字段。改白名单走 `PUT /api/v1/admin/accounts/{id}`：按 `EditAccountModal.vue` 的既有约定，**请求不携带 `api_key` 字段即保留原加密凭证**。不要试图在 UI 上逐个删模型 chip——14×14 的删除按钮被 `.modal-footer` 覆盖（`elementFromPoint` 命中 footer），误点会关掉弹窗丢改动。**但「加」模型 UI 是安全的**：编辑弹窗底部有 `自定义模型名称` 输入框 + `填入` 按钮，不碰 chip。同一弹窗里 `同步最新支持模型` / `同步上游支持的模型` 会用上游清单**整体替换**白名单（对 ai-genesis 账号 = 放进图像模型并删掉四个在售条目），`清除所有模型` 字面意思，三个都别碰。提交后按钮会卡在「更新中...」但 toast 已报成功、数据已落库，**不要重复提交**；**分组编辑弹窗是同款症状**——`PUT` 已返回 200、数据已落库，按钮却长期停在「更新中...」。
    ⚠️ **`PUT /admin/accounts/{id}` 成功后会异步重跑 Responses 能力探测**（`account_handler.go:1041` → `ProbeOpenAIAPIKeyResponsesSupport`，仅 `platform=openai && type=apikey`），**幂等覆盖 `extra.openai_responses_supported`**——哪怕你只传了 `credentials`。探测有一条「非 2xx（401/422/400/5xx）→ 保守写 true」，所以上游只要不是干脆的 404/405 就会被写成 true，**足以把一个此前被判定为不支持、已稳定走 Chat Completions 的账号切回 Responses**。2026-09-22 改账号 1129 白名单时就这么把它从 `false` 翻成了 `true`（该账号 09-05 起就因故切到 Chat Completions，近 7 天分组 10 九成请求是 `inbound=/v1/responses → upstream=/v1/chat/completions`）。**修法是设 `extra.openai_responses_mode=force_chat_completions`**（走 `bulk-update`，坑 26），因为 `ResolveResponsesSupport` 先读 mode、命中就不看 supported，且 mode 不是探测会写的键，扛得住以后任何一次 PUT。只把 supported 改回 false 没用，下次谁再 PUT 一下就又翻回来。见坑 31 与 `docs/ai/context/20260922-160000-tighten-gpt-whitelists-74-10_CN.md`。
    **所以只改白名单时优先用 `POST /admin/accounts/bulk-update` `{account_ids:[id], credentials:{model_mapping:{...}}}`**：
    JSONB 键级合并（`mergeAccountCredentials`）、事务内加锁、不碰 `api_key`/`base_url`，而且**不触发上面那个重探测**
    （它只挂在单账号 `Update` 上）。2026-09-23 用它改了 #2/#5/#1129/#1166，回读 `openai_responses_supported` 均未变。
    #5（Kimi）至今是 `supported=false` 且**没有** mode 锁，对它用 `PUT` 就有被翻成 Responses 的风险。
    白名单**不能清空**：`model_mapping` 为空时 `IsModelSupported` 放行全部模型。
21. **分组「复制」会把源分组已绑定的账号一并绑到副本**。用复制建新分组后必须检查并解绑，否则新分组的请求会调度到旧账号并按新分组倍率计费。
22. **「提前刷新周额度」走专用端点，别手写 SQL、更别拿「改余额」变通**。`POST /api/v1/admin/payment/balance-packages/{package_id}/credit-next`（`package_id` = `user_balance_packages.id`；2026-09-08 上线，09-09 首次生产验证，09-12 双用户复用）：一次只发一期、金额按锁内实时值算、DB 级幂等（`payment_audit_logs` 对 `(order_id, action)` 唯一）、`next_credit_at` 取「原值 + interval」保持节奏、末期自动置 `completed` 并清空 `next_credit_at`；审计、`balance_debt_ledger` 欠费抵扣、余额缓存 + API Key 鉴权快照失效全在事务内完成。**前端仍无按钮，后台也没有任何只读接口返回 `package_id`**（订单、审计、系统日志里都没有；端点又不校验套餐归属，**绝不能按编号推算**），只能查库 `SELECT * FROM user_balance_packages WHERE user_id = …`。调用可以用 admin `x-api-key`（key 在 `settings` 表 `admin_api_key`，只能从生产 Mac 的库里读，VPS 那条路已作废），也可以在已登录的后台页面里用会话 JWT 同源 fetch（2026-09-16 实测）。**调用前先看该订单状态**：`REFUND_FAILED`/`REFUND_REQUESTED` 说明用户正在退款，而退款成功会撤销套餐并**连本周未用额度一起从余额扣回**（2026-09-18 起，见坑 32），提前发放的额度会被这一下扣掉，所以要先让管理员拍板。⚠️ **不要为「验证」重复调**：请求级幂等只在短 TTL 内去重，TTL 过后再调 = 发下一期（多发），不是无操作。`POST /admin/users/:id/balance` 变通会让 `next_credit_at` 不推进、定时任务到原日期重复发放且不写审计——永远别用。历史手写 SERIALIZABLE 事务模板仅在端点不可用时参考：`docs/ai/context/20260905-172724-user565-early-weekly-credit-execution_CN.md`；最近一次执行记录 `docs/ai/context/20260921-134013-3876129758-user491-early-weekly-credit-period3-pkg204-renewed_CN.md`。**续费过的套餐（`renewal_count>0`）核对「这期发过没有」必须按当前 `payment_order_id` 查**：续费会改绑订单并把 `credited_count` 重置为 1，端点幂等也只查当前订单，旧订单上残留的 `EARLY_WEEKLY_CREDIT_N` 是上一周期的，照它判断会误中止。**提前发放与定时发放的审计 action 串不同**（`EARLY_WEEKLY_CREDIT_N` vs `WEEKLY_CREDIT_N`），`(order_id, action)` 唯一索引拦不住彼此，防重发靠 `next_credit_at` 被推到下一期（定时任务筛选条件是 `NextCreditAtLTE(now)`，事务内再校验一次）——所以在预定到账时间前几小时执行也安全，2026-09-21 实测只差 1 小时 23 分也没有重复发放。**Chrome 扩展掉线时的退路**：SSH 进生产 Mac 读 `settings.admin_api_key`、主机内 `curl 127.0.0.1:8080` 调同一端点，2026-09-21 实测可用，审计 operator 与后台 JWT 通道同为 `admin:448`；但这条路会绕过 auto mode 对该写操作的拦截，**必须由管理员明确授权才用，不能自己换通道**。⚠️ **远端 heredoc 脚本里调 `docker exec` 别带 `-i`**：它会把脚本自身当成容器进程的 stdin 吃掉，第一条命令之后静默中止，表现是「零输出 + exit 0」，极易被误读成执行成功；每条命令再加 `</dev/null` 更稳。

23. **`schema_migrations` 行数多于迁移文件数是正常的**，不能据此判断代码来源。生产已应用 284 条而仓库只有 258 个文件，多出的 26 条是数据库从旧实例 pg_restore 带来的历史痕迹（旧实例跑过更新的上游构建）。迁移运行器只执行「文件存在但未应用」的，多余的行不影响启动。**据此误判过两次**：先认为「生产跑的不是本仓库代码」，再认为「部署本仓库是降级、须先合并落后 1095 提交的上游」，甚至已 merge 出 47 个冲突才发现搞错。判断代码来源要去看 `${OPS_DEPLOY_DIR}/src` 的实际文件，不看迁移行数。

24. **同一个 `base_url`，不同 API Key 的可用模型清单不一样**。2026-09-05 实测 `api.ai-genesis.app`：`#1129` 的凭证返回 8 个模型**含 `gpt-6-astra`**，`#1128` 的凭证只返回 6 个**不含**它。所以**「同上游 ⇒ 同模型」是错的**，按主机推广结论会把一个调不通的模型上架给用户（白名单只做准入，不保证上游真的有）。**逐账号问，别猜**——有两个零成本只读通道：
    - `POST /api/v1/admin/accounts/{id}/models/sync-upstream`：用该账号凭证打上游 `/v1/models` 并返回，**不写库**。（前端「同步最新支持模型」按钮点了没反应是前端的问题，**接口本身是好的**。）
    - `POST /api/v1/admin/accounts/{id}/test`（UI：账号行「更多 → 测试连接」，弹窗里可**指定模型**）：用账号凭证直连上游发一次真实请求，**不经用户网关、不写 `usage_logs`、不扣任何余额**。
      验模型可用性用它就够了。历史上为此建过 4 个临时 API Key 再删（`388`~`391`），也真实扣过 `$0.0496`——都是不必要的代价。**只有验「扣费金额对不对」才需要走真实网关。**
      副作用：测试成功会清空 `extra.model_rate_limits` 的冷却记录。**测试失败也有副作用**（`account_test_service.go`）：上游返回 401/403 会执行 `SetError`，把账号标成 error，直接停止调度；返回 429 会改写限流状态。所以对正在服务用户的账号，要慎用它批量探测，只查模型清单时用 `sync-upstream` 就够了。

25. **核对历史扣费必须用 `usage_logs.rate_multiplier`，绝不能 JOIN `groups.rate_multiplier`**。每条 `usage_logs` 都存了**扣费当时生效的分组倍率**快照；`groups.rate_multiplier` 是**当前值**。分组倍率一改，用当前值做除数去反推最终倍率就会凭空造出「计费异常」。2026-09-05 核验时就据此报出「DeepSeek 分组比值恒为 12.857143」——实际是 `18 × 3.5 ÷ 4.9`（该分组当天刚从 `3.5x` 改成 `4.9x`，那 208 条是改之前扣的），按行内快照倍率重算全部精确等于 18。**这类假异常看起来极有依据**，与坑 16「测错字符串」同类。正确写法：`actual_cost = total_cost * usage_logs.rate_multiplier * 最终倍率`，用 numeric 精确等值比较而不是浮点容差。**最终倍率按 `created_at` 分段**：2026-09-17 14:42:20（+08）之前是 18，之后是 17；`usage_logs` 不存这个值，跨这个时间点统一除以 18 会凭空造出「少收 5.6%」。另注意 `total_cost = 0` 的行比值无定义会被聚合静默排除，要单独确认它们 token 数也为 0（是占位记录）而不是坑 12 的少收。

26. **`schedulerSnapshot` 有两个同名不同物的东西，别把它们当成一个**。`repository.schedulerCache`（Redis `sched:acc:`/`sched:meta:`）确实把 `Extra` 过了 `filterSchedulerExtra()` **白名单**、把 `Credentials` 过了 `buildSchedulerCredentialMetadata()`（**连 `api_key` 和 `base_url` 都不留**）。但网关持有的 `schedulerSnapshot` 是 **`service.SchedulerSnapshotService`** 这层包装，它的 `GetAccount()` 读完缓存**直接丢弃**、恒走 `accountRepo.GetByID` 回源（`scheduler_snapshot_service.go:266-289`，注释：「生产路径必须回源，确保请求只从受控仓储取得解密后的凭证」；`accountRepo == nil` 才返回 cached，那只是测试接线，`wire.go:341` 注入的是真 repo）。
    **所以请求期拿到的 `account.Extra` 是完整的**，`filterSchedulerExtra` 白名单只影响**候选集筛选**，不影响计费和任何按 `Extra` 分支的请求行为。2026-09-05 差点据此得出「改 `openai_long_context_billing_enabled` 不会生效、必须改代码加白名单」的错误结论。
    **反向自检**：快照连 `api_key`/`base_url` 都没有——如果请求真用快照 account，所有 apikey 账号会直接认证失败、自定义上游会被打到 `api.openai.com`。**生产明明是通的，就说明用的不是快照。** 这条比读调用链更快证伪。
    另：改账号 `extra` 单个键有个更干净的接口 **`POST /api/v1/admin/accounts/bulk-update`**（`{account_ids, extra}`），走 JSONB **key 级合并**（`account_repo.go:2971+`）；而 `PUT /accounts/{id}` 是**整体替换**（见坑 15 的同款陷阱），必须 GET 完整 `extra` 再整体回传。

27. **`PUT /api/v1/admin/groups/{id}` 有三个字段不是偏量更新**。`admin_group.go` 的 `UpdateGroup` 里 40 多个字段全有 nil / 判空守卫，**唯独 `daily_limit_usd` / `weekly_limit_usd` / `monthly_limit_usd` 是无条件覆盖**（`group.XxxLimitUSD = normalizeLimit(input.Xxx)`，注释写「前端始终发送这三个字段，无需 nil 守卫」——但直接调 API 时不成立）。三种写法三种结果：**key 缺省** → `nil` → DB 里 `Clear` 成 NULL；**传 `null` 或空字符串** → `&0.0` → 写成 `0`；**传负数** → NULL。所以改分组任何一个字段，都要**先 GET 再把这三个值原样回传**。
    配套事实：`HasDailyLimit()` 判的是 `!= nil && > 0`（`group.go:128`），所以 **`0` 和 NULL 在执行层等价、都等于「无限制」**——服务层那句「0 表示不允许用量」的注释是错的，别照它推断出「分组被限死了」。且限额只在订阅型分组的 `calculateSubscriptionRemaining` 里生效，`subscription_type=standard` 的分组根本不读。
    好消息（都已核过代码，不用重复怀疑）：`copy_accounts_from_group_ids` **不传就完全不碰账号绑定**，坑 21 只在显式传它时触发；`name`/`status`/`platform`/`subscription_type` 是 `string` 判空守卫，不传即保留。**改分组倍率不需要动代码、不需要出镜像**——广场展示与真实扣费共用同一个 `groups.rate_multiplier`，接口内已含 `InvalidateAuthCacheByGroupID`。对照：`PUT /admin/channels/{id}` 是**干净的**偏量更新（`applyUpdateInput` 全字段有守卫）。另外，每次 PUT 分组都会执行 `sanitizeGroupMessagesDispatchFields`，把非 OpenAI 分组的 `messages_dispatch_model_config` 等三个 dispatch 字段清空。这对非 OpenAI 分组没有影响，但整行比对改前改后时会看到这几个字段变了，别误判为改坏了（2026-09-17，4 号分组）。
    **下架分组**（2026-09-22 实操）：`groups.status` 的合法值是 **`active` / `inactive`**，不是 channel 那套的 `active` / `disabled`（`group_handler.go` binding 为 `oneof=active inactive`，传 disabled 会 400）。改成 `inactive` 后 `groupRepo.ListActive()` 就不再返回它，**模型广场和用户建 Key 的可选列表同时消失**；但这不是软性隐藏——**已绑定该分组的现有 Key 会被 `validateAPIKeyGroupAvailable` 硬拒为 `403 GROUP_DISABLED`**（`api_key_auth.go:451`），所以下架前要先数清楚有多少 Key/用户绑着它。停用不动 `deleted_at`，`account_groups` 绑定完整保留，恢复就是把 `status` 改回 `active`（别忘了 description 也要改回去）。

28. **OpenAI API Key 账号的 `model_mapping` 会改写「计费模型名」，不只是上游模型名**。`openai_gateway_chat_completions*.go` 里 `billingModel := resolveOpenAIForwardModel(account, originalModel, …)` 直接套用账号映射，所以把 `A` 映射成 `B` 后，计费先按 `B` 取价（候选循环 `[B, A]`，`B` **取不到价才**回退 `A`）。后果有两种：`B` 在远端目录里有价 → **按 `B` 的价扣、广场却按 `A` 的价显示**；`B` 哪儿都没价 → 回退 `A`，碰巧没事。**只有渠道 `billing_model_source=requested` 才会强制按用户请求名计费**（`openai_gateway_usage.go:192`）；`channel_mapped` 在没有渠道级映射时等于跟随账号映射。
    实例（2026-09-16）：上游把 `deepseek-v4-flash` 改名 `deepseek-flash`，我方想让用户照旧请求 `deepseek-v4-flash`。但远端目录里 `deepseek-flash` 是 `$0.30/$1.20/读$0.006`，而我方校准价 `deepseek-v4-flash` 是 `$0.22/$0.66/读$0.007`——**只加账号映射会让输出多扣 82%、缓存读少扣，且与广场显示不一致**。另注意广场和 `/v1/models` 都列映射的**键**（`channel_plaza.go:197`、`gateway_service.go` `GetAvailableModels`），映射后用户看不到上游名。
    渠道级定价还会被广场当成「该渠道所有分组都支持这个模型」列出来（新分组没账号时就能看到模型），所以**别往多分组共用的渠道（如渠道 3）里加单一供应商的模型价**。

29. **火神上游（`huoshenai.net`，同样是 sub2api）会把客户端的 User-Agent 原样转给它的上游。`Python-urllib/*` 会被拒，返回 502 `Upstream service temporarily unavailable`，约 2 秒返回；连续失败后，火神还会给该模型加冷却，此后请求秒回 503。**
    - **影响我方用户**：我方网关默认也透传用户 UA，raw chat 和 Responses 转换两条路径都放行 `user-agent`。
    - **接火神账号时的做法**：在 `credentials.user_agent` 设一个固定 UA，77～79 用的是实测可用的 `OpenAI/Python 1.109.1`。
      - 后台编辑弹窗没有这个字段，只能通过 `PUT /admin/accounts/{id}` 设置；
      - 弹窗保存时会带上 `...currentCredentials`，所以不会冲掉它。
    - **火神的「标准价」不等于官网价**：这个价是火神自己的 sub2api 取价逻辑算出来的，带着和我们同款的漏洞（DeepSeek 全系按 flash 价，kimi-k2.7-code 按旧 K2 价，qwen 和 doubao 目前 0 元）。
      - 火神实扣 = 标准价 × 0.4。火神充值有赠送（2026-09 的订单是 ¥200 到账 $220，约 1:1.1），所以我方实际成本 ≈ 标准价 × 0.36 元。
      - **我方登记价要按官网价，别照抄火神价**，它随时可能被修正。
      - 实测数据见 `docs/ai/context/20260917-114913-huoshenai-guomo-price-measurement_CN.md`。

30. **流量卡欠费舍入 bug 已修复，历史漏扣已补扣完毕，不要再补一次。**
    - **bug**：余额为负、改用流量卡付费时，欠费按未舍入的费用计算，留下不足 5e-11 的尾差，写库后违反 `traffic_credit_debt_amount_check`，整笔计费回滚，请求成功却没扣费。日志关键字是 `record_usage_failed`。
    - **修复**：PR #38，2026-09-17 13:25（+08）上线。17:10 复查：
      - 上线后没有再出现 `record_usage_failed`；
      - 21 个流量卡付费请求的账本全部对平；
      - 有账号参与的 200 请求 388 个，386 个已扣费，其余 2 个是上游中途失败。
    - **补扣**：08-22 至 09-17 共失败 1,470 次，已补扣其中 1,455 次、$1,142.37，涉及 34 人；另 15 次找不到上游记录，没有补扣。
    - **记录位置**：两者都在 `billing_reconciliation_cases`，条件是 `error_code='TRAFFIC_DEBT_ROUNDING_ROLLBACK'`。已补扣的 `status='reconciled'`，没补扣的 `status='pending_external_usage'`。审计记录是 `payment_audit_logs.action='BALANCE_MANUAL_BACKCHARGE'`。
    - **手工补扣别用后台「调余额」**：
      - 这个接口不允许扣成负数；
      - 它只改 `balance`、不动当前套餐的 `remaining_usd`。周刷新按 `balance − remaining_usd` 算基础余额，所以只改余额的话，用户本周额度用满时，同一笔钱会在本周和下周**各扣一次**。
    - **正确做法是按正常用量扣**：先扣当前套餐剩余额度，并写 `balance_package_usage_ledger`；不足的部分成为负余额，由之后的套餐到账先抵扣。同时写 `redeem_codes`（`admin_balance` 负数，**备注用户可见**）、`payment_audit_logs`，并调用 `enqueue_auth_cache_invalidation`。
    - **漏扣可以精确对账**：火神和 ai-genesis 的控制台都在 Chrome 里登录着，能读逐条用量记录。步骤：
      1. 我方已扣费的请求先按 token 完全一致认领上游记录；
      2. 剩下的上游记录，按「模型一致、完成时间差在 −6～+1.5 秒、开始时间对齐」一对一分给失败请求；
      3. 按我方当时的计价规则重算金额，不照搬上游费用。
    - **与第六节不冲突**：第六节指的是缺少这些逐笔证据的情况；找不到对应上游记录的，仍然只记待核对、金额留空。
    - 完整方法、事务模板和逐人结果见 `docs/ai/context/20260917-144451-traffic-debt-rounding-fix-backcharge_CN.md`。

31. **ai-genesis 的 GLM key（`api2.ai-genesis.app`，上游分组 52）的 Responses 流式接口不可用，账号必须强制走 Chat Completions。**
    - **症状**：上游 `/v1/responses` 流式请求不发 `response.completed`（上游照样扣费），或者直接返回空的 502。
    - **为什么会误判**：保存 key 时的能力探测只发非流式请求，拿到 200 就写入 `openai_responses_supported=true`。所以「探测结果为支持」不代表流式能用。
    - **做法**：账号 1173 的 `extra.openai_responses_mode=force_chat_completions`，也就是编辑弹窗里的「Responses 模式」选项。
      - 这样入站的 `/v1/responses` 和 `/v1/messages` 都会转成 Chat Completions 再发给上游，2026-09-17 已实测扣费正确；
      - 只有显式生图的 Responses 请求会跳过这个账号。
      - 不要改回「自动」。
    - **新接 OpenAI 兼容上游时**：要用 `/test` 实际跑一次流式；测试失败就先切到这个模式再试。
    - 记录见 `docs/ai/context/20260917-153213-glm06-group76-aigenesis-account_CN.md`。

32. **余额套餐退款的两处多退已修复（PR #39，2026-09-18 01:15 UTC 上线），历史订单没有追溯处理。**
    - **修之前**：退款只按已用比例退钱，却不收回本周已到账、没用完的额度（`revokeBalancePackage` 只把 `remaining_usd` 置 0，不改 `users.balance`，而到期回收会扣）。买完立刻退几乎全额退回、第一周额度白拿，还能反复做。管理员重试 `REFUND_FAILED` 订单时又直接沿用订单上存的旧金额，不重算、也跳过人工审核。
    - **修之后**：退款成功在同一事务里 `balance -= remaining_usd`（用户被软删除时只撤销不扣，避免网关已退款却回滚），写进 `REFUND_SUCCESS` 审计并清余额缓存与认证快照；管理员取消套餐时留给用户的额度计入报价已用额度（取自取消审计的 `remaining_usd_before`，按当时余额封顶，**取消记录缺失的转人工审核**）；用户和管理员重试都重新报价；新增 `GET /admin/payment/orders/:id/refund-quote`，后台弹窗显示实时报价。
    - **要记住的三件事**：
      1. 本周额度没用完就退款的用户，余额会少掉这部分，退款弹窗已提前说明，用户问起照此解释；
      2. 那 35 笔旧套餐的 `REFUND_FAILED` 订单（存着 ¥1,194.12）现在会显示「需人工审核」，后台点不动了，要退只能人工处理；
      3. 订单 819（user 454）历史多留的 $46.19 仍在他余额里，**要不要扣回由管理员决定**，代码不会追溯。
    - **没修**：续费过的套餐比例会失真（分母含上一单顺延的期数、时间窗口被拉长，退款基数只有新订单价格），已退款的 16 笔都没续费过。
    - 排查与修复记录：`docs/ai/context/20260917-182925-balance-package-refund-proportional-audit_CN.md`、`docs/ai/context/20260917-195859-balance-package-refund-overpay-fix_CN.md`。

33. **加通知邮件事件时，声明了却没传的占位符会把"预览示例值"发给真实用户。**
    `NotificationEmailService.runtimeVariables` 先铺一层 `notificationEmailSampleVariables`
    再用调用方传入的变量覆盖，所以漏传一个占位符，用户收到的就是 `张三` /
    `https://example.com/...` / `Claude Pro` 这类假数据。`ops.scheduled_report` 里那段
    `if _, ok := input.Variables["report_html"]; !ok` 就是专门防这个的。
    **新增事件必须把事件定义里声明的占位符全部显式传满**（框架自己补的只有
    `site_name` / `recipient_name` / `recipient_email` / `unsubscribe_url`）。
    回归测试写法见 `purchase_notify_service_test.go`：逐事件断言占位符覆盖，
    再真渲染一遍断言结果不含示例值片段。
    另两条配套事实：① `EmailService.SendEmail` 是**同步 SMTP**，
    挂在支付回调等链路上必须异步，否则易支付/微信会判超时重推回调；
    ② 加设置项会让 `internal/server/api_contract_test.go` 的
    `GET /admin/settings` 字段快照变红，要同步改（有两处）。

34. **模型广场按「厂商」分类，靠的是模型 ID，不是分组的 `platform`。**
    `platform` 是**协议**（anthropic / openai），DeepSeek、GLM、Kimi、Grok 走中转站，
    `platform` 一律是 `openai`——所以广场那一行筛选以前只有 openai / anthropic 两个。
    2026-09-22 改成按厂商分类展示（小节 + 筛选 + 配色），判定在
    `frontend/src/utils/modelVendor.ts` 的 `RULES`（模型 ID 前缀 → 厂商，前缀锚定在开头）。
    **上新厂商时（第七节待上的 qwen / doubao 就是）要同步往 `RULES` 和 `VENDORS` 里加一条**，
    否则新分组会静默落到「其他」小节，光建分组是不够的。认不出的模型归「其他」不会丢，
    但也不会有厂商配色和 logo。判定是逐模型做的，所以一个分组混装多家模型时，
    它会在每个厂商小节各出现一次、每次只列本厂商的模型。
    改完跑 `npx vitest run src/utils/__tests__/modelVendor.spec.ts`（已钉住在架全部模型的归属）。

## 五点五、待处理的计费偏差（已确认，未修复）

> **口径（管理员 2026-09-05 明确）：只有「少收」是缺陷，「多收」不是。**
> 「即使上游没多收，我这里多收了也是我的收入。」
> 因此本节的排查与修复**一律只针对平台少收 / 零成本放行**；
> 「我们收得比上游成本高」不算问题，不要作为缺陷上报，也不要主动往下调价。
> 判断一条偏差要不要修，先问方向：**钱是漏出去了还是留下来了。**

- **`gpt-5.6-sol` / `gpt-5.6` 在向用户超收**：按 OpenAI 2026-08-24 降价前的旧价计费——输入 `$5` vs 官方 `$4`、缓存读 `$0.50` vs `$0.40`、缓存写 `$6.25` vs `$5`、输出 `$30` vs `$20`（**输入 +25%、输出 +50%**）。从 `usage_logs` 反推确认，非推断。27 小时敞口 `$104.40`、约 `$93/天`，6 个在售 GPT 分组全量受影响。**三层取价同错**：远端目录该键就是旧价、硬编码兜底是同样的错值；目录里裸 `gpt-5.6` 键的值反而是对的，但别名归一化在查价前就把它改写成 sol，导致正确记录永不可达。对照组 `gpt-5.6-terra` 正确，说明只是这一条没跟上降价。**管理员已决定：先自行核实官方价再改，历史超收不退、只修当下。修复时注意 $4/$20 是促销价，官方只承诺至少持续到 2026-11-21。** 详见 `docs/ai/context/20260905-154436-gpt56-sol-overcharge-finding_CN.md`。
  **补充（2026-09-05）：这个旧价是从上游继承的，不是我们算错。** 上游 `api.ai-genesis.app` 的模型广场对 `gpt-5.6-sol` 同样报 `$5/$30`。所以改价会让我们和上游口径不一致——**上游按 $5 结算给我们、我们按 $4 收用户，差价由平台吃**。
  **⚠️ 本条按上面的新口径已不构成缺陷**：方向是多收、且广场展示的就是同一组数字（用户看到什么就付什么），上游也按同价结算给我们。**在管理员另行指示前不要"修复"它**——照官方 `$4/$20` 改反而会主动放弃收入并造成对上游倒挂。上面那句「先自行核实官方价再改」是新口径之前的决定，**已过期，别照做**。
- ~~**长上下文计费一道都没打开**~~ **2026-09-05 已对 6 个 GPT 账号开启**（`#1128`/`#1129`/`#1130`/`#1132`/`#1164`/`#1168`，`extra.openai_long_context_billing_enabled = true`）。**GLM(#4) / Kimi(#5) / DeepSeek(#6) / 生图(#1131) 仍为 false，按管理员要求先不动。** 开启前 34 小时的敞口实测：153 条 >272K 请求全部按短价收，少收约 `$189`（约 `$134/天`），最大单条上下文 1,067,149 token。详见 `docs/ai/context/20260905-213000-gpt-long-context-billing-enablement_CN.md`。
  背景机制：**OpenAI 网关是唯一默认关闭该计费的路径**（Claude/Gemini/Grok 不传该指针、`applyLongCtx` 默认 true）。扣费侧 `billing_service.go:1062` 是 `applyLongCtx := len(resolved.Intervals) == 0 && *LongContextBillingEnabled`，`resolved.Intervals` 只认**渠道/分组里真正配置过**的区间定价（GPT 全系为空），所以账号开关是唯一闸门。
  **广场展示的 `>272K` 档位曾是「看得见、收不到」的**：广场那两个区间是 `channel_plaza.go:269` 的 `pricingIntervalsFromBilling()` **纯展示合成**的（拿 `ModelPricing` 的 `LongContext*` 倍率现算），**不是区间定价行**，不进 `resolved.Intervals`。所以**广场有档位 ≠ 会按档位收**，别拿广场当计费依据。
  **开关只对登记了长上下文字段的模型有效**：`gpt-6-astra`/`gpt-6`、`gpt-5.6`/`-sol`/`-terra`/`-luna`、`gpt-5.4`、`gpt-5.5`/`-pro` 有效（`applyModelSpecificPricingPolicy` 会回填阈值 272000 + 2x/1.5x，不依赖远端目录）；`gpt-5.3-codex`、`gpt-5.2`、`gpt-5.4-mini/nano` 等**开了也是空操作**（`LongContextInputThreshold` 为 0）。影子账号按**母账号**的开关判定。
- **客户端可以自己在请求体里写 `"service_tier":"flex"` 拿五折**（`priority` 则是 2 倍）。`serviceTierCostMultiplier`（`billing_service.go:135`）对 `flex` 硬编码 `0.5`，且乘在 `TotalCost` 求和**之前**——所以 `usage_logs.total_cost` 一起被砍半，**坑 25 那条对账恒等式照样成立，事后 SQL 查不出来**，只能看 `usage_logs.service_tier` 列。tier 取自客户端 body（`openai_gateway_request_body.go:849`），**从不与上游返回的 tier 核对，也没有模型门槛**——GLM/Kimi/DeepSeek/Grok 在任何价目表里都没有 flex 档，照样五折。
  **2026-09-05 实测敞口为零**：48 小时 6000 条 `usage_logs` 的 `service_tier` 全是 `null`，无人使用。属**潜伏**漏洞，不是正在流血。**2026-09-22 复测：`flex` 仍然是零**，但 `service_tier` 已经不全是 null——近 30 天有 960 条 `priority`（`actual_cost` 合计 $880.81），**全部来自 `/v1/responses` 入站**，Chat Completions 入站一条没有。**别误读成「Responses 端口按 priority 收费」**：带 tier 的只占各分组 Responses 请求的 0.2%~5.8%，其余都是 null 走标准价；而且 **948/960 来自同一个用户 `user 525` 的 `codex-tui`**（Codex 开 priority processing 会主动发这个字段），不是渠道属性。链路是：客户端 body 带值 → `fast` 归一化为 `priority` → `serviceTierCostMultiplier` 乘 2（乘在 `TotalCost` 之前，故 `total_cost` 已含 2 倍）→ 生产 `openai_fast_policy_settings` 为 `{"rules":[]}`，action=pass，**字段原样透传给上游**。`priority` 方向是多收、按本节口径不算缺陷，但**上游是否也按 priority 收我们没核对过**（两个上游都是 sub2api，大概率同样乘 2，则利润率不变）。**想关掉是纯配置**：后台 OpenAI fast policy 加规则，`filter` 删字段 / `block` 拒绝 / `force_priority` 强制，当前规则为空即全放行。修法是把 `flex` 分支改回 `1.0`（要改代码出镜像）。**在修掉之前别对外宣传这个字段。**
- **Claude 没有长上下文加价档，这是正确状态，不是漏收（2026-09-06 查官网确认，上一轮误报已撤回）。** Anthropic 官方定价页「Long context pricing」明确写：**Claude 4.6 及以后模型，完整 1M 上下文窗口全程按标准单价收**（原文「A 900k-token request is billed at the same per-token rate as a 9k-token request」）。我们白名单里的 Claude 全是 4.5~5 代，官方均无 >200K 涨价档。因此：① 账号上的 `openai_long_context_billing_enabled` 对 Claude **本就不生效**——`gateway_usage_billing.go:907` 的 Anthropic 计费路径根本不传该指针（那是 OpenAI 网关专用）；② Claude 模型没登记 `LongContext*` 字段是**对的**，不是缺陷；③ 我们的 Claude 基准价与官方标准价**逐个精确一致**（sonnet-5 `$2/$10`、opus-4-8/opus-5 `$5/$25`、sonnet-4-6 `$3/$15`、haiku-4-5 `$1/$5`、fable-5 `$10/$50`），再叠 `分组倍率 × 最终倍率`，全程平价——已按官方口径正确计费。
  - **想让 Claude 多赚**：干净的做法是抬 Claude 分组 `rate_multiplier`（配置即可、全上下文一律涨）；**不要**去代码里硬造一个 >200K 档——那是脱离上游的假分档，广场会露出比真实 Anthropic 更贵的价，且要出镜像。
  - 附：官方注明 **4.7 及以后模型换了新分词器、同文本约多产 30% token**（opus-4-7/4-8/5 受影响），我们按 token 收，这部分收入天然已在账上。
  - GPT 那次要开长上下文是因为 GPT/GLM **真有**涨价档（GPT >272K、GLM >32K）；Claude 没有，别照搬。
- **`batch_image.enabled` 默认 `false`，保持关闭**。批量生图结算不经 `applyFinalBillingMultiplier`（`batch_image_settlement.go:138`），开了会**少收 (N−1)/N**（N 为隐藏倍率，当前 17，即约 94.1%），且冻结额同样少 N 倍所以不会报错、静默通过。要开必须先补乘最终倍率。
- **超阈倍率是 2x 输入、2x 缓存读、2x 缓存写、1.5x 输出**——缓存同样翻倍，漏掉缓存会把长会话的主要成本项低估一半。边界是**严格大于** 272,000；我们广场 `tier_label` 写的「`<272K` / `>=272K`」措辞是错的（上游同款软件已修为「`≤272K` / `>272K`」，我们的构建更旧）。判定口径为 `输入 + 缓存写 + 缓存读` 三者之和。
- **`allow_live` 开启前必须先落地时长计费，否则平台单向净亏且不可追回。**
  `service/openai_live.go` 的 `finalizeLiveCall` 写死 `TotalCost=0 / ActualCost=0 /
  RateMultiplier=1`，完全绕过 `applyUsageBilling`（全仓非测试调用点只有
  `gateway_usage_billing.go` 与 `openai_gateway_usage.go` 两处，Live 不在其中）。
  用户余额一分不扣，因此**欠费闸永远不触发**，余额 `$0.01` 的用户可无限重复
  最长 `liveMaxSessionDuration`（默认 1 小时）的会话；落库的 `actual_cost` 就是 0，
  事后无法重建。**2026-09-05 已在 `handler/openai_live.go` 的 `liveEnabledForAPIKey`
  加了 `liveBillingImplemented = false` 硬拒绝**，一处早退覆盖全部 Live 路由；
  接通计费时连同早退分支与 `openai_live_test.go` 的对应断言一起改回。
  开启前置：① 先有按时长的定价并确认命中三级取价（只加 `fallbackPrices` 无效，见坑 10）——
  **仓库当前完全没有音频/时长价格**，接了计费管道而无价只会按 0 放行（坑 12）；
  ② 每分钟单价是业务决策，需管理员先拍板；③ 同步确认
  `openai_live.go` 里 `WithOpenAIProfitControlSuppressed` 的豁免是否仍成立
  （该行原注释谎称「Live 按通话时长计费」，已于 2026-09-05 更正）；
  ④ `TestFinalizeLiveCallIsIdempotentAndWritesZeroUsage` 用 `require.Zero` 钉死了零值，
  是有意设的绊线不是待修 bug，同一个 PR 里必须一并改断言。
  另注意分组「复制」会连 `allow_live` 一起复制（与坑 21 同源）。
  **上游 `Wei-Shaw/sub2api` 至 2026-09-05 仍是同款零计费，同步上游修不了它。**
- **不要把「Linux 上打开 `allow_live` 只会 503」当成 Live 安全。**
  `internal/platform/liveattestation` 是 **darwin-only**（`//go:build !darwin` 返回
  `ErrUnsupportedPlatform`），而 `prepareLiveAttestation` 在 `CreateLiveCall` 里是
  **选账号之前的硬闸**，所以生产 Alpine/Linux 镜像下 Live 结构性起不来。
  但这是平台的偶然属性、不是计费保护——**任何一次改成 macOS 部署或给非 darwin 补 provider，
  这道闸就没了**。判断 Live 敞口要看计费有没有接上，不能拿平台限制当理由。


## 六、负面教训（结论已撤回，不要重复）

- **不能用"同模型 + 时间窗口正负 N 秒"把共享上游账号的账单归属到本地用户**。据此得出的 `$652.17178575` 未追回扣费和单用户 `$384.62055975` 均已撤回，**不得用于补扣**。相关失败事件缺请求 ID、Token 和费用快照，`billing_reconciliation_cases` 一律保持待核对且金额留空——**不用次数或平均价伪造扣费**。
- **上游 Usage 页面的"费用"就是标准费用**，本地按 `标准成本 × 分组倍率 × 最终倍率` 扣费。曾因把它误读为"已含上游加价的用户价"，在一天内把 Kimi 倍率来回改了四次。
- **三种验证假阳性**（都真实误导过）：`nc -z` 对开放端口报不可达；`wget` 收到 `401` 被当成网络不通；命令管道到 `head` 使退出码恒为 0，脚本从头到尾没检查任何东西却报"✓ 通"。

## 七、未完成

- **`deepseek-v4-flash` 自 2026-09-16 前起调不通**（上游 #6 `api.ai-genesis.app` 已改名为 `deepseek-flash`，请求旧名回 `model_not_found`，我方对用户表现为 502，失败请求不扣费）。管理员要求「用户仍请求 `deepseek-v4-flash`、内部转 `deepseek-flash`」。**修复方案待管理员拍板**，原因见坑 28：只加账号映射会按目录里 `deepseek-flash` 的价扣费。两个干净方案：① 分组 8 移出共享渠道 3、单建 `billing_model_source=requested` 的 DeepSeek 渠道并显式登记现价（无需部署；改渠道 3 被自动权限拦下，需管理员确认）；② 代码里把 `deepseek-flash` 加进 `usesCalibratedFallbackPricing` 和 `getFallbackPricing`，当作 v4-flash 别名（需出镜像）。**两种方案都要等计费修好，才能在 #6 上加映射。**
  另一条路：火神分组 77 自 2026-09-17 起**仍以原名**提供 `deepseek-v4-flash`，可以引导用户切过去。
- **广场上 `deepseek-v4-flash` 的「官方」列仍是过时价**：
  - 显示的是 `$0.22/$0.66/读$0.007`。这组数来自代码里的校准兜底价，是已下线的 V4-Flash-0731 的低谷价。
  - 现行官方高峰价：v4-flash `$0.30/$1.20`，v4-pro `$1.32/$3.96`。
  - 影响：用户对照广场，会以为分组 77 不止 4 折；分组 8 也按这组旧价计费。
  - 要修得改 `billing_service.go` 并出镜像，待管理员决定。
- **火神的 qwen、doubao 分组管理员决定暂不上**（2026-09-17）：火神目前对这两家不收费，官网价已整理进 `20260917-114913` 实测文档。已上的 77～79 与火神逐条对账过：token 数一致，DeepSeek、GLM、k3 的标准费用与火神分毫不差；按最便宜的套餐、额度用满算（1 元 = 10.48 余额美元，扣费再乘隐藏倍率 17），用户实付约为官网人民币价的 6.7 折，火神成本约为官网人民币价的 5.5%，**收入约为成本的 12.5 倍**（= 分组倍率之比 2.8/0.4 = 7，× 「17 ÷ 10.48」与「1 ÷ 1.1」之比 1.78）。隐藏倍率为 18 时这两个数是 7 折和 13.2 倍。

- **在售分组的白名单已于 2026-09-23 按逐模型实测收紧**（管理员指示：调不通的剔除，用户只能用白名单模型）。
  12 个在售分组的每个白名单模型都用账号「测试连接」实测过一次：
  - #2 去掉 `claude-opus-4-5-20251101`，#5 只留 `kimi-k3`，#1129 去掉 `gpt-5.6`，#1166 去掉 11 个 404 的旧 Claude；
    其余 8 个账号全通、未改。分组 4、10、71 的备注已同步，渠道监控随之自动收敛到白名单（监控只列白名单模型）。
  - `inactive` 分组（8、9、13、69、70、77～79，含 #1128、#6）没测也没改。
  - **分组 71 三轮实测全挂，同日已停用**，见第三节表格。
  - 本次只删不加，上游有而我方没开的模型清单、改前白名单（回滚用）见
    `docs/ai/context/20260923-122309-group-whitelist-availability-audit_CN.md`。
  - **白名单改了或上游恢复了，要同步改分组备注**；监控不用手动改，10 分钟内自动同步。
- **分组 5 和 13 实际不可用，但仍有用户持有这两个分组的 key**（**13 已于 2026-09-22 连同 9 一起下架为 `inactive`**，详见 `docs/ai/context/20260922-154800-disable-gpt-groups-9-13_CN.md`；5 已于 2026-09-23 09:06 软删，见第三节分组表下的说明）：
  - 分组 5 `Claude0.5倍率（日常一）`：没有绑定账号，但仍有 15 个有效 key，用户请求全部返回 503，最近一次在 09-13；
  - 分组 13 `GPT0.16倍率（日常二）`：唯一账号 #1132 从 09-15 16:34 起停止调度（当时后台对它做过测试连接），仍有 35 个有效 key，09-17 仍有请求返回 503。
  - 两组的备注都已写明「暂不可用 / 暂停使用」并指引用户改选。
  - **13 待管理员决定**：补账号恢复调度，还是继续下架。恢复后记得把备注改回模型列表。
- **火神国模三组 77/78/79 自 2026-09-21 晚起全部不可用**：火神「国模」分组的 `/v1/models` 变成只有 GPT 模型，国产模型请求一律秒回 `503 Service temporarily unavailable`（火神分组里没有可用账号），火神余额、key、分组状态都正常，**不是我方问题**。09-22 起 77 有 67 次、79 有 7 次失败（未扣费）。**管理员已于 2026-09-23 把三组都改为 `status=inactive`**（停用时 77 有 12 个有效 key / 11 人，79 有 2 个 / 2 人，78 为 0；这些 key 现在会 403），火神恢复后改回 `active`，**同时把 7、76、81、82、83 的备注按排查记录改回去**（停用时删掉了其中指向「日常一」的说明）。排查记录 `docs/ai/context/20260923-092000-huoshen-guomo-groups-77-79-down_CN.md`。
- **分组 70 Gemini 的可用性无法确认**：账号 #1165 调 `sync-upstream` 返回 502（`kind=upstream`），分组自 08-28 以来没有请求。
- **Grok 账号 #1169 没设 `credentials.user_agent`**（坑 29）：它也是火神账号，火神把客户端 UA 原样转给上游，`Python-urllib/*` 会被拒。77~79 都设了 `OpenAI/Python 1.109.1`，#1169 建于 09-10、早于这条经验，一直是空的。补只能走 `PUT /admin/accounts/1169`（后台弹窗没这个字段）。
- **`build.yml` 从未实际触发过**——需手动跑一次确认能出镜像且前端不 OOM。
- **外部拨测未配置**——机器宕机时无人知晓。
- **单点故障无冗余**——全部服务跑在一台家用 MacBook 上（比 VPS 时更脆：笔记本 + 家用网络 + 依赖 GUI 登录/不睡眠，见第二节 durability 与防睡眠告警）。VPS 是唯一后路（冷备，数据已冻结）。
- **报销/开票的发票 PDF 落本机数据卷、无异地备份**（`/app/data/reimbursement/`）。功能已于 2026-09-15 上线（PR #34），DeepSeek key 已写入生产 `settings` 表并「测试解析」通过；换 key 只需在后台「报销开票管理 → 解析设置」重填。实现记录见 `docs/ai/context/20260915-131500-reimbursement-invoice-feature_CN.md`。
- `api_base_url` 仍为空（`/keys` 页面已硬编码 `https://api.aaccx.pw/v1` 兜底，不依赖该设置）。
- `billing_reconciliation_cases` 3,933 条余额不足案件待外部逐笔账单核对。
- **0 token、0 费用的占位记录（请求类型 2）不是漏收**：2026-09-17 与 ai-genesis 逐条核对当天 47 条（账号 1128、1129），上游对应记录同样是 0 token、$0。以后看到这类记录不用再查。
- **Cloudflare 约 125 秒超时后，上游照样收费，我方不扣费（待管理员决定）**：
  - 原因：流式请求要等上游返回响应头，网关才开始回数据；`stream_keepalive_interval` 也要等收到上游响应头才生效。上游首字节超过约 125 秒时，用户收到 CF 524，我方记为 499、不扣费，而上游跑完照常收费。
  - 2026-09-17 全天有 71 个这样的 499（1128 有 36 个，1129 有 25 个，1172 有 10 个）。其中 15 个在 ai-genesis 上找到了收费记录，上游实扣约 $0.94。
  - 用户拿到的是报错，所以不建议补扣。根治需要改代码：在上游返回响应头之前先发 SSE 头和保活，出错改用 SSE 错误事件。
  - 记录见 `docs/ai/context/20260917-181146-billing-recheck-after-fix-and-multiplier17_CN.md`。
- 续费订单与套餐的多对一审计映射缺失：当前套餐 `payment_order_id` 只绑定最新续费订单，历史续费订单无法各自独立退款。
- **易支付（ZPay）退款只在支付后很短时间内打得出去，超窗口一律失败**（2026-09-21 查证）：
  - **用户报「无法退款」，先查审计再查代码。** `payment_audit_logs` 里 `action='REFUND_FAILED'` 的
    `detail::jsonb->>'detail'` 直接写着原因。27 笔都是易支付返回的 `卖家余额不足`
    （HTTP 200、业务 code 非成功，`easypay.go:483` 原样透传），**我方报价和调用都是对的**。
    另有 18 笔 `proxyconnect ... 7897 connection refused` 是 VPS 时期出网问题，迁 Mac 后已不再出现。
  - ⚠️ **「卖家余额不足」≠ 商户后台那个余额。** 管理员后台显示 ¥29.90 且标注「抵扣手续费使用」，
    那是手续费预存款，不是退款资金池。**别去充值，充了也不解决。**
    按「支付→退款」间隔分桶（已排除代理故障那 18 笔）：**8 小时内 10 笔全成、0 失败**；
    8 小时~2 天 40%；2 天以上仅 12.5%。金额完全不相关——¥44.31 在支付后 43 分钟退成，
    ¥1.04 在 5 天后退不掉。符合支付宝行为：退款优先走原交易未结算资金，钱一旦结算走就得由
    收款账户余额出，不足即报此错。（强相关推断，无 ZPay 后台佐证。）
  - **管理员已定方向（2026-09-21）：限制可退期限，超窗口走人工打款。** 产品改动尚未实现。
  - 2026-08-20 以来 12 笔卡住（当时报价合计约 ¥218.79），最早一笔 07-10。重试会**重新报价**
    （PR #39），用户这期间继续用额度，金额只会更低——**别拿订单上存的 `refund_amount` 当应退额**。
  - **无人知道它在发生**：没有告警，只有用户来问才发现。是否通知这批用户、是否加监控，待管理员定。
  - 排查记录与逐笔清单：`docs/ai/context/20260921-164601-user525-refund-failed-easypay-seller-balance_CN.md`。
  - 查审计的两个语法坑：`payment_audit_logs.order_id` 是 `varchar`（要 `IN ('824')`，传整数报
    `operator does not exist`）；JSON 列叫 `detail`、类型 `text`（要 `detail::jsonb->>'key'`）。

- **人工打款后「取消订单」只有一个操作可做，且它不改订单状态**（2026-09-21 实操确认）：
  - `POST /admin/payment/orders/{id}/cancel` **只收 `PENDING`**，对已支付订单直接 `400 INVALID_STATUS`。
    能用的是 `POST /admin/payment/orders/{id}/cancel-balance-package`（后台同名按钮，
    订单详情的 `can_cancel_balance_package` 为 true 时可用）。
  - 它做：套餐 `completed/active → cancelled` + 写 `BALANCE_PACKAGE_MANUAL_CANCELLATION` 审计。
    它**不做**：不向支付服务商发起退款（`gateway_refund_executed:false`）、不动 `users.balance`、
    **不改订单状态**——订单仍停在 `REFUND_FAILED`，用户端照样显示退款失败。
  - **它真正的用途是释放「每用户最多一个有效套餐」的占位**：套餐即使 4 期发完、`remaining_usd=0`，
    只要 `expires_at` 未到就仍占位，用户买不了新套餐。取消后才能重新买。
  - **取消不会破坏后续退款报价**：`balancePackageRetainedQuota` 会读这条审计的 `remaining_usd_before`
    计入已用；只要审计正常写入就不转人工审核（手工 SQL 取消或旧动作名会让它 `known=false` → 转人工，
    **所以永远走接口、别手写 SQL**）。
  - 报价以 `GET /admin/payment/orders/{id}/refund-quote` 为准（只读）：基数是 `amount`（**不含**手续费，
    `pay_amount - amount` 那部分不退），分母 `weekly_credit_usd × refresh_count`，
    取 `Max(时间比例, 用量比例)`。用量口径限 `created_at >= starts_at`。
