# 超过 3 美元普通余额日志退款执行记录

## 执行范围

按管理员授权，对复核表 `20260805-192221-over-3-final-review-detail.csv` 中最终保留的 85 条日志执行整笔退款。该批记录已排除订阅大上下文、Shadow 观测、可完整匹配流量卡流水、无法完整匹配流量卡流水以及已退款 K3 异常记录。

- 日志数量：85 条
- 涉及用户：11 位
- 退款总额：`389.0187845250 USD`
- 原扣费来源：全部为普通余额（`billing_type=0`）
- 退款去向：全部退回普通余额
- 流量卡退款：`0 USD`
- 原 `usage_logs`：保留不变，仅新增退款余额与审计流水

## 用户退款汇总

| 用户 ID | 用户 | 日志数 | 退款 USD |
| ---: | --- | ---: | ---: |
| 448 | xiaobianfuai@gmail.com | 1 | 7.5120937500 |
| 452 | changjunwang123@gmail.com | 1 | 6.1837260000 |
| 454 | xunskyler@gmail.com | 1 | 4.4751660000 |
| 463 | itjiangzengwen@gmail.com | 2 | 6.8589795000 |
| 479 | 3056163754@qq.com | 10 | 42.1425120000 |
| 505 | 1032726009@qq.com | 36 | 149.3559957750 |
| 506 | 961109198@qq.com | 4 | 18.7239937500 |
| 524 | caogang@sdufe.edu.cn | 2 | 18.0428400000 |
| 537 | xi3187744@gmail.com | 12 | 56.3255325000 |
| 565 | 2197057077@qq.com | 15 | 73.5926512500 |
| 574 | 2047431647@qq.com | 1 | 5.8052940000 |

## 审计与缓存

- PostgreSQL 事务使用 `pg_advisory_xact_lock` 防止同批退款并发执行。
- 每条日志新增唯一 `payment_audit_logs` 记录：`action=BALANCE_MANUAL_REFUND`、`order_id=BALANCE-OVER3-<usage_log_id>`。
- 退款审计中记录用户、日志 ID、整笔实际扣费、退款金额和退款去向；重复执行会被唯一约束阻止。
- 清理了余额缓存 `billing:balance:*` 和 API Key 认证缓存 `apikey:auth:*`，共 4 个缓存键。
- 执行后公网健康检查：`aaccx.pw`、`www.aaccx.pw`、`api.aaccx.pw` 均返回 HTTP 200。
