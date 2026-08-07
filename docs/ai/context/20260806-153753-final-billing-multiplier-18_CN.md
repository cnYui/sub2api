# 18082 服务端最终计费倍率调整

- 时间：2026-08-06
- 配置：`deploy/docker-compose.18082.yml`
- 变更：`BILLING_FINAL_MULTIPLIER` 从 `15` 调整为 `18`
- 影响：后续模型请求按标准成本、模型分组倍率及服务端最终 `18x` 计算；历史用量、余额和分组倍率不回写。
- 发布：仅重建并替换 `sub2api-official-18082` 应用容器；PostgreSQL、Redis、数据卷、Nginx 与 Cloudflare Tunnel 不重建。
- 验证：发布后核验容器环境变量、健康检查、本地 18082、本地 Nginx 与公网域名。
