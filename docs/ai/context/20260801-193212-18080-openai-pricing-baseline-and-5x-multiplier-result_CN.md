# 18080 OpenAI 基础价格恢复与 5 倍倍率结果

## 已完成

- 已修改公网外层 `sub2api-dev:18080` 挂载的 `deploy/data/model_pricing.json`：
  - `gpt-5.5` 已是官方 Standard 基础价，保持不变。
  - `gpt-5.6-sol`、`gpt-5.6-terra`、`gpt-5.6-luna` 已恢复为 OpenAI 官方 Standard 基础价，并同步恢复长上下文、Flex、Priority 字段。
- 已将 `deploy/data/config.yaml` 的 `billing.unit_price_multiplier` 从 `2.5` 修改为 `5`。
- 只重启了外层 `sub2api-dev`；未修改数据库、Redis、Nginx、Cloudflare Tunnel 或内层 `sub2api-upstream-latest:18086`。

## 生效后的 Standard 短上下文单价

单位为 USD / 1M Token，括号内为全局 5 倍后的实际计费价格。

| 模型 | 输入 | 缓存读取 | 输出 |
| --- | ---: | ---: | ---: |
| `gpt-5.5` | 5.00 (25.00) | 0.50 (2.50) | 30.00 (150.00) |
| `gpt-5.6-sol` | 5.00 (25.00) | 0.50 (2.50) | 30.00 (150.00) |
| `gpt-5.6-terra` | 2.00 (10.00) | 0.20 (1.00) | 12.00 (60.00) |
| `gpt-5.6-luna` | 0.20 (1.00) | 0.02 (0.10) | 1.20 (6.00) |

## 备份与验证

- 变更前快照目录：`deploy/backups/20260801-192828-18080-openai-pricing-prechange/`。
- 已验证 `config.yaml` 与 `model_pricing.json` 备份 SHA-256 与变更前源文件一致，JSON、YAML 均可读取。
- 离线校验确认四个目标模型的 Standard 与长上下文基础价符合官方矩阵，倍率配置为 `5`。
- 重启后容器内 `/app/data/config.yaml` 为 `unit_price_multiplier: 5`，模型价格文件哈希与宿主机一致，且无环境变量覆盖该倍率。
- `18080/health`、`18086/health`、`8080/health` 均返回 HTTP 200；外层容器状态为 `running/healthy`。

## 回滚

若需回滚，恢复上述备份目录中的两份文件至 `deploy/data/`，再仅重启 `sub2api-dev`。历史 usage 与已经创建的价格快照不会重算。
