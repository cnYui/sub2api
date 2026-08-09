# 项目协作约定

- 默认使用中文；代码注释只说明原因。
- 支付订单的金额、退款金额和订单状态以服务端为准，前端金额只用于展示。
- 退款必须绑定创建订单时的支付服务商实例，并保留可审计的订单状态变化。
- 设计与实现上下文写入 `docs/ai/context/`，历史文档只新增不覆盖。
- 2026-08-09：本地具名分支均已是 `main` 祖先，无待合并分支；账号恢复二进制/工具和 billing 临时 Compose 含本地敏感或过期发布状态，仅保留在工作区并由 Git/Docker 排除。其余未跟踪源码、迁移、测试和上下文文档纳入 `main`，整合边界见 `docs/ai/context/20260809-201602-main-consolidation-and-public-deploy-plan_CN.md`。
- 2026-08-09：`/keys` 的 API 端点复制入口固定展示 `https://api.aaccx.pw/v1`，不依赖公开设置中的 `api_base_url`；公开设置中的自定义端点仍作为补充线路展示。详见 `docs/ai/context/20260809-190459-keys-fixed-api-endpoint-copy_CN.md`。
- 模型广场价格换算：外部模型上游按 1 美元=1 人民币展示，国产模型按 1 美元=7 人民币展示；本地若采用 1 美元=10.5 人民币，外部模型页面价乘 10.5，国产模型页面价需先除以 7 再乘 10.5。`BALANCE_RECHARGE_MULTIPLIER` 是每支付 1 CNY 获得多少 USD，应为 `1/10.5`；不要把汇率直接写入模型扣费倍率。详见 `docs/ai/context/20260806-154500-model-pricing-exchange-rate-analysis_CN.md`。
- 2026-08-06：按隐藏最终倍率之外的口径核对本地与上游模型价格：Grok、Claude Max、GLM、Kimi、DeepSeek 分组倍率未对齐；GPT 三档、Kiro、GPT-Image-2 已对齐。当时运行中的 18082 实际 `BILLING_FINAL_MULTIPLIER=18`。GLM 缓存写入/读取基础单价也与上游不同。详见 `docs/ai/context/20260806-161037-upstream-local-pricing-multiplier-audit_CN.md`。
- 2026-08-06：按管理员要求完成价格对齐，生产 `BILLING_FINAL_MULTIPLIER` 恢复为 `15`，活跃分组倍率和名称已与上游现有对应分组对齐，GLM 5.1/5.2 缓存价固定为上游口径；部署与验证见 `docs/ai/context/20260806-162623-pricing-alignment-and-final-multiplier-15_CN.md`。
- 2026-08-06：模型广场已接入左侧用户/管理员侧栏，入口跳转 `/model-plaza?embedded=1`；展示价格只计算模型基础价 × 分组/用户倍率，刻意不展示或叠加隐藏的 `BILLING_FINAL_MULTIPLIER=15x`。模型目录同时复用计费服务的本地兜底价，避免 `glm-5.2`、`kimi-k3` 等目录缺项显示为空；发布与验证见 `docs/ai/context/20260806-171033-model-plaza-sidebar-and-pricing_CN.md`。
- 2026-08-07：Kimi 分组恢复为 `0.5x`，K3/K2.6/K2.5 的计费与模型广场统一使用校准美元基础价 `$3/$15/$0.30`、`$0.95/$4/$0.16`、`$0.60/$3/$0.10`（输入/输出/缓存读取，每百万 token）；广场不含最终 `15x`，依次显示 `$1.50/$7.50/$0.15`、`$0.475/$2.00/$0.08`、`$0.30/$1.50/$0.05`。详见 `docs/ai/context/20260807-000000-kimi-pricing-and-model-plaza-correction_CN.md`。
- 2026-08-07：GLM、DeepSeek 分组均设为 `0.5x`。GLM-5.1 按智谱国内官网的 `<32K` / `>=32K` 两档人民币价先除以 7 再打五折；GLM-5.2 同样按国内官网 ¥8/¥28/¥2 换算。DeepSeek V4 Flash/Pro 固定使用官网美元价后打五折。模型广场与实际计费复用同一基础价，广场不含最终 `15x`。详见 `docs/ai/context/20260807-095113-glm-deepseek-official-pricing-50pct_CN.md`。
- 2026-08-06：按管理员要求将 Claude Kiro 分组 `groups.id=5` 改为 `0.45X Claude - Kiro`、`rate_multiplier=0.45`，账号 `id=3` 名称同步为“Claude Kiro反代官方0.45倍价格”。模型广场、API Key 计费接口和公网展示均为 `0.45x`；真实扣费为基础成本 × `0.45` × 最终 `15`。本次上游账号 `/v1/sub2api/billing` 仍返回 `0.35`，该上游差异未由本地变更改写；详见 `docs/ai/context/20260806-172524-claude-kiro-045-rate-sync_CN.md`。
- 2026-08-06：按管理员要求撤销用户 `1032726009@qq.com`（ID `505`）的临时手动 `520 USD`。该笔额度自兑换码 `id=43` 使用后产生 `40.3352458125 USD` 用量；保留入账前 `0.4004884125 USD` 后，用户余额调整为 `-39.93475740 USD`，写入欠费账本和 `BALANCE_MANUAL_GRANT_REVOKED` 审计。当前余额套餐 `id=21`（订单 `163`）标记为 `cancelled`、剩余额度归零，订单保持 `COMPLETED` 以保留支付审计；有效套餐校验已无记录，用户可重新购买。详见 `docs/ai/context/20260806-195007-user-1032726009-manual-520-revoke-and-debt_CN.md`。

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
- 2026-08-06：基于本地 `main` 提交 `b0765e243` 重建 `deploy-sub2api` 并替换 `sub2api-official-18082` 应用容器；PostgreSQL、Redis、数据卷和 Cloudflare Tunnel 未重建。`127.0.0.1:18082`、本地 Nginx 与 `aaccx.pw` 三个公网域名健康检查均为 200，公网新版 Codex 使用方法 chunk 和 5 张截图均返回 200。记录见 `docs/ai/context/20260806-100809-18082-codex-guide-production-deploy_CN.md`。
- 2026-08-09：按管理员要求从当前工作区重建 `deploy-sub2api:latest` 并仅替换 `sub2api-official-18082`；新镜像 `sha256:99730d92b42caae2babafeb7d951cbfc46b15ab6e5284baf902a12a2b4ad5474`，应用容器 healthy。PostgreSQL、Redis、Nginx、Cloudflare Tunnel 与数据卷未重建；本地与三个公网健康检查均为 200。详见 `docs/ai/context/20260809-172730-public-docker-rebuild-and-replace_CN.md`。

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
- 2026-08-07：实测发现运行库曾缺失 `RECHARGE_FEE_RATE`，服务端因此等效按 0% 创建订单；已补写并回读为 `1`。余额套餐 `balance-29` 待支付订单 `605` 为 `amount=29.00`、`pay_amount=29.29`，流量卡 `gpt_traffic_5usd_2cny` 订单 `606` 为 `amount=2.00`、`pay_amount=2.02`，两笔 `fee_rate=1.0000` 且均已取消，未发生真实付款或额度到账。手续费仅增加实付金额，套餐和流量卡权益按标价履约；发布与完整核验见 `docs/ai/context/20260807-135549-payment-fee-live-verification-and-deploy_CN.md`。

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
- 2026-08-06：DeepSeek 分组 `ID 8` 当前展示名称为“DeepSeek模型官方0.7折价格”；生产库、实时名称快照表和 Redis 分组缓存均已核验，未修改计费倍率、模型映射或历史审计记录。记录见 `docs/ai/context/20260806-100616-deepseek-channel-name-verification_CN.md`。
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
- 2026-08-05：按同一业务口径复核全渠道后，确认所有上游加价分组均不得再叠加加价层：Claude Max `groups.id=4`、GLM `id=6`、Kimi `id=7`、DeepSeek `id=8` 已统一为 `1.0x`；Grok、Claude Kiro、GPT 折扣分组和 GPT-Image-2 保持各自折扣/标准倍率。账号统计倍率不变，历史多扣合计 `$34.355294625`，补偿须单独授权。记录见 `docs/ai/context/20260805-105146-all-channel-markup-multiplier-correction_CN.md`。
- 2026-08-05：对照上游中转站 `2026/08/05 11:30:20-21` 费用截图逐条复核 11 个模型：Kiro 与 GPT 0.15 的本地 15 倍前金额一致；Claude Max、GLM、Kimi 截图仍含上游 `1.5x/3.5x` 加价，而生产已统一为 `1x`，两者价格层不同；Kimi K2.5 另发现动态价格文件 `$2.25/M` 与代码 fallback `$3.00/M` 冲突，待单独定价决策。部分分组名称仍是历史折扣/加价文字，不能作为实时倍率依据。记录见 `docs/ai/context/20260805-114218-upstream-fee-screenshot-reconciliation_CN.md`。
- 2026-08-05：用户最新确认上游截图“费用”就是标准费用，本地必须按“截图费用 ×15”扣费。生产已将 Claude Max、GLM、Kimi、DeepSeek 分组倍率恢复为 `1.5x/3.5x/3.5x/3.5x`；Kimi K2.5 计费优先使用已核对的 `$0.60/$3.00/$0.098` fallback 单价，避免动态目录 `$2.25/M` 输出价导致本地 15 倍前偏低。记录见 `docs/ai/context/20260805-120042-screenshot-standard-fee-calibration_CN.md`。
- 2026-08-05：用户进一步确认 Kimi 上游“原始费用”不应再乘 `3.5x`。`xiaobianfuai@gmail.com` 最近 5 笔 Kimi 请求因倍率快照 `3.5x` 多扣 `$6.2844075`；生产 `groups.id=7` 已恢复为 `1.0x`，缓存失效完成，后续按 `标准成本 × 15` 扣费。历史记录和余额未改写，补偿须单独授权。记录见 `docs/ai/context/20260805-123642-xiaobianfuai-kimi-overcharge-correction_CN.md`。
- 2026-08-05：按管理员授权执行 Kimi K3 7 条历史 `3.5x` 多扣退款，按“实际扣费 - `total_cost × 15`”计算，合计 `$17.665965`，全部原路退回普通余额；写入 7 条 `BALANCE_MANUAL_REFUND` 审计并清理余额/API Key 缓存。执行记录见 `docs/ai/context/20260805-191805-kimi-k3-overcharge-refund-execution_CN.md`。
- 2026-08-05：按用户最新要求，生产所有活跃模型分组 `groups.rate_multiplier`（ID `3` 至 `12`）统一为 `1.0x`；`BILLING_FINAL_MULTIPLIER=15`、账号统计倍率、图片/视频独立倍率保持不变。168 条认证缓存失效事件已清空，用户专属倍率覆盖为 0，历史用量和余额未改写。记录见 `docs/ai/context/20260805-131242-all-group-rate-multipliers-reset-to-one_CN.md`。
- 2026-08-05：统一 1 倍率后，通过 `https://aaccx.pw/keys` 对 7 个活跃 API Key 各发起 1 次真实模型请求，全部 HTTP `200`；新记录均为 `rate_multiplier=1.0`，实际扣费均满足 `标准成本 × 15`。记录见 `docs/ai/context/20260805-131745-all-api-keys-post-reset-live-billing-verification_CN.md`。
- 2026-08-05：按用户追加要求恢复 GPT 三个折扣渠道倍率：GPT 0.15 为 `0.15x`、GPT 0.35 为 `0.35x`、GPT 0.1 为 `0.1x`；其它活跃分组保持 `1.0x`，最终倍率 `15x` 不变。153 条认证缓存失效事件已清空。记录见 `docs/ai/context/20260805-132816-gpt-three-channel-rate-restore_CN.md`。
- 2026-08-05：按用户要求将 Claude Kiro 分组 `groups.id=5` 恢复为 `0.35x`；最终倍率 `15x` 不变，5 条认证缓存失效事件已清空，历史用量与余额未修改。记录见 `docs/ai/context/20260805-133012-claude-kiro-rate-restore_CN.md`。
- 2026-08-05：按用户要求将 Grok 分组 `groups.id=3` 恢复为 `0.6x`；最终倍率 `15x` 不变，2 条认证缓存失效事件已清空，历史用量与余额未修改。记录见 `docs/ai/context/20260805-133157-grok-rate-restore_CN.md`。
- 2026-08-05：按管理员要求将 `18082` 实例的服务端最终计费倍率从 `15x` 调整为 `18x`；分组倍率、账户统计倍率、图片/视频独立倍率、历史用量和余额均未修改。配置与发布核验见 `docs/ai/context/20260805-181500-final-billing-multiplier-18_CN.md`。
- 2026-08-06：按管理员要求将 `18082` 实例的服务端最终计费倍率从 `18x` 恢复为 `15x`；分组倍率、账户统计倍率、图片/视频独立倍率、历史用量和余额均未修改。配置与发布核验见 `docs/ai/context/20260806-101213-final-billing-multiplier-15_CN.md`。
- 2026-08-07：按管理员要求将 `18082` 实例的服务端最终计费倍率恢复为 `18x`；仅替换 `sub2api-official-18082` 应用容器，PostgreSQL 与 Redis 未重建。本地和三个公网健康检查均为 200，模型广场展示价格不含该最终倍率。详见 `docs/ai/context/20260807-115049-final-billing-multiplier-18-deploy-result_CN.md`。
- 2026-08-08：按管理员要求将 `18082` 实例的服务端最终计费倍率调整为 `20x`。仅替换应用容器，PostgreSQL 与 Redis 未重建；发布期间应用和既有 Nginx 同时被外部正常停止，已在不修改 Nginx 或 Tunnel 配置的前提下恢复 Nginx 与 Cloudflared。应用容器环境变量为 `20`，本地与三个公网健康检查均为 200；模型广场展示价格不含最终倍率。详见 `docs/ai/context/20260808-100204-final-billing-multiplier-20-deploy-result_CN.md`。
- 2026-08-08：按管理员要求将 `18082` 实例的服务端最终计费倍率恢复为 `18x`；仅替换应用容器，PostgreSQL 与 Redis 未重建，凭证文件挂载保持存在。应用容器环境变量为 `18`，本地与三个公网健康检查均为 200；模型广场展示价格不含最终倍率。详见 `docs/ai/context/20260808-142102-final-billing-multiplier-18-deploy-result_CN.md`。
- 2026-08-09：按管理员要求将 `18082` 实例的隐藏最终计费倍率调整为 `16x`；仅替换 `sub2api-official-18082` 应用容器，PostgreSQL、Redis、Nginx 和 Cloudflare Tunnel 未重建或修改。应用运行时环境变量为 `16`，本地与三个公网健康检查均为 200；模型广场展示价格不含最终倍率，历史用量与余额未改写。详见 `docs/ai/context/20260809-092852-final-billing-multiplier-16-deploy-result_CN.md`。
- 2026-08-09：按管理员要求基于当前工作区重新构建 `deploy-sub2api:latest` 并替换 `sub2api-official-18082`，将隐藏最终倍率 `16x` 更新到公网应用镜像。PostgreSQL、Redis、Nginx 和 Cloudflare Tunnel 未重建或修改；新应用容器为 healthy，运行态倍率为 `16`，本地与三个公网健康检查均为 200。详见 `docs/ai/context/20260809-100232-final-billing-multiplier-16-public-rebuild_CN.md`。
- 2026-08-05：按管理员授权，对最终复核保留的 85 条单笔超过 `$3` 的普通余额日志执行整笔退款，合计 `$389.0187845250`，涉及 11 位用户；全部原路退回普通余额，写入 85 条 `BALANCE_MANUAL_REFUND` 审计并清理余额/API Key 缓存。执行记录见 `docs/ai/context/20260805-203905-over-3-balance-refund-execution_CN.md`。
- 2026-08-06：只读审计最近一小时用量确认 673 条记录全部满足 `actual_cost = total_cost × rate_multiplier × 15`，无空费用、负费用、基础成本非零未扣费或最终倍率不匹配。模型广场只展示“基础单价 × 生效倍率”，不含最终 `15x`；15 条 Claude Kiro 记录保留调价前 `0.35x` 快照，其余当前记录按页面倍率一致。Kiro 仅由账号 3 承接，持续使用者为 `1032726009@qq.com`（近 24 小时 38 次、近一小时 18 次）。记录见 `docs/ai/context/20260806-180013-recent-usage-billing-and-kiro-audit_CN.md`。

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
- 2026-08-05：余额套餐首期到账和周刷新统一先抵扣用户负余额，周额度不足时继续保持负数，只有偿还后的剩余部分进入 `user_balance_packages.remaining_usd`。新增 `balance_debt_ledger` 保存不可变欠费/还款流水；新增 `billing_reconciliation_cases` 保存无法从历史响应恢复 token 的计费失败请求，金额留空等待外部逐笔账单，不得用次数或平均价伪造扣费。实现与生产核验见 `docs/ai/context/20260805-192000-billing-reconciliation-and-debt-first-credit_CN.md`。
- 2026-08-05：18082 的请求前资格检查统一要求普通余额或 OpenAI 流量卡至少为 `BILLING_MINIMUM_BALANCE_RESERVE=0.01 USD`；余额和流量卡均为碎额时直接 `403`，不进入上游、不产生用量。已放行请求仍可在结算后形成负余额，后续请求会被该预检阻断。线上复测见 `docs/ai/context/20260805-193133-billing-preflight-reserve-verification_CN.md`。
- 2026-08-05：核查用户 `1032726009@qq.com`（ID `505`）确认其当前为 ¥199 余额套餐（订单 `163`），4/4 次到账，2026-08-08 到期；当前窗口用量 `544.1321814000 USD` 超过每周 `520 USD`，故 `remaining_usd=0`。同日 `19:37` 执行的 36 条 `BALANCE_MANUAL_REFUND` 合计 `149.3559957750 USD` 已退回普通余额，不会回填套餐周额度；核查记录见 `docs/ai/context/20260805-213153-user-1032726009-subscription-balance-investigation_CN.md`。
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
- 2026-08-05：按用户提供的最新版截图更新“Codex 接入”主题为 4 步流程：创建或切换 GPT API Key、在 CC Switch 的 GPT 栏目新建凭证、填写 `https://api.aaccx.pw/v1` 并保存、启用凭证后重启 Codex；新增 5 张截图并复用既有步骤图片组件。记录见 `docs/ai/context/20260805-224935-codex-guide-latest-images_CN.md`。

