# 负余额硬拦截生产发布记录

## 发布范围

- 本地 `main` 已包含功能提交 `a191f34a5`（`fix: 阻断负余额调用并暂停后续周额度`）。
- 仅替换 `sub2api-official-18082` 应用容器；未重建 PostgreSQL、Redis、数据卷、Nginx 或 Cloudflare Tunnel。
- 应用镜像为 `deploy-sub2api:latest`，镜像 ID 为 `sha256:b8123ffca112db5af7376d3ab9e7850f996718c8e8c12cbb400ef7e6bdb428d5`，容器启动后状态为 `healthy`。
- `BILLING_FINAL_MULTIPLIER` 仍为 `18`。

## 数据库迁移与欠费套餐

- `204_pause_negative_balance_packages.sql` 已于 `2026-08-07 23:35:00+08` 执行。
- `205_batch_image_balance_package_source.sql` 已于 `2026-08-07 23:35:00+08` 执行。
- 上线时共 7 个负余额用户；4 个仍有后续额度且未到期的套餐（ID `10`、`48`、`51`、`124`）均为 `debt_paused`。
- 这 4 个套餐的 `credited_count`、`refresh_count`、`next_credit_at` 和 `expires_at` 保持原值，未被提前发放、消费或延长。
- `payment_audit_logs` 中存在 4 条 `BALANCE_PACKAGE_DEBT_PAUSED_MIGRATION_*`，操作人均为 `migration:204`，可追溯暂停原因。

## 线上准入验证

- `liyutong2883@gmail.com` 余额为 `-10.48717737 USD`，即使有约 `6.0256471960 USD` 有效 OpenAI 流量卡，请求仍返回 `403 INSUFFICIENT_BALANCE`；请求前后余额、流量卡余额和用量记录数均未变化。
- 余额为 `0` 且有足额 OpenAI 流量卡的账户，准入未被欠费规则误拦截；使用无效 JSON 作为无费用探针，返回业务解析 `400`，请求前后余额、流量卡余额和用量记录数均未变化。

## 链路健康检查

公网验证前执行 `docker exec sub2api-public-nginx-local nginx -t`，结果为配置语法正确且测试成功。

以下端点均返回 `HTTP 200` 和 `{"status":"ok"}`：

- `http://127.0.0.1:18082/health`
- `http://127.0.0.1:8080/health`
- `https://aaccx.pw/health`
- `https://www.aaccx.pw/health`
- `https://api.aaccx.pw/health`

## 结论

负余额用户已在数据库事实层被硬拦截，后续周额度已暂停并保留人工恢复入口；18082 与公网链路已运行当前应用镜像。所有验证均未产生额外上游模型费用。
