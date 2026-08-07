# 18082 最终计费倍率 18x 发布结果

- 时间：2026-08-06
- 生效配置：`deploy/docker-compose.18082.yml` 中 `BILLING_FINAL_MULTIPLIER=18`。
- 发布方式：执行 `docker compose -f docker-compose.dev.yml -f docker-compose.18082.yml up -d --build --no-deps sub2api`，仅重建并替换 `sub2api-official-18082` 应用容器。
- 未变更组件：PostgreSQL、Redis、数据卷、Nginx 与 Cloudflare Tunnel。
- 容器核验：应用容器状态为 `running (healthy)`，运行时环境变量为 `BILLING_FINAL_MULTIPLIER=18`。
- 链路核验：Nginx 配置语法检查通过；`http://127.0.0.1:18082/health`、`http://127.0.0.1:8080/health`、`https://aaccx.pw/health`、`https://www.aaccx.pw/health` 和 `https://api.aaccx.pw/health` 均返回 `200`。
- 影响范围：仅后续请求的服务端最终计费倍率变为 `18x`；历史用量、余额、模型分组倍率和账户统计倍率均未修改。
