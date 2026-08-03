# 真实请求最终计费倍率验证

## 请求

使用用户提供的 API Key 调用本地 `POST http://127.0.0.1:18082/v1/chat/completions`。首次使用 `gpt-5.6-sol` 因该 Key 所属分组没有可用渠道返回 404，未产生扣费；随后使用可用模型 `kimi-k2.5` 请求成功，响应内容为 `OK`。

## 计费结果

- usage log：`id=285711`
- 用户：`id=2`，`2799523972@qq.com`
- 模型：`kimi-k2.5`
- token：`input_tokens=42`、`cache_read_tokens=4352`、`output_tokens=14`
- `TotalCost=0.0004919`
- 分组倍率：`3.5`
- `ActualCost=0.0172165`

计算关系为：`0.0004919 × 3.5 × 10 = 0.0172165`。用户余额从此前的 `157.918070` 变为 `157.90085350`，减少额与 `ActualCost` 一致，证明最终倍率已真实落库并参与扣费。

## 安全

本记录不保存或回显 API Key。
