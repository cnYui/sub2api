# 上游账号错误诊断

## 结论

当前页面的“14 启用、1 错误”中的错误账号是：

- 账号 ID：`6`
- 名称：`DeepSeek模型官方0.7折价格`
- 本地平台/类型：`openai` / `apikey`
- 上游地址：`https://api.ai-genesis.app`
- 本地绑定分组：ID `8`，`【国产】DeepSeek（5折）`
- 当前状态：`error`，`schedulable=false`

上游最近连续 3 次返回 HTTP `403`，错误码为 `GROUP_DELETED`，消息为“API Key 所属分组已删除”。这表示该 API Key 在上游的归属分组已经被删除或失效，不是本地 PostgreSQL、Redis、网络健康或余额问题。

## 证据

数据库只读核查时间：`2026-08-24 17:01:22 +08:00`。

- 最近 24 小时账号 6 的错误共 3 条，时间为 `10:31:18`、`10:41:18`、`10:58:21`。
- 三条均为本地 `502 upstream_error`，上游状态码均为 `403`，上游详情均为 `{"code":"GROUP_DELETED","message":"API Key 所属分组已删除"}`。
- 最后一条错误写入账号 `error_message`：`Access forbidden (403): API Key 所属分组已删除 | consecutive_403=3/3`。
- 当前所有未删除账号共 15 个，其中 `active + schedulable` 为 14 个，`error` 为 1 个，`schedulable=false` 为 1 个。
- 账号的上游计费探测也已失败，累计失败次数为 111，最近探测 HTTP 状态为 403。

源码中的 OpenAI 403 规则为：在 180 分钟窗口内累计达到 3 次时调用永久错误处理；第 1、2 次只进入 10 分钟临时不可调度。对应实现见 `backend/internal/service/ratelimit_service.go` 的 `handleOpenAI403`。

## 处理建议

1. 在上游服务商后台确认该 Key 对应的分组是否被删除；如果确认删除，创建新分组或申请新的 API Key。
2. 在账号 ID `6` 中替换为新的上游凭证，并确认上游模型权限和余额。
3. 替换凭证后再执行“测试连接/恢复状态”，确认返回成功后才重新启用调度。
4. 不建议在凭证未更换前直接重置状态；否则下一次请求仍会收到同样的 403，并再次被自动标记为错误。

本次仅做只读诊断，未修改账号、凭证、缓存或生产容器。
