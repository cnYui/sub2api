# 退款业务逻辑修复结果

## 结果

已在分支 `codex/fix-refund-business-logic` 完成退款业务逻辑修复。本轮只修改本地代码、迁移、测试和文档，未部署、未修改 PostgreSQL/Redis/容器/Nginx，也未导入或伪造旧消费记录。

## 退款口径

- 支付宝和余额支付统一使用 `payment_orders.amount` 作为退款基数，不退手续费；`pay_amount` 不再参与订阅退款金额计算。
- 使用固定 UTC+8 北京时间自然日：购买日算第 1 天，当天未用完也按已使用 1 天计算。
- 第 6 个自然日按已用 6 天、剩余 24 天计算；29 元 30 天套餐退款为 `29 * 24 / 30 = 23.2` 元。
- 退款金额首次申请时持久化，失败重试复用原金额和稳定请求号，不因跨日重试重新计算。

## 状态机与一致性

- `payment_orders` 新增准确 `subscription_id`、`refund_request_id`、网关状态、权益状态和 Provider 退款编号。
- 网关状态区分 `NOT_STARTED/PENDING/SUCCEEDED/FAILED/UNKNOWN`；只有明确确认未退款的 `FAILED` 可自动重试，`PENDING/UNKNOWN` 禁止自动重试。
- 权益状态区分 `NOT_STARTED/SUCCEEDED/FAILED/MANUAL_REVIEW`。网关已成功时，用户和管理员重试只处理权益，不再调用 Provider。
- 网关成功后的订阅锁定、独占关系和期限复验、撤权、订单最终状态及成功审计在同一事务完成。共享订阅或购买后被续期、赠送、人工调整时失败关闭为 `MANUAL_REVIEW`，不撤销整行权益。
- 余额退款在单个 Ent 事务中完成订单锁定、订阅重新加载和复验、余额增加、订阅撤销、订单收尾及审计；任一步失败全部回滚。
- 退款事务审计采用持有订单锁后的 create-if-absent，已有 `REFUND_REQUESTED` 或 `REFUND_SUCCESS` 不会因唯一约束阻断恢复。

## Provider

- 支付宝、微信、Stripe、Airwallex 使用稳定 `refund_request_id` 作为幂等请求标识。
- EasyPay 没有独立退款幂等键，每次只使用一个订单标识发起一次请求，优先 `out_trade_no`，不再因“订单不存在”立即改用 `trade_no` 发第二次请求。
- EasyPay 仅“卖家余额不足/商户余额不足”判定为明确未退款、允许重试；“已退款、处理中、重复请求、订单不存在”等响应进入未知状态并要求人工核验。

## 订单关联与迁移边界

- 新履约直接持久化订阅 ID；若历史履约已在订阅 notes 中存在精确 `payment order <id>`，只恢复关联，不重复续期。
- 迁移 `162_refund_state_machine.sql` 只回填当前 `payment_orders` 中能与现有订阅唯一匹配的记录，并创建 `subscription_id -> user_subscriptions.id` 外键和索引。
- 未读取、更新或导入旧 SQLite 的 `orders/subscription_orders`；老用户消费历史仍按用户要求留待后续单独处理。

## 前端

- 用户端和管理员端订单响应增加 `refund_retryable`。
- `REFUND_FAILED` 只有服务端明确允许时显示“重试退款”；处理中、未知结果、人工核验状态不显示重试入口。
- 增加共享权益、期限变化、关联缺失等中英文错误提示。

## TDD 证据

RED 阶段确认：

- `TestRequestRefundContinuesGatewaySucceededRefundingState` 返回 `INVALID_STATUS`。
- 网关收尾缺少事务 helper，最终写入失败无法证明撤权回滚。
- 余额退款事务内未复验已变化的订阅期限。
- EasyPay 模糊业务响应被统一包装为可重试失败，并会使用两个标识连续请求。
- 已有 `REFUND_REQUESTED/REFUND_SUCCESS` 审计时，退款事务触发唯一约束失败。

GREEN 阶段新增并通过对应测试，覆盖续接、事务回滚、人工核验、余额事务内复验、EasyPay 未知分类和审计恢复。

## 完整验证

- `go test -count=1 -tags=unit ./internal/service`：PASS，`87.742s`。
- `go test -count=1 ./internal/payment/provider ./internal/repository`：PASS。
- `go test -count=1 ./cmd/server ./internal/handler ./internal/handler/admin`：PASS。
- `go test -count=1 -tags=integration ./internal/repository -run TestMigrationsRunner_AuthIdentityAndPaymentSchemaStayAligned`：PASS，实际执行迁移 schema/FK 测试。
- 退款与履约目标 unit、Provider refund 目标测试：PASS。
- 前端退款、支付 UX、订单工具测试：3 files、17 tests 全部通过。
- `pnpm typecheck`：PASS。
- `pnpm build`：PASS；仅有项目既有 Browserslist、动态/静态导入和 chunk size 警告。
- `go test -count=1 -tags=embed ./internal/web`：PASS。
- `git diff --check`：PASS。
- 独立只读复审：无 Critical 或 Important。

## 剩余风险

- 尚未新增真实 PostgreSQL 并发集成测试，验证“退款锁定订阅时另一订单并发履约/人工延长”的实际阻塞顺序；当前代码按订单行和订阅行锁定，并在撤权前再次失败关闭复验。
- 运行库已有失败退款订单不会自动处理，发布后应逐单核验网关真实状态再决定恢复方式。
- 发布仍需先备份 PostgreSQL、执行迁移，再发布兼容新字段的应用；本轮未执行这些运行态操作。
