# 项目协作约定

- 默认使用中文；代码注释只说明原因。
- 支付订单的金额、退款金额和订单状态以服务端为准，前端金额只用于展示。
- 退款必须绑定创建订单时的支付服务商实例，并保留可审计的订单状态变化。
- 设计与实现上下文写入 `docs/ai/context/`，历史文档只新增不覆盖。

## 支付实现上下文

- 余额套餐退款报价和执行实现位于 `backend/internal/service/payment_balance_package_refund.go`，公式固定为时间比例与周期用量比例的最大值。
- 用户退款必须绑定订单创建时的支付服务商实例；ZPay 易支付订单优先使用 `out_trade_no` 发起退款。
- 退款成功只撤销对应 `user_balance_packages` 的后续到账，不跨订单扣减用户既有余额；网关失败进入 `REFUND_FAILED` 并支持重新报价重试。
- 最新实现与验证记录见 `docs/ai/context/20260803-191808-payment-refund-implementation_CN.md`。

## 18080 到 18082 数据迁移上下文

- 迁移脚本位于 `scripts/migrate-18080-users.sql` 和 `scripts/migrate-18080-users.ps1`；脚本默认全量，使用 `-SamplePercent 10` 可先做抽样验证。
- 用户、登录身份和 API Key 全量迁移；订单、支付审计、使用记录和余额套餐按抽样比例迁移。
- 流量订单及流量卡数据不迁移；旧订阅订单转换为 18082 的余额套餐并按 7 天刷新。
- 10% 已提交迁移的结果与核验记录见 `docs/ai/context/20260803-205649-18080-to-18082-10pct-migration_CN.md`。

## 18080 下线与备份上下文

- 2026-08-04 已停止 `sub2api-dev`，18080 端口关闭；18082 保持运行且健康检查通过。
- 完整源库备份位于 `D:\CodeWorkSpace\migration-backups\18080-offline-20260804-083350`。
- 停服与备份校验记录见 `docs/ai/context/20260804-083350-18080-stop-and-backup_CN.md`。

## aaccx.pw 公网链路上下文

- 2026-08-04：公网链路为 `aaccx.pw / www.aaccx.pw / api.aaccx.pw -> Cloudflare Tunnel 7f5fafd9-8a59-4013-ba42-3116dfc29463 -> 127.0.0.1:8080 -> sub2api-public-nginx-local -> host.docker.internal:18082`。
- Cloudflare Tunnel 配置位于 `D:\CodeWorkSpace\sub2api\deploy\cloudflared-windows-aaccx.yml`，当前 Tunnel 进程已在运行，域名 DNS 已由 Cloudflare 托管，无需改动 DNS 记录。
- Nginx 宿主机配置位于 `D:\CodeWorkSpace\sub2api\deploy\nginx-public-local-18080.conf`；18080 下线后必须将 upstream 改为 `host.docker.internal:18082`，修改前备份保存在同目录 `backups`。
- 每次调整公网链路必须先执行 `docker exec sub2api-public-nginx-local nginx -t`，通过后再执行 `nginx -s reload`；不要直接重建 Cloudflare Tunnel。
- 本次公网切换记录见 `docs/ai/context/20260804-112415-aaccx-public-chain_CN.md`。
- 2026-08-04：为发布当前源码，重建并替换 `sub2api-official-18082` 应用容器；旧应用容器已停止，`sub2api-dev` 保持退出状态，18080 无监听。发布核验见 `docs/ai/context/20260804-112927-aaccx-public-deploy-result_CN.md`。
- 2026-08-04：修复公网 `/login` 白屏。Vite 公共依赖分包由 `vendor-*` 改为 `lib-*`，规避 Cloudflare 对 `vendor-*` 静态资源路径的 403；只重建并替换 18082 应用容器，数据库和 Redis 未重建。入口脚本、Vue、i18n、CSS 资源均已通过公网 200 校验，浏览器页面正常渲染。记录见 `docs/ai/context/20260804-114424-login-blank-fix_CN.md`。
- 2026-08-05：基于 `main` 最新提交重建并替换 `sub2api-official-18082` 应用容器，仅更新应用镜像，PostgreSQL、Redis 和数据卷未重建；本地、Nginx 与三个公网健康检查均返回 200。记录见 `docs/ai/context/20260805-114632-18082-production-rebuild-hide-image-guide_CN.md`。

