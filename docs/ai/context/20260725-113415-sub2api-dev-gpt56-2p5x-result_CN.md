# 18080 gpt5.6 三款模型计费调整到 2.5x 结果

时间：2026-07-25 11:34:15

## 结果

已将当前 18080 外层 `sub2api-dev` 的三款 gpt5.6 模型从 `2.0x` 调整为 `2.5x` 计费。

影响模型：

- `gpt-5.6-luna`
- `gpt-5.6-sol`
- `gpt-5.6-terra`

未修改全局倍率：

```yaml
billing:
    unit_price_multiplier: 2.0
```

因此其他模型仍按现有 `2.0x` 基础单价计费。

## 修改点

运行态文件：

- `deploy/data/model_pricing.json`
- `deploy/data/model_pricing.sha256`
- `deploy/data/config.yaml`

处理方式：

- 三款 gpt5.6 的可计费单价字段整体乘 `1.25`。
- 保持全局 `2.0x`，最终等效 `2.0 * 1.25 = 2.5x`。
- 显式设置：

```yaml
pricing:
    hash_url: ""
    update_interval_hours: 87600
```

原因：默认远程哈希同步每 10 分钟检查一次；如果不关闭，运行态本地定制价格可能被远程价格覆盖。

## 备份

修改前备份：

- `C:\tmp\sub2api-config-backups\20260725-113033-sub2api-dev-config-pre-gpt56-2p5x.yaml`
- `C:\tmp\sub2api-config-backups\20260725-113033-sub2api-dev-model-pricing-pre-gpt56-2p5x.json`
- `C:\tmp\sub2api-config-backups\20260725-113033-sub2api-dev-model-pricing-pre-gpt56-2p5x.sha256`

## 重启与健康检查

- 已重启 `sub2api-dev`
- `http://127.0.0.1:18080/health` 返回 `{"status":"ok"}`
- Docker 映射仍为 `127.0.0.1:18080->8080/tcp`
- 启动日志确认 `PricingService` 从 `data/model_pricing.json` 加载 `217` 个模型

## 真实落账验证

重启后 `usage_logs` 已出现新的 `gpt-5.6-terra` 记录，反推单价如下：

| 口径 | 重启前 | 重启后 |
|---|---:|---:|
| input | `5.0 USD/MTok` | `6.25 USD/MTok` |
| output | `30.0 USD/MTok` | `37.5 USD/MTok` |
| cache read | `0.5 USD/MTok` | `0.625 USD/MTok` |

这与从 `2.0x` 调整到 `2.5x` 一致。

## 说明

- 本次只影响 18080 外层 `sub2api-dev` 的运行态价格文件。
- 未修改内层 `18086`。
- 未修改套餐价格、用户分组倍率、用户专属倍率或历史用量。
- 历史 `usage_logs` 不回算；只影响重启后新请求。
