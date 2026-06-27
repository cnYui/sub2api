# `/available-channels` 实际计费价格展示设计

## 背景

用户截图中 `/available-channels` 顶部只显示生图价格，`gpt-5.4` 和 `gpt-5.5` 的 token 价格没有显示；下方渠道表为空。

根因不是模型没有价格，而是前端上一版把 GPT 价格来源绑定在 `/api/v1/channels/available` 的渠道响应里。该接口会按用户可访问分组和渠道分组交集过滤渠道，一旦当前运行态没有返回可见渠道，顶部价格摘要也拿不到模型定价。

实际扣费链路在后端：

- token 请求：`ModelPricingResolver.Resolve` 解析价格，顺序为渠道模型价覆盖、LiteLLM 价格表、业务 fallback。
- 当前本地 LiteLLM 价格表已有 `gpt-5.4` 和 `gpt-5.5`：
  - `gpt-5.4`：输入 `$2.5/1M`，输出 `$15/1M`，缓存读取 `$0.25/1M`，priority 输入 `$5/1M`，priority 输出 `$30/1M`，priority 缓存读取 `$0.5/1M`。
  - `gpt-5.5`：输入 `$5/1M`，输出 `$30/1M`，缓存读取 `$0.5/1M`，priority 输入 `$10/1M`，priority 输出 `$60/1M`，priority 缓存读取 `$1/1M`。
- 生图请求：当前在售分组已开启 `allow_image_generation=true`，按分组 `image_price_1k/2k/4k` 计费，当前为 `$0.10/$0.20/$0.40` 每张。

## 目标

- `/available-channels` 顶部价格摘要必须展示当前实际计费口径下的 `gpt-5.4`、`gpt-5.5` 和生图价格。
- 即使 `/channels/available` 当前返回空数组，价格摘要仍然展示。
- 不放宽渠道表权限过滤；渠道表为空时仍显示“暂无可用渠道”。

## 方案

新增用户侧只读接口 `GET /api/v1/channels/prices`：

- 复用 `AvailableChannelHandler`，避免新增无关业务边界。
- token 价格由 `ModelPricingResolver.Resolve(ctx, PricingInput{Model: name})` 获取，不传 group id，保持当前“全局实际计费价”口径。
- 返回字段只包含用户需要的白名单：模型名、计费模式、价格来源、输入、输出、缓存写入、缓存读取、priority 输入、priority 输出、priority 缓存读取。
- 生图价格仍由前端已有 `/groups/available` 数据派生，因为图片价格当前确实按用户可用分组配置生效。

前端：

- `frontend/src/api/channels.ts` 增加 `getPrices()`。
- `AvailableChannelsView.vue` 并发读取 `getPrices()`、`getAvailable()`、`groups` 和倍率。
- 顶部价格摘要优先使用 `getPrices()`；如果接口失败，再回退到渠道模型定价，避免新接口异常导致旧页面完全不可用。

## 取舍

- 不在前端硬编码价格。原因是实际价格可能由 LiteLLM 价格表或后端 fallback 更新，前端硬编码会再次漂移。
- 不把 `/channels/available` 改成总是返回模型价格。原因是这个接口语义是“可见渠道”，放宽它会混淆权限过滤和公开价格展示。