## GPT 渠道监控上下文

- 2026-08-04：用户侧 `/monitor` 已加入 GPT 0.35 稳定、GPT 0.1 低价和 GPT-Image-2 三个渠道；前两者通过当前服务按 60 秒持续检测，首次均为 `operational`。
- GPT 0.35 最新一次检测曾因响应 7512ms 超过 6 秒阈值显示 `degraded`，这代表慢响应，不是认证、路由或模型不可用。
- GPT-Image-2 不得用聊天请求或周期性生图探测。当前线上版本只完成一次已认证的 `/v1/models` 校验并展示该事实状态，未重启或重建公网 `18082`；后续持续监控必须先发布无费用的模型目录探测协议。完整记录见 `docs/ai/context/20260804-191548-three-gpt-channel-monitor-status_CN.md`。
- 2026-08-07：补齐 `/monitor` 新增 GPT 渠道的模型集合：8 号 GPT 0.35 监控返回 7 个模型，9 号 GPT 0.1 监控返回 5 个文本/Codex 模型，10 号 GPT-Image-2 监控保持单模型；数据库去重、最近调度历史和 `18082/health` 均验证通过。认证详情接口因当前浏览器无登录会话未做 UI 点击验证，执行记录见 `docs/ai/context/20260807-104311-monitor-complete-channel-models-execution_CN.md`。
- 2026-08-09：按管理员要求将 `/monitor` 全部渠道统一改为低成本 `GET /v1/models` 目录探测；不再接受 `chat_completions` / `responses` 监控模式，不发送真实推理请求。每个监控一次请求共享全部模型目录，生产检测间隔统一为 1800 秒；实现与发布记录见 `docs/ai/context/20260809-104516-channel-monitor-models-probe_CN.md`。
- 2026-08-09：最终收紧监控运行链路为每个监控只发起一次带鉴权的 `GET /v1/models`，移除额外 HEAD 探测；空模式默认归一为 `models`，服务端缺省监控间隔回退改为 1800 秒，迁移同时把已有监控间隔统一为 1800 秒。历史 `ping_latency_ms` 字段保留兼容但新探测不再写入；详见 `docs/ai/context/20260809-112631-channel-monitor-models-only-final_CN.md`。
- 2026-08-09：生产发布时发现已应用的 207 迁移 checksum 保护，已恢复 207 原始内容并新增不可变 208 迁移更新已有监控间隔；最终 `sub2api-official-18082` healthy，本地、Nginx 和公网健康检查均为 200，12 条监控和 5 个模板已核验为目标配置。详见 `docs/ai/context/20260809-114442-channel-monitor-models-only-production_CN.md`。
- 2026-08-09：`/monitor` 用户卡片随目录探测协议调整为仅展示“模型目录延迟”，不再展示无数据来源的“对话延迟”或“端点 PING”；状态徽章和时间线继续将上游鉴权失败等真实 `error/failed` 结果显示为红色，不得以 UI 样式掩盖探测失败。详见 `docs/ai/context/20260809-133546-monitor-models-ui-alignment_CN.md`。
- 2026-08-09：目录探测展示调整已重新构建并仅替换 `sub2api-official-18082` 应用容器；PostgreSQL、Redis、Nginx 和 Cloudflare Tunnel 未重建，容器健康且本地、Nginx、公网健康检查均为 200。详见 `docs/ai/context/20260809-135007-monitor-models-ui-production_CN.md`。
- 2026-08-09：监控 `1-9` 的独立 API Key 快照已同步为账号 `1-6`、`1128-1130` 当前有效凭证；同步通过服务层重新加密，未暴露明文。每条仅执行一次认证 `GET /v1/models`，全部 `operational`，间隔保持 1800 秒；详见 `docs/ai/context/20260809-143815-channel-monitor-current-key-sync_CN.md`。

