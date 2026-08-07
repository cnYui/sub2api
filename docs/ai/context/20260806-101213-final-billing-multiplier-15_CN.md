# 18082 最终计费倍率恢复为 15 倍

## 背景

管理员要求将当前中转站隐藏的服务端最终计费倍率由 `18x` 恢复为 `15x`。

## 变更

- 修改 `deploy/docker-compose.18082.yml`：`BILLING_FINAL_MULTIPLIER=18` 恢复为 `BILLING_FINAL_MULTIPLIER=15`。
- 该环境变量仅影响服务端最终扣费金额，不改变前端展示的基础价格。
- 未修改 `groups.rate_multiplier`、账户统计倍率、图片/视频独立倍率、历史用量记录或用户余额。

## 生效口径

- 应用容器重建后，新请求按 `标准成本 × 分组倍率 × 15` 扣费。
- 已完成的用量记录保留原始倍率快照和扣费金额，不追溯重算。

## 发布核验

- 已使用 `docker-compose.dev.yml + docker-compose.18082.yml` 仅强制重建 `sub2api` 应用容器；PostgreSQL、Redis 和数据卷未重建。
- 容器 `sub2api-official-18082` 状态为 `running (healthy)`。
- 容器环境变量已核对为 `BILLING_FINAL_MULTIPLIER=15`。
- 本地 `http://127.0.0.1:18082/health` 与公网 `https://aaccx.pw/health` 均返回 `{"status":"ok"}`。
