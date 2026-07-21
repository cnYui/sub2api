# 2026-07-21 周滚动订阅额度与 28 天周期实现结果

## 已完成

- 新增 `174_weekly_rolling_subscription_quota_schema.sql`：周窗口锚点、订单快照、退款依据、权益周期周额度字段、usage fact 权益归属和超额债务审计表。
- 新增唯一滚动周窗口计算器。公共 Codex 七个分组使用订阅锚点推进窗口，尾窗按实际天数/7 精确折算；预授权、窗口维护、缓存资格校验、订阅进度和 DTO 使用该结果。
- 新购公共 Codex 订阅建立 28 天、7 天窗口锚点；订单创建写入不可变套餐快照，支付履约从快照传递周额度、周期总额度和窗口规则。
- Redis 订阅缓存新增周锚点；旧缓存缺失锚点时仍兼容读取，cutover 后通过缓存失效重建。
- 订阅 API 返回 `weekly_anchor_at`、`weekly_window_resets_at`、`effective_weekly_limit_usd`、`weekly_remaining_usd`；顶部订阅进度优先显示后端有效周额度。
- 新增 `backend/migrations/tools/weekly-quota-cutover.sh`，默认只输出本地分类与预览；`--apply` 才会在事务中更新套餐、存量锚点、权益快照和唯一可归属的 usage facts。订阅 21、53 仅登记 `already_applied` 超额审计，禁止二次扣减。

## 验证

- `backend`: `go test ./internal/service ./internal/repository ./internal/handler/dto` 通过。
- `backend`: `go test ./migrations` 通过。
- `frontend`: `pnpm typecheck` 通过。
- `frontend`: `pnpm test:run` 通过。
- 未执行 cutover 脚本，未连接或修改本地/公网数据库、Redis、Nginx、Cloudflare 或 CLIProxyAPI。

## 后续门禁

- 运行前必须用本地附件库执行 cutover dry-run，人工处理“完成订单缺少订阅链接、缺权益段、重叠权益、无法唯一归属用量”清单。
- 自动退款 quote、按权益段 usage facts 汇总、退款 UI 与管理员强制退款审计仍需在该清单清零后接入；当前历史退款路径仍保留原有日数公式，不能直接用于周额度已切换的生产环境。
