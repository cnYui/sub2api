# Priority/Fast 1.5 倍计费设计

## 背景

用户要求把当前 `priority` / `fast` 的 2 倍定价改为 1.5 倍定价，并确保真实使用这两个服务等级时按 1.5 倍扣费。

当前链路中，OpenAI 请求里的 `service_tier=fast` 会被归一化为 `priority`；扣费时 `BillingService.computeTokenBreakdown` 会优先使用 `ModelPricing` 中的 priority 单价。由于 `gpt-5.4` / `gpt-5.5` 的 LiteLLM 或 fallback 价格表里 priority 字段是基础价的 2 倍，真实扣费和 `/available-channels` 价格展示都会表现为 2 倍。

## 设计选择

### 方案 A：只改前端展示

- 优点：改动最小。
- 缺点：真实扣费仍是 2 倍，和用户目标冲突。
- 结论：不采用。

### 方案 B：修改数据库价格覆盖

- 优点：可以只影响当前运行态。
- 缺点：当前 `gpt-5.4/gpt-5.5` 没有数据库覆盖价，实际来自代码价格表；同时会让代码默认行为和运行态漂移。
- 结论：不采用。

### 方案 C：在计费服务统一归一 priority 单价为基础价 1.5 倍

- 优点：真实扣费、`/api/v1/channels/prices` 展示、测试用 fallback 价格都同源；`fast` 已归一为 `priority`，无需额外分支。
- 缺点：需要更新后端计费测试和价格展示测试期望。
- 结论：采用。

## 具体设计

- 新增统一常量 `openAIPriorityServiceTierMultiplier = 1.5`。
- `serviceTierCostMultiplier("priority")` 从 `2.0` 改为 `1.5`，覆盖没有显式 priority 单价的模型。
- `BillingService.GetModelPricing` 读取 LiteLLM 或 fallback 定价后，对普通模型价格应用 priority 单价策略：若基础输入/输出/cache_read 单价大于 0，则对应 priority 单价统一设为基础价 `* 1.5`。
- `ChannelModelPricing` 显式覆盖价保持原样：管理员在渠道定价中配置 flat/interval 价格时，priority 仍等于该覆盖价，不额外乘 1.5，避免数据库显式定价被隐式改写。
- `/available-channels` 顶部价格通过 `/api/v1/channels/prices` 读取 `ModelPricingResolver`，会自动展示 1.5 倍后的 priority 价格。

## 验证范围

- 后端：新增/调整单元测试，覆盖 fallback、LiteLLM 动态 priority 字段和无显式 priority 字段三条路径。
- 前端：更新 `/available-channels` 测试中 priority 价格期望，确保页面展示新的 1.5 倍价格。
- 构建：运行相关 Go 单测、前端测试和前端 build。
