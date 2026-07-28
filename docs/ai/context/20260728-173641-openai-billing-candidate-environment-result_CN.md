# OpenAI 计费候选双层环境结果

时间：2026-07-28 17:36:41 +09:00

## 已完成

- 建立独立 Compose 项目 `sub2api-openai-billing-candidate`，仅暴露 `127.0.0.1:18081` 和 `127.0.0.1:18087`。
- 外层、内层各自使用独立 PostgreSQL 数据目录和空 Redis；Redis 固定禁用 RDB 与 AOF 持久化。
- 候选数据库 dump 均通过 `pg_restore --list` 可读性校验，并恢复到候选容器。
- 候选外层和内层均执行副作用隔离 SQL；支付、退款、SMTP、通知、监控和渠道监控均关闭。
- 候选外层唯一可调度 OpenAI 内部路由已改为 `http://billing-inner:8080/v1`，其他外层 OpenAI 路由已禁用。

## 验证证据

- 候选外层 public schema：84 张表；内层 public schema：83 张表。
- 两层 `payment_enabled=false`。
- 外层指向候选内层的 OpenAI 路由数量：1。
- 公网 `18080` 与 `18086` health 均为 200；`sub2api-dev`、`sub2api-upstream-latest` 及两套公网 PostgreSQL 容器启动时间与备份前一致。

## 边界

- 未启动候选应用容器：外层计费修改完成前不能构建最终候选镜像，此动作按实施计划留至 Task 14。
- 本地候选 dump、状态记录、数据目录与本地 `.env` 均位于 `deploy/openai-billing-candidate/` 或被忽略的候选环境文件中，不进入 Git。
- 全程未停止、重启、迁移或写入公网 `18080/18086`、其 PostgreSQL、Redis、Nginx 或流量。
