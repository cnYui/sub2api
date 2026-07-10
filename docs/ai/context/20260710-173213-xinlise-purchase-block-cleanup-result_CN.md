# xinlise@gmail.com 退款失败后购买阻塞清理结果

时间：2026-07-10 17:32 JST / 16:32 +08

## 结论

- `xinlise@gmail.com` 是 `users.id=69/status=active`。
- 用户仍无法购买的根因是：手动取消订阅时只写入了 `user_subscriptions.id=88.deleted_at`，但 `status` 仍为 `active`；当前购买保护/活跃订阅查询按 `status='active' AND expires_at > now()` 识别，未统一显式排除软删除记录，因此这条已软删除订阅仍可能被识别为 active。
- 已把 `user_subscriptions.id=88` 从 `active` 改为 `expired`，保留用户手动写入的 `deleted_at=2026-07-10 16:27:50.204049+08`。
- 复核后该用户购买保护命中数为 0，应可重新购买订阅。

## 写库前备份

- 备份文件：`deploy/backups/20260710-172913-sub2api-candidate-before-xinlise-subscription-cancel-cleanup.dump`
- 大小：47MB
- 已用容器内 `pg_restore -l` 验证可读。

## 写库操作

仅更新一行：

```sql
update user_subscriptions
set status = 'expired', updated_at = now()
where id = 88 and user_id = 69 and status = 'active';
```

返回 `UPDATE 1`。

## 复核

- `user_subscriptions.id=88` 当前：
  - `status=expired`
  - `deleted_at=2026-07-10 16:27:50.204049+08`
  - `expires_at=2026-08-07 03:57:35.501097+08`
- 购买保护等价查询：
  - `status='active' AND expires_at > now()`：0
  - `status='active' AND expires_at > now() AND deleted_at IS NULL`：0
- `user_allowed_groups`：0 行。
- API Key：
  - `api_keys.id=99/codex`，active，`group_id=NULL`
  - `api_keys.id=102/佳一老师`，active，`group_id=NULL`
- 订单状态：
  - 仅有 `payment_orders.id=140/status=REFUND_FAILED/order_type=subscription`
  - 没有 pending 待支付订单。
- Redis：
  - `DEL billing:sub:69:8` 返回 0。
  - `billing:*69*` 无相关键。
- 健康检查：`http://127.0.0.1:18084/health` 返回 200。

## 后续注意

- 订单 `payment_orders.id=140` 仍是 `REFUND_FAILED`，失败原因为 `easypay refund failed (HTTP 200): 卖家余额不足`；本次未改退款状态、未执行退款。
- 如果后续从管理员退款接口继续处理该订单，应仍按上一轮核算的 `89.10` 元口径。
- 代码层面长期建议：`ListActiveByUserID` / `GetActiveByUserIDAndGroupID` 应显式过滤 `deleted_at IS NULL`，避免软删除 active 记录再次被识别为活跃订阅。

## 本轮影响

- 已取消该用户当前所属套餐：`subscription_id=88/group_id=8/codex-pool-89-usd`。
- 未删除 usage_logs，未改 API Key，未改订单退款状态。
- 未重启容器、未发布代码。
