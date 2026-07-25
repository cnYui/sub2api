# 外层 `sub2api-dev` 实际计费倍率调整到 `2x` 的结果

时间：2026-07-24 11:39:53

## 结果

已将外层 `sub2api-dev` 的实际计费倍率从 `1.8` 调整为 `2.0`。

修改点：

```yaml
billing:
    unit_price_multiplier: 2.0
```

## 影响对象

- 影响：外层 `18080`
- 不影响：内层 `18086`
- 当前不使用：CPA

## 备份

修改前已备份原配置：

`C:\tmp\sub2api-config-backups\20260724-113511-sub2api-dev-config-pre-unit-price-2x.yaml`

## 重启与验证

- 已重启 `sub2api-dev`
- 服务恢复健康
- 线上落账验证通过

验证到的单价：

- `input_price = 10 USD/MTok`
- `output_price = 60 USD/MTok`
- `cache_read_price = 1 USD/MTok`

## 说明

- 这次只改了运行时实际计费倍率，没有改套餐价格。
- 公网入口会短暂受重启影响，随后恢复。