## 流量卡 10% 迁移上下文

- 2026-08-04 已将流量包订单、用户流量额度和额度流水纳入迁移；管理员 `xiaobianfuai@gmail.com` 的全部流量卡关联数据强制纳入抽样。
- 目标流量包目录保持 18082 的 28 天配置，历史额度保留源库实际过期时间；`reserved_usd` 字段在目标缺失时由迁移事务补齐。
- 10% 迁移与逐条核验记录见 `docs/ai/context/20260804-100101-traffic-pack-10pct-migration_CN.md`。
- 源库专有的 `billing_authorizations`、`billing_authorization_traffic_credit_items`、`traffic_credit_exhaustion_events` 因目标无对应 schema 暂未迁移，不能将本次结果表述为这些表的完整迁移。

## 流量卡购买与额度展示上下文

- 2026-08-04：`/purchase` 只负责购买流程，流量卡套餐由 `traffic_packs` 服务端数据驱动，当前默认三档为 2 元/5 USD、3 元/10 USD、5 元/20 USD，均为到账后 28 天有效。
- 支付订单使用 `traffic_pack` 类型；创建订单时保存套餐快照和支付服务商实例，支付完成后幂等写入 `user_traffic_credits`，并在 `traffic_credit_ledger` 留存购买、扣费、退款流水。
- 模型计费优先扣普通余额；余额不足且请求平台为 OpenAI 时，按最早到期、最早到账顺序原子扣除流量卡。流量卡额度不并入 `users.balance`，不足以覆盖本次请求时沿原余额不足拒绝链路处理。
- 订单退款复用服务端报价，按 `Max(已用时间比例, 已用额度比例)` 计算可退金额，退款成功只撤销当前订单对应的流量卡剩余额度；网关失败保留 `REFUND_FAILED` 供重试。
- 顶部用户余额组件旁新增独立的“流量卡额度”展示，移动端用户菜单同步展示；数据来自登录用户的 `/payment/checkout-info`，购买支付成功后通过 `traffic-credit-updated` 事件刷新。普通余额和流量卡额度禁止合并展示。
- 实现与验证记录见 `docs/ai/context/20260804-094456-traffic-pack-final-verification_CN.md`。
- 2026-08-04：使用管理员 `xiaobianfuai@gmail.com` 的 API Key 做最小真实请求时，因普通余额为 0，API Key 认证预检直接返回 `INSUFFICIENT_BALANCE`，未进入上游和流量卡结算；管理员仍有 40 USD 有效 OpenAI 流量卡。入口缺口与实测证据见 `docs/ai/context/20260804-121657-admin-traffic-card-api-test_CN.md`。
- 2026-08-04：已修复上述入口缺口：余额小于等于 0 时，OpenAI API Key 认证预检自动检查有效流量卡；流量卡无额度或结算无法覆盖时拒绝并禁止余额透支。管理员真实请求已从流量卡扣减 0.0043335000 USD，完整实现与验证见 `docs/ai/context/20260804-125014-traffic-card-auto-switch-implementation_CN.md`。
- 2026-08-04：购买页的余额套餐和流量卡必须复用 `frontend/src/components/payment/PurchaseProductCard.vue`，流量卡只负责映射为同一 `PurchaseProductCardModel`，禁止新增平行卡片样式；统一卡片调整时两类商品应同步更新。具体变更见 `docs/ai/context/20260804-111916-traffic-pack-card-reuse_CN.md`。
- 2026-08-04：购买页余额套餐和流量卡容器统一复用 `catalogGridClass`，按两组商品数量较大值共享列数、间距和最小行高；流量卡保持独立区块并在余额套餐之后单独成行。记录见 `docs/ai/context/20260804-120018-traffic-pack-layout_CN.md`。

