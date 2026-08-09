# 18082 最终计费倍率调整为 16 倍设计

## 目标

将 18082 的隐藏最终计费倍率由 `20x` 调整为 `16x`，使后续模型请求按 `标准成本 × 分组倍率 × 16` 结算。

## 范围

- 只修改 `deploy/docker-compose.18082.yml` 的 `BILLING_FINAL_MULTIPLIER`。
- 仅重建 `sub2api-official-18082` 应用容器，保留 PostgreSQL、Redis、Nginx、Cloudflared 和数据卷。
- 不修改同文件中已有的凭证 Secret 配置、分组倍率、历史账单、用户余额或模型广场展示价格。

## 验收

- Compose 渲染与运行中容器环境变量均为 `BILLING_FINAL_MULTIPLIER=16`。
- 应用容器为 `running healthy`，本地和三个公网健康接口均返回 HTTP 200。
- 新建发布记录并追加 `AGENTS.md` 长期上下文，不覆盖既有历史文件。
