# Dashboard 额度展示、shadow 与图片计费回填排查计划

## 背景

用户反馈本地前端仍看不到当前使用量/额度，且历史图片请求存在未计费或计费错误。数据库中可能存在 shadow 观测计费来源，需确认它是否导致扣费缺失或面板不展示。

## 边界

- 只处理本地开发环境：`sub2api-dev`、`sub2api-postgres-dev`、`sub2api-redis-dev`、`cliproxyapi-local-dev`。
- 不触碰公网、生产数据库、生产 Redis、Nginx、Cloudflare 或 CPA 真实账号池。
- 任何写入本地数据库前先备份并记录回滚边界。
- 不记录完整 API Key、内部 token、支付密钥或 SMTP 密码。

## 排查顺序

1. 核对前端 Dashboard 实际调用的 quota/stats 接口与字段名，确认是否是响应字段、缓存或渲染条件问题。
2. 核对后端 quota 读模型：订阅权益周期、每日额度、周期用量、今日用量分别从哪些表计算。
3. 查询本地数据库：
   - active 订阅与 `subscription_entitlement_periods` 是否匹配；
   - `usage_logs.billing_type` 中套餐、流量卡、余额、shadow 的分布；
   - 图片请求的 token、成本、`billing_incomplete`、`actual_cost` 分布；
   - `usage_facts.billing_status` 与 payload 中可否重建缺失日志。
4. 通过本地 API 验证 Dashboard 返回值，定位前端不展示是数据为 0/NULL、接口失败，还是字段映射不一致。
5. 如需回填：
   - 优先回填缺失的订阅权益周期；
   - 再按现有 pricing 口径重算本地 `usage_logs` 的图片成本；
   - 必要时从 settled `usage_facts` 回填缺失 `usage_logs`；
   - 刷新或重建 Dashboard 聚合/缓存。

## 初步判断

`traffic_credit_reservation_shadow` 是流量卡预授权 shadow 模式：用于观测，不应实际扣减余额或流量卡。若真实用户请求长期落入 shadow，面板不增长是预期结果，但这也说明本地或当时运行态没有开启真实 reservation/debt gate，需要从配置、billing source 与 usage log 的 `billing_type=3` 分布确认影响范围。