## 注册与 SMTP 测试上下文

- 2026-08-03：当前 `18082` 实例的公开注册通过数据库 `settings.registration_enabled` 控制；本次任务按管理员请求开启。
- SMTP 测试复用已有管理端发送测试邮件接口，收件人为 `xiaobianfuai@gmail.com`，不在仓库或输出中暴露 SMTP 密码。
- 2026-08-04：数据清空会移除 `settings.registration_enabled`，服务随后按安全默认值关闭注册；已在清空后的 `18082` 实例重新持久化为 `true`，公开设置接口已验证。

## 充值手续费配置上下文

- 2026-08-03：当前 `18082` 实例通过数据库 `settings.RECHARGE_FEE_RATE` 配置充值手续费，已按管理员要求设为 `1%`（数据库值为 `1`）。

## 模型最终计费倍率上下文

- 2026-08-03：当前 `18082` 实例通过 `deploy/docker-compose.18082.yml` 的 `BILLING_FINAL_MULTIPLIER` 配置模型最终计费倍率，已按管理员要求设为 `15x`。
- 2026-08-04：恢复数据后重新写入 7 个模型分组的 `groups.rate_multiplier`（Grok 0.6、Claude Max 1.5、Claude Kiro 0.35、GLM 1.4、Kimi 3.5、DeepSeek 3.0、GPT 0.15）；该分组倍率与服务端最终扣费 `15x` 分开计算，记录见 `docs/ai/context/20260804-111918-group-rate-multipliers-reapply_CN.md`。
- 2026-08-04：使用用户 2799523972@qq.com 的真实 GPT 请求验证 `0.001928 × 0.15 × 15 = 0.004338`，余额变化与 `usage_logs.actual_cost` 一致，15x 已真实生效；记录见 `docs/ai/context/20260804-112617-final-billing-multiplier-15x-live-verification_CN.md`。
- 2026-08-04：用户 1850226892@qq.com 的 Kimi K3 请求 `usage_logs.id=287142` 实际扣费 `2.815407`，计算为 `0.0536268 × 3.5 × 15`；若 Kimi 业务预期只按分组倍率扣费，则属于全局 `15x` 配置口径导致的过扣，核查记录见 `docs/ai/context/20260804-124052-user-1850226892-kimi-k3-billing-audit_CN.md`。
- 2026-08-04：进一步从 Kimi 中转站 `/v1/sub2api/billing` 实时确认该 Key 的 `resolved_rate_multiplier=3.5`。本次请求按 token 推算上游原始价为 `0.1876938`，本地 `15x` 后为 `2.815407`；是否去掉本地 `3.5x` 取决于 15 倍基准是“上游中转价”还是“官方基础价”，记录见 `docs/ai/context/20260804-124828-kimi-k3-upstream-price-and-multiplier-analysis_CN.md`。
- 2026-08-04：通过 `api.ai-genesis.app/usage` 浏览器实测确认 K3 汇总“标准 `$0.429`、实际 `$1.50`”约为 `3.5x`；具体 `138` 输入、`52` 输出的记录费用 `$0.004179 = 0.001194 × 3.5`。本地 `2.815407` 与该上游实际价口径再乘 `15x` 一致，不能用页面小 token 记录直接对比长上下文请求；记录见 `docs/ai/context/20260804-125352-browser-kimi-k3-upstream-usage-comparison_CN.md`。
- 2026-08-04：用户截图确认上游页面“费用”字段已经是 `3.5x` 后的最终费用，业务口径应为“上游最终费用 × 15”。本地 `total_cost` 只是标准成本，当前代码通过 `total_cost × 3.5` 重建上游费用后再乘 `15`，记录见 `docs/ai/context/20260804-125541-kimi-k3-upstream-final-fee-confirmation_CN.md`。
- 2026-08-04：进一步核对上游 GPT 汇总“标准 `$272.0701`、实际 `$40.8105`”为 `0.15x`；本地 GPT 记录 `0.001928 × 0.15 × 15 = 0.004338` 与“上游最终费用 × 15”一致。曾短暂将分组倍率统一改为 `1` 做验证，确认会破坏 GPT/Kimi 的上游价格口径，随后已恢复 `0.6/1.5/0.35/1.4/3.5/3.0/0.15`；详见 `docs/ai/context/20260804-130822-gpt-kimi-cross-check-and-rate-restore_CN.md`。
- 2026-08-04：计费链路可追溯本地用户/API Key/上游账号/请求 ID/模型/token/费用快照，但不能把每笔本地请求一一映射到上游 `/usage` 页面具体账单行；`/v1/sub2api/billing` 只返回 Key 级倍率元数据，逐笔费用由本地标准价格重算。详见 `docs/ai/context/20260804-131437-billing-traceability-explanation_CN.md`。
- 2026-08-04：按中转站单条明细和官方单价重新核对后，确认 GLM 与 DeepSeek 的中转有效倍率均为 `3.5x`；将 18082 的 GLM `1.4x`、DeepSeek `3.0x` 修正为 `3.5x`，保留其它已核对分组倍率和最终 `15x`。新公式为“标准成本 × 中转倍率 = 中转站实际费用；再 ×15 扣本地余额”，记录见 `docs/ai/context/20260804-140327-upstream-pricing-calibration_CN.md`。
- 2026-08-04：DeepSeek 分组 `ID 8` 的用户展示名称从“官方0.42折价格”修正为“官方0.5折价格”；仅改名称，平台、状态和 `3.5x` 计费倍率不变。记录见 `docs/ai/context/20260804-201035-deepseek-group-display-name-correction_CN.md`。
- 2026-08-04：浏览器复核 Claude Kiro 的 `claude-fable-5`：`36/1` token 按 `$10/$50` 和 `0.35x` 得 `$0.0001435`，页面显示 `$0.000143`；本地最终余额扣费为 `$0.0021525`（再乘 `15x`），记录见 `docs/ai/context/20260804-142941-kiro-fable-price-verification_CN.md`。
- 2026-08-04：浏览器复核 Kimi K3 的 `135/33` token：按 `$3/$15` 和 `3.5x` 得 `$0.003150`，与中转站页面完全一致；本地最终余额扣费为 `$0.047250`（再乘 `15x`），记录见 `docs/ai/context/20260804-143527-kimi-k3-price-verification_CN.md`。
- 2026-08-04：浏览器复核 GLM-5.1 的 `62/128` token：按 `$1.40/$4.40` 和 `3.5x` 得 `$0.002275`，与中转站页面完全一致；本地最终余额扣费为 `$0.034125`（再乘 `15x`），记录见 `docs/ai/context/20260804-143650-glm-price-verification_CN.md`。
- 2026-08-04：浏览器复核 DeepSeek V4 Flash 的 `54/22` token：按 `$0.14/$0.28` 和 `3.5x` 得 `$0.00004802`，页面显示 `$0.000048`；本地最终余额扣费为 `$0.00072030`（再乘 `15x`），记录见 `docs/ai/context/20260804-144129-deepseek-price-verification_CN.md`。
- 2026-08-04：浏览器复核 Claude Max 的 `claude-opus-4-5-20251101`（`79/3`，约 `1.9K` 缓存读取）：按 `$5/$25/$0.50` 和 `1.5x` 得约 `$0.002149`，与中转站页面一致；本地最终余额扣费约 `$0.032235`（再乘 `15x`），记录见 `docs/ai/context/20260804-144648-claude-max-price-verification_CN.md`。
- 2026-08-04：浏览器复核 GPT `gpt-5.6-luna` 的 `596/5` token（约 `3.8K` 缓存读取）：按 `$0.20/$1.20/$0.02` 和 `0.15x` 得 `$0.00003033`，页面显示 `$0.000030`；本地最终余额扣费约 `$0.00045495`（再乘 `15x`），记录见 `docs/ai/context/20260804-144844-gpt-luna-price-verification_CN.md`。
- 2026-08-04：最终复核 Grok `grok-4.20-0309-reasoning` 的 `108/238/128` token：按 `$1.25/$2.50/$0.20` 和 `0.6x` 得 `$0.00045336`，页面显示 `$0.000453`；本地最终余额扣费约 `$0.00680040`（再乘 `15x`），记录见 `docs/ai/context/20260804-145232-grok-price-verification_CN.md`。
- 2026-08-04：新增公开标准 OpenAI 分组“GPT模型官方0.35倍稳定”，上游 `/v1/sub2api/billing` 已确认有效倍率为 `0.35`；用户 API Key 页面从活跃、非专属的标准分组自动读取，因而无需新增前端选择逻辑。该账号复用既有 GPT 模型映射，分组倍率参与用户扣费，账号统计倍率 `3.5` 沿用现有渠道口径。密钥仅存于生产账号凭证，不写入仓库或上下文。记录见 `docs/ai/context/20260804-175807-gpt-035-stable-channel_CN.md`。
- 2026-08-04：使用 GPT 0.35 稳定渠道的真实记录 `usage_logs.id=288990` 复核 `gpt-5.6-sol`（`853/5/3584` token）：标准成本 `$0.006207`，上游 `0.35x` 后为 `$0.00217245`（页面显示 `$0.002172`），本地最终实际扣费为 `$0.03258675 = 0.006207 × 0.35 × 15`；账号统计倍率 `3.5` 未参与用户扣费。记录见 `docs/ai/context/20260804-183526-gpt-035-stable-channel-billing-verification_CN.md`。
- 2026-08-04：新增公开标准 OpenAI 分组“GPT0模型官方0.1倍低价不稳定”（`0.1x`）和“GPT-Image-2生图”（`1x`）。低价渠道只暴露上游确认可用的 5 个文本/Codex 模型；上游虽同时列出图像模型，但为防止图像请求按 `0.1x` 结算而绕过独立生图渠道，已从低价渠道映射中排除。生图渠道仅映射 `gpt-image-2` 并开启图像生成权限。账号统计倍率分别为 `1` 与 `10`，不参与用户扣费。记录见 `docs/ai/context/20260804-185647-gpt-low-price-and-image2-channels_CN.md`。
- 2026-08-05：用户确认 Kimi 中转站截图中的“用户扣费”已是其上游用户价格层，本平台 Kimi 不得再叠加 `3.5x`。生产 `groups.id=7` 已从 `3.5x` 改为 `1.0x`，后续用户扣费固定为“标准成本 × 最终 15x”；账号 `35x` 统计倍率未动。此前 5 条 Kimi 用量累计多扣 `$12.1738275`，补偿须经单独授权并以可审计余额流水执行。记录见 `docs/ai/context/20260805-093842-kimi-user-billing-multiplier-correction_CN.md`。

