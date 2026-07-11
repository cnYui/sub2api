# OpenAI 官方 Priority 计费修复结果

## 结果

本地分支 `codex/fix-openai-priority-pricing` 已按 OpenAI 官方 Priority 价格修复模型级计费：

- `gpt-5.4` 及日期快照/既有后缀：`2x`
- `gpt-5.5` 及日期快照/既有后缀：`2.5x`
- `gpt-5.6-sol`、`gpt-5.6-terra`、`gpt-5.6-luna`：`2x`
- 其他模型继续使用现有 Priority `1.5x`

`fast` 仍归一为 `priority`。Standard、Flex `0.5x`、长上下文倍率、渠道 flat/interval 定价、分组倍率和 reasoning token 口径均未改变。

## 实现

- 计费核心改为按明确模型族选择 Priority 倍率，旧路径和 `CalculateCostUnified` 共用同一逻辑。
- 模型族匹配不再复用宽泛的 Codex fallback 归一，避免裸 `gpt-5`、`gpt-5.1` 被误识别为 GPT-5.4。
- `gpt-5.4-mini/nano/pro`、`gpt-5.5-pro` 和其他模型继续保持 `1.5x`。
- 内置 pricing JSON 同步更新 GPT-5.5、`gpt-5.5-2026-04-23` 与三款 GPT-5.6 的 Priority 字段；GPT-5.6 同步补齐缓存写入 Priority 字段。

## TDD 与验证

RED 阶段稳定复现：

- 裸 `gpt-5`、`gpt-5.1` 期望 `1.5x`，实际错误为 `2x`。
- `gpt-5.5-2026-04-23` 静态 Priority 输入价期望 `1.25e-05`，实际为旧值 `1e-05`。

GREEN 后验证通过：

- `GOMAXPROCS=2 go test -p=1 -count=1 -tags=unit ./internal/service`，`88.961s`
- `GOMAXPROCS=2 go test -p=1 -count=1 -tags=unit ./internal/pkg/openai ./internal/handler`
- `GOMAXPROCS=2 go test -p=1 -count=1 ./cmd/server`
- `jq empty backend/resources/model-pricing/model_prices_and_context_window.json`
- `git diff --check`

测试覆盖 GPT-5.4/5.5 日期快照、GPT-5.6 紧凑别名、输入/输出/缓存读取/缓存写入、usage 落库、Flex、其他模型 1.5x，以及统一计费下渠道 flat/interval 价格与非 1.0 分组倍率组合。

## 范围

本轮未构建镜像、未部署 18084、未修改 DB、Redis、nginx、容器或运行态配置，未补扣历史 usage。上游实际降级 Priority 后按响应 tier 计费仍不在本次范围内。
