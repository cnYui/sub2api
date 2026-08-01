# 18080 OpenAI 基础价格恢复与 5 倍倍率计划

## 目标

按用户明确授权，修改公网外层 `sub2api-dev:18080` 的挂载配置：

1. 将 `gpt-5.5`、`gpt-5.6-sol`、`gpt-5.6-terra`、`gpt-5.6-luna` 的基础价格恢复为 OpenAI 官方 Standard API 价格。
2. 将 `billing.unit_price_multiplier` 从 `2.5` 调整为 `5`。

本次只处理刚刚讨论的四个 GPT 模型，不重写其他供应商或其他模型的基础价格。全局倍率仍会作用于所有模型，这是用户明确要求的既有全局语义。

## 已核验事实

- 当前主链路为 `8080 Nginx -> 18080 sub2api-dev -> 18086 upstream`；`18080/health` 为 200，容器为 healthy。
- `sub2api-dev` 挂载宿主机 `deploy/data` 到容器 `/app/data`，运行时读取 `deploy/data/config.yaml` 与 `deploy/data/model_pricing.json`。
- 当前 `billing.unit_price_multiplier=2.5`。
- 数据库当前 `channels`、`channel_model_pricing`、`channel_pricing_intervals` 均无记录，基础价格不会被渠道配置覆盖。
- OpenAI 官方 Standard 短上下文价格（USD / 1M Token）：

| 模型 | 输入 | 缓存读取 | 缓存写入 | 输出 |
| --- | ---: | ---: | ---: | ---: |
| `gpt-5.5` | 5.00 | 0.50 | - | 30.00 |
| `gpt-5.6-sol` | 5.00 | 0.50 | 6.25 | 30.00 |
| `gpt-5.6-terra` | 2.00 | 0.20 | 2.50 | 12.00 |
| `gpt-5.6-luna` | 0.20 | 0.02 | 0.25 | 1.20 |

长上下文价格沿官方矩阵同步恢复：输入、缓存读取与缓存写入为短上下文 2 倍，输出为 1.5 倍。

## 执行与验证

1. 在 `deploy/backups/` 创建两份配置快照，并验证 JSON、YAML 可解析及 SHA-256 一致。
2. 仅修改四个模型的基础价字段和全局倍率。
3. 重启 `sub2api-dev`，不重启 PostgreSQL、Redis、Nginx、Cloudflare Tunnel 或内层 `18086`。
4. 验证容器健康、`18080/health`，并在容器内读取实际生效的倍率和价格文件关键字段。

## 回滚边界

本次没有数据库迁移或数据写入；回滚仅需把两份配置文件恢复为本次备份并重启 `sub2api-dev`。变更生效期间产生的 usage log 会保留原始价格快照，不会随配置回滚而重算。
