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
- 设计与验证记录见 `docs/ai/context/20260804-120632-subscriptions-balance-packages_CN.md`。

## 使用方法页面迁移上下文

- 2026-08-04：旧 18080 项目的 `/usage-guide` 已完整迁移到当前项目，保留七个教程栏目、13 张步骤截图、CCSwitch 本地视频与封面、API 表格和代码示例。
- 使用方法页面必须通过认证用户路由访问，并复用用户侧栏声明；页面字体继承当前全局字体，颜色、卡片、表格和深色模式按当前项目设计系统维护。
- 迁移与验证记录见 `docs/ai/context/20260804-150708-usage-guide-migration_CN.md`。
