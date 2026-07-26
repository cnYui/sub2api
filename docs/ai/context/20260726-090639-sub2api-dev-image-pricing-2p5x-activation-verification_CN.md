# 18080 图片生成模型 2.5x 价格生效验证

时间：2026-07-26 09:06:39

## 服务状态

- `sub2api-dev`：`running / healthy`
- `http://127.0.0.1:18080/health`：`{"status":"ok"}`
- 启动日志：`[Pricing] Loaded 217 models from data/model_pricing.json`

## 价格文件校验

运行态价格文件 SHA-256 与声明哈希一致：

```text
a6ea78219e851f4829f22a83fadb7f89f2dcc2cb0b30e32f6f8e2824539be670
```

## GPT 图片模型最终计费单价

以下价格已包含全局 `billing.unit_price_multiplier: 2.0`，单位均为 USD / 百万 Token：

| 模型 | 文本输入 | 图片输入 | 文本输出 | 图片输出 |
|---|---:|---:|---:|---:|
| `gpt-image-1` | 12.5 | 25 | - | 100 |
| `gpt-image-1-mini` | 5 | 6.25 | - | 20 |
| `gpt-image-1.5` | 12.5 | 20 | 25 | 80 |
| `gpt-image-2` | 12.5 | 20 | 25 | 75 |

其余图片生成模型也已按同一规则将全部可计费字段乘以 `1.25`。非图片模型未调整。
