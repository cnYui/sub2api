# xunskyler@gmail.com 请求阻断结果

## 目标

- 阻止 `xunskyler@gmail.com/users.id=19` 继续通过现有 API Key 请求模型。
- 按用户明确要求，仅将 `users.status` 设置为 `disabled`；不删除 API Key，也不修改两把 Key 自身的状态。
- 不重启整站，避免影响其他用户。

## 执行结果

- 写入前备份 PostgreSQL：`deploy/backups/20260714-202431-sub2api-candidate-before-disable-user-19.dump`。
- 备份大小 `74,074,988` 字节，权限 `600`，`pg_restore -l` 验证可读。
- 北京时间 `2026-07-14 19:25:16.759+08` 将 `users.id=19` 从 `active` 更新为 `disabled`。
- API Key 保持原记录：
  - `api_keys.id=9`：`status=active`，未删除。
  - `api_keys.id=137`：`status=active`，未删除。
- 已按两把 Key 的 SHA-256 缓存标识精确删除 Redis `apikey:auth:*` 鉴权缓存，并发布 `auth:cache:invalidate`，使应用进程 L1 鉴权缓存同步失效。

## 验证

- 使用两把 Key 分别请求本地公网应用 `/v1/models`，均返回 `HTTP 401 / USER_INACTIVE`。
- 禁用后目标来源继续尝试 `/v1/responses`：
  - `19:25:43+08`：HTTP 401。
  - `19:26:33+08`：HTTP 401。
  - `19:26:36+08`：HTTP 401。
- `19:26:05+08` 有一条 HTTP 200，这是禁用前已进入上游、耗时约 84 秒的在途流式请求；无法在不影响整站的前提下按单用户强制取消，完成后未再出现新的 HTTP 200。
- Redis `concurrency:api_key:9` 和 `concurrency:api_key:137` 当前活跃槽均为 0。
- 应用、PostgreSQL、Redis 容器保持 healthy，未重启任何组件。

## 回滚

- 如需恢复该用户，可将 `users.id=19.status` 改回 `active`，并再次精确失效该用户两把 Key 的鉴权缓存。
- 两把 API Key、套餐、流量卡和余额数据本次均未修改。
