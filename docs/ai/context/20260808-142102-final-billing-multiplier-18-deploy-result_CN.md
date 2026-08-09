# 18082 最终计费倍率恢复为 18 倍发布结果

## 变更

- 将 `deploy/docker-compose.18082.yml` 的 `BILLING_FINAL_MULTIPLIER` 从 `20` 调整为 `18`。
- 后续模型请求按 `标准成本 × 分组倍率 × 18` 结算。
- 未修改分组倍率、账户统计倍率、图片或视频独立倍率、历史用量、用户余额和模型广场展示价格。

## 发布

- 仅替换 `sub2api-official-18082` 应用容器，使用 `--force-recreate --no-deps sub2api`；PostgreSQL 和 Redis 未重建。
- Compose 凭证文件变量不在当前终端或 `deploy/.env` 中保存，发布时从原应用容器的挂载来源取得路径并仅作为当前命令的环境变量注入，未读取或记录密钥内容。
- 替换后应用容器 ID 为 `b8c315813ad97e02de9a39a32fd226164cae879f8f10511232f396eb81500006`，状态为 `running healthy`，运行态环境变量为 `BILLING_FINAL_MULTIPLIER=18`，凭证文件挂载仍存在。
- PostgreSQL 容器 ID 保持 `d94d74cddbcb30fd0481c1f20b81cda63a1ea65d5ed6e4c92811c72ce846d7cf`，Redis 容器 ID 保持 `d6ea60b580181b4d084fef022192b623e5db3fa44caa567b186cceda4e00cd66`。

## 验证

- 合并 Compose 配置已校验包含 `BILLING_FINAL_MULTIPLIER: "18"`。
- `TestLoadBillingFinalMultiplierFromEnvironment` 通过。
- `http://127.0.0.1:18082/health`、`https://aaccx.pw/health`、`https://www.aaccx.pw/health`、`https://api.aaccx.pw/health` 均返回 HTTP 200。
