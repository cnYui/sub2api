# Sub2API 上游账号并发调整结果

## 背景

此前排查确认：

- 四个 OpenAI 套餐分组都共用同一个 Sub2API 上游账号 `cliproxy-local-openai`。
- 该账号指向内网 CLIProxyAPI 聚合上游。
- 原 Sub2API 账号并发为 `3`，高峰时会出现本地排队，但主要错误仍来自 CLIProxyAPI/官方上游 overloaded、429 usage limit reached、auth unavailable。

用户要求将当前 Sub2API 并发修改为 `10`。

## 执行内容

只修改运行态数据库中的账号并发：

```sql
update accounts
set concurrency = 10, updated_at = now()
where id = 1 and name = 'cliproxy-local-openai'
returning id, name, concurrency, updated_at;
```

未修改：

- 用户并发。
- 分组绑定关系。
- CLIProxyAPI 配置。
- API Key、token、secret。
- Docker 容器、nginx、cloudflared。

## 验证

修改后只读核验：

| id | name | concurrency | status | schedulable | pool_mode |
|---|---|---:|---|---|---|
| 1 | `cliproxy-local-openai` | 10 | active | true | true |

Sub2API 日志显示修改后仍有 `/v1/responses` 请求正常返回 200。

## 注意事项

并发从 3 提到 10 会减少 Sub2API 本地账号槽排队，但也会让更多请求更快进入 CLIProxyAPI/官方上游。后续需要重点观察：

- `openai.account_wait_queue_full`
- `openai.account_slot_acquire_failed`
- `openai.forward_failed`
- `Our servers are currently overloaded`
- 上游 429 `usage_limit_reached`
- 流式 502 / missing terminal event

如果 overloaded/429/502 明显上升，应回调到 5 或拆分 CLIProxyAPI 内部账号池/套餐分组。
