# 模型广场侧栏与隐藏最终倍率展示

## 决策

- 左侧用户导航和管理员“我的账户”导航新增“模型广场”，入口使用 `/model-plaza?embedded=1`，从而保留后台顶部栏和左侧栏。
- 模型广场的“实付价格”只展示隐藏最终倍率前的价格：模型基础价 × 分组倍率（或登录用户专属倍率）。`BILLING_FINAL_MULTIPLIER=15x` 只在服务端最终扣费使用，不进入接口展示字段，也不在前端重复相乘。

## 实现

- `frontend/src/components/layout/AppSidebar.vue`：新增受 `FeatureFlags.modelPlaza` 控制的侧栏项，并支持带 query 的路由目标。
- `backend/internal/service/channel_plaza.go`：模型来源兼容活跃渠道和账号 `model_mapping`；缺少 LiteLLM 目录价格时复用 `BillingService.GetModelPricing` 的本地兜底/校准价格。
- `backend/internal/service/channel_service.go`、`backend/cmd/server/wire_gen.go`：注入账号仓储和计费服务，保持模型广场与实际计费口径一致。

## 发布核验

- 仅重建并替换 `sub2api-official-18082` 应用容器；PostgreSQL、Redis、Nginx、Cloudflare Tunnel 未重建。
- `http://127.0.0.1:18082/health` 返回 `200`，容器健康。
- `GET /api/v1/model-plaza` 返回 `10` 个分组、`81` 个模型，价格为空的模型数为 `0`；`glm-5.2` 使用 `$1.40/$4.40`，`kimi-k3` 使用 `$3.00/$15.00`（均为最终 `15x` 前口径）。
- 容器环境核验为 `BILLING_FINAL_MULTIPLIER=15`。
- 本地 Nginx `http://127.0.0.1:8080/health` 与公网 `https://aaccx.pw/health` 均返回 `200`；公网 `/api/v1/model-plaza` 同样返回 `10` 个分组、`81` 个模型。
- 内置浏览器可匿名打开模型广场并看到筛选、倍率和价格表；由于浏览器会话未登录，`embedded=1` 按设计降级为独立布局，登录后侧栏入口由前端单测覆盖。

## 测试

- `go test -tags unit ./internal/service -run 'TestListPlazaGroups' -count=1`
- `go test ./cmd/server -run '^$'`
- `pnpm exec vitest run src/components/layout/__tests__/AppSidebar.spec.ts src/components/modelPlaza/__tests__/PlazaModelPricingTable.spec.ts`
- `pnpm typecheck`
