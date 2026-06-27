# OpenAI `service_tier=fast` 计费验证结果

## 背景

需要确认用户请求 `gpt-5.5` 时，如果显式传 `service_tier: "fast"`，当前运行态是否真的会按 priority 价格计费。

## 测试方式

- 使用本机管理员自用 API Key（掩码 `sk-LOCAL-454...e28804`），避免消耗普通用户额度。
- 请求入口：`http://127.0.0.1:18080/v1/responses`
- 模型：`gpt-5.5`
- 普通请求：不传 `service_tier`
- Fast 请求：传 `service_tier: "fast"`
- 请求内容相同，`max_output_tokens=8`

## 结果

两次请求均返回 200。对应 usage 记录：

| usage_log.id | service_tier | input_tokens | output_tokens | cache_read_tokens | input_cost | output_cost | cache_read_cost | total_cost |
| --- | --- | ---: | ---: | ---: | ---: | ---: | ---: | ---: |
| `5897` | 空 | 523 | 5 | 4864 | 0.002615 | 0.000150 | 0.002432 | 0.005197 |
| `5898` | `priority` | 523 | 5 | 4864 | 0.005230 | 0.000300 | 0.004864 | 0.010394 |

## 定论

- 当前 `fast` 会被归一化为 `priority`。
- 在 token 数完全相同的情况下，`priority` 请求的输入、输出、缓存读取成本均为普通请求的 2 倍。
- 当前运行态后台没有 OpenAI fast policy 过滤规则，因此用户显式传 `service_tier: "fast"` 或 `"priority"` 时会按 priority 价格扣费。
