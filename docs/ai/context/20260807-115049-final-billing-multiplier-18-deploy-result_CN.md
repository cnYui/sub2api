# 18082 最终计费倍率调整为 18 倍发布结果

## 变更

- 将 `deploy/docker-compose.18082.yml` 的 `BILLING_FINAL_MULTIPLIER` 由运行配置对应的 `15` 恢复为 `18`。
- 未修改分组倍率、账户统计倍率、图片或视频独立倍率、前端模型广场、历史用量记录和用户余额。
- 新请求的服务端扣费口径为 `标准成本 × 分组倍率 × 18`；模型广场继续不展示或叠加最终倍率。

## 发布

- 使用 `docker compose -f deploy/docker-compose.dev.yml -f deploy/docker-compose.18082.yml up -d --force-recreate --no-deps sub2api` 替换应用容器。
- 替换前应用容器 ID 为 `73dbc388867091556b1637a2eb6c5c29510d48de009a1baca3db972b7865ce98`，环境变量为 `BILLING_FINAL_MULTIPLIER=15`。
- 替换后应用容器 ID 为 `ef1eb915cd07699eba7c460b0c7f67ab1974fd3805bad5ec4a387bf447700387`，状态为 `running healthy`，环境变量为 `BILLING_FINAL_MULTIPLIER=18`。
- PostgreSQL 容器 ID 保持 `d94d74cddbcb30fd0481c1f20b81cda63a1ea65d5ed6e4c92811c72ce846d7cf`，Redis 容器 ID 保持 `d6ea60b580181b4d084fef022192b623e5db3fa44caa567b186cceda4e00cd66`，均未重建。

## 验证

- 合并后的 Compose 配置已确认包含 `BILLING_FINAL_MULTIPLIER: "18"`。
- `http://127.0.0.1:18082/health` 返回 HTTP 200。
- `https://aaccx.pw/health`、`https://www.aaccx.pw/health`、`https://api.aaccx.pw/health` 均返回 HTTP 200。
- Compose 仅提示 `REDIS_PASSWORD` 未设置并按既有配置使用空值；该告警在修改前后均存在，未影响应用健康状态。
