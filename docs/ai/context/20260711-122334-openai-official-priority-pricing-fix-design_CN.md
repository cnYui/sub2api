# OpenAI 官方 Priority 计费修复设计

## 背景

当前 `BillingService` 对所有 `service_tier=priority` 请求统一乘 `1.5`。OpenAI 官方现行短上下文 Priority 价格不是统一倍率：

- GPT-5.4：Standard 的 `2x`
- GPT-5.5：Standard 的 `2.5x`
- GPT-5.6 Sol / Terra / Luna：Standard 的 `2x`

入站 `service_tier=fast` 会归一化为 `priority`，因此 Fast 与 Priority 使用同一计费规则。

## 目标

只修正以下模型的 Priority 倍率：

- `gpt-5.4`：`2x`
- `gpt-5.5`：`2.5x`
- `gpt-5.6-sol`：`2x`
- `gpt-5.6-terra`：`2x`
- `gpt-5.6-luna`：`2x`

保持以下行为不变：

- Standard 基础价格
- Flex `0.5x`
- Fast 到 Priority 的归一化
- 长上下文阈值和输入 `2x`、输出 `1.5x` 策略
- 分组 `rate_multiplier`
- 未列入本次范围的其他模型 Priority 规则
- reasoning tokens 只包含在 output tokens 中计费一次

## 方案比较

### 方案 A：按模型返回 Priority 倍率

在计费核心增加单一模型级倍率解析函数。已知 GPT-5.4/5.5/5.6 返回官方倍率，其他模型继续返回现有 `1.5`；Flex 和其他 tier 保持原逻辑。

优点：改动最小；运行时不依赖远程价格镜像是否及时更新；缓存写入等所有成本分量自然使用同一官方倍率；不会改变范围外模型。

缺点：官方后续调价时需要同步修改代码和测试。

### 方案 B：直接使用动态价格表的 `*_priority` 字段

让输入、缓存读取、输出分别选择动态价格表中的 Priority 单价，并扩展缓存写入 Priority 字段。

优点：数据驱动，长期扩展性更好。

缺点：当前远程价格镜像中的 GPT-5.5 Priority 仍为过时的 `2x`；运行时同步会覆盖内置资源，无法保证本次上线后立即符合官方 `2.5x`。改动面也会覆盖所有模型。

### 方案 C：全局改为 `2x`，只对 GPT-5.5 特判

优点：代码最少。

缺点：会把范围外模型从现有 `1.5x` 改成 `2x`，违反“其他不变”。

## 选择

采用方案 A。

## 设计细节

### 模型级倍率解析

新增纯函数，根据规范化后的 OpenAI 模型名返回 Priority 倍率：

- `gpt-5.4`、`gpt-5.6-sol`、`gpt-5.6-terra`、`gpt-5.6-luna` 返回 `2.0`
- `gpt-5.5` 返回 `2.5`
- 其他模型返回现有默认值 `1.5`

模型名继续复用 `normalizeKnownOpenAICodexModel()`，因此日期快照、紧凑别名等既有归一行为保持一致；未知 `gpt-5.6-*` 不应误匹配。

### 计费数据流

`computeTokenBreakdown()` 在已经获得 `model` 的入口计算 tier 倍率：

```text
Standard 单价
  -> 可选长上下文倍率
  -> 模型级 Priority 倍率或 Flex 倍率
  -> 分组 rate_multiplier
```

Priority 倍率继续统一作用于：

- 输入费用
- 输出费用
- 缓存读取费用
- 缓存写入费用
- 图片输出费用

这保持现有计算顺序，只替换 Priority 倍率的来源。

### 定价资源

同步修正内置 `model_prices_and_context_window.json` 中目标模型的 `*_priority` 字段，避免管理界面、价格查询和后续数据驱动改造继续展示错误值：

- GPT-5.4：已有 `2x`，保持不变
- GPT-5.5：改为 `2.5x`
- GPT-5.6 三款：改为 `2x`，并补齐缓存写入 Priority 单价

运行时计费仍以模型级倍率函数为最终保障，不依赖远程镜像即时更新。

## 明确不处理

- 不处理上游把 Priority 请求降级为 `service_tier=default` 后按响应实际 tier 计费的问题。
- 不改变 Priority 长上下文当前行为。
- 不修改运行态远程价格文件、数据库、Redis、容器或部署配置。
- 不补扣历史 usage。

## 测试设计

按 TDD 先修改或新增失败测试：

1. GPT-5.4 Priority 为 Standard 的 `2x`。
2. GPT-5.5 Priority 为 Standard 的 `2.5x`。
3. GPT-5.6 Sol / Terra / Luna Priority 均为 Standard 的 `2x`，覆盖输入、缓存读取、缓存写入和输出。
4. `fast` 继续归一化为 `priority`。
5. Flex 继续为 `0.5x`。
6. 范围外模型继续使用现有默认 Priority `1.5x`。
7. 内置价格资源中的目标模型 Priority 字段符合官方价格。
8. OpenAI usage 记录路径对 GPT-5.4 Priority 使用 `2x`。

## 验证范围

- 目标计费单测红绿循环
- `backend/internal/service` 完整 unit
- `backend/internal/pkg/openai`、`backend/internal/handler` unit，确认模型归一和入口行为无回归
- `backend/cmd/server` 测试
- pricing JSON 解析
- `git diff --check`
