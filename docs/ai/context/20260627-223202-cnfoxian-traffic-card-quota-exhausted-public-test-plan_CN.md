# cnfoxian@gmail.com 套餐额度耗尽后流量卡公网实测计划

时间：2026-06-27 22:32 JST

## 目标

确认 `cnfoxian@gmail.com` 在今日套餐额度已耗尽的情况下，使用其现有 API Key 发起公网真实模型请求时，是否会使用并扣减 10 USD OpenAI/GPT 流量卡。

## 当前公网运行态

- 公网应用容器：`sub2api-candidate`
- 镜像：`sub2api-candidate:20260627-221441-traffic-card-fix`
- 端口：`127.0.0.1:18084->8080`
- 说明：该镜像由当前本地 `main` 工作树构建，包含尚未提交的订阅缺失/流量卡兜底中间件修复。

## 约束

- 不打印、不记录完整 API Key。
- 不手工修改用户余额、订阅、流量卡或 API Key。
- 只执行真实最小模型请求导致的正常计费写入。
- 请求前后分别记录流量卡余额、ledger、usage_logs 和 subscription usage。

## 步骤

1. 查询 `cnfoxian@gmail.com` 的用户、API Key、订阅、group、流量卡状态。
2. 记录请求前的流量卡余额和用量/账单计数。
3. 从数据库读取该用户 active API Key 到 shell 变量，不输出明文。
4. 优先使用 `/v1/responses` + 当前可用 OpenAI 模型发最小请求。
5. 请求后查询流量卡余额、`traffic_credit_ledger`、`usage_logs`、订阅用量变化。
6. 写入实测结果文档并汇报。
