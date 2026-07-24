# /purchase 新增 249/299 元订阅套餐结果

## 结论

- 已在分支 `codex/purchase-249-299-plans` 完成 249 元与 299 元公共 Codex 订阅套餐接入。
- 周限额按 29 元 `76 USD/周` 到 199 元 `520 USD/周` 的线性曲线外推：
  - 249 元：`651 USD/周`，28 天周期总额度 `2604 USD`
  - 299 元：`781 USD/周`，28 天周期总额度 `3124 USD`
- 购买和退款复用现有订阅订单、权益快照、手续费与退款 quote 流程，没有新增独立状态机。

## 代码改动

- 新增 migration `backend/migrations/178_seed_codex_249_299_subscription_plans.sql`：
  - `codex-pool-651-usd` / `249 元订阅池` / `price=249.00` / `weekly_limit_usd=651`
  - `codex-pool-781-usd` / `299 元订阅池` / `price=299.00` / `weekly_limit_usd=781`
  - 只 seed `groups` 和 `subscription_plans`，不修改历史订单、历史权益段和 usage facts，不绑定上游账号。
- 后端公共 Codex 周额度映射和 Dashboard 当前套餐周额度查询加入两档。
- 前端 `/purchase` 套餐额度映射和展示名加入两档。
- 修复前端 1% 手续费计算的浮点误差，避免 `249 * 1%` 被错误抬成 `2.50`，保持应付 `251.49`。

## 测试

- `backend`: `go test ./...` 通过。
- `frontend`: `pnpm typecheck` 通过。
- `frontend`: `pnpm lint:check` 通过。
- `frontend`: `pnpm test:run` 通过；输出存在既有的测试用例 console error、Vue stub warning 和 Browserslist 提示，退出码为 0。
- `frontend`: `pnpm build` 通过；输出存在既有 Vite dynamic import/chunk 警告、Browserslist 过期提示和 Node `DEP0190` 提示，退出码为 0。

## 运行态

- 未执行 Docker、Compose、Nginx 或公网数据库操作。
- 未重启任何服务。
- 未触碰公网当前 `18080` 与 `18086` 端口对应运行态。