## Usage 双端对照与计费绕过审计

- 2026-08-05：已登录并对照 `api.ai-genesis.app/usage` 与 `aaccx.pw/admin/usage`。4 条同 Token 请求的左侧上游用户价约为右侧 `A` 的 `0.15x`；左侧费用除以 `0.15` 后与右侧悬浮窗原始 `total_cost` 按显示精度逐条相同，右侧绿色用户费用符合 `原始成本 × 0.15 × 18`。两侧时间相差约 2-3 秒，左侧页面存在展示延迟。未发现费用不一致或重复扣费。
- 生产 `18082` 未覆盖 `gateway.usage_record.*`，使用默认 `overflow_policy=sync`；队列满和进程停止均有同步兜底，最近日志未发现 `usage_record.task_dropped`。`count_tokens` 为明确非计费接口，不属于绕过。详细记录见 `docs/ai/context/20260805-222239-usage-cross-check-and-billing-bypass-audit_CN.md`。
- 2026-08-06：勘误此前基于“同模型、时间差正负 5 秒”的上游 Usage 对账。该方法不能把共享上游账号账单可靠归属到本地用户；`xunskyler@gmail.com` `$384.62055975` 和总计 `$652.17178575` 均撤回，不得用于补扣。相关失败事件缺少请求 ID、Token 和费用快照，所有 reconciliation case 继续保持待核对且金额为空。详见 `docs/ai/context/20260806-111430-upstream-usage-reconciliation-correction_CN.md`。

