# luzhiyuan2026@163.com 账号禁用结果

## 变更时间

- 数据库时间：`2026-07-24 17:05:44 +08`。

## 已执行

- 将 `users.id=35 / luzhiyuan2026@163.com` 从 `active` 改为 `disabled`。
- 将该用户未删除 API Key `api_keys.id=41 / codex_used` 从 `active` 改为 `inactive`。
- 删除 Redis L2 认证缓存 `apikey:auth:49d596b0e86c5bfc58808ac5e505df1ca0a65aea30574e9e87704b120880b960`，实际删除数为 `0`，表示当时 L2 未命中或已不存在。
- 向 `auth:cache:invalidate` 发布 L1 失效消息，返回订阅者数 `1`。

## 验证

- 使用该用户 Key 请求本地外层 `http://127.0.0.1:18080/v1/models` 返回 `401`。
- 响应体为 `{"code":"API_KEY_DISABLED","message":"API key is disabled"}`。
- 备份文件：`backups/20260724-180154-luzhiyuan-disable-account-prechange.sql`。

## 回滚

- 按备份文件执行即可恢复用户与 Key 的原状态。
- 本次未修改历史 usage、套餐、流量卡、reservation 或账本记录。
