# 用户计费基础单价 1.8 倍调整结果

时间：2026-07-23

## 结论

- 已采用“调整模型/渠道基础单价”的方案，不改 `rate_multiplier`。
- 外层 `sub2api-dev` 通过 `billing.unit_price_multiplier=1.8` 将原始单价统一放大到 1.8 倍。
- `/usage` 页面倍率快照仍保持 1x，费用按新基础单价计算。
- 内层 `18086` 未改。

## 代码变更

- `backend/internal/config/config.go`
  - 新增 `billing.unit_price_multiplier`，默认值 `1.0`，校验必须大于 0。
- `backend/internal/service/pricing_service.go`
  - 对外价格读取会应用单价倍率。
- `backend/internal/service/billing_service.go`
  - 计费入口改为先拿原始价，再统一乘倍率。
- `backend/internal/service/model_pricing_resolver.go`
  - 渠道 token / per-request 价格也统一乘倍率。
- `deploy/data/config.yaml`
  - 写入 `billing.unit_price_multiplier: 1.8`。

## 验证

- `go test ./...`
- `sub2api-dev` 已按新镜像重建并恢复到 `127.0.0.1:18080`
- `http://127.0.0.1:18080/health` 返回 200
- `http://127.0.0.1:18086/health` 返回 200，未受影响

