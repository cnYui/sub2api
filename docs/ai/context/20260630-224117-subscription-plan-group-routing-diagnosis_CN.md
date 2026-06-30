# 订阅套餐额度与上游绑定排查结果

时间：2026-06-30 22:41 JST

## 目标

核对当前 29/39/49/79/99 额度对应的运行态套餐、group 和上游账号绑定，判断 `2864533153@qq.com` 当前 API Key 需要补哪个配置才能访问通。

## 公网 18084 当前状态

公网候选库 `sub2api-candidate-postgres` 当前在售订阅套餐为 4 个：

| 价格 | 套餐 | group | 日额度 | 上游绑定 |
| --- | --- | --- | --- | --- |
| 29 | `29 元订阅池` | `codex-pool-19-usd` | 19 USD/day | 已绑定 `cliproxy-local-openai` |
| 39 | `39 元订阅池` | `codex-pool-29-usd` | 29 USD/day | 已绑定 `cliproxy-local-openai` |
| 59 | `59 元订阅池` | `codex-pool-49-usd` | 49 USD/day | 已绑定 `cliproxy-local-openai` |
| 99 | `99 元订阅池` | `codex-pool-89-usd` | 89 USD/day | 未绑定上游账号 |

公网 18084 当前没有 `79 元订阅池`，也没有 `codex-pool-69-usd` group。

## 本地 main-preview 18080 当前状态

本地 main-preview 库 `sub2api-main-preview-postgres` 当前在售订阅套餐为 5 个：

| 价格 | 套餐 | group | 日额度 | 上游绑定 |
| --- | --- | --- | --- | --- |
| 29 | `29 元订阅池` | `codex-pool-19-usd` | 19 USD/day | 已绑定 `cliproxy-local-openai` |
| 39 | `39 元订阅池` | `codex-pool-29-usd` | 29 USD/day | 已绑定 `cliproxy-local-openai` |
| 59 | `59 元订阅池` | `codex-pool-49-usd` | 49 USD/day | 已绑定 `cliproxy-local-openai` |
| 79 | `79 元订阅池` | `codex-pool-69-usd` | 69 USD/day | 未绑定上游账号 |
| 99 | `99 元订阅池` | `codex-pool-89-usd` | 89 USD/day | 未绑定上游账号 |

## 对 2864533153@qq.com 的影响

该用户当前 active API Key 绑定的是公网 18084 的：

- `groups.id=8`
- `codex-pool-89-usd`
- 99 元套餐对应的 89 USD/day 额度

请求失败根因不是缺少 API Key、订阅或额度，而是 `codex-pool-89-usd` 没有绑定任何可调度上游账号；因此账号选择阶段返回 `no available accounts`。

## 结论

要让该用户当前 Key 访问通，需要补的是 `codex-pool-89-usd` 的上游账号绑定，而不是再添加一个价格额度。

如果目标是让完整 5 档都能公网可用，则需要两类运行态配置：

1. 将 79 元套餐及 `codex-pool-69-usd` 同步/迁移到公网 18084。
2. 给 `codex-pool-69-usd` 和 `codex-pool-89-usd` 都绑定可调度 OpenAI 上游账号，例如当前唯一可用上游 `cliproxy-local-openai`。

本轮只做只读排查，没有写入数据库、没有改源码。
