# 18080 全局倍率调整至 7.5 倍结果

## 已完成

- 公网外层 `sub2api-dev:18080` 的 `billing.unit_price_multiplier` 已从 `5` 调整为 `7.5`。
- `gpt-5.5` 与 `gpt-5.6-sol/terra/luna` 的官方基础价格保持不变。
- 仅重启外层 `sub2api-dev`；未重启或修改数据库、Redis、Nginx、Cloudflare Tunnel 或内层 `18086`。

## 生效后的 Standard 短上下文单价

单位为 USD / 1M Token。

| 模型 | 输入 | 缓存读取 | 输出 |
| --- | ---: | ---: | ---: |
| `gpt-5.5` | 37.50 | 3.75 | 225.00 |
| `gpt-5.6-sol` | 37.50 | 3.75 | 225.00 |
| `gpt-5.6-terra` | 15.00 | 1.50 | 90.00 |
| `gpt-5.6-luna` | 1.50 | 0.15 | 9.00 |

## 备份与验证

- 变更前配置已备份至 `deploy/backups/20260801-193748-18080-unit-price-multiplier-5x-prechange/config.yaml`，SHA-256 一致且内容可读。
- 重启后容器内 `/app/data/config.yaml` 为 `unit_price_multiplier: 7.5`，未发现环境变量覆盖。
- `18080/health`、`18086/health`、`8080/health` 均返回 HTTP 200；外层容器恢复为 `running/healthy`。

## 回滚

若需回滚，将本次备份的 `config.yaml` 恢复至 `deploy/data/config.yaml` 后，仅重启 `sub2api-dev`。历史 usage 和已创建的价格快照不会重算。
