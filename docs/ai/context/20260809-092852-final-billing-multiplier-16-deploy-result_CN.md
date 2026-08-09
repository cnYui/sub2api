# 18082 最终计费倍率调整为 16 倍发布结果

## 变更

- 将 `deploy/docker-compose.18082.yml` 的 `BILLING_FINAL_MULTIPLIER` 从 `18` 调整为 `16`。
- 后续模型请求按 `标准成本 × 分组倍率 × 16` 结算。
- 未修改分组倍率、账户统计倍率、图片或视频独立倍率、模型广场展示价格、历史用量记录和用户余额。

## 发布

- 使用 `docker compose -f deploy/docker-compose.dev.yml -f deploy/docker-compose.18082.yml up -d --force-recreate --no-deps sub2api` 仅替换应用容器。
- 替换前应用容器的运行时环境变量为 `BILLING_FINAL_MULTIPLIER=18`。
- 替换后应用容器 ID 为 `cd09d161adcb781375f3b5fb55ebd458d3b5ca3739056fe2cfd9bc839889f919`，状态为 `running healthy`，运行时环境变量为 `BILLING_FINAL_MULTIPLIER=16`。
- PostgreSQL 容器 ID 保持 `d94d74cddbcb30fd0481c1f20b81cda63a1ea65d5ed6e4c92811c72ce846d7cf`，Redis 容器 ID 保持 `d6ea60b580181b4d084fef022192b623e5db3fa44caa567b186cceda4e00cd66`，均未重建。Nginx 和 Cloudflare Tunnel 未修改。

## 验证

- Compose 渲染配置包含 `BILLING_FINAL_MULTIPLIER: "16"`。
- `go test -run '^TestLoadBillingFinalMultiplierFromEnvironment$' -count=1 ./internal/config` 通过。
- `http://127.0.0.1:18082/health`、`https://aaccx.pw/health`、`https://www.aaccx.pw/health`、`https://api.aaccx.pw/health` 均返回 HTTP 200。
