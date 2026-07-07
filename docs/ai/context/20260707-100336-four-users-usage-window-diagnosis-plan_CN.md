# 4 个用户订阅用量窗口只读排查计划

## 背景

用户截图中 4 个 active 订阅显示已有当日用量，其中部分行仍提示“14 小时 57 分钟后重置”。用户担心东八区 0 点刷新没有成功。

涉及邮箱：

- `897858381@qq.com`
- `3056163754@qq.com`
- `qixiaocheng777@gmail.com`
- `daleselaji@gmail.com`

## 只读排查范围

- 只查询当前公网候选数据库 `sub2api-candidate-postgres`。
- 核对 4 个用户的 active subscription、group、daily usage/window 字段。
- 汇总这些用户最近成功请求的 `usage_logs` 时间，按 UTC 与 Asia/Shanghai 展示。
- 检查 Redis `billing:sub:*` 缓存窗口是否与 DB 一致。
- 不修改 Postgres、Redis、容器或 nginx。

## 判断标准

- 如果 `daily_window_start` 已是 `2026-07-07 00:00:00 +08`，且今日成功用量聚合值接近页面显示值，则东八区 0 点刷新已生效。
- 如果窗口仍停在 `2026-07-06` 或更早，且页面用量包含昨天请求，则刷新失败。
- 如果 DB 正常但 Redis 缓存旧，则问题在缓存回源或缓存失效。