## 上游 Usage 未追回扣费对账

- 2026-08-06：用用户提供的上游 Usage Excel（97,500 行）与生产 `record_usage_failed` 事件对账。GPT 0.15 上游账号 `1128` 的 2,952 条余额不足后扣失败中，2,870 条可按模型与 +-3 秒窗口同上游账单一一匹配；按历史公式“上游原始 × 上游倍率 × 15”确认未追回 `$652.17178575`，涉及 10 位用户。另有 62 条 `$15.99961050` 仅在 3-5 秒窗口匹配，20 条无唯一候选，均待人工复核。未执行补扣或修改数据。明细、边界和安全执行要求见 `docs/ai/context/20260806-104500-upstream-usage-unrecovered-charge-reconciliation_CN.md`。

## 忘记密码与 SMTP

- 2026-08-06：`18082` 已启用邮箱验证与忘记密码，数据库设置为 `email_verify_enabled=true`、`password_reset_enabled=true`、`frontend_url=https://aaccx.pw`；重置链接使用既有 SMTP 异步队列投递。未发送测试邮件，接口与单元测试验证见 `docs/ai/context/20260806-165637-password-reset-enable_CN.md`。
- 2026-08-07：按管理员要求取消用户 `xunskyler@gmail.com`（ID `454`）的当前余额套餐（`user_balance_packages.id=4`，订单 `540`）；套餐标记为 `cancelled`、剩余额度归零并停止下一次刷新。订单继续保持 `REFUND_FAILED`，普通余额和 OpenAI 流量卡额度未改动；审计为 `payment_audit_logs.id=1398`。详见 `docs/ai/context/20260807-104755-user-xunskyler-balance-package-cancellation_CN.md`。
- 2026-08-07：购买页余额套餐与流量卡统一按 `RECHARGE_FEE_RATE` 展示实付价、标价和手续费。手续费只增加订单 `pay_amount`，不改变套餐到账额度或流量卡额度；服务端仍以商品服务端价格重新计算收费，禁止信任前端金额。详见 `docs/ai/context/20260807-124800-payment-fee-display-plan_CN.md`。
- 2026-08-07：用户 `3867878292@qq.com`（ID `599`）的 Codex remote compact v2 报错根因为上游返回 `compaction_summary`，客户端只识别 `compaction`，导致“expected exactly one compaction output item, got 0 from 2 output items”。已在 Responses SSE、SSE/JSON bridge 和 JSON 写回路径统一将该别名转换为 `compaction`，其它压缩字段保持不变；代码与验证见 `docs/ai/context/20260807-182146-codex-remote-compact-summary-alias-fix_CN.md`。当前仅完成工作区修改，尚未发布到 `18082`。
- 2026-08-07：已将包含 Codex remote compact 别名兼容修复的本地 `main` 重建为 `deploy-sub2api:latest` 并替换 `sub2api-official-18082` 应用容器。PostgreSQL、Redis、数据卷、Nginx 和 Cloudflare Tunnel 未重建；本地、Nginx 和三个公网健康检查均返回 200，最终计费倍率保持 `18x`。本地分支审计与发布记录见 `docs/ai/context/20260807-195829-local-branch-audit-and-18082-remote-compact-deploy_CN.md`。
- 2026-08-08：负余额准入改为以 PostgreSQL `users.balance` 为最终事实；余额 `< 0` 时订阅、流量卡、SimpleMode、Gemini、Live 和 WebSocket 均不能绕过，余额 `= 0` 仍可使用足额 OpenAI 流量卡。用量资金结算改为同步完成，标准模式缺失统一计费仓库时 fail-closed。余额套餐首周抵扣后仍欠费则进入明确的 `debt_paused`，后续周额度暂停并只能由管理员在余额非负后恢复；迁移号为 204。批量生图冻结新增套餐来源字段，迁移号为 205，套餐在冻结期间到期时不得转成永久普通余额。实现与验证见 `docs/ai/context/20260808-002400-negative-balance-guard-implementation_CN.md`。
- 2026-08-08：负余额硬拦截已发布到 `sub2api-official-18082` 并同步公网。仅替换应用容器，PostgreSQL、Redis、数据卷、Nginx 和 Cloudflare Tunnel 未重建；204/205 迁移已执行，4 个活动欠费套餐均进入 `debt_paused`，公网五个健康端点均返回 200。发布与线上核验见 `docs/ai/context/20260808-004200-negative-balance-guard-production-release_CN.md`。
- 2026-08-08：为使运行实例与最新本地 `main` 完全一致，基于提交 `dceca8676` 再次构建并替换 18082 应用容器；数据库、Redis、数据卷、Nginx 和 Cloudflare Tunnel 未重建。最终镜像与替换后拒绝探针核验见 `docs/ai/context/20260808-004236-negative-balance-guard-final-image-verification_CN.md`。
- 2026-08-08：按管理员要求硬删除 `xiaobianfuai@gmail.com` 的软删除旧用户 ID `461` 及其旧 API Key，保留有效用户 ID `448` 和历史系统安全审计。该有效用户的 GPT Key 认证与模型目录正常，但 GPT 分组唯一账号 `1128` 上游账户已失效：`gpt-5.6-luna` 返回上游 502，`gpt-5.6-terra` 返回“账号未激活”。需恢复上游账户或增加备用账号，不能通过改用户余额解决；详见 `docs/ai/context/20260808-094445-xiaobianfuai-duplicate-user-delete-and-gpt-upstream-diagnosis_CN.md`。
- 2026-08-08：按管理员要求停止当前公网服务。已停止 `sub2api-public-nginx-local`、`sub2api-official-18082` 和 Cloudflare Tunnel 进程；数据库与 Redis 保留运行，公网三个域名健康检查均返回 502。详见 `docs/ai/context/20260808-100452-public-service-stop_CN.md`。
- 2026-08-08：只读审计当前上游 API 配置：10 个 active 的 `openai/apikey` 账号凭证位于 PostgreSQL `accounts.credentials` JSONB，统一上游地址为 `https://api.ai-genesis.app`；Redis `sched:acc:*`/`sched:*` 保存含凭证的调度快照，轮换必须同时考虑缓存同步。生产入口保持停止，详见 `docs/ai/context/20260808-102128-upstream-api-config-locations_CN.md`。
- 2026-08-08：按管理员授权，从已登录的上游 Key 页面轮换 10 个本地 OpenAI API Key 账号（1-6、1128-1131）。通过数据库与 Redis `sched:acc:*` 快照逐条同步，最终 10 个 Key 指纹均不同且数据库/缓存一一对应；上游 Qwen 无本地对应渠道，未创建。轮换过程保持公网关闭，最终应用、Nginx 与 Cloudflare Tunnel 均已停止。详见 `docs/ai/context/20260808-104356-upstream-api-key-rotation_CN.md`。

