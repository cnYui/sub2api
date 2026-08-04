# 18082 模型最终计费倍率调整记录

时间：2026-08-03（Asia/Tokyo）

## 变更

- 目标实例：`sub2api-official-18082`
- 配置项：`BILLING_FINAL_MULTIPLIER`
- 原值：`10`
- 新值：`15`
- 影响范围：服务端实际模型扣费；前端模型价格、套餐价格和基础成本展示不变。

## 验证

- 已更新 `deploy/docker-compose.18082.yml` 的持久化环境变量。
- 应用容器重建后回读运行环境变量为 `BILLING_FINAL_MULTIPLIER=15`。
- `/health` 返回正常。
