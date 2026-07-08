# 单用户单生效订阅购买拦截实施计划

时间：2026-07-08 17:27 JST

## 目标

修复同一用户可购买多个生效订阅的 bug：已有 active 订阅时，新订阅订单创建必须失败，并提示 `需要先和管理员联系来进行退款`。

## 步骤

1. 后端 TDD：在支付服务单测中补订阅下单拦截测试。
   - 外部支付订阅：已有 active 订阅时 `CreateOrder()` 返回 `ACTIVE_SUBSCRIPTION_EXISTS`，且不创建 `payment_orders`。
   - 余额支付订阅：已有 active 订阅时 `BalancePayOrder()` 返回同一错误，且不扣余额、不创建订单。
   - 流量包或余额充值不纳入本次拦截测试。
2. 后端实现：
   - 新增统一错误构造，错误码 `ACTIVE_SUBSCRIPTION_EXISTS`，message `需要先和管理员联系来进行退款`。
   - 在 `validateSubOrder()` 读取用户 active 订阅并拦截。
   - 确保 `req.UserID == 0` 的旧测试/内部校验不误触发。
3. 前端 TDD：
   - 在购买页相关测试或轻量逻辑测试中覆盖 `ACTIVE_SUBSCRIPTION_EXISTS` 文案映射。
   - 如可低成本挂载 `PaymentView`，补点击订阅卡片时已有 active 订阅会提示且不进入确认页。
4. 前端实现：
   - 购买页点击订阅卡片前检查 active 订阅并弹提示。
   - `payment.errors` 中加入中英文错误码映射。
5. 验证：
   - 运行后端目标单测。
   - 运行前端目标 vitest。
   - 运行必要的类型检查或局部构建命令。
   - `git diff --check`。
6. 文档：
   - 新建结果文档。
   - 更新 `AGENTS.md` 高优先级记忆。