## 上游账号凭证安全改造上下文

- 2026-08-08：在隔离分支 `codex/secure-account-credentials` 完成 Redis 调度快照去凭证、PostgreSQL `accounts.credentials` 服务端 AES-256-GCM 加密、HMAC 指纹 CAS/Ollama 查询、旧明文迁移命令和 Docker Secret 配置。生产迁移前必须停止应用、准备 `ACCOUNT_CREDENTIALS_ENCRYPTION_KEY_HOST_FILE`、先 dry-run 再 `-Apply`；本轮未启动公网服务、未执行生产迁移。实现见 `docs/ai/context/20260808-140000-account-credential-security-implementation_CN.md`，最终验证及服务层既有测试失败记录见 `docs/ai/context/20260808-125202-account-credential-security-final-verification_CN.md`。
- 2026-08-08：只读核查用户 `xinlise@gmail.com`（ID `504`）确认其账户 active、余额 `258 USD`、API Key 均 active；其 `codex`/`古月` Key 绑定的分组 `9` 唯一账号 `1128` 因上游 `401 API key is disabled` 进入 error 且不可调度，导致该分组无法调用。分组 `13` 账号 `1132` 当前可调度，建议迁移 Key 或恢复账号凭证；详见 `docs/ai/context/20260808-144229-xinlise-user-unavailability-diagnosis_CN.md`。
- 2026-08-08：按管理员授权，用户 `441565547@qq.com`（ID `472`）的余额套餐 `id=10`（订单 `541`）第二期 `76 USD` 已提前结算，用于优先抵扣 `0.30025848 USD` 欠费；余额与本周剩余额度均为 `75.69974152 USD`，套餐从 `debt_paused` 恢复为 `active` 的第 `2/4` 期，下次刷新为 `2026-08-15 13:01:56 +08:00`。订单保持 `COMPLETED` 且未退款；已记录欠费还款账本和订单审计。详见 `docs/ai/context/20260808-171014-user-441565547-early-second-credit-debt-offset_CN.md`。
- 2026-08-08：按管理员要求移除用户 `441565547@qq.com`（ID `472`）的余额套餐 `id=10`（订单 `541`），套餐标记为 `cancelled`、剩余额度清零并停止后续到账；普通余额从 `-0.95582240 USD` 清零为 `0 USD`。订单继续保持 `REFUND_FAILED`，退款金额 `12.43 CNY` 未改动；流量卡、历史用量和 API Key 未修改。已写入 `BALANCE_PACKAGE_MANUAL_CANCELLATION` 与 `BALANCE_MANUAL_RESET` 审计并清理余额缓存。详见 `docs/ai/context/20260808-215412-user-441565547-package-remove-and-balance-reset_CN.md`。
- 2026-08-09：按管理员要求取消用户 `380361319@qq.com`（ID `603`）的余额套餐 `id=128`（订单 `617`）；套餐标记为 `cancelled`、剩余额度归零并停止下一次刷新，普通余额 `-0.26459774 USD` 保持不变。订单继续保持 `REFUND_FAILED`，退款金额 `44.22 CNY` 未改动；审计为 `payment_audit_logs.id=1505`。详见 `docs/ai/context/20260809-084722-user-380361319-package-cancellation_CN.md`。
- 2026-08-09：购买页商品卡、支付方式、订单摘要和支付状态面板统一复用监控卡/模型广场的半透明形状：`rounded-2xl`、低对比度边框、`backdrop-blur` 与 `shadow-card`；原有中文文案、金额计算和支付流程保持不变。验证记录见 `docs/ai/context/20260809-113222-purchase-glass-ui-alignment_CN.md`。
- 2026-08-09：购买页商品目录改为 `auto-fit + minmax(280px, 1fr)` 自适应网格，标题、价格、明细和值列避免窄卡片逐字换行；删除仅用于视觉核对的 `/purchase-preview` 临时入口。验证记录见 `docs/ai/context/20260809-114656-purchase-card-responsive_CN.md`。
- 2026-08-09：购买页自适应改动已完成类型检查、购买卡单测、定向 ESLint、生产构建和 `git diff --check`，并在内置浏览器默认、窄桌面和等效手机视口完成核对。详见 `docs/ai/context/20260809-114942-purchase-card-responsive-verification_CN.md`。

