# 29 元套餐用户 API Key 401 排查结果

## 范围

- 排查时间：2026-07-11 21:30 JST。
- 运行态：`sub2api-candidate`、`sub2api-candidate-postgres`、`sub2api-candidate-redis`。
- 本轮未修改业务代码、订单、订阅、API Key、Redis、nginx 或容器配置。

## 结论

今天模型入口确实存在 401，但不是 29 元套餐未生效，也不是套餐缓存导致 Key 失效。

- nginx 今天记录 77 次 OpenAI 模型入口 401：
  - 67 次响应体为 54 字节，对应 `{"code":"INVALID_API_KEY","message":"Invalid API key"}`，表示提交的 Key 在当前有效 Key 集合中不存在。
  - 10 次响应体为 143 字节，对应 `API_KEY_REQUIRED`，表示请求没有按要求携带 Authorization/x-api-key/x-goog-api-key。
- 401 在 `authenticateAPIKeyCore()` 的 Key 查找阶段返回，发生在套餐、额度和 effective group 判断之前。购买套餐不会恢复已删除、填错或未发送的 Key。

近期 29 元套餐订单中，最符合用户反馈的是 `405045701@qq.com`（`user_id=96`）：

- 订单 `payment_orders.id=156` 已于 2026-07-10 18:47:33 +08 完成，基础价 29 元、实付 29.29 元。
- 订阅 `user_subscriptions.id=100/group_id=2` 为 `active`，未删除，有效期至 2026-08-09 18:47:33 +08；今日额度窗口正常且用量为 0。
- 用户和当前 Key 均为 active。
- 第一把 Key `api_keys.id=124/name=Codex-used` 创建约 5 分钟后被用户删除，删除审计存在；该旧 Key 当前稳定返回 401/54 字节 `INVALID_API_KEY`。
- 后续两把 Key `id=125/name=Codex_used`、`id=126/name=codex 2.0` 均未删除。使用它们只读请求本机 `/v1/models` 均返回 200，证明鉴权、自动分组和套餐识别正常。
- 该用户至今没有任何成功模型 usage。诊断用 `/v1/models` 不产生 usage 或费用，但按正常鉴权行为更新了两把当前 Key 的 `last_used_at`。

另外两个 2026-07-10 完成的 29 元用户不是同类故障：

- `3415991811@qq.com/user_id=88`：购买后有 2 次成功 usage。
- `1902115052@qq.com/user_id=95`：购买后有 13 次成功 usage。

## 日志模式

- `8.219.150.152` 今天有 32 次 `/v1/responses` 401，并有 18 次 Gemini 入口 401；该 IP 在 `usage_logs` 中历史成功请求为 0，表现为某个外部客户端持续使用错误/旧 Key。
- `119.40.69.129` 的另一组 401 可关联到 `1032726009@qq.com/user_id=70` 的 Key 更换过程；其创建新 Key 后已恢复成功请求，不属于 29 元套餐问题。
- 当前 `ops_monitoring_enabled=false`，所以这些鉴权早退没有写入 `ops_error_logs`，无法从运行态日志证明 `8.219.150.152` 提交的明文 Key 一定就是 `user_id=96` 已删除的 `Codex-used`。但订单、订阅和当前 Key 的验证已排除服务端套餐失效与鉴权缓存问题。

## 根因判断

最可能根因是用户客户端仍配置了已删除的第一把 Key，或复制了掩码/不完整 Key；不是 29 元套餐发放失败。

用户侧应把客户端 Key 替换为后台当前存在的完整 Key，并确认请求头为 `Authorization: Bearer <完整 Key>`。不要继续使用已删除的 `Codex-used`，也不要使用列表页显示的掩码字符串。替换后重启或重新加载客户端配置即可。

## 后续观察项

- 若需要以后把 `INVALID_API_KEY` 精确归因到已删除 Key，可单独评估开启 Ops monitoring；开启前应确认错误日志留存和隐私策略。
- 本轮无需修改订单、订阅、Key 状态或缓存。
