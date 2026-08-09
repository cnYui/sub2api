# 固定计费倍率 16 倍核验

- `deploy/docker-compose.18082.yml` 已配置 `BILLING_FINAL_MULTIPLIER=16`。
- 公网容器 `sub2api-official-18082` 运行时环境已回读为 `BILLING_FINAL_MULTIPLIER=16`。
- 容器当前状态为 `running/healthy`，无需重复重建或重启；数据库、Redis 和数据卷保持不变。

