# Usage 双端对照与计费绕过审计

## 核对范围

- 左侧用户 Usage：`https://api.ai-genesis.app/usage`
- 右侧管理端 Usage：`https://aaccx.pw/admin/usage`
- 两侧均已登录；按模型、输入/输出/缓存 Token、端点和时间配对，时间允许页面落后约 2-3 秒。

## 浏览器实测

再次抓到 4 条同一请求：

| Token（输入/输出/缓存） | 左侧上游用户价 | 右侧管理端 A（账号侧） | 右侧管理端本地用户费 | 时间偏移 |
| --- | ---: | ---: | ---: | --- |
| `3455 / 107 / 30.2K` | `$0.005338` | `$0.035589` | `$0.096090` | 左侧 `22:08:01`，右侧 `22:08:04` |
| `9971 / 568 / 192.9K` | `$0.024501` | `$0.163343` | `$0.441026` | 左侧 `22:07:54`，右侧 `22:07:56` |
| `982 / 63 / 30.2K` | `$0.003286` | `$0.021904` | `$0.059141` | 左侧 `22:07:35`，右侧 `22:07:37` |
| `1005 / 29 / 30.2K` | `$0.003150` | `$0.020999` | `$0.056697` | 左侧 `22:07:33`，右侧 `22:07:35` |

结论：

1. 左侧用户页费用与右侧 `A` 不相等，左侧约等于右侧 `A × 0.15`。这是上游 GPT 0.15 用户价，不是漏扣或重复扣费。
2. 将左侧费用除以 `0.15` 得到的标准原始成本，与右侧悬浮窗 `原始 = total_cost` 按页面显示精度逐条相同；当前这些 GPT 行的 `A` 账号侧值也等于该原始成本。
3. 右侧绿色用户费用满足 `原始成本 × 0.15 × 18`；生产最终倍率为 `18x`。因此当前费用字段是三层口径：标准原始成本、上游用户价、本站最终余额扣费。
4. 时间只相差约 2-3 秒。本次抓取左侧显示时间比右侧早 2-3 秒，符合用户所说的左侧页面有延迟；配对时应优先使用 Token、模型、端点和请求序列，不能用秒级时间单独判定缺失。

## 悬浮窗口径

`frontend/src/components/admin/usage/UsageTable.vue` 的费用悬浮窗明确显示：

- `原始`：`tooltipData.total_cost`
- `用户扣费`：`tooltipData.actual_cost`
- 管理端另显示账号侧费用：`account_stats_cost` / `account_rate_multiplier`

因此右侧 `A` 是账号侧计费值（当前 GPT 行等于原始成本），右侧绿色字段才是本地用户最终扣费；左侧用户页直接展示上游用户价。

## 是否存在直连绕过

- 正常 Responses、Chat Completions、Embeddings、图片、Gemini、Grok 等成功请求路径均提交 `RecordUsage` / `RecordUsageWithLongContext` 任务。
- `count_tokens` 是明确设计为“只校验资格并透传/估算，不产生 usage_logs、不扣费”的非计费接口，不属于绕过。
- 生产容器 `sub2api-official-18082` 未设置任何 `gateway.usage_record.*` 覆盖项，使用源码默认：128 个 worker、16384 队列、自动扩容、`overflow_policy=sync`。
- 队列满时 `sync` 会在提交方内联执行计费任务；进程停止时 handler 也有同步兜底。显式配置 `drop` 或 `sample` 才可能在队列溢出时丢弃计费任务，但当前生产没有启用。
- 线上日志最近窗口未发现 `usage_record.task_dropped`、`*_sync_fallback` 或相关丢弃事件。
- 针对 worker 池和提交兜底的定向 Go 测试已通过：`go test ./internal/service ./internal/handler -run 'UsageRecordWorkerPool|SubmitUsageRecord|CountTokens'`。

最终判断：本次对照未发现“跨过本地网关直接访问 API，导致不扣费、不记日志”的证据。当前残余风险仅是未来有人显式把 `gateway.usage_record.overflow_policy` 改为 `drop`/`sample`；生产应继续保持 `sync`，并监控 `usage_record.task_dropped`。
