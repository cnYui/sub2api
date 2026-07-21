# 额度削减、用量可见性与图片扣费修复计划

## 背景

- 用户要求在本地开发环境跑起 Sub2API 与 `D:\CodeWorkSpace\CLIProxyAPI-private`，并使用附件 `20260721-162852-sub2api-billing-error-runtime-snapshot.zip` 里的数据库/Redis 快照作为固定参考数据。
- 当前本地开发容器已经存在：`sub2api-dev:8080`、`cliproxyapi-local-dev:8317`、`sub2api-postgres-dev`、`sub2api-redis-dev`。
- 本轮只允许修改本地开发环境和代码，不触碰公网、Nginx、Cloudflare、生产容器或生产数据库。

## 必须解决的问题

1. 统一削减后的每日额度：公开前端、商店、用户展示页、管理员页与后端计费/套餐事实源必须一致。
2. 修复用户前端看不到当前使用量且出现报错的问题。
3. 修复图片请求无法按真实结果正确扣费的问题，避免只修预算或展示。

## 第一性拆解

- 额度必须有单一事实源。前端硬编码、商店文案、管理端表格和后端订阅发放如果各改一份，会再次产生展示与实际扣费不一致。
- 当前使用量是读模型问题，不能靠前端吞错解决。应先定位后端响应结构、鉴权、历史数据兼容或前端 normalizer 哪一层断裂。
- 图片扣费必须以成功响应中的可确认 usage/token 明细或 CLIProxyAPI usage event 为事实，不应再按请求体大小、图片张数或静态尺寸价格扣费。

## 执行计划

1. 本地快照恢复
   - 校验 zip 可列目录与 SHA256。
   - 对当前本地 Postgres custom dump、Redis RDB 和关键容器元数据做新备份。
   - 停止 `sub2api-dev`，恢复附件里的 Postgres dump 与 Redis RDB 到本地开发数据目录。
   - 重启 Sub2API 与 CLIProxyAPI，验证 `/health`、数据库连接、Redis 连接、共享网络 DNS/TLS。

2. 额度链路定位
   - 查询 `subscription_plans`、订阅权益周期、套餐 group、前端静态价格/额度文案、管理员套餐编辑页面。
   - 确认“规定的削减后额度”是否已经存在于快照数据；如果不存在，从代码和历史上下文找旧额度与目标映射，必要时只用迁移/seed 方式固化。
   - 优先让前端读取后端套餐/权益字段，删除或收敛重复硬编码额度。

3. 用量可见性定位
   - 用测试用户登录本地前端/API，复现 dashboard 当前使用量报错。
   - 沿 `/api/v1/usage/dashboard/stats`、`/api/v1/usage/dashboard/quota`、前端 API client 和组件 normalizer 追踪错误。
   - 先写失败测试覆盖当前报错响应，再修后端或前端契约。

4. 图片扣费定位
   - 读取 OpenAI 图片入口、usage fact 持久化、CLIProxyAPI usage event 插件与图片 usage 解析代码。
   - 用 fixture 复现图片成功响应包含 token 明细但未正确落账或落错来源的路径。
   - 先补失败测试，再让图片生成/编辑成功路径持久化唯一 usage fact，并按请求前授权来源结算。

5. 验证
   - 后端聚焦测试：额度、dashboard quota、OpenAI 图片计费、usage fact/outbox。
   - 前端聚焦测试：套餐/商店/用户/管理端额度展示与当前使用量组件。
   - 必要时运行 `pnpm typecheck`、`pnpm test:run`、`go test` 聚焦包、容器内健康检查。

## 风险与边界

- 不修改线上运行态，不推送远端。
- 不在文档或日志记录完整 API Key、内部 token、HMAC secret、SMTP 或支付密钥。
- 如果附件快照与本地迁移版本不匹配，先停止恢复并记录差异，不做破坏性迁移。
- 如果“削减后的额度”无法从快照或文档确认，需要向用户确认具体数值；不能凭空猜额度。
