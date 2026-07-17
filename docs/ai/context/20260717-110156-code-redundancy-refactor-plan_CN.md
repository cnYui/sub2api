# 代码冗余治理与可靠计费重构计划

## 背景

当前主分支经过多轮功能迭代，存在以下需要集中治理的问题：

- 通用 Gateway 仍通过可能丢任务的内存队列记录用量，OpenAI 已使用持久化 `usage_facts`。
- 支付二维码页面与废弃弹窗重复维护轮询、倒计时和取消状态机。
- 余额充值倍率出现在配置和接口中，但实际充值固定按人民币 1:1 入账。
- 套餐与流量卡购买流程重复维护手续费、组合支付和下单分支。
- OpenAI HTTP failover、用量统计聚合和一次性付款补录工具存在明显重复或边界污染。

## 执行原则

- 在 `codex/code-redundancy-refactor` 分支分阶段实施，每阶段独立测试和提交。
- 不修改运行态数据库、Redis、容器、Nginx 或公网链路，不执行付款补录。
- 不新增数据库迁移；复用现有 `usage_facts` durable outbox。
- 保留 `/payment/qrcode` 兼容入口，余额充值固定人民币 1:1。
- 保留用户未提交的 `backend/internal/repository/migrations_schema_integration_test.go` 修改，不纳入本次提交。

## 阶段

1. 修复默认测试基线，使共享测试 helper 不再被 `unit` build tag 隐藏。
2. 为通用 Gateway 增加 `BuildUsageFact` / `PersistUsageFact`，使用协议终态闸门保证成功响应结束前先持久化计费事实。
3. 删除 `UsageRecordWorkerPool`、`gateway.usage_record.*` 和 `usage_fact_worker.enabled`，结算 worker 始终启动。
4. 删除充值倍率全链路字段，复用 `PaymentStatusPanel`，统一套餐与流量卡购买计算和提交逻辑。
5. 提取 OpenAI HTTP failover 状态对象，合并四套聚合统计 SQL 和重复统计类型。
6. 将线下付款补录逻辑移动到 `internal/tool/offlinepaymentbackfill`，保持行为和测试不变。
7. 运行全量后端、前端、集成、构建、静态检查和敏感信息检查，并新建结果文档。

## 回滚

所有阶段按独立提交落地，出现问题时按相反顺序执行 `git revert`。由于没有 schema 和运行态修改，回滚不需要数据迁移。