## 18082 数据清空上下文

- 2026-08-03：按管理员要求清空 `18082` 实例 PostgreSQL 的全部业务表数据，包含用户、管理员、设置、套餐、分组、支付渠道和使用记录；保留 `schema_migrations` 与 `atlas_schema_revisions` 以维持数据库结构版本。
- 同步清空该实例 Redis 键和应用日志；保留 `data-18082` 中的连接配置、安装锁和模型定价缓存，避免启动时自动重建管理员或破坏运行配置。
- 清空后业务数据核验：用户 `0`、管理员 `0`、余额套餐 `0`、分组 `0`、支付渠道 `0`、支付订单 `0`、用量记录 `0`；应用健康检查通过。
- 应用运行中的运维组件会自动维护少量运行时设置和缓存键，这些不属于用户/支付业务数据，服务启动后可能重新出现。

## 邀请返利上下文

- 邀请返利默认开启，默认比例为 8%，返利产生时直接增加邀请人 `users.balance`，并在 `user_affiliate_ledger.frozen_until` 记录 24 小时冻结截止时间。
- 冻结返利不影响模型扣费：余额扣费事务会在普通余额不足时同步扣减 `user_affiliates.aff_frozen_quota`，冻结期只限制冻结状态，不限制模型使用。
- 旧版尚未手动转入余额的 `aff_quota` / `aff_frozen_quota` 由 `backend/migrations/197_affiliate_auto_balance_rebate.sql` 一次性归集，迁移标记防止重复入账。
- 设计与验证记录见 `docs/ai/context/20260803-223829-affiliate-auto-balance-rebate_CN.md`。
- 2026-08-03：18082 实例已重建并执行 197 迁移；数据库实测 `affiliate_enabled=true`、`affiliate_rebate_rate=8`、`affiliate_rebate_freeze_hours=24`，应用健康检查通过。

