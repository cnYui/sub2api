# /monitor 模型目录展示生产发布

## 变更

- 用户侧监控卡片仅展示“模型目录延迟”。
- 删除目录探测协议下没有数据来源的“对话延迟”和“端点 PING”指标。
- 状态徽章与时间线继续按真实 `operational`、`degraded`、`error`、`failed` 状态着色。

## 发布范围

构建 `deploy-sub2api:latest` 后，仅使用 Compose `--no-deps --force-recreate sub2api` 替换 `sub2api-official-18082`。PostgreSQL、Redis、Nginx、Cloudflare Tunnel 与数据卷未重建。

第一次未带 `--no-deps` 的 Compose 调用因尝试创建既有 PostgreSQL 容器产生名称冲突后退出；未删除、停止或重建任何数据库和 Redis 容器。随后使用 `--no-deps` 完成应用容器替换。

## 验证

- `pnpm vitest run src/components/user/monitor/MonitorMetricPair.spec.ts`：2 项通过。
- `pnpm typecheck`：通过。
- 定向 ESLint：通过。
- `pnpm build`：通过。
- 容器状态：`sub2api-official-18082` 为 `healthy`。
- `http://127.0.0.1:18082/health`、`http://127.0.0.1:8080/health`、`https://aaccx.pw/health`：均为 200。
- 线上入口引用的 `ChannelStatusView` 异步资源已更新；源码与构建产物中监控卡片仅引用 `modelCatalogLatency`，不再引用旧的 `dialogLatency`、`endpointPing` 或 `primary_ping_latency_ms`。

## 已知限制

当前内置浏览器没有登录会话，无法直接看到需认证的 `/monitor` 卡片数据。发布包、组件测试、类型检查和生产健康端点已完成核验。
