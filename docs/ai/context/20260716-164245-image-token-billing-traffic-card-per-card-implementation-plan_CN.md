# 生图实际 Token 计费与逐张流量卡耗尽提醒实施计划

> 执行代理要求：必须使用 subagent-driven-development（推荐）或 executing-plans 按任务执行；每一步用复选框跟踪，并在每个任务完成后独立提交。

**目标：** 删除旧生图尺寸固定价和独立倍率，让 OpenAI 生图按最终返回的主模型、图片模型实际 Token 统一结算，同时按单张流量卡展示、扣费并投递一次性“流量卡已用完”提醒。

**架构：** OpenAI 各入口先把上游 usage 归一化为主模型和图片模型两个 Token 组件，使用强制 Token 定价解析器与同一个普通有效倍率分别计算，再把价格快照、缺失组件和最终费用写入 durable usage fact。流量卡继续按 credit_id 预留和结算，统一 TrafficCreditPolicy 负责 $0.01 门槛；余额跨过门槛时在同一事务写耗尽事件，/auth/me 只投递事件 ID，前端复用全局右上角 Toast 并批量确认。

**技术栈：** Go、Gin、PostgreSQL、Ent、Wire、Vue 3、Pinia、TypeScript、Vitest、pnpm。

---

## 实施约束

- 以 docs/ai/context/20260716-162311-image-token-billing-traffic-card-per-card-design_CN.md 为唯一需求基线；调研文档中的最低图片价、按张收费和缺失 Token 固定价兜底已经失效。
- 使用 migration 168；若执行时仓库已出现 168，顺延到下一个空号，并同步本文命令和断言。
- 不修改历史 migration 028/134/154-161，避免已部署环境 checksum 和历史回归失真。
- 不连接或修改运行态 PostgreSQL、Redis、Nginx、容器、公网服务和用户数据。
- 当前工作区存在其他任务改动；执行时先使用 using-git-worktrees 创建隔离分支 codex/image-token-billing-traffic-card-events，不得带入或回退现有脏文件。
- “失败请求有 Token 也收费”限定为客户端最终收到的终止尝试：最终成功、失败、不完整或取消只要有任意 Token 就结算；内部被 failover 且未返回客户端的中间尝试不收费。中间尝试可能跨多个账号，现有 request 级 usage fact、reservation 和账号配额无法正确表达多账号 attempt 结算；该扩展另立设计。
- OpenAI 生图必须强制走 Token 模式。非 OpenAI 平台显式配置的通用 per_request/image 渠道能力保留，但不得再读取 groups 的旧三档图片价或独立图片倍率。
- billing.traffic_credit_minimum_reserve_usd 是 $0.01 门槛的唯一事实来源；前端不比较该值，也不自行判断卡片耗尽。

## 文件边界

新建文件：

- backend/migrations/168_image_token_billing_and_traffic_credit_events.sql
- backend/internal/service/openai_usage_billing.go
- backend/internal/service/openai_usage_billing_test.go
- backend/internal/service/traffic_credit_policy.go
- backend/internal/service/traffic_credit_exhaustion.go
- backend/internal/repository/traffic_credit_exhaustion_repo.go
- backend/internal/repository/traffic_credit_exhaustion_repo_integration_test.go
- backend/internal/handler/traffic_credit_exhaustion_handler_test.go
- frontend/src/api/__tests__/user.trafficCreditExhaustion.spec.ts
- frontend/src/views/admin/__tests__/groupsImagePricingRemoval.spec.ts
- 执行完成时在 docs/ai/context/ 新建结果文档，前缀用 date +%Y%m%d-%H%M%S 生成，后缀固定为 image-token-billing-traffic-card-per-card-result_CN.md

主要修改文件：

- 计费与价格：backend/internal/service/billing_service.go、pricing_service.go、model_pricing_resolver.go、openai_traffic_credit_budget.go
- OpenAI usage：backend/internal/service/openai_gateway_service.go、openai_gateway_chat_completions.go、openai_gateway_messages.go、openai_images.go、openai_images_responses.go、openai_ws_forwarder.go、openai_ws_v2/passthrough_relay.go
- usage fact/log：backend/internal/service/usage_fact.go、usage_billing.go、usage_log.go、backend/internal/repository/usage_log_repo.go
- 流量卡：backend/internal/service/traffic_pack.go、traffic_credit_reservation.go、backend/internal/repository/traffic_pack_repo.go、traffic_credit_reservation_repo.go、usage_billing_repo.go
- API/Wire：backend/internal/handler/auth_handler.go、payment_handler.go、user_handler.go、backend/internal/server/routes/user.go、backend/internal/repository/wire.go、backend/internal/service/wire.go、backend/cmd/server/wire_gen.go
- 前端：frontend/src/types/index.ts、types/payment.ts、views/admin/GroupsView.vue、views/user/AvailableChannelsView.vue、views/user/UsageGuideView.vue、views/user/SubscriptionsView.vue、views/user/PaymentView.vue、stores/auth.ts、api/user.ts

### 任务 1：Migration 168 与 Ent schema

**文件：**

