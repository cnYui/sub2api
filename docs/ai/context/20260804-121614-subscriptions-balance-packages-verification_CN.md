# `/subscriptions` 余额套餐展示验证

## 验证结果

- `frontend/pnpm typecheck`：通过。
- `frontend/pnpm vitest run src/api/__tests__/payment.spec.ts`：3/3 通过，包含 `/payment/balance-packages` 请求路径校验。
- `frontend/pnpm build`：通过，Vite 成功转换 971 个模块并生成 `SubscriptionsView` 产物。
- `backend/go test ./internal/service -run '^$' -count=0`：通过，仅编译服务包。
- `backend/go test ./internal/service -run 'TestBalancePackage' -count=1`：通过。
- `backend/go test ./internal/handler ./internal/server/routes -count=1`：通过。
- 后端服务包全量测试受仓库现有集成测试环境影响失败；handler/routes 仍通过，未发现本次新增接口的编译或路由错误。
- 前端全量测试存在 3 个既有失败：`HomeView.compact.spec.ts`、`admin.system.rollback.spec.ts` 两个用例及 GroupsView mock 未提供 `getLiveCapability`，与本次改动无关。
