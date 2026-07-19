# OpenAI 计费来源顺序与图片预算修复结果

## 已完成

- 新请求的 OpenAI 预授权只会选择套餐或流量卡，账户余额已从授权输入、余额缓存读取和来源枚举中删除。
- 无有效套餐时直接预留流量卡；套餐预算能完整覆盖时仍优先套餐。
- `shadow` 成为显式观测来源，不会返回 402、改写请求或生成余额/套餐/流量卡扣费命令。
- 文本预算使用内嵌 `o200k_base` tokenizer；`b64_json`、`file_id`、`file_data` 和图片 data URL 不参与文本 Token 估算。
- 图片 multipart 预算只包含模型、prompt、尺寸、质量等语义字段，文件传输字节不参与预算，也不会改变转发给上游的原始 body。
- 新增流量卡 usage log 类型 `2`，管理端可筛选；迁移 `169_traffic_credit_billing_type.sql` 按扣款 ledger 回填历史误标为余额的日志。
- CLIProxy usage event 在缺少 Sub2API 预授权关联键时只返回 `skipped`，不再独立创建余额 usage fact。

## 验证

- 服务层全量 unit tests。
- 计费、图片和 usage fact 聚焦回归测试。
- 迁移运行器 `TestApplyMigrationsFS`。
- 前端 UsageFilters Vitest 和 `vue-tsc --noEmit`。

## 未部署边界

- 未修改任何运行中数据库、Redis、容器、Nginx 或公网链路。
- 套餐的并发 reservation 仍需单独实现。它必须同时处理日/周/月窗口、请求取消、上游未知结果和 usage fact 结算；仅在 `user_subscriptions` 增加单一 reserved 字段会在跨窗口场景出错，因此本次没有以不完整方案落库。
