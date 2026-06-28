# /admin/promo-codes 注册优惠码逻辑梳理

## 结论

- `/admin/promo-codes` 管理的是注册优惠码，不是兑换码、充值码或支付折扣券。
- 管理员创建优惠码后，可复制 `/register?promo=CODE` 注册链接给新用户。
- 新用户注册时填写优惠码，注册成功后后端给该用户增加 `bonus_amount` 余额，并写入使用记录。
- 该优惠码只影响注册赠送余额，不会改变套餐价格、支付订单金额、订阅计划、流量卡余额或 API 计费倍率。

## 主要入口

- 前端管理页：`frontend/src/views/admin/PromoCodesView.vue`
- 管理 API：`/api/v1/admin/promo-codes`
- 公开校验 API：`POST /api/v1/auth/validate-promo-code`
- 注册 API：`POST /api/v1/auth/register`
- 数据表：`promo_codes`、`promo_code_usages`

## 后台管理能力

- `GET /api/v1/admin/promo-codes`：分页列表，支持 `status`、`search`、`sort_by`、`sort_order`。
- `POST /api/v1/admin/promo-codes`：创建优惠码。`code` 可为空，后端自动生成 16 位大写十六进制码。
- `PUT /api/v1/admin/promo-codes/:id`：编辑优惠码、赠送金额、最大使用次数、状态、过期时间和备注。
- `DELETE /api/v1/admin/promo-codes/:id`：硬删除优惠码；使用记录随外键级联删除。
- `GET /api/v1/admin/promo-codes/:id/usages`：查看该优惠码使用记录。

## 注册执行链路

1. 注册页加载公开设置，只有 `promo_code_enabled=true` 时展示优惠码输入框。
2. 如果访问 `/register?promo=CODE`，前端自动填入优惠码并调用 `/auth/validate-promo-code`。
3. 输入优惠码时，前端 500ms debounce 调校验接口。无效时前端阻止提交。
4. 邮箱注册成功创建用户后，`AuthService.RegisterWithVerification()` 才调用 `PromoService.ApplyPromoCode()`。
5. `ApplyPromoCode()` 开事务，`FOR UPDATE` 锁定优惠码行，重新校验状态。
6. 同事务内检查 `promo_code_usages` 是否已有同一用户同一优惠码记录。
7. 同事务内增加用户余额、创建使用记录、增加 `used_count`。
8. 提交后失效用户认证/余额相关缓存。

## 可用性规则

- 空优惠码：校验时不报错，注册时不处理。
- 状态必须是 `active`。
- `expires_at` 为空表示永不过期；过期后不可用。
- `max_uses=0` 表示不限次数；大于 0 时 `used_count >= max_uses` 后不可用。
- `promo_code_usages` 有唯一约束 `(promo_code_id, user_id)`，同一用户不能重复使用同一个优惠码。

## 注意点

- 注册服务中优惠码应用失败不会回滚注册，只记录日志；用户仍会注册成功。
- 余额发放调用通用 `UpdateBalance()`，该函数对正数金额也会增加 `total_recharged`，所以注册赠送余额会进入累计充值口径。
- 当前 shell 中 `docker` 不在 PATH，本次未查询运行态数据库；截图中的空表可由代码路径解释为 `promo_codes` 当前列表无数据。
