# AGENTS 记忆压缩沉淀

> 来源：`AGENTS.md` 当前内容，压缩时间 2026-06-24 19:56。
> 后续长期上下文继续新建到 `docs/ai/context/YYYYMMDD-HHMMSS-*.md`，不要覆写、重命名或删除历史文档。

## 核心定论

- 采用“三项目串联方案 A”：Sub2API 是唯一公网 API 入口，也是唯一用户 Key、计费和用量事实源；CLIProxyAPI 只作为内网账号池、OAuth、协议转换和轮询上游；yui.web/shop 只保留展示、说明和跳转。
- 当前主链路：`Cloudflare Tunnel -> nginx 127.0.0.1:8080 -> Sub2API 127.0.0.1:18080 -> CLIProxyAPI 127.0.0.1:8317`。
- 不要让 Sub2API、yui.web、CLIProxyAPI 同时对同一个用户 Key 做状态判定或扣费。
- 不要在文档、提交或日志中记录完整 API Key、内部 token、HMAC secret、SMTP 密码。
- 前端 UI 采用 `yui.web` 作品集黑白灰风格，默认品牌名为「天才程序员小站」，保留 `siteName/siteLogo` 后台配置优先逻辑；信息架构、后端、API、路由和计费逻辑默认不改。

## 当前运行态

- `aaccx.pw/shop` 归 yui.web；`aaccx.pw/v1/*`、`/api/*` 和 Sub2API 控制台路由归 Sub2API；`api.aaccx.pw` 也是 Sub2API 入口。
- CLIProxyAPI 是聚合上游，不是单个静态 OpenAI Key；作为 Sub2API 上游账号时必须启用 `credentials.pool_mode=true`，并让 401/403/429 在同账号内重试。
- 套餐分组显示名为 `codex-pool-19-usd`、`codex-pool-29-usd`、`codex-pool-49-usd`，分别对应每日 19/29/49 USD；`codex-pool-local-unlimited` 是本机自用无限额分组。
- 当前售卖套餐里 `29 元订阅池` 对应 `subscription_plans.id=1 -> group_id=2 -> codex-pool-19-usd`；`codex-pool-29-usd` 实际对应 `39 元订阅池`，不要按分组名误绑 29 元/月用户。
- yui.web 旧 Key 已按 `orders` 迁入 Sub2API；旧邀请码和旧 API Key 发放业务应退役为 410/只读历史，不要继续写入。
- `xiaobianfuai@gmail.com` 是管理员和本机 Codex Local Key 所属账号（原 `15951875192@phone.com` 已改名），不要按普通用户删除；如需隐藏，用角色筛选或备注标识。

## 易踩坑

- 前端 chunk 命名已从源头改为 `pkg-*`；不要再把真实 `/assets/pkg-*` 反向 rewrite 到 `/assets/vendor-*`。公网入口兼容只保留 `app-index-* -> index-*` 方向。
- Docker 构建需要复制 `docs/legal/*.md`，否则前端 raw import 会失败。
- 更新 Sub2API account credentials 时要带回 `base_url` 等非敏感字段；后端只保留未提交的敏感字段，非敏感字段会被 incoming credentials map 覆盖。
- SMTP 已配置为 Gmail 发信，发件邮箱 `xiaobianfuai@gmail.com`，`email_verify_enabled=true`、`password_reset_enabled=true`；忘记密码是邮件重置链接 token，不是用户手输验证码。曾在对话中暴露过 Gmail 应用专用密码，后续建议重新生成并替换。
- `/purchase` 在未配置支付服务商时展示手动收款码，引导用户去 `/redeem`；该路径不创建支付订单、不自动开通订阅、不写账单或用量。

## 用户与账号变更摘要

- 多个手机号迁移用户已改为真实邮箱，均保持用户 id、API Key、订阅、余额、用量和密码 hash 不变，并清理或广播 API Key auth cache 失效。详见对应 `docs/ai/context/20260620-*` 与 `20260621-*real-email*` 记录。
- `18014503779 -> 1915474749@qq.com` 迁移时合并了真实邮箱空壳账号 `users.id=25` 到有权益主账号 `users.id=16`，软删除空壳账号。
- 已直接创建 `2316095427@qq.com` 用户，绑定 29 元/月套餐 `subscription_plans.id=1` / `group_id=2`，创建默认 API Key，公网登录和模型接口验证通过。
- 管理员账号已改为 `xiaobianfuai@gmail.com`，仍保持 `admin`；原占用该邮箱的普通测试账号已软删除并清理相关 token/cache。

## 功能与前端变更摘要