## 原生 Gemini 与 Claude 渠道

- 2026-08-09：生产新增公开标准分组 `Gemini1倍率`（ID 70、`gemini`、`1.0x`）和 `Claude0.78倍率`（ID 71、`anthropic`、`0.78x`）；对应账号 ID 为 1165/1166，均 active 且仅绑定各自分组。用户 API Key 页面会自动显示 active、非专属的标准分组。两个 `/monitor` 监控 ID 为 13/14，均通过带鉴权 `GET /v1/models` 每 1800 秒刷新，首轮所有公开模型为 operational。凭证仅以服务端加密形式保存，详见 `docs/ai/context/20260809-170900-native-gemini-claude-channel-addition_CN.md`。
- 2026-08-09：基于当前工作区重建 `deploy-sub2api:latest` 并仅替换 `sub2api-official-18082`，新镜像为 `sha256:f2f422b244fe4f9a792ad71d08ac97a25eb09432d9e67ac873538c7a81c984f3`；PostgreSQL、Redis、Nginx、Cloudflare Tunnel 与数据卷未重建，应用容器 healthy，监控调度器加载 14 条任务，本地与三个公网健康检查均为 200。详见 `docs/ai/context/20260809-181925-gemini-claude-public-docker-rebuild_CN.md`。
- 2026-08-09：按管理员要求修正旧渠道平台类型：Grok0.9 分组/账号/监控 ID 3/1/1 改为 `grok`，Claude1.5 分组/账号/监控 ID 4/2/2 与 Claude0.45 分组/账号/监控 ID 5/3/3 改为 `anthropic`；同步清理 Kiro 监控一个不存在模型并刷新调度与认证缓存。三把对应 Key 的 `/v1/models` 均 200，监控配置模型全部 operational。详见 `docs/ai/context/20260809-185941-legacy-channel-platform-type-correction_CN.md`。
