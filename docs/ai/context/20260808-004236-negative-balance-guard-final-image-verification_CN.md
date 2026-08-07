# 负余额硬拦截最终镜像复核

## 最终运行实例

- 构建时本地 `main` 为 `dceca8676`（`docs: 记录负余额拦截生产发布核验`）；功能代码来自 `a191f34a5`。
- 重新构建后的 `deploy-sub2api:latest` 镜像 ID 为 `sha256:3271c8cad426ee48578f580788c35d864b422e1f71ce5cd9a18d4aa13d40bdd5`。
- 当前 `sub2api-official-18082` 容器 ID 为 `002de1431f2fd6d036a6caf4f4904c9bf7e3d5b5071fb949d32569cf5dca4f04`，状态为 `healthy`。
- PostgreSQL 容器 ID `d94d74cddbcb30fd0481c1f20b81cda63a1ea65d5ed6e4c92811c72ce846d7cf`、Redis 容器 ID `d6ea60b580181b4d084fef022192b623e5db3fa44caa567b186cceda4e00cd66` 未变化。
- `BILLING_FINAL_MULTIPLIER=18`。

## 替换后验证

- 替换后再次执行 `docker exec sub2api-public-nginx-local nginx -t`，配置语法检查成功。
- 本地 18082、本地 Nginx、`aaccx.pw`、`www.aaccx.pw`、`api.aaccx.pw` 的 `/health` 均返回 `HTTP 200` 和 `{"status":"ok"}`。
- `liyutong2883@gmail.com` 的负余额请求再次返回 `403 INSUFFICIENT_BALANCE`；请求前后余额 `-10.48717737 USD`、有效 OpenAI 流量卡 `6.0256471960 USD` 和用量记录数 `5112` 均未变化。
- 204/205 迁移仍已执行，4 个活动欠费套餐仍为 `debt_paused`，暂停审计数为 4。

本记录补充并固定重新构建后的最终镜像事实；此前发布记录保留不覆盖。
