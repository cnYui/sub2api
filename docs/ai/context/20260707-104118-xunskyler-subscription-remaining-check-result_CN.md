# xunskyler@gmail.com 订阅剩余时间只读核验

## 结论

- 公网事实源为 `sub2api-candidate-postgres`，对应当前 18084 公网候选链路。
- `xunskyler@gmail.com` 用户真实存在：`users.id=19`，`status=active`，未软删除。
- 当前存在 1 条未软删除 active 订阅：`user_subscriptions.id=21`。
- 订阅套餐为 `29 元订阅池`，绑定分组 `codex-pool-19-usd`，每日额度 19 USD。
- 订阅到期时间为 `2026-07-13 10:01:08.657+08`。
- 查询时数据库时间为 `2026-07-07 09:40:54+08` 左右，精确剩余约 `6.014` 天，即 `6 天 00:20` 左右。
- 如果前端/管理端显示“7 天”，这是按剩余天数向上取整后的展示值；按完整 24 小时天数计算并不是完整 7 天。

## 补充信息

- 订阅来源备注为 `migrated_from_yuiweb_legacy_key_manual_subscription_20260618 order_expires_at=2026-07-13 10:01:08.657+08`，属于历史迁移/手工订阅记录。
- 该用户还有 2 张 OpenAI/GPT 流量卡，合计剩余 `20.0000000000` USD，最晚到期 `2027-07-03 10:49:25.749511+08`。
- 该用户历史用量记录 2 条，累计 `0.0072320000` USD，最后一次用量时间 `2026-06-18 11:37:38.661711+08`。

## 本次操作

- 只执行 PostgreSQL 只读查询。
- 未修改数据库、Redis、容器、nginx 或公网链路。
- 未查看、记录或输出完整 API Key、内部 token、HMAC secret、SMTP 密码。