## `/subscriptions` 余额套餐展示上下文

- 2026-08-04：购买页余额套餐支付成功后写入 `user_balance_packages`，原 `/subscriptions` 仅读取 `user_subscriptions` 会导致已购 ¥29、¥39、¥49、¥299 等套餐不展示。
- 新增认证接口 `/api/v1/payment/balance-packages` 返回当前用户套餐的服务端状态、价格、额度和到账进度；`/subscriptions` 同时兼容余额套餐与历史模型订阅。
- 余额套餐标准周期固定为 28 天，每 7 天到账一次，共 4 次；服务端和订阅页只返回当前用户未退款、未过期的最新有效套餐，已退款、已失效历史记录保留审计但不展示。
- 每个用户最多一个有效余额套餐：购买同一档套餐视为续费，只把当前套餐 `expires_at` 延长 28 天，不新增刷新次数或重置到账进度；购买其他档位由服务端返回 `BALANCE_PACKAGE_ACTIVE`，要求先完成退款。创建订单和履约均在 PostgreSQL 用户行锁内再次校验，防止并发绕过限制。
- 199 号迁移已在 18082 执行，统一历史套餐生命周期并清理重复有效记录；生产库核验为 114 条记录、62 条当前有效记录、当前有效记录异常刷新/周期 0 条、重复有效用户 0 个。
- 后续若要让同一套餐的每笔续费订单都独立支持自动退款，需要为续费订单增加套餐关联审计/映射；当前套餐记录的 `payment_order_id` 绑定最新续费订单，退款闭环以当前订单为准。
- 设计记录见 `docs/ai/context/20260804-120632-subscriptions-balance-packages_CN.md`；本轮实现与验证见 `docs/ai/context/20260804-151715-balance-package-lifecycle-rules_CN.md`；基础接口验证见 `docs/ai/context/20260804-121614-subscriptions-balance-packages-verification_CN.md`。
- 2026-08-04：余额套餐改为按 7 天窗口刷新而非追加；`user_balance_packages.remaining_usd` 仅表示本周套餐未用额度。刷新只调整 `weekly_credit_usd - old_remaining_usd`，到期只清除该字段，普通充值、返利和 18080 迁移承接的非套餐余额不被清理。200 号迁移已按旧周期审计和窗口实际用量回收可识别的错误结转，并重建当前窗口额度；`/subscriptions` 显示“本周剩余额度”和“下次刷新”。生产核验、公式与测试结果见 `docs/ai/context/20260804-182800-balance-package-weekly-refresh-production_CN.md`。
- 2026-08-04：用户 `1850226892@qq.com` 的订单 `537` 因容器退款代理拒绝连接而保持 `REFUND_FAILED`；按管理员要求已人工取消其余额套餐权益、将普通余额归零并保留订单与失败审计，未误标退款成功，也未动其独立流量卡额度。记录见 `docs/ai/context/20260804-203510-user-1850226892-manual-package-cancellation_CN.md`。
- 2026-08-05：用户 `3415991811@qq.com` 的 29 元余额套餐（订单 `149`）因退款网关失败保持 `REFUND_FAILED`；按管理员要求仅人工取消当前套餐权益，将 `user_balance_packages.id=29` 标记为 `cancelled` 并清零剩余额度，普通余额 `0.45314656 USD` 和独立流量卡额度均未改动。记录见 `docs/ai/context/20260805-102620-user-3415991811-manual-package-cancellation_CN.md`。
- 2026-08-05：在上述套餐取消后，按管理员追加要求将用户 `3415991811@qq.com` 的普通余额从 `0.45314656 USD` 归零；套餐、退款失败订单和独立流量卡额度未改变，并写入 `BALANCE_MANUAL_RESET` 审计。记录见 `docs/ai/context/20260805-103207-user-3415991811-balance-reset_CN.md`。

