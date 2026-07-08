# cnfoxian@gmail.com step-3.7-flash 来源追踪补充

## 结论

- `step-3.7-flash` 不是当前 Sub2API 的 group model routing 生成的。
- `codex-pool-19-usd` 的 `model_routing_enabled=false`，`model_routing={}`。
- `traffic-pack-openai` 的 `model_routing_enabled=false`，`model_routing={}`。
- 当前唯一 OpenAI 上游账号 `cliproxy-local-openai` 没有配置 `credentials.model_mapping` 或 `extra.model_mapping`。
- 当前公网 `channels` 表无 active channel，也没有 channel model mapping 参与。
- `ops_system_logs.extra.model` 与日志 `model` 都是 `step-3.7-flash`，错误处理代码在没有显式映射模型时会从请求体 `model` 字段提取模型名。

## 含义

- 该模型名大概率来自用户客户端请求体的 `model` 字段。
- 当前链路把它作为 OpenAI 请求发给唯一上游账号 `account_id=1`，上游返回 `502`。
- failover 时没有第二个可切账号，最终报 `openai.account_select_failed: no available accounts`。
- 这不是流量卡扣费失败；此前已经证明同一用户在 `gpt-5.4/gpt-5.4-mini/gpt-5.5` 上能自动使用流量卡。

## 后续方向

- 如果 `step-3.7-flash` 不应开放给用户，需要在模型列表、模型白名单或请求准入层阻断，并返回明确错误。
- 如果希望支持该别名，需要配置明确 model mapping，把 `step-3.7-flash` 映射到真实可用上游模型，或增加支持该模型的上游账号。
