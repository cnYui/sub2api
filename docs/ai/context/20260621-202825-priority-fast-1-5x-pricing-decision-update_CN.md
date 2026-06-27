# Priority/Fast 1.5 倍计费设计修正

## 修正原因

初版设计保留了“渠道显式覆盖价不额外乘 1.5”的例外。但用户要求是“遇到 `priority` 和 `fast`，按 1.5 倍进行扣费”，这应是统一服务等级规则，不应因为价格来源不同而出现例外。

## 最终规则

- `fast` 仍先归一化为 `priority`。
- 扣费时不再使用 LiteLLM 或 fallback 价格表里的 priority 专用单价直接计费。
- `priority` 统一作为最终 token 成本的 `1.5` 倍服务等级倍率处理，覆盖输入、输出、缓存写入、缓存读取和图片输出 token 成本。
- `flex` 继续按 `0.5` 倍服务等级倍率处理。
- 为了让 `/available-channels` 展示与扣费一致，模型价格 DTO 中的 priority 展示字段仍归一为基础价 `* 1.5`。