## 使用方法页面迁移上下文

- 2026-08-04：旧 18080 项目的 `/usage-guide` 已完整迁移到当前项目，保留七个教程栏目、13 张步骤截图、CCSwitch 本地视频与封面、API 表格和代码示例。
- 使用方法页面必须通过认证用户路由访问，并复用用户侧栏声明；页面字体继承当前全局字体，颜色、卡片、表格和深色模式按当前项目设计系统维护。
- 迁移与验证记录见 `docs/ai/context/20260804-150708-usage-guide-migration_CN.md`。
- 2026-08-05：使用方法页面新增“Claude Code 桌面端接入”主题，复用既有步骤图片组件，新增 3 步和 6 张截图；实现记录见 `docs/ai/context/20260805-111702-claude-code-desktop-usage-guide_CN.md`。
- 2026-08-05：使用方法页面暂时隐藏“生图方法”主题，保留源码数据和接口文档，不影响生图能力；记录见 `docs/ai/context/20260805-113427-hide-image-generation-guide_CN.md`。
- 2026-08-05：重写 `/usage-guide` 的“规范使用”栏目，使其与当前网关真实路由一致：`/v1` 是 OpenAI/Claude 推荐 Base URL，Gemini 原生入口为 `/v1beta`；Responses、Chat Completions、Embeddings、图片、Models 和 Codex 直连保留部分无 `/v1` 兼容别名，但 `/messages`、`/usage` 等裸路径不应省略版本前缀。所有可见使用方法栏目新增 `updatedAt`，按更新时间从新到旧排序；详见 `docs/ai/context/20260805-153528-usage-guide-formal-api-and-date-sort_CN.md`。
- 2026-08-05：更新“错误编号参考”栏目：当前 `main` 仍以通用 `{code,message,reason,metadata}` 和 OpenAI/Anthropic/Gemini 协议错误格式为准，未统一输出 `X-Sub2API-Error-ID` / `S2A-*`；页面改列当前 API Key、订阅、余额、限流和上游常见代码，并明确 S2A 目录不能作为当前稳定响应契约。记录见 `docs/ai/context/20260805-160411-usage-guide-error-catalog-refresh_CN.md`。

## GPT 渠道监控上下文

- 2026-08-04：用户侧 `/monitor` 已加入 GPT 0.35 稳定、GPT 0.1 低价和 GPT-Image-2 三个渠道；前两者通过当前服务按 60 秒持续检测，首次均为 `operational`。
- GPT 0.35 最新一次检测曾因响应 7512ms 超过 6 秒阈值显示 `degraded`，这代表慢响应，不是认证、路由或模型不可用。
- GPT-Image-2 不得用聊天请求或周期性生图探测。当前线上版本只完成一次已认证的 `/v1/models` 校验并展示该事实状态，未重启或重建公网 `18082`；后续持续监控必须先发布无费用的模型目录探测协议。完整记录见 `docs/ai/context/20260804-191548-three-gpt-channel-monitor-status_CN.md`。
