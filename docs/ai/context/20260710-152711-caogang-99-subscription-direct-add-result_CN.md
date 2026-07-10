# caogang@sdufe.edu.cn 99 元套餐直加结果

## 结论

- 当前运行态数据库 `sub2api-candidate-postgres/sub2api` 中存在用户 `caogang@sdufe.edu.cn`：`users.id=89`，状态 `active`。
- 后台 `/admin/subscriptions` 查不到的根因不是用户不存在，而是该页展示和过滤的是 `user_subscriptions` 订阅记录；直加前该用户没有任何未删除 active 订阅。
- 已按用户要求绕过支付流程，直接给该用户新增 99 元订阅池。

## 浏览器复现

- 已用内置浏览器打开 `https://aaccx.pw/admin/subscriptions`。
- 首次使用小写 `w` 密码登录返回 `invalid email or password`。
- 用户随后确认密码可能为 `Wu15951875192`；使用大写 `W` 后登录成功并进入订阅管理页。
- 订阅页默认列表第一行已可见 `caogang@sdufe.edu.cn`，分组 `codex-pool-89-usd`，每日 `$0.00 / $89.00`，到期 `2026/08/09`，状态 `生效中`。
- 数据库侧确认管理员邮箱当前有一条未删除 admin 用户 `users.id=13`，另有一条同邮箱软删除普通用户 `users.id=26`；小写密码失败属于凭据不匹配，不是订阅页查找失败。

## 写库前备份

- 已备份 Postgres：`deploy/backups/20260710-152449-sub2api-candidate-before-caogang-99-subscription.dump`。
- 宿主机没有 `pg_restore`，已改用容器内 `pg_restore -l` 验证备份可读，TOC 读取成功。

## 写入结果

- 新增订阅：`user_subscriptions.id=95`
- 用户：`user_id=89 / caogang@sdufe.edu.cn`
- 分组：`group_id=8 / codex-pool-89-usd`
- 套餐来源：`subscription_plans.id=6 / 99 元订阅池 / 30 天`
- 额度：每日 `89 USD`
- 状态：`active`
- 生效时间：`2026-07-10 14:26:32.799089+08`
- 到期时间：`2026-08-09 14:26:32.799089+08`
- 分配人：`assigned_by=13 / xiaobianfuai@gmail.com`
- 初始用量：daily/weekly/monthly 均为 `0`

## 缓存核验

- Redis 精确订阅缓存 key `billing:sub:89:8` 写入后不存在，执行删除返回 `0`，无需清理。
- 直写库不会主动失效应用进程内 L1 订阅缓存；该用户直加前没有订阅且 key 不存在，实际风险低。若此前刚用该用户请求过 API 并命中过进程内空缓存，最多等待订阅 L1 TTL 自然过期。

## 未做事项

- 未创建支付订单。
- 未构建镜像、未重启容器、未改 nginx。
- 未修改后台搜索逻辑。
