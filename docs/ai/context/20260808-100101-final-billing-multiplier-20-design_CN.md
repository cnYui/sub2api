# 18082 最终计费倍率调整为 20 倍设计

## 目标

将 18082 实例的隐藏最终计费倍率从 `18x` 调整为 `20x`，使后续模型请求按 `标准成本 × 分组倍率 × 20` 结算。

## 范围

- 修改 `deploy/docker-compose.18082.yml` 中的 `BILLING_FINAL_MULTIPLIER` 为 `20`。
- 仅重建 `sub2api-official-18082` 应用容器，使用 `--no-deps` 保持 PostgreSQL、Redis 和数据卷不变。
- 不修改分组倍率、账户统计倍率、图片/视频独立倍率、历史用量、用户余额或模型广场展示价格。

## 验收

- 合并后的 Compose 配置渲染出 `BILLING_FINAL_MULTIPLIER: "20"`。
- 应用容器运行态环境变量为 `BILLING_FINAL_MULTIPLIER=20` 且状态为 `running healthy`。
- 本地 18082 健康接口以及三个公网健康接口返回 HTTP 200。
- 写入新的执行结果记录，并将决策追加到 `AGENTS.md`，不覆盖历史上下文文件。

## 方案选择

采用持久化 Compose 配置加仅应用容器重建。只改文件不会立即影响运行中请求，临时环境覆盖无法保证重启后的口径，因此不采用。