- 新建 backend/migrations/168_image_token_billing_and_traffic_credit_events.sql
- 修改 backend/internal/repository/migrations_schema_integration_test.go
- 修改 backend/ent/schema/group.go
- 修改 backend/ent/schema/usage_log.go
- 生成 backend/ent/**

- [ ] 步骤 1：先写 schema 集成失败用例

在 TestMigrationsRunner 中增加以下检查：

~~~go
requireColumn(t, tx, "usage_logs", "image_input_tokens", "integer", 0, false)
requireColumn(t, tx, "usage_logs", "image_input_cost", "numeric", 0, false)
requireColumn(t, tx, "usage_logs", "billing_incomplete", "boolean", 0, false)
requireNoColumn(t, tx, "groups", "image_rate_independent")
requireNoColumn(t, tx, "groups", "image_rate_multiplier")
requireNoColumn(t, tx, "groups", "image_price_1k")
requireNoColumn(t, tx, "groups", "image_price_2k")
requireNoColumn(t, tx, "groups", "image_price_4k")

var eventsRegclass sql.NullString
require.NoError(t, tx.QueryRowContext(
    context.Background(),
    "SELECT to_regclass('public.traffic_credit_exhaustion_events')",
).Scan(&eventsRegclass))
require.True(t, eventsRegclass.Valid)
requireColumn(t, tx, "traffic_credit_exhaustion_events", "credit_id", "bigint", 0, false)
requireColumn(t, tx, "traffic_credit_exhaustion_events", "acknowledged_at", "timestamp with time zone", 0, true)
requireUniqueConstraint(t, tx, "traffic_credit_exhaustion_events", []string{"user_id", "credit_id"})
~~~

若测试文件没有 requireNoColumn，新增一个查询 information_schema.columns 并断言 sql.ErrNoRows 的 helper；不要用字符串搜索 migration 代替真实 schema 断言。

- [ ] 步骤 2：运行集成测试确认失败

~~~bash
cd backend
GOMAXPROCS=2 go test -p=1 -count=1 -tags=integration ./internal/repository -run TestMigrationsRunner
~~~

预期：缺少 migration 168、新 usage 字段和事件表，测试失败。

- [ ] 步骤 3：新增 migration 168

文件内容：

~~~sql
ALTER TABLE usage_logs
    ADD COLUMN IF NOT EXISTS image_input_tokens INTEGER NOT NULL DEFAULT 0,
    ADD COLUMN IF NOT EXISTS image_input_cost DECIMAL(20,10) NOT NULL DEFAULT 0,
    ADD COLUMN IF NOT EXISTS billing_incomplete BOOLEAN NOT NULL DEFAULT FALSE;

CREATE TABLE IF NOT EXISTS traffic_credit_exhaustion_events (
    id BIGSERIAL PRIMARY KEY,
    user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    credit_id BIGINT NOT NULL REFERENCES user_traffic_credits(id) ON DELETE RESTRICT,
    request_id VARCHAR(255) NOT NULL,
    batch_key VARCHAR(255) NOT NULL,
    reason VARCHAR(32) NOT NULL DEFAULT 'depleted',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    acknowledged_at TIMESTAMPTZ NULL,
    CONSTRAINT traffic_credit_exhaustion_events_reason_check CHECK (reason = 'depleted'),
    CONSTRAINT traffic_credit_exhaustion_events_user_credit_unique UNIQUE (user_id, credit_id)
);

CREATE INDEX IF NOT EXISTS idx_traffic_credit_exhaustion_events_pending
    ON traffic_credit_exhaustion_events (user_id, created_at, id)
    WHERE acknowledged_at IS NULL;

ALTER TABLE groups
    DROP COLUMN IF EXISTS image_rate_independent,
    DROP COLUMN IF EXISTS image_rate_multiplier,
    DROP COLUMN IF EXISTS image_price_1k,
    DROP COLUMN IF EXISTS image_price_2k,
    DROP COLUMN IF EXISTS image_price_4k;
~~~

不做历史耗尽事件回填，防止上线后给已耗尽旧卡补弹提醒。

- [ ] 步骤 4：同步 Ent schema 并生成代码

从 backend/ent/schema/group.go 删除五个旧字段；在 backend/ent/schema/usage_log.go 增加：

~~~go
field.Int("image_input_tokens").Default(0),
field.Float("image_input_cost").Default(0),
field.Bool("billing_incomplete").Default(false),
~~~

运行：

~~~bash
cd backend
go generate ./ent
~~~

预期：Ent 生成代码不再出现五个 group 字段，并出现三个 usage log 字段。

- [ ] 步骤 5：运行 migration 测试和格式检查

~~~bash
cd backend
GOMAXPROCS=2 go test -p=1 -count=1 -tags=integration ./internal/repository -run TestMigrationsRunner
git diff --check
~~~

预期：测试通过，diff 无空白错误。

- [ ] 步骤 6：提交

~~~bash
git add backend/migrations/168_image_token_billing_and_traffic_credit_events.sql backend/internal/repository/migrations_schema_integration_test.go backend/ent/schema/group.go backend/ent/schema/usage_log.go backend/ent
git commit -m "feat: add image token billing schema"
~~~

### 任务 2：删除后端旧图片固定价与独立倍率

**文件：** backend/internal/service/group.go、group_service.go、admin_service.go、api_key_auth_cache.go、api_key_auth_cache_impl.go、backend/internal/repository/group_repo.go、api_key_repo.go、backend/internal/handler/admin/group_handler.go、backend/internal/handler/dto/types.go、mappers.go、backend/migrations/tools/generate-subscription-plan.sh；删除 backend/internal/service/image_billing_multiplier.go；测试 group_test.go、admin_service_group_test.go、gateway_record_usage_test.go、api_contract_test.go。

- [ ] 步骤 1：把旧字段契约测试改成失败断言

删除测试 fixture 中的五个旧字段，并新增源码/API 契约断言：创建、更新和响应 JSON 均不得包含 image_rate_independent、image_rate_multiplier、image_price_1k、image_price_2k、image_price_4k；allow_image_generation 必须继续存在。gateway_record_usage_test.go 断言通用 Gateway 不再读取 group 三档图片价或独立图片倍率。

- [ ] 步骤 2：运行目标测试确认失败

~~~bash
cd backend
GOMAXPROCS=2 go test -p=1 -count=1 -tags=unit ./internal/service ./internal/handler -run 'Test(Group|AdminService.*Group|Gateway.*Image|APIContract)'
~~~

预期：旧字段仍存在，测试失败或编译失败。

- [ ] 步骤 3：删除领域、DTO、缓存和仓储字段

从上述 service、handler、repository 和 API Key 鉴权缓存快照中完整删除五个字段及其校验、默认值、复制、扫描、Set/Select 逻辑。保留 allow_image_generation。删除 resolveImageRateMultiplier，所有图片请求统一使用已解析的普通 RateMultiplier。

- [ ] 步骤 4：删除旧固定价 helper，收敛通用 Gateway

从 billing_service.go 删除 ImagePriceConfig、CalculateImageCost、getImageUnitPrice 以及只服务于三档固定价的硬编码表。GatewayService 图片结果改为调用 CalculateCostUnified，允许非 OpenAI 渠道显式配置 per_request/image；无显式配置时按 Token 计费。不得重新构造 group 三档价格。

~~~go
cost, err := s.billingService.CalculateCostUnified(CostInput{
    Ctx:            ctx,
    Model:          billingModel,
    GroupID:        apiKey.GroupID,
    Tokens:         tokens,
    RequestCount:   result.ImageCount,
    SizeTier:       NormalizeImageBillingTierOrDefault(result.ImageSize),
    RateMultiplier: multiplier,
    Resolver:       s.resolver,
})
~~~

- [ ] 步骤 5：更新未来套餐 migration 生成脚本

从 backend/migrations/tools/generate-subscription-plan.sh 的 INSERT、SELECT、UPSERT 中删除五个字段。历史 migration 及 auth_identity_payment_migrations_regression_test.go 对历史 SQL 的断言保持不变。

- [ ] 步骤 6：运行目标测试

~~~bash
cd backend
GOMAXPROCS=2 go test -p=1 -count=1 -tags=unit ./internal/service ./internal/handler -run 'Test(Group|AdminService.*Group|Gateway.*Image|APIContract)'
git diff --check
~~~

预期：测试通过，rg 只在历史 migration、历史 migration 回归测试和本次设计/计划文档中命中旧字段。

- [ ] 步骤 7：提交

~~~bash
git add backend/internal backend/migrations/tools/generate-subscription-plan.sh
git commit -m "refactor: remove legacy image group pricing"
~~~

### 任务 3：删除前端旧图片价格控件和说明

**文件：** frontend/src/types/index.ts、frontend/src/views/admin/GroupsView.vue、frontend/src/views/user/AvailableChannelsView.vue、frontend/src/views/user/UsageGuideView.vue、新建 frontend/src/views/admin/__tests__/groupsImagePricingRemoval.spec.ts、修改对应 user tests。

- [ ] 步骤 1：先写前端失败测试

新增 groupsImagePricingRemoval.spec.ts，断言页面保留“允许图片生成”，但不出现独立图片倍率和 1K/2K/4K 固定价控件。更新用户侧测试，断言：

~~~ts
expect(wrapper.text()).not.toContain('$0.10 / 张')
expect(wrapper.text()).not.toContain('$0.20 / 张')
expect(wrapper.text()).not.toContain('$0.40 / 张')
expect(wrapper.text()).toContain('按实际 Token 用量和套餐有效倍率计费')
~~~

- [ ] 步骤 2：运行测试确认失败

~~~bash
pnpm --dir frontend exec vitest run src/views/admin/__tests__/groupsImagePricingRemoval.spec.ts src/views/user/__tests__/AvailableChannelsView.spec.ts src/views/user/__tests__/UsageGuideView.spec.ts
~~~

预期：旧 UI 和文案仍存在，测试失败。

- [ ] 步骤 3：删除类型与管理端控件

从 Group、CreateGroupRequest、UpdateGroupRequest 删除五个旧字段。GroupsView.vue 删除创建/编辑表单对应控件、state、reset、hydrate、payload normalize 和价格预览 helper，只保留 allow_image_generation。

- [ ] 步骤 4：更新用户侧价格展示

AvailableChannelsView.vue 删除 group 三档价格请求和分支；模型有图片 Token 单价时按 / 1M Token 展示，没有可确认 Token 单价时不构造固定 /张价格。UsageGuideView.vue 删除三档价格表和只为该表服务的类型/CSS，文案为：

~~~text
图片生成按上游实际返回的 Token 用量和套餐有效倍率计费；图片数量和文件大小不作为单独收费单位。
~~~

- [ ] 步骤 5：运行前端目标测试和类型检查

~~~bash
pnpm --dir frontend exec vitest run src/views/admin/__tests__/groupsImagePricingRemoval.spec.ts src/views/user/__tests__/AvailableChannelsView.spec.ts src/views/user/__tests__/UsageGuideView.spec.ts
pnpm --dir frontend typecheck
~~~

- [ ] 步骤 6：提交

~~~bash
git add frontend/src/types/index.ts frontend/src/views/admin/GroupsView.vue frontend/src/views/admin/__tests__/groupsImagePricingRemoval.spec.ts frontend/src/views/user/AvailableChannelsView.vue frontend/src/views/user/UsageGuideView.vue frontend/src/views/user/__tests__/AvailableChannelsView.spec.ts frontend/src/views/user/__tests__/UsageGuideView.spec.ts
git commit -m "refactor: remove fixed image pricing ui"
~~~

### 任务 4：建立强制 Token 定价组件和价格快照

**文件：** 新建 backend/internal/service/openai_usage_billing.go、openai_usage_billing_test.go；修改 billing_service.go、pricing_service.go、model_pricing_resolver.go、usage_fact.go；测试 billing_service_test.go、pricing_service_test.go、model_pricing_resolver_test.go。

- [ ] 步骤 1：先写组件计费失败测试

使用 input 130、image input 20、output 70、cache read 10、cache creation 5、image output 40 的 usage，断言主组件只计算普通输入 100、缓存读取 10、缓存创建 5、文本输出 30；图片组件只计算图片输入 20 和图片输出 40。两组件使用不同模型价格但相同普通有效倍率，多图 ImageCount=3 不改变费用；图片模型为空时使用 gpt-image-2。价格加载测试直接读取项目真实 gpt-image-2 JSON，断言图片输入和输出 Token 价格进入运行时价格。

- [ ] 步骤 2：运行目标测试确认失败

~~~bash
cd backend
GOMAXPROCS=2 go test -p=1 -count=1 -tags=unit ./internal/service -run 'Test(OpenAIUsageBilling|PricingService.*ImageToken|BillingService.*ImageInput|ModelPricingResolver.*ForceToken)'
~~~

预期：新类型和强制 Token 解析入口不存在，测试失败。

- [ ] 步骤 3：扩展价格模型与成本明细

给 LiteLLMRawEntry、LiteLLMModelPricing 和 ModelPricing 增加 input_cost_per_image_token；raw struct 使用指针，runtime 使用值，最终写入 ModelPricing.ImageInputPricePerToken。给 CostBreakdown 增加 ImageInputCost，computeTokenBreakdown 将图片输入费用写入该字段，普通文本输入继续写 InputCost，TotalCost 包含两者。

- [ ] 步骤 4：增加只允许 Token 模式的解析入口

增加 ModelPricingResolver.ResolveToken(ctx, input)。该方法始终先解析 LiteLLM/fallback Token 价格；只有渠道配置自身是 BillingModeToken 时才应用 token override。渠道 per_request/image 不得覆盖 OpenAI 生图 Token 结算。

~~~go
func (r *ModelPricingResolver) ResolveToken(ctx context.Context, input PricingInput) *ResolvedPricing
~~~

- [ ] 步骤 5：定义组件、缺失标记和快照

openai_usage_billing.go 定义以下类型：

~~~go
type OpenAIUsagePresence struct {
    Input bool
    CacheCreation bool
    CacheRead bool
    Output bool
    ImageInput bool
    ImageOutput bool
}

type OpenAIUsageExpectation struct {
    Input bool
    CacheRead bool
    Output bool
    ImageInput bool
    ImageOutput bool
}

type OpenAIBillingComponent struct {
    Kind string
    Model string
    Tokens UsageTokens
}

type OpenAIModelPricingSnapshot struct {
    Source string
    InputPricePerToken float64
    ImageInputPricePerToken float64
    OutputPricePerToken float64
    CacheCreationPricePerToken float64
    CacheReadPricePerToken float64
    ImageOutputPricePerToken float64
}

type OpenAIBillingComponentSnapshot struct {
    Component OpenAIBillingComponent
    Pricing OpenAIModelPricingSnapshot
    Cost CostBreakdown
}

type OpenAIUsageBillingSnapshot struct {
    Components []OpenAIBillingComponentSnapshot
    ServiceTier string
    RateMultiplier float64
    MissingUsageComponents []string
    BillingIncomplete bool
}
~~~

新增纯函数 BuildOpenAIBillingComponents、MergeCostBreakdowns、MissingOpenAIUsageComponents 和 HasBillableOpenAIUsage。组件构建使用：

~~~go
mainInput := maxInt(usage.InputTokens-usage.CacheReadInputTokens-usage.ImageInputTokens, 0)
textOutput := maxInt(usage.OutputTokens-usage.ImageOutputTokens, 0)
imageModel := strings.TrimSpace(imageBillingModel)
if imageModel == "" {
    imageModel = "gpt-image-2"
}
~~~

同文件定义最小 helper，避免引用不存在的函数：

~~~go
func maxInt(value, floor int) int {
    if value < floor {
        return floor
    }
    return value
}
~~~

组件合并不得使用 ImageCount。

- [ ] 步骤 6：把价格快照写入 usage fact payload

给 UsageFactPayload 增加可选 OpenAIBilling 字段，旧 payload 解码时该字段为 nil。worker 按 payload 中已经算好的 BillingCommand 扣费，不重新读取实时价格。

- [ ] 步骤 7：运行计费测试

~~~bash
cd backend
GOMAXPROCS=2 go test -p=1 -count=1 -tags=unit ./internal/service -run 'Test(OpenAIUsageBilling|PricingService.*ImageToken|BillingService.*ImageInput|ModelPricingResolver.*ForceToken|UsageFact)'
~~~

- [ ] 步骤 8：提交

~~~bash
git add backend/internal/service/openai_usage_billing.go backend/internal/service/openai_usage_billing_test.go backend/internal/service/billing_service.go backend/internal/service/billing_service_test.go backend/internal/service/pricing_service.go backend/internal/service/pricing_service_test.go backend/internal/service/model_pricing_resolver.go backend/internal/service/model_pricing_resolver_test.go backend/internal/service/usage_fact.go
git commit -m "feat: add token component image billing"
~~~

### 任务 5：统一 JSON、SSE、WS、Images usage 与终止状态

**文件：** openai_gateway_service.go、openai_gateway_chat_completions.go、openai_gateway_messages.go、openai_images.go、openai_images_responses.go、openai_ws_forwarder.go、openai_ws_v2/passthrough_relay.go、handler/usage_fact_response_gate.go、相关 handlers 和测试。

- [ ] 步骤 1：先写 parser 和失败终止测试

覆盖 usage fixture：input_tokens 120、output_tokens 80、cached_tokens 10、image input 20、image output 50；字段显式为 0、字段缺失、response.failed、response.incomplete、response.cancelled、image_generation.completed、image_edit.completed、Images error 携带 usage、WS passthrough 图片输入/输出 Token 和图片工具模型。handler 测试证明 0 张图但任意 Token 大于 0 的最终失败结果仍持久化 fact；完全无 Token 的失败结果不产生费用 fact。

- [ ] 步骤 2：运行目标测试确认失败

~~~bash
cd backend
GOMAXPROCS=2 go test -p=1 -count=1 -tags=unit ./internal/service ./internal/handler -run 'Test(OpenAIUsageFrom|.*Failed.*Usage|.*Incomplete.*Usage|.*Cancelled.*Usage|.*Image.*Usage|UsageFactResponseGate)'
~~~

- [ ] 步骤 3：扩展 ForwardResult，删除单一 BillingModel 语义

OpenAIForwardResult 增加 MainBillingModel、ImageBillingModel、UsagePresence、UsageExpectation、TerminalStatus、BillingSnapshot。迁移所有调用方后删除 BillingModel。主模型映射只写 MainBillingModel；响应 tools[].model、image generation 事件写 ImageBillingModel，无法确认时留空并在计费组件构建时回退 gpt-image-2。

- [ ] 步骤 4：统一 parser 签名和字段存在性

~~~go
func openAIUsageFromGJSON(value gjson.Result) (OpenAIUsage, OpenAIUsagePresence, bool)
~~~

按顺序读取 input_tokens/prompt_tokens、output_tokens/completion_tokens、input_tokens_details.cached_tokens/prompt_tokens_details.cached_tokens、input_tokens_details.image_tokens/prompt_tokens_details.image_tokens、output_tokens_details.image_tokens/completion_tokens_details.image_tokens。存在性使用 gjson.Result.Exists()，不能用数值是否为 0 推断。

- [ ] 步骤 5：让最终失败结果保留 usage

buffered、stream、SSE-to-JSON、Chat converted、Messages、Images native/OAuth 路径在最终终止错误时返回 result, err，不得先把 result 置为 nil。内部继续 failover 时丢弃中间 result；只有决定不再重试、已经向客户端 flush 或最终成功的尝试才持久化，并使用对应尝试的 account。

- [ ] 步骤 6：扩展 HTTP/SSE 终止帧屏障

usage_fact_response_gate.go 增加 response.failed、response.incomplete、response.cancelled、response.canceled、image_generation.completed、image_edit.completed、error。覆盖拆帧、多个 data 行、持久化失败不放行终止帧；普通 delta/partial image 继续实时透传。

- [ ] 步骤 7：给 WS 增加终止前 usage fact barrier

增加窄接口：

~~~go
type OpenAIWSTerminalUsageBarrier interface {
    BeforeTerminal(context.Context, *OpenAIForwardResult) error
}
~~~

forwarder 在发送 terminal frame 前完成 usage 解析并调用 barrier；失败时不发送终止帧，并关闭连接。HTTP handler 构造的 barrier 捕获当前用户、Key、订阅和账号，调用现有 PersistUsageFact，不在 WS service 内重建计费上下文。

- [ ] 步骤 8：运行 parser、终止状态和 handler 测试

~~~bash
cd backend
GOMAXPROCS=2 go test -p=1 -count=1 -tags=unit ./internal/service ./internal/handler -run 'Test(OpenAIUsageFrom|.*Failed.*Usage|.*Incomplete.*Usage|.*Cancelled.*Usage|.*Image.*Usage|.*WSTerminal|UsageFactResponseGate)'
~~~

- [ ] 步骤 9：提交

~~~bash
git add backend/internal/service backend/internal/handler
git commit -m "feat: preserve image usage on terminal responses"
~~~

### 任务 6：接入双模型结算、usage log 和 Images 请求前预留

**文件：** openai_gateway_service.go、openai_traffic_credit_budget.go、openai_billing_authorization.go、usage_log.go、usage_billing.go、repository/usage_log_repo.go、handler/openai_images.go 和对应测试。

- [ ] 步骤 1：先写双模型结算与日志失败测试

证明 Responses 主模型与 gpt-image-2 分别取价后求和；ImageCount=1 和 4 在 Token 相同情况下费用相同；所有组件只乘一次普通有效倍率；usage log 写入 image_input_tokens、image_input_cost、image_output_tokens、image_output_cost、billing_incomplete；缺失图片输入按 0 并记录 missing_usage_components；最终失败有 Token 时生成非零 BillingCommand，无 Token 时不生成收费 command。

- [ ] 步骤 2：运行测试确认失败

~~~bash
cd backend
GOMAXPROCS=2 go test -p=1 -count=1 -tags=unit ./internal/service ./internal/repository -run 'Test(OpenAIGatewayService.*Usage|OpenAITrafficCreditBudget.*Image|OpenAIBillingAuthorization.*Image|UsageLog.*ImageInput)'
~~~

- [ ] 步骤 3：替换 OpenAI 计费入口

buildOpenAIUsageRecord 使用任务 4 的组件构建器和 ResolveToken，分别计算后合并。删除 calculateOpenAIImageCost、图片倍率分支和 OpenAI 对 BillingModeImage/PerRequest 的读取。usageLog.RateMultiplier 始终是普通有效倍率，BillingMode 固定为 token。

~~~go
components := BuildOpenAIBillingComponents(result.Usage, result.MainBillingModel, result.ImageBillingModel)
snapshot, cost, err := s.calculateOpenAIUsageBillingSnapshot(ctx, apiKey, components, multiplier, serviceTier, missing)
~~~

- [ ] 步骤 4：扩展 usage log 与 raw SQL repository

UsageLog 和 UsageBillingCommand 增加 ImageInputTokens、ImageInputCost、ImageOutputTokens、BillingIncomplete；同步 usage_log_repo.go 的 SELECT、INSERT、参数顺序、scan 和导出查询。更新 TotalTokens，使图片输入/输出不会因已经包含在总 input/output 中再次相加。

- [ ] 步骤 5：扩展图片预留预算

给预算输入增加 ImageModel、ImageInputTokenUpperBound、ImageOutputTokenUpperBound、DoNotClampOutputLimit。预算费用等于主模型输出上限费用加图片模型输入/输出上限费用。PricingSnapshot 必须保存 request_id、request_fingerprint、最终出站 body 的 SHA-256、主/图片模型名、service tier、普通有效倍率、输入/输出上限、两套实际价格和 reserve_usd，不能只保存最终金额。已知 Images 请求用 size、quality、n、partial_images 计算图片输出 Token 上限；标准表：

~~~go
var gptImage2OutputTokenUpperBounds = map[string]map[string]int{
    "1024x1024": {"low": 196, "medium": 1756, "high": 7024},
    "1536x1024": {"low": 158, "medium": 1372, "high": 5488},
    "1024x1536": {"low": 158, "medium": 1372, "high": 5488},
}
const gptImage2UnknownOutputTokenUpperBound = 23719
~~~

未知或 auto 尺寸/质量使用 23719 × max(n,1)，只影响预留，不是最终收费。partial image 每张增加 100 Token。图片编辑输入无法准确预估时使用配置内保守上限，并写入预留快照；响应后仍按实际 Token 释放差额。

- [ ] 步骤 6：Dedicated Images 在上游调用前预授权

暴露窄方法 AuthorizeImagesRequest(ctx, c, parsed, body, mappedModel)，handler 在 account loop 之前调用一次；failover 复用同一 authorization。每次真正发出上游请求前 MarkDispatched，最终 result 写入同一个 BillingAuthorization；上游前失败释放 reservation，上游状态未知保持 unknown。

- [ ] 步骤 7：运行双模型结算、日志和预留测试

~~~bash
cd backend
GOMAXPROCS=2 go test -p=1 -count=1 -tags=unit ./internal/service ./internal/repository ./internal/handler -run 'Test(OpenAIGatewayService.*Usage|OpenAITrafficCreditBudget.*Image|OpenAIBillingAuthorization.*Image|OpenAIImages.*Authorization|UsageLog.*ImageInput)'
~~~

- [ ] 步骤 8：提交

~~~bash
git add backend/internal/service backend/internal/repository/usage_log_repo.go backend/internal/handler/openai_images.go
git commit -m "feat: settle image requests from actual tokens"
~~~

### 任务 7：统一 $0.01 流量卡策略与逐卡查询

**文件：** 新建 traffic_credit_policy.go；修改 traffic_pack.go、traffic_credit_reservation.go、traffic_pack_repo.go、traffic_credit_reservation_repo.go、usage_billing_repo.go、service/wire.go；测试 traffic_pack_test.go、traffic_pack_repo_test.go、traffic_credit_reservation_repo_integration_test.go。

- [ ] 步骤 1：先写单卡门槛和展示失败测试

覆盖 remaining=0.0100000001 可参与，remaining=0.01 和 0.0099999999 不参与；remaining=1.00、reserved=1.00 仍出现在逐卡列表且 available_usd=0；多卡按 expires_at、credited_at、id，不能把多张 <=0.01 零头聚合成可用额度；已有 reservation item 即使结算时卡片到门槛也继续结算。

- [ ] 步骤 2：运行目标测试确认失败

~~~bash
cd backend
GOMAXPROCS=2 go test -p=1 -count=1 -tags=unit ./internal/service ./internal/repository -run 'Test.*TrafficCredit.*(Threshold|List|Plan)'
GOMAXPROCS=2 go test -p=1 -count=1 -tags=integration ./internal/repository -run TestTrafficCreditReservationRepository
~~~

- [ ] 步骤 3：新增单一策略对象

~~~go
type TrafficCreditPolicy struct {
    MinimumReserveUSD float64
}

func (p TrafficCreditPolicy) IsDepleted(remainingUSD float64) bool {
    return roundTrafficCreditAmount(remainingUSD) <= roundTrafficCreditAmount(p.MinimumReserveUSD)
}

func (p TrafficCreditPolicy) AvailableUSD(remainingUSD, reservedUSD float64) float64 {
    if p.IsDepleted(remainingUSD) {
        return 0
    }
    return roundTrafficCreditAmount(math.Max(remainingUSD-reservedUSD, 0))
}
~~~

Wire 从 cfg.Billing.TrafficCreditMinimumReserveUSD 构造 policy，并注入三个 repository 和预算器；不得在 SQL 之外硬编码 0.01。

- [ ] 步骤 4：统一 SQL 和规划语义

GetSummary、HasAvailableCredit、reservation GetAvailableUSD/Reserve、旧直扣和超预留补扣查询增加 remaining_usd > threshold。逐卡列表只过滤 remaining_usd > threshold 且 expires_at > now，返回 available_usd = GREATEST(remaining_usd - reserved_usd, 0)。不要给 reservation item 结算查询加门槛过滤。

- [ ] 步骤 5：运行门槛和集成测试

~~~bash
cd backend
GOMAXPROCS=2 go test -p=1 -count=1 -tags=unit ./internal/service ./internal/repository -run 'Test.*TrafficCredit.*(Threshold|List|Plan)'
GOMAXPROCS=2 go test -p=1 -count=1 -tags=integration ./internal/repository -run TestTrafficCreditReservationRepository
~~~

- [ ] 步骤 6：提交

~~~bash
git add backend/internal/service/traffic_credit_policy.go backend/internal/service/traffic_pack.go backend/internal/service/traffic_credit_reservation.go backend/internal/repository/traffic_pack_repo.go backend/internal/repository/traffic_credit_reservation_repo.go backend/internal/repository/usage_billing_repo.go backend/internal/service/wire.go
git commit -m "refactor: centralize traffic credit threshold"
~~~

### 任务 8：在结算事务中创建逐卡耗尽事件

**文件：** 新建 traffic_credit_exhaustion.go、traffic_credit_exhaustion_repo.go、traffic_credit_exhaustion_repo_integration_test.go；修改 usage_billing_repo.go、traffic_pack_repo.go、repository/wire.go；测试 usage_billing_repo_integration_test.go、traffic_pack_repo_integration_test.go。

- [ ] 步骤 1：先写事件事务失败测试

覆盖 reservation item 实扣、超预留补扣、无 reservation 兼容直扣：before > 0.01 且 after <= 0.01 创建一条事件；未跨门槛不创建；同一请求耗尽多卡时每卡一条且 batch_key 相同；同一 credit_id 重放只保留一条；后续 ledger/配额更新失败时余额和事件一起回滚；自然过期与 reservation 释放不创建；新卡 INSERT 成功时同事务确认旧 pending 事件。

- [ ] 步骤 2：运行集成测试确认失败

~~~bash
cd backend
GOMAXPROCS=2 go test -p=1 -count=1 -tags=integration ./internal/repository -run 'Test(UsageBillingRepository.*TrafficCreditExhaustion|TrafficPackRepository.*AcknowledgesOldExhaustion)'
~~~

- [ ] 步骤 3：定义事件服务契约

~~~go
type TrafficCreditExhaustionNotice struct {
    EventIDs []int64
}

type TrafficCreditExhaustionRepository interface {
    ListPendingEventIDs(ctx context.Context, userID int64) ([]int64, error)
    AcknowledgeEvents(ctx context.Context, userID int64, eventIDs []int64, now time.Time) error
}
~~~

service 负责去重、正整数和空列表校验；repository ack 在同一事务先确认所有 ID 属于当前用户，再整体更新，已确认事件允许幂等重试。

- [ ] 步骤 4：在三条扣费路径写事件

抽取事务内 helper：

~~~go
func recordTrafficCreditExhaustion(
    ctx context.Context,
    tx *sql.Tx,
    policy service.TrafficCreditPolicy,
    userID int64,
    creditID int64,
    requestID string,
    batchKey string,
    beforeUSD float64,
    afterUSD float64,
) error
~~~

仅在未耗尽的 before 跨到已耗尽的 after 时执行 ON CONFLICT (user_id, credit_id) DO NOTHING。扣费 UPDATE 使用 CTE 锁定并返回 before_usd/after_usd，不能用浮点推算旧余额。batch_key 固定为 request_id + ':' + api_key_id。

- [ ] 步骤 5：购卡履约确认旧 pending

新卡 INSERT 和 purchase ledger 都成功后、事务提交前执行：

~~~sql
UPDATE traffic_credit_exhaustion_events
SET acknowledged_at = $2
WHERE user_id = $1 AND acknowledged_at IS NULL
~~~

- [ ] 步骤 6：运行事务集成测试

~~~bash
cd backend
GOMAXPROCS=2 go test -p=1 -count=1 -tags=integration ./internal/repository -run 'Test(UsageBillingRepository.*TrafficCreditExhaustion|TrafficPackRepository.*AcknowledgesOldExhaustion|TrafficCreditExhaustionRepository)'
~~~

- [ ] 步骤 7：提交

~~~bash
git add backend/internal/service/traffic_credit_exhaustion.go backend/internal/repository/traffic_credit_exhaustion_repo.go backend/internal/repository/traffic_credit_exhaustion_repo_integration_test.go backend/internal/repository/usage_billing_repo.go backend/internal/repository/usage_billing_repo_integration_test.go backend/internal/repository/traffic_pack_repo.go backend/internal/repository/traffic_pack_repo_integration_test.go backend/internal/repository/wire.go
git commit -m "feat: record per-card traffic credit exhaustion"
~~~

### 任务 9：暴露逐卡 checkout、notice 和 ack API

**文件：** traffic_pack.go、payment_service.go、payment_handler.go、auth_handler.go、user_handler.go、server/routes/user.go、service/wire.go、cmd/server/wire_gen.go；新建 traffic_credit_exhaustion_handler_test.go；测试 payment_handler_checkout_test.go、auth_current_user_test.go。

- [ ] 步骤 1：先写 API 失败测试

覆盖 checkout 返回两张卡且保持 repository 顺序；完全预留卡仍返回且 available_usd=0；/auth/me 有 pending 时返回事件 IDs，无 pending 时省略字段，查询失败仍 HTTP 200；ack 空列表、负数、越权返回参数错误且不部分确认，合法 IDs 一次确认，重复确认成功。

- [ ] 步骤 2：运行 handler 测试确认失败

~~~bash
cd backend
GOMAXPROCS=2 go test -p=1 -count=1 -tags=unit ./internal/handler -run 'Test(PaymentHandler.*TrafficCredits|AuthHandler.*TrafficCreditExhaustion|TrafficCreditExhaustionHandler)'
~~~

- [ ] 步骤 3：定义逐卡返回类型和 service 方法

~~~go
type TrafficCredit struct {
    ID int64
    OrderID *int64
    PackID *int64
    InitialUSD float64
    RemainingUSD float64
    ReservedUSD float64
    AvailableUSD float64
    CreditedAt time.Time
    ExpiresAt time.Time
}

func (s *TrafficPackService) ListUserCredits(ctx context.Context, userID int64, now time.Time) ([]TrafficCredit, error)
~~~

PaymentService 只转发该查询；checkoutInfoResponse 增 traffic_credits，空值返回 [] 而不是 null；保留 summary 兼容其他页面。

- [ ] 步骤 4：给 /auth/me 增可选 notice

Auth handler 局部响应增加 traffic_credit_exhaustion_notice，查询失败只记录日志并省略字段，不影响登录/鉴权响应。

- [ ] 步骤 5：增加批量 ack 路由

请求体为 event_ids 数组，校验非空、正整数、全部属于当前用户；注册 POST /user/traffic-credit-exhaustion-events/ack，成功返回 204，错误使用现有参数错误响应。

- [ ] 步骤 6：更新 Wire 并运行测试

~~~bash
cd backend
go generate ./cmd/server
GOMAXPROCS=2 go test -p=1 -count=1 -tags=unit ./internal/handler -run 'Test(PaymentHandler.*TrafficCredits|AuthHandler.*TrafficCreditExhaustion|TrafficCreditExhaustionHandler)'
GOMAXPROCS=2 go test -p=1 -count=1 ./cmd/server
~~~

- [ ] 步骤 7：提交

~~~bash
git add backend/internal/service/traffic_pack.go backend/internal/service/payment_service.go backend/internal/handler/payment_handler.go backend/internal/handler/auth_handler.go backend/internal/handler/user_handler.go backend/internal/server/routes/user.go backend/internal/service/wire.go backend/cmd/server/wire_gen.go backend/internal/handler/traffic_credit_exhaustion_handler_test.go backend/internal/handler/payment_handler_checkout_test.go backend/internal/handler/auth_current_user_test.go
git commit -m "feat: expose traffic credit exhaustion notice"
~~~

### 任务 10：前端逐张展示流量卡

**文件：** frontend/src/types/payment.ts、frontend/src/views/user/SubscriptionsView.vue、frontend/src/views/user/PaymentView.vue、对应前端测试。

- [ ] 步骤 1：先写逐卡展示失败测试

fixture 返回两张不同 id 和 expires_at 的卡，断言保持 API 顺序，分别显示真实初始额度、已结算用量、当前可用额度和到期时间；后端只返回剩余卡时页面只显示该卡；空数组且无订阅进入空态。

- [ ] 步骤 2：运行测试确认失败

~~~bash
pnpm --dir frontend exec vitest run src/views/user/__tests__/SubscriptionsView.spec.ts src/views/user/__tests__/PaymentView.spec.ts
~~~

- [ ] 步骤 3：增加逐卡 TypeScript 类型

~~~ts
export interface TrafficCredit {
  id: number
  order_id: number | null
  pack_id: number | null
  initial_usd: number
  remaining_usd: number
  reserved_usd: number
  available_usd: number
  credited_at: string
  expires_at: string
}
~~~

CheckoutInfoResponse 增 traffic_credits: TrafficCredit[]，保留 traffic_credit_summary。

- [ ] 步骤 4：逐卡渲染 SubscriptionUsageCard

SubscriptionsView 使用 trafficCredits ref 和 API 顺序，已使用为 max(initial_usd - remaining_usd, 0)，当前可用直接显示 available_usd；模板用 key=credit.id；真实到期复用 formatExpirationDate。不要排序、不要比较 $0.01、不要从在售 pack 反查名称。

- [ ] 步骤 5：补齐 PaymentView fixture 和加载字段

PaymentView 不渲染逐卡列表，只在 checkout state/reload 中保存 traffic_credits: response.data.traffic_credits ?? []。

- [ ] 步骤 6：运行测试与类型检查

~~~bash
pnpm --dir frontend exec vitest run src/views/user/__tests__/SubscriptionsView.spec.ts src/views/user/__tests__/PaymentView.spec.ts
pnpm --dir frontend typecheck
~~~

- [ ] 步骤 7：提交

~~~bash
git add frontend/src/types/payment.ts frontend/src/views/user/SubscriptionsView.vue frontend/src/views/user/PaymentView.vue frontend/src/views/user/__tests__/SubscriptionsView.spec.ts frontend/src/views/user/__tests__/PaymentView.spec.ts
git commit -m "feat: show traffic credits per card"
~~~

### 任务 11：前端首次 Toast 与批量确认

**文件：** frontend/src/types/index.ts、frontend/src/api/user.ts、新建 frontend/src/api/__tests__/user.trafficCreditExhaustion.spec.ts、frontend/src/stores/auth.ts、frontend/src/stores/__tests__/auth.spec.ts。

- [ ] 步骤 1：先写 Toast/ack 失败测试

覆盖单 ID 一次 Toast；多 ID 一次 Toast加一次批量 ack；相同 ID 再刷新不弹但再次 ack；旧 ID 加新 ID 时再弹一次；notice 不进入 auth_user；密码登录、2FA 登录和 OAuth setToken 后立即请求 /auth/me；无 notice 不弹。

- [ ] 步骤 2：运行测试确认失败

~~~bash
pnpm --dir frontend exec vitest run src/api/__tests__/user.trafficCreditExhaustion.spec.ts src/stores/__tests__/auth.spec.ts
~~~

- [ ] 步骤 3：增加 notice 类型与 ack API

只扩展 CurrentUserResponse，不污染持久化 User：

~~~ts
export interface TrafficCreditExhaustionNotice {
  event_ids: number[]
}

export interface CurrentUserResponse extends User {
  run_mode?: 'standard' | 'simple'
  traffic_credit_exhaustion_notice?: TrafficCreditExhaustionNotice
}

export async function ackTrafficCreditExhaustionEvents(eventIds: number[]): Promise<void> {
  await apiClient.post('/user/traffic-credit-exhaustion-events/ack', { event_ids: eventIds })
}
~~~

- [ ] 步骤 4：在 auth store 处理一次性 Toast

新增会话内 Set<number>。处理流程：规范化并去重正整数；存在新 ID 时先写入 Set，再调用 useAppStore().showError('流量卡已用完')；每次非空 notice 都调用批量 ack，ack 失败保留下一次重试机会；clearAuth 清空 Set。refreshUser 解构时剥离 run_mode 和 traffic_credit_exhaustion_notice，不能写入 user/localStorage。

- [ ] 步骤 5：登录后立即刷新 /auth/me

login、login2FA 成功后非阻断调用 refreshUser；checkAuth、setToken 已立即刷新，保持单一入口。不要修改 app store 或 Toast 组件，不按文案全局去重。

- [ ] 步骤 6：运行测试与类型检查

~~~bash
pnpm --dir frontend exec vitest run src/api/__tests__/user.trafficCreditExhaustion.spec.ts src/stores/__tests__/auth.spec.ts
pnpm --dir frontend typecheck
~~~

- [ ] 步骤 7：提交

~~~bash
git add frontend/src/types/index.ts frontend/src/api/user.ts frontend/src/api/__tests__/user.trafficCreditExhaustion.spec.ts frontend/src/stores/auth.ts frontend/src/stores/__tests__/auth.spec.ts
git commit -m "feat: notify exhausted traffic credits once"
~~~

### 任务 12：完整回归、独立审查和项目记忆

**文件：** 在 docs/ai/context/ 新建结果文档，文件名前缀由 date +%Y%m%d-%H%M%S 生成，后缀固定为 image-token-billing-traffic-card-per-card-result_CN.md；修改 AGENTS.md。

- [ ] 步骤 1：运行后端完整目标回归

~~~bash
cd backend
GOMAXPROCS=2 go test -p=1 -count=1 -tags=unit ./internal/service ./internal/handler ./internal/repository ./internal/server
GOMAXPROCS=2 go test -p=1 -count=1 -tags=integration ./internal/repository -run 'Test(MigrationsRunner|TrafficPackRepository|TrafficCreditReservationRepository|TrafficCreditExhaustionRepository|UsageBillingRepository)'
GOMAXPROCS=2 go test -p=1 -count=1 ./cmd/server
~~~

- [ ] 步骤 2：运行前端目标回归

~~~bash
pnpm --dir frontend exec vitest run src/views/user/__tests__/SubscriptionsView.spec.ts src/views/user/__tests__/PaymentView.spec.ts src/stores/__tests__/auth.spec.ts src/api/__tests__/user.trafficCreditExhaustion.spec.ts src/views/user/__tests__/AvailableChannelsView.spec.ts src/views/user/__tests__/UsageGuideView.spec.ts src/views/admin/__tests__/groupsImagePricingRemoval.spec.ts
pnpm --dir frontend typecheck
pnpm --dir frontend build
~~~

- [ ] 步骤 3：做静态残留检查

~~~bash
rg -n 'image_rate_independent|image_rate_multiplier|image_price_1k|image_price_2k|image_price_4k' backend frontend --glob '!backend/migrations/*.sql' --glob '!backend/migrations/*_test.go'
rg -n 'ImageCount > 0' backend/internal/handler backend/internal/service
rg -n 'CalculateImageCost|resolveImageRateMultiplier' backend/internal
git diff --check
~~~

预期：业务代码和前端无旧字段、旧固定价 helper 或 ImageCount > 0 失败计费门槛；允许历史 migration 和历史 migration 回归测试保留命中。

- [ ] 步骤 4：使用 requesting-code-review 做独立审查

审查重点：Token 是否重复计费、价格快照是否可重放、最终失败是否在终止帧前落 fact、Images 是否上游前预留、reservation item 是否被错误门槛过滤、耗尽事件与余额是否同事务、ack 是否越权、notice 是否写入 localStorage。

- [ ] 步骤 5：使用 verification-before-completion 复跑受审查修复影响的命令

所有声称通过的测试必须有本轮新鲜输出，不能沿用中间结果。

- [ ] 步骤 6：写结果文档和 AGENTS 记忆

结果文档记录最终 schema、Token 组件公式、失败请求边界、预留与最终费用差异、逐卡门槛、事件幂等、Toast 规则、完整验证命令与结果、未部署声明。AGENTS.md 只追加一条最高优先级定论，不覆写其他任务内容。

- [ ] 步骤 7：提交文档和最终修复

~~~bash
git add AGENTS.md docs/ai/context backend frontend
git commit -m "docs: record image token billing rollout result"
~~~

- [ ] 步骤 8：使用 finishing-a-development-branch 交付集成选项

未获得用户明确授权前，不推送、不创建 PR、不合并到 main、不部署。

## 需求覆盖自检

- 实际 Token、双模型、统一倍率、多图不乘数：任务 4、5、6。
- Token 缺失按 0、billing_incomplete、失败终止收费：任务 4、5、6。
- Images 请求前 reservation、响应后实际结算：任务 6。
- 删除尺寸固定价、按张计费和独立图片倍率：任务 1、2、3、6。
- 单卡 $0.01、FEFO、完全预留仍展示：任务 7。
- 单卡跨门槛事件、同卡一次、多卡单批、购卡关闭旧事件：任务 8。
- checkout 逐卡、/auth/me notice、批量 ack：任务 9。
- 右上角固定“流量卡已用完”、会话去重、新卡可再次提醒：任务 11。
- 完整回归、独立审查、结果文档、AGENTS 记忆：任务 12。

## 明确不做

- 不按图片字节、尺寸、图片数量或最低调用费计算最终费用。
- 不补扣历史生图费用，不给历史耗尽卡补建事件。
- 不改变订阅、余额、流量卡的统一结算优先级。
- 不修改流量卡售价、有效期、退款、邮件或短信规则。
- 不在本次实现 attempt 级多账号 failover 计费。
- 不部署、不改运行态、不启用新的生产开关。