- 已完成 Sub2API 前端 yui.web 黑白灰视觉重设计，默认品牌为「天才程序员小站」。
- 首页未登录态只保留 Hero 区「立即登录」，套餐卡片为 29/39/59 元，支持模型只展示 GPT 5.3、Codex 5.4、GPT 5.5。
- `/purchase` 默认展示订阅页签；订阅卡片和首页套餐文案已精简为月度订阅、30 天、日限额和 24 点刷新。
- `/purchase` 已修复 GPT 一次性流量包在仅手动收款码运行态下无法继续购买的问题，次卡与订阅统一复用手动收款弹窗。
- 普通用户 `/usage-guide` 已演进为使用方法控制台，包含「Codex 接入」「生图方法」「Trae 接入」等栏目；不展示管理员分组 id、本地端口或后台 API 细节。
- 用户仪表盘已移除「模型分布」卡片，保留 Token 使用趋势。
- 普通用户侧边栏已移除 `/monitor`「渠道状态」入口，但保留相关路由和管理员监控；`/available-channels` 展示 GPT 价格和生图价格。
- `/api/v1/channels/prices` 为用户侧只读价格接口，复用 `ModelPricingResolver` 实际计费口径；priority/fast 展示价按基础价 `* 1.5`。

## 生图与计费

- 代码支持 `/v1/images/generations`、`/v1/images/edits` 和 `/v1/responses` 的 `image_generation` 工具。
- 已移除 CLIProxyAPI 中错误的 `gpt-image-2 -> gemini-3.1-flash-image` Antigravity 别名映射，并确认 CLIProxyAPI 直连真实生图可用。
- 已开启在售订阅分组和本机自用分组的生图能力：`groups.id=2/3/4/5` 均设置 `allow_image_generation=true`。
- 生图价格统一为 `1K=0.10 USD/张`、`2K=0.20 USD/张`、`4K=0.40 USD/张`；三档在售分组日限额仍为 19/29/49 USD。
- OpenAI `service_tier=priority` 和客户端别名 `fast` 的源码计费规则已从 2 倍改为 1.5 倍；公网或本地服务需重启/发布新代码后才会生效。

## 路由、发布与运维

- `aaccx.pw` 的 Sub2API SPA 认证入口已补齐，包括 `reset-password`、`forgot-password`、`email-verify` 等，避免邮件重置链接 404。
- yui.web 旧 Shop 私有业务路由已在公网分流层下线并返回 410；`/shop/` 和 `/shop/guide/` 保留 200。
- 已补充公网 Sub2API 重启与低影响发布方法文档：直接重启会短暂影响 `https://api.aaccx.pw/v1/*`，可选择直接重启、蓝绿切换或仅前端样式小修的零重启临时覆盖。
- 已新增 `deploy/restart-sub2api.sh` 和 dry-run/mock 回归测试；脚本只重启 Sub2API 本体，不触碰 Postgres、Redis、CLIProxyAPI、nginx 或 Cloudflare Tunnel。
- 已新增 `deploy/redeploy-sub2api-image.sh` 和 dry-run/mock 回归测试；默认后台 detached 执行“构建镜像 -> 只重建 sub2api 容器 -> 健康检查”。
- 已执行本地重部署 skill 相关流程，具体结果见 `20260624-183228-sub2api-local-redeploy-skill-result_CN.md` 和 `20260624-185315-sub2api-local-redeploy-result_CN.md`。

## 最近排障记录

- 手机端触屏点击异常根因是 `AppSidebar` 移动遮罩和 `AppHeader` 同为 `z-30`，触屏命中顺序不稳定；已将移动遮罩提升为 `z-[35]`，保持低于侧栏 `z-40`、高于 Header `z-30`。
- 管理员运维页面日志来源：成功慢请求在 `usage_logs`，错误和恢复错误在 `ops_error_logs`，请求明细接口 `/api/v1/admin/ops/requests` 会合并二者。
- 2026-06-24 最近 1 小时 TTFT P99 约 `29.2s`，超过 8s `52/320`、超过 15s `13/320`；慢尾主要集中在 `gpt-5.5` 和 150k-220k tokens 大上下文请求，当时不是 provider 502 爆发。
- 已检测 `~/.codex/logs_2.sqlite` 是否因 TRACE 日志持续高频写盘；确认 Codex Desktop app-server PID 3250 持有主库/WAL/SHM 并持续写入，TRACE 为主要来源，详见 `20260624-195506-codex-logs2-trace-high-frequency-write-diagnosis_CN.md`。

## 重要历史文档入口

- 初始压缩记忆：`docs/ai/context/20260619-151920-sub2api-compressed-memory_CN.md`
- UI 重设计：`docs/ai/context/20260620-214101-sub2api-yuiweb-black-white-ui-redesign-design_CN.md`、`docs/ai/context/20260620-214423-sub2api-yuiweb-black-white-ui-redesign-result_CN.md`
- 生图能力与计费：`docs/ai/context/20260621-155950-image-generation-access-billing-current-state_CN.md`、`docs/ai/context/20260621-162628-enable-image-generation-sale-and-local-groups-result_CN.md`
- priority/fast 1.5 倍计费：`docs/ai/context/20260621-204154-priority-fast-1-5x-pricing-result_CN.md`
- 公网发布脚本：`docs/ai/context/20260624-104042-sub2api-one-click-restart-script-result_CN.md`、`docs/ai/context/20260624-110251-sub2api-image-redeploy-script-result_CN.md`
- 购买页次卡手动支付修复：`docs/ai/context/20260624-154901-gpt-traffic-pack-purchase-fix-result_CN.md`
- 运维日志与慢请求诊断：`docs/ai/context/20260624-191644-ops-logs-visibility-slow-requests-current_CN.md`
