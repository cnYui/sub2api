# `/available-channels` 实际计费价格展示执行计划

## 目标

让 `/available-channels` 顶部价格摘要独立于渠道表数据，按后端实际计费 resolver 展示 `gpt-5.4`、`gpt-5.5` 和生图价格。

## 文件

- 修改：`backend/internal/handler/available_channel_handler.go`
- 修改：`backend/internal/handler/available_channel_handler_test.go`
- 修改：`backend/internal/handler/handler.go`
- 修改：`backend/internal/handler/wire.go`
- 修改：`backend/cmd/server/wire_gen.go`
- 修改：`backend/internal/server/routes/user.go`
- 修改：`frontend/src/api/channels.ts`
- 修改：`frontend/src/views/user/AvailableChannelsView.vue`
- 修改：`frontend/src/views/user/__tests__/AvailableChannelsView.spec.ts`
- 修改：`AGENTS.md`

## 步骤

1. 后端先写失败测试：验证 `toUserModelPrice` 从 `ResolvedPricing` 中输出 `gpt-5.4` / `gpt-5.5` 所需字段，且不泄漏内部 channel id。
2. 前端先写失败测试：当 `getAvailable()` 返回空数组，但 `getPrices()` 返回 GPT 定价且 `getAvailableGroups()` 返回生图价格时，顶部摘要仍显示 `gpt-5.4`、`gpt-5.5` 和生图，渠道表仍为空。
3. 后端实现：
   - `AvailableChannelHandler` 增加 `pricingResolver` 依赖。
   - 新增 `ListPrices` handler 和 `GET /channels/prices` 路由。
   - 新增 DTO 转换函数，输出 token 单价和 priority 单价。
4. 前端实现：
   - `channels.ts` 增加价格 DTO 和 `getPrices()`。
   - `AvailableChannelsView.vue` 增加 `modelPrices` 状态。
   - `featuredPriceItems` 优先使用 `modelPrices`，失败时回退渠道内模型价格。
5. 验证：
   - `go test -tags unit ./backend/internal/handler -run 'TestUserModelPrice|TestUserAvailableChannel'`
   - `pnpm --dir frontend test:run src/views/user/__tests__/AvailableChannelsView.spec.ts src/components/layout/__tests__/AppSidebar.spec.ts`
   - `pnpm --dir frontend build`
6. 记录结果：
   - 新增 result 文档。
   - 追加 `AGENTS.md` 运行记录。
