# GPT-5.6 完整模型名与 Priority 1.5x 计费实施结果

## 背景

用户确认只有 `service_tier=priority` 按 1.5 倍计费；`Advanced`、`Faster`、`Consumes Usage Limits Faster`、`Smarter` 等思考程度文案不作为独立计费 key，按实际 usage token 和官方基础定价计费。

本轮基于计划 `docs/ai/context/20260710-095810-gpt56-models-priority15-billing-implementation-plan_CN.md` 执行，目标是永久支持三款完整模型名：

- `gpt-5.6-sol`
- `gpt-5.6-terra`
- `gpt-5.6-luna`

不展示、不鼓励、不新增裸 `gpt-5.6` alias。

## 已完成改动

- `backend/internal/pkg/openai/constants.go`：默认 OpenAI 模型列表新增三款完整 GPT-5.6 模型，显示名分别为 `GPT-5.6 Sol`、`GPT-5.6 Terra`、`GPT-5.6 Luna`。
- `backend/internal/pkg/openai/models_test.go`：新增默认模型列表测试，确保包含三款完整名且不包含裸 `gpt-5.6`。
- `backend/internal/handler/gateway_models_test.go`：新增 `/v1/models` OpenAI fallback 测试，确保默认模型响应包含三款完整名且不含裸 alias。
- `backend/internal/service/openai_model_alias.go`、`backend/internal/service/openai_compat_model.go`：支持 `gpt5.6-sol`、`gpt5.6terra`、`gpt_5.6_luna` 等紧凑写法归一到完整名；裸 `gpt-5.6` 与未知 `gpt-5.6-*` 不回退到 GPT-5.4。
- `backend/internal/service/pricing_service.go`：解析 LiteLLM pricing JSON 中的 long context 字段；阻止 `gpt-5.6-terra/luna/unknown` 缺价时误匹配裸 `gpt-5.6` 泛型价。
- `backend/resources/model-pricing/model_prices_and_context_window.json`：新增三款 GPT-5.6 定价资源；`priority` 字段为基础价 1.5 倍，`flex` 字段为基础价 0.5 倍；未新增裸 `gpt-5.6` 条目。
- `backend/internal/service/billing_service.go`：增加三款 GPT-5.6 fallback 定价；长上下文阈值和倍率覆盖三款 GPT-5.6；`priority` 仍由现有 1.5 倍策略统一生效。
- `backend/internal/service/openai_gateway_service_test.go`：新增 usage 解析回归，明确 `output_tokens_details.reasoning_tokens` 已包含在 `output_tokens` 内，不额外叠加扣费。

## 计费结论

- 基础计费仍按 `input_tokens`、cached input、cache write、`output_tokens` 乘模型单价。
- `service_tier=priority` 使用 priority 单价，当前三款模型均为 1.5 倍。
- 其他 reasoning effort / reasoning mode / 产品 UI 文案不改变倍率。
- `reasoning_tokens` 不单独扣费，不与 `output_tokens` 重复相加。
- 数据库不新增每模型字段；继续使用现有 `usage_logs`、`channel_model_pricing`、`channel_pricing_intervals` 等结构。

## 验证结果

已在本地重新运行：

```bash
GOMAXPROCS=2 go test -p=1 -count=1 -tags=unit ./internal/pkg/openai ./internal/handler ./internal/service
```

结果：

```text
ok  	github.com/Wei-Shaw/sub2api/internal/pkg/openai	0.309s
ok  	github.com/Wei-Shaw/sub2api/internal/handler	22.754s
ok  	github.com/Wei-Shaw/sub2api/internal/service	87.795s
```

已运行：

```bash
GOMAXPROCS=2 go test -p=1 -count=1 ./cmd/server
```

结果：

```text
ok  	github.com/Wei-Shaw/sub2api/cmd/server	0.500s
```

已运行：

```bash
python3 -m json.tool backend/resources/model-pricing/model_prices_and_context_window.json >/tmp/sub2api-model-pricing.json
git diff --check
```

结果：两个命令均 exit 0，无错误输出。

## 未执行事项

- 未构建镜像。
- 未部署到公网 18084。
- 未重启容器。
- 未修改 DB、nginx、Redis。
- 未执行真实 `/v1/models` 或三款 GPT-5.6 模型公网请求验收；这些属于发布后验收任务。
- 当前工作区存在另一组 dashboard 前端改动，本轮 GPT-5.6 计费实现未触碰这些前端改动。
