# OpenAI 套餐计费、展示和限额修复 Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use `superpowers:subagent-driven-development`（推荐）或 `superpowers:executing-plans` 逐任务执行。步骤使用 checkbox (`- [ ]`) 语法追踪。

**Goal:** 修复 OpenAI 生图预算过高、订阅用量前端显示为 0、以及新每日上限没有被热路径真实限制的问题。

**Architecture:** 计费入口必须先选择唯一计费来源，顺序为“订阅套餐 -> 流量卡”，账户余额不参与 OpenAI 模型请求计费。生图最终结算继续按 OpenAI 返回的完整 usage token，不按图片张数、尺寸或请求体字节扣费；请求前预算只负责安全预授权，不能再把未知输出最大值套到输入图或 auto 输出上。展示层以订阅实际扣费口径显示今天用量，运行态用一次可审计回补把今天 Shadow 用量同步进订阅窗口。

**Tech Stack:** Go、PostgreSQL、Redis、Ent、Vue 3、TypeScript、Vitest、Docker Compose。

---

## 根因结论

1. 生图“单张很贵”的直接原因在请求前预算，不是最终结算。
   - `backend/internal/service/openai_images.go` 当前 `openAIImagesInputTokenUpperBound()` 对每张输入图使用 `23719`。
   - `openAIImagesOutputTokenUpperBound()` 在 `size` 或 `quality` 为 `auto`/空时也使用 `23719`。
   - `backend/internal/service/openai_traffic_credit_budget.go` 会把这些上界换算成 `ReserveUSD`，导致预授权费用看起来过高。
2. 订阅用量显示为 0 的根因是 OpenAI 预授权默认处于 `enabled=false + shadow=true`，`Authorize()` 提前返回 `BillingSourceShadow`。
   - Shadow 会让 `buildOpenAIUsageRecord()` 设置 `skipBilling=true`、`subscription=nil`、`billing_type=3`。
   - 结果是 `user_subscriptions.daily_usage_usd` 没有累计，管理端订阅列表自然显示 `$0.00 / 新额度`。
3. 用户端 Dashboard quota 走 `usage_facts/usage_logs` 聚合，管理端订阅列表走 `user_subscriptions.daily_usage_usd`，两个展示口径目前不一致。
4. 订阅日窗口校准 SQL 目前按 `usage_logs.total_cost` 聚合，热路径扣订阅按 `actual_cost`，这会在倍率存在时产生展示和限额偏差。
5. 新每日额度已经在运行态写入：`15 / 25 / 39 / 53 / 66 / 100 / 133`。计划不再修改价格、有效天数、订单、余额或 API Key。
6. 2026-07-21 运行态只读复查确认，用户持续请求期间 `usage_logs` 和 `usage_facts` 都在按分钟实时写入；当前问题不是用量事实丢失，而是这些事实大多以 `billing_type=3/shadow` 保存，未进入 `user_subscriptions.daily_usage_usd`。

## 文件边界

- 修改：`backend/internal/service/openai_images.go`
  - 负责解析生图请求、计算生图预授权输入/输出 token 上界。
- 修改：`backend/internal/service/openai_billing_authorization.go`
  - 负责 OpenAI 请求前选择唯一计费来源，避免 Shadow 覆盖订阅。
- 修改：`backend/internal/config/config.go`
  - 将 OpenAI 流量卡预授权默认切到真实模式，避免未显式配置时继续 Shadow。
- 修改：`backend/internal/repository/user_subscription_repo.go`
  - 统一日窗口校准使用 `actual_cost`。
- 修改：`backend/internal/service/openai_images_test.go`
  - 覆盖 multipart/data URL 输入图维度解析、auto 输出预算。
- 修改：`backend/internal/service/openai_billing_authorization_test.go`
  - 覆盖 shadow 配置下 active subscription 仍走订阅扣费。
- 修改：`backend/internal/repository/user_subscription_repo_integration_test.go`
  - 覆盖订阅日窗口校准使用 `actual_cost`。
- 修改：`frontend/src/views/admin/SubscriptionsView.vue` 的相关测试文件
  - 确认 API 返回非零 `daily_usage_usd` 时管理端不吞值，进度条和文本都按新额度显示。
- 新建：`docs/ai/context/YYYYMMDD-HHMMSS-openai-quota-billing-display-limit-result_CN.md`
  - 执行完成后记录测试、部署、DB 回补和验证结果。

## Task 1: 生图预授权预算改为可解释 token 上界

**Files:**
- Modify: `backend/internal/service/openai_images.go`
- Test: `backend/internal/service/openai_images_test.go`
- Test: `backend/internal/service/openai_traffic_credit_budget_test.go`

- [ ] **Step 1: 写 auto 输出预算红灯测试**

在 `backend/internal/service/openai_images_test.go` 追加：

```go
func TestOpenAIImagesOutputTokenUpperBound_AutoUsesSupportedOutputMax(t *testing.T) {
	req := &OpenAIImagesRequest{
		Endpoint: openAIImagesGenerationsEndpoint,
		Model:    "gpt-image-2",
		Size:     "auto",
		Quality:  "auto",
		N:        1,
	}

	got := openAIImagesOutputTokenUpperBound(req)

	require.Equal(t, 7024, got)
	require.Less(t, got, gptImage2UnknownOutputTokenUpperBound)
}
```

运行：

```bash
go test -count=1 -tags=unit ./internal/service -run TestOpenAIImagesOutputTokenUpperBound_AutoUsesSupportedOutputMax
```

期望：失败，当前返回 `23719`。

- [ ] **Step 2: 写输入图维度预算红灯测试**

在同一测试文件追加：

```go
func TestOpenAIImagesInputTokenUpperBound_MultipartUploadUsesDecodedDimensions(t *testing.T) {
	req := &OpenAIImagesRequest{
		Endpoint:      openAIImagesEditsEndpoint,
		InputFidelity: "high",
		Uploads: []OpenAIImagesUpload{{
			FieldName: "image",
			FileName:  "source.png",
			Width:     1024,
			Height:    1024,
		}},
	}

	got := openAIImagesInputTokenUpperBound(req)

	require.Equal(t, 4354, got)
	require.Less(t, got, gptImage2UnknownOutputTokenUpperBound)
}
```

运行：

```bash
go test -count=1 -tags=unit ./internal/service -run TestOpenAIImagesInputTokenUpperBound_MultipartUploadUsesDecodedDimensions
```

期望：失败，当前返回 `23719`。

- [ ] **Step 3: 写上传图片解析维度红灯测试**

在同一测试文件追加 PNG 测试辅助函数和测试：

```go
func testPNGBytes(t *testing.T, width, height int) []byte {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, width, height))
	var buf bytes.Buffer
	require.NoError(t, png.Encode(&buf, img))
	return buf.Bytes()
}

func TestOpenAIGatewayServiceParseOpenAIImagesRequest_MultipartDetectsUploadDimensions(t *testing.T) {
	gin.SetMode(gin.TestMode)
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	require.NoError(t, writer.WriteField("model", "gpt-image-2"))
	require.NoError(t, writer.WriteField("prompt", "replace background"))
	part, err := writer.CreateFormFile("image", "source.png")
	require.NoError(t, err)
	_, err = part.Write(testPNGBytes(t, 1024, 1024))
	require.NoError(t, err)
	require.NoError(t, writer.Close())

	req := httptest.NewRequest(http.MethodPost, "/v1/images/edits", bytes.NewReader(body.Bytes()))
	req.Header.Set("Content-Type", writer.FormDataContentType())
	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = req

	parsed, err := (&OpenAIGatewayService{}).ParseOpenAIImagesRequest(c, body.Bytes())

	require.NoError(t, err)
	require.Len(t, parsed.Uploads, 1)
	require.Equal(t, 1024, parsed.Uploads[0].Width)
	require.Equal(t, 1024, parsed.Uploads[0].Height)
}
```

运行：

```bash
go test -count=1 -tags=unit ./internal/service -run TestOpenAIGatewayServiceParseOpenAIImagesRequest_MultipartDetectsUploadDimensions
```

期望：失败，当前 `parseOpenAIImageDimensions()` 固定返回 0。

- [ ] **Step 4: 实现输出 auto 上界**

在 `backend/internal/service/openai_images.go` 中新增辅助函数，并让 auto/空值走已知支持组合的最大值：

```go
func maxOpenAIImagesOutputTokenUpperBoundForQuality(quality string) int {
	quality = strings.ToLower(strings.TrimSpace(quality))
	maxBound := 0
	for _, perQuality := range gptImage2OutputTokenUpperBounds {
		if quality == "" || quality == "auto" {
			for _, bound := range perQuality {
				if bound > maxBound {
					maxBound = bound
				}
			}
			continue
		}
		if bound, ok := perQuality[quality]; ok && bound > maxBound {
			maxBound = bound
		}
	}
	if maxBound <= 0 {
		return gptImage2UnknownOutputTokenUpperBound
	}
	return maxBound
}

func maxOpenAIImagesOutputTokenUpperBoundForSize(size string) int {
	size = strings.ToLower(strings.TrimSpace(size))
	perQuality, ok := gptImage2OutputTokenUpperBounds[size]
	if !ok {
		return gptImage2UnknownOutputTokenUpperBound
	}
	maxBound := 0
	for _, bound := range perQuality {
		if bound > maxBound {
			maxBound = bound
		}
	}
	return maxBound
}
```

更新 `openAIImagesOutputTokenUpperBound()`：

```go
if size == "" || size == "auto" {
	base := maxOpenAIImagesOutputTokenUpperBoundForQuality(quality)
	return base * n
}
if quality == "" || quality == "auto" {
	base := maxOpenAIImagesOutputTokenUpperBoundForSize(size)
	if req.PartialImages != nil && *req.PartialImages > 0 && base < gptImage2UnknownOutputTokenUpperBound {
		base += *req.PartialImages * 100
	}
	return base * n
}
```

- [ ] **Step 5: 实现输入图维度解析和预算**

在 `backend/internal/service/openai_images.go` 中把 `parseOpenAIImageDimensions(_ textproto.MIMEHeader)` 改为按图片内容解析：

```go
func parseOpenAIImageDimensions(data []byte) (int, int) {
	if len(data) == 0 {
		return 0, 0
	}
	cfg, _, err := image.DecodeConfig(bytes.NewReader(data))
	if err != nil || cfg.Width <= 0 || cfg.Height <= 0 {
		return 0, 0
	}
	return cfg.Width, cfg.Height
}
```

调用处改为：

```go
width, height := parseOpenAIImageDimensions(data)
```

新增输入 token 估算函数，预算参考 OpenAI 图片/视觉输入 token 规则，使用 512 tile、基础 token 和 high-fidelity 额外 token；远程 URL 无维度时使用当前支持组合中的最高输入兜底，不再复用输出未知最大值：

```go
const (
	gptImageInputBaseTokens          = 65
	gptImageInputTileTokens          = 129
	gptImageInputTileSize            = 512
	gptImageInputMaxEdge             = 2048
	gptImageInputHighSquareExtra     = 4160
	gptImageInputHighNonSquareExtra  = 6240
	gptImageInputUnknownUpperBound   = 7853
)

func openAIImageInputTokenUpperBoundForDimensions(width, height int, fidelity string) int {
	if width <= 0 || height <= 0 {
		return gptImageInputUnknownUpperBound
	}
	scaledWidth, scaledHeight := scaleOpenAIImageInputDimensions(width, height)
	tiles := ceilDiv(maxInt(scaledWidth, 1), gptImageInputTileSize) * ceilDiv(maxInt(scaledHeight, 1), gptImageInputTileSize)
	base := gptImageInputBaseTokens + tiles*gptImageInputTileTokens
	if strings.EqualFold(strings.TrimSpace(fidelity), "low") {
		return base
	}
	if scaledWidth == scaledHeight {
		return base + gptImageInputHighSquareExtra
	}
	return base + gptImageInputHighNonSquareExtra
}

func scaleOpenAIImageInputDimensions(width, height int) (int, int) {
	maxEdge := maxInt(width, height)
	if maxEdge <= gptImageInputMaxEdge {
		return width, height
	}
	return ceilDiv(width*gptImageInputMaxEdge, maxEdge), ceilDiv(height*gptImageInputMaxEdge, maxEdge)
}

func ceilDiv(a, b int) int {
	if b <= 0 {
		return 0
	}
	return (a + b - 1) / b
}
```

更新 `openAIImagesInputTokenUpperBound()`：

```go
total := 0
for _, upload := range req.Uploads {
	total += openAIImageInputTokenUpperBoundForDimensions(upload.Width, upload.Height, req.InputFidelity)
}
if req.MaskUpload != nil {
	total += openAIImageInputTokenUpperBoundForDimensions(req.MaskUpload.Width, req.MaskUpload.Height, req.InputFidelity)
}
for range req.InputImageURLs {
	total += gptImageInputUnknownUpperBound
}
if strings.TrimSpace(req.MaskImageURL) != "" {
	total += gptImageInputUnknownUpperBound
}
return total
```

需要新增导入：

```go
import (
	"image"
	_ "image/gif"
	_ "image/jpeg"
	_ "image/png"
)
```

- [ ] **Step 6: 跑生图预算测试**

运行：

```bash
go test -count=1 -tags=unit ./internal/service -run 'TestOpenAIImages(OutputTokenUpperBound_AutoUsesSupportedOutputMax|InputTokenUpperBound_MultipartUploadUsesDecodedDimensions|GatewayServiceParseOpenAIImagesRequest_MultipartDetectsUploadDimensions|TrafficCreditBudget_ImageUsesImageTokenBounds)'
```

期望：全部通过。

## Task 2: Shadow 配置不能覆盖 active subscription

**Files:**
- Modify: `backend/internal/service/openai_billing_authorization.go`
- Test: `backend/internal/service/openai_billing_authorization_test.go`
- Test: `backend/internal/service/openai_gateway_record_usage_test.go`

- [ ] **Step 1: 写 Shadow 配置下订阅仍扣订阅的红灯测试**

在 `backend/internal/service/openai_billing_authorization_test.go` 追加：

```go
func TestOpenAIBillingAuthorization_ShadowModeUsesSubscriptionWhenBudgetFits(t *testing.T) {
	dailyLimit := 10.0
	repo := &openAIBillingAuthorizationReservationRepoStub{availableUSD: 0}
	estimator := &openAIBillingAuthorizationEstimatorStub{budget: &OpenAITrafficCreditBudget{
		Body:        []byte(`{"model":"gpt-5.1"}`),
		ReserveUSD: 0.25,
	}}
	svc := NewOpenAIBillingAuthorizationService(repo, estimator, 15*time.Minute, false, true)
	input := newOpenAIBillingAuthorizationTestInput()
	input.Group = &Group{ID: 2, DailyLimitUSD: &dailyLimit}
	input.Subscription = &UserSubscription{ID: 3, DailyUsageUSD: 1}

	got, err := svc.Authorize(context.Background(), input)

	require.NoError(t, err)
	require.Equal(t, BillingSourceSubscription, got.Source)
	require.Zero(t, repo.reserveCalls)
}
```

运行：

```bash
go test -count=1 -tags=unit ./internal/service -run TestOpenAIBillingAuthorization_ShadowModeUsesSubscriptionWhenBudgetFits
```

期望：失败，当前返回 `BillingSourceShadow`。

- [ ] **Step 2: 写订阅超额不能落回 Shadow 的红灯测试**

在同一文件追加：

```go
func TestOpenAIBillingAuthorization_ShadowModeDoesNotBypassExceededSubscription(t *testing.T) {
	dailyLimit := 1.1
	repo := &openAIBillingAuthorizationReservationRepoStub{availableUSD: 0}
	estimator := &openAIBillingAuthorizationEstimatorStub{budget: &OpenAITrafficCreditBudget{
		Body:        []byte(`{"model":"gpt-5.1"}`),
		ReserveUSD: 0.25,
	}}
	svc := NewOpenAIBillingAuthorizationService(repo, estimator, 15*time.Minute, false, true)
	input := newOpenAIBillingAuthorizationTestInput()
	input.Group = &Group{ID: 2, DailyLimitUSD: &dailyLimit}
	input.Subscription = &UserSubscription{ID: 3, DailyUsageUSD: 1}

	_, err := svc.Authorize(context.Background(), input)

	require.ErrorIs(t, err, ErrTrafficCreditInsufficient)
	require.Zero(t, repo.reserveCalls)
}
```

运行：

```bash
go test -count=1 -tags=unit ./internal/service -run TestOpenAIBillingAuthorization_ShadowModeDoesNotBypassExceededSubscription
```

期望：失败，当前会返回 Shadow。

- [ ] **Step 3: 调整授权顺序**

把 `OpenAIBillingAuthorizationService.Authorize()` 顶部的 Shadow 快返移到订阅判断之后，并在 active subscription 已存在但预算不满足时禁止回落到 Shadow：

```go
subscriptionChecked := input.Subscription != nil && input.Group != nil
if subscriptionChecked {
	budget, err := s.estimate(ctx, input, math.MaxFloat64)
	if err != nil {
		return nil, err
	}
	daily, weekly, monthly := input.Subscription.CheckAllLimits(input.Group, budget.ReserveUSD)
	if daily && weekly && monthly {
		return &OpenAIBillingAuthorization{
			Source:             BillingSourceSubscription,
			RequestFingerprint: input.RequestFingerprint,
			ReserveUSD:         budget.ReserveUSD,
			PricingSnapshot:    budget.PricingSnapshot,
			EffectiveBody:      effectiveOpenAIBillingBody(input, budget.Body),
			Enforced:           true,
		}, nil
	}
}

if !s.enabled && s.shadow {
	if subscriptionChecked {
		recordTrafficCreditPreauthorizationRejected(ErrTrafficCreditInsufficient)
		return nil, ErrTrafficCreditInsufficient
	}
	recordTrafficCreditPreauthorizationSuccess()
	return &OpenAIBillingAuthorization{
		Source:             BillingSourceShadow,
		RequestFingerprint: input.RequestFingerprint,
		EffectiveBody:      append([]byte(nil), input.Body...),
	}, nil
}
```

保留后续真实流量卡 reservation 分支；当 `enabled=true` 时，订阅预算不足会继续尝试流量卡预授权。

- [ ] **Step 4: 验证 usage fact 仍会写订阅字段**

在 `backend/internal/service/openai_gateway_record_usage_test.go` 增加一个 source 明确为订阅的断言：

```go
func TestOpenAIGatewayServiceBuildUsageFact_SubscriptionAuthorizationSetsSubscriptionFields(t *testing.T) {
	svc := newOpenAIRecordUsageServiceForTest(&openAIRecordUsageLogRepoStub{}, &openAIRecordUsageUserRepoStub{}, &openAIRecordUsageSubRepoStub{}, nil)
	subscription := &UserSubscription{ID: 99}

	payload := buildOpenAIUsageFactPayloadForTest(t, svc, context.Background(), &OpenAIRecordUsageInput{
		Result: &OpenAIForwardResult{
			RequestID: "resp_subscription_authorized",
			Usage:     OpenAIUsage{InputTokens: 10, OutputTokens: 5},
			Model:     "gpt-5.1",
			Duration:  time.Second,
			BillingAuthorization: &OpenAIBillingAuthorization{
				Source:             BillingSourceSubscription,
				RequestFingerprint: "fp-subscription",
			},
		},
		APIKey:       &APIKey{ID: 100, GroupID: i64p(88), Group: &Group{ID: 88, SubscriptionType: SubscriptionTypeSubscription, RateMultiplier: 1.0}},
		User:         &User{ID: 200},
		Account:      &Account{ID: 300},
		Subscription: subscription,
	})

	require.Equal(t, BillingTypeSubscription, payload.UsageLog.BillingType)
	require.NotNil(t, payload.UsageLog.SubscriptionID)
	require.Equal(t, subscription.ID, *payload.UsageLog.SubscriptionID)
	require.Greater(t, payload.BillingCommand.SubscriptionCost, 0.0)
	require.Zero(t, payload.BillingCommand.BalanceCost)
}
```

运行：

```bash
go test -count=1 -tags=unit ./internal/service -run 'TestOpenAIBillingAuthorization_(ShadowModeUsesSubscriptionWhenBudgetFits|ShadowModeDoesNotBypassExceededSubscription)|TestOpenAIGatewayServiceBuildUsageFact_SubscriptionAuthorizationSetsSubscriptionFields'
```

期望：全部通过。

## Task 3: 默认启用真实预授权，避免重新部署后继续 Shadow

**Files:**
- Modify: `backend/internal/config/config.go`
- Test: `backend/internal/config` 相关配置测试，若没有覆盖默认值则新增最小单测。

- [ ] **Step 1: 修改默认值**

把默认值改为：

```go
viper.SetDefault("billing.traffic_credit_reservation_enabled", true)
viper.SetDefault("billing.traffic_credit_reservation_shadow", false)
```

原因：当前业务红线已经要求 OpenAI 请求按“订阅 -> 流量卡”真实预授权，默认 Shadow 会让新部署环境继续跳过扣费。

- [ ] **Step 2: 确认运行态 config 没有显式覆盖**

部署前只读检查：

```bash
docker exec sub2api-candidate sh -lc "grep -n 'traffic_credit_reservation\\|billing:' /app/data/config.yaml || true"
```

期望：若无输出，则新镜像默认值会生效；若有输出，先把 `/app/data/config.yaml` 备份，再显式改成 `enabled=true`、`shadow=false`。

- [ ] **Step 3: 跑配置相关测试**

运行：

```bash
go test -count=1 ./internal/config
```

期望：通过。

## Task 4: 订阅日用量校准统一为 actual_cost

**Files:**
- Modify: `backend/internal/repository/user_subscription_repo.go`
- Test: `backend/internal/repository/user_subscription_repo_integration_test.go`

- [ ] **Step 1: 写校准口径红灯测试**

在 `backend/internal/repository/user_subscription_repo_integration_test.go` 增加集成测试：

```go
func TestUserSubscriptionRepository_CalibrateActiveDailyUsageWindowsUsesActualCost(t *testing.T) {
	ctx := context.Background()
	repo := NewUserSubscriptionRepository(integrationEntClient)
	user := createIntegrationUser(t, "calibrate-actual-cost@test.com")
	group := createIntegrationSubscriptionGroup(t, "calibrate-actual-cost-group", 10)
	sub := createIntegrationSubscription(t, user.ID, group.ID, service.SubscriptionStatusActive)
	apiKey := createIntegrationAPIKey(t, user.ID, group.ID, "sk-calibrate-actual-cost")
	account := createIntegrationAccount(t)
	now := timezone.Now()
	dailyStart := timezone.StartOfDay(now)
	yesterday := dailyStart.Add(-24 * time.Hour)
	_, err := integrationDB.ExecContext(ctx, `
		UPDATE user_subscriptions
		SET daily_window_start = $1, daily_usage_usd = 0
		WHERE id = $2
	`, yesterday, sub.ID)
	require.NoError(t, err)
	_, err = integrationDB.ExecContext(ctx, `
		INSERT INTO usage_logs (
			user_id, api_key_id, account_id, subscription_id, group_id, request_id,
			model, input_tokens, output_tokens, total_cost, actual_cost, billing_type,
			created_at, updated_at
		) VALUES ($1,$2,$3,$4,$5,'req-calibrate-actual-cost','gpt-5.1',1,1,10,3,1,$6,$6)
	`, user.ID, apiKey.ID, account.ID, sub.ID, group.ID, dailyStart.Add(time.Hour))
	require.NoError(t, err)

	result, err := repo.CalibrateActiveDailyUsageWindows(ctx, dailyStart, now, now, 100)

	require.NoError(t, err)
	require.Equal(t, int64(1), result.UpdatedCount)
	var dailyUsage float64
	require.NoError(t, integrationDB.QueryRowContext(ctx, `SELECT daily_usage_usd FROM user_subscriptions WHERE id = $1`, sub.ID).Scan(&dailyUsage))
	require.InDelta(t, 3.0, dailyUsage, 1e-9)
}
```

运行：

```bash
go test -count=1 ./internal/repository -run TestUserSubscriptionRepository_CalibrateActiveDailyUsageWindowsUsesActualCost
```

期望：失败，当前会得到 `10`。

- [ ] **Step 2: 修改校准 SQL**

把 `CalibrateActiveDailyUsageWindows()` 中的：

```sql
SELECT ul.subscription_id, COALESCE(SUM(ul.total_cost), 0) AS total_cost
```

改为：

```sql
SELECT ul.subscription_id, COALESCE(SUM(ul.actual_cost), 0) AS total_cost
```

不要用 `total_cost` fallback，因为 `RateMultiplier == 0` 的免费订阅请求也可能有非零 `total_cost`，但不能消耗用户额度。

- [ ] **Step 3: 跑日窗口相关测试**

运行：

```bash
go test -count=1 ./internal/repository -run 'TestUserSubscriptionRepository_(CalibrateActiveDailyUsageWindowsUsesActualCost|RefreshExpiredUsageWindows)'
go test -count=1 -tags=unit ./internal/service -run 'TestCheckBillingEligibility|TestNormalizeExpiredWindows|TestGetSubscriptionProgress'
```

期望：全部通过。

## Task 5: 管理端和用户端展示保持同一扣费事实

**Files:**
- Test: `frontend/src/views/admin/SubscriptionsView.vue` 对应 spec 文件；若当前没有 spec，则新增 `frontend/src/views/admin/__tests__/SubscriptionsView.spec.ts`。
- Test: `frontend/src/views/user/__tests__/DashboardView.spec.ts`
- Test: `frontend/src/views/user/__tests__/SubscriptionsView.spec.ts`

- [ ] **Step 1: 写管理端非零用量展示测试**

新增或扩展管理端订阅列表测试，mock API 返回：

```ts
const subscription = {
  id: 1,
  user_id: 7,
  group_id: 2,
  status: 'active',
  daily_usage_usd: 12.34,
  daily_window_start: '2026-07-21T00:00:00+08:00',
  expires_at: '2026-08-20T00:00:00+08:00',
  group: {
    id: 2,
    name: 'codex-pool-19-usd',
    daily_limit_usd: 15,
    subscription_type: 'subscription',
  },
  user: {
    id: 7,
    email: 'display-check@example.com',
  },
}
```

断言页面包含：

```ts
expect(wrapper.text()).toContain('$12.34')
expect(wrapper.text()).toContain('$15.00')
```

运行：

```bash
pnpm test:run -- SubscriptionsView
```

期望：通过；若失败，修复组件中用量展示 helper，优先使用 API 返回的 `daily_usage_usd`。

- [ ] **Step 2: 写用户 Dashboard 新额度测试**

扩展 `frontend/src/views/user/__tests__/DashboardView.spec.ts`，mock `quota.today_limit_usd=15`、`quota.today_usage_usd=12.34`，断言用户控制面板展示新额度和实际使用量。

运行：

```bash
pnpm test:run -- DashboardView
```

期望：通过。

- [ ] **Step 3: 写用户订阅页新额度测试**

扩展 `frontend/src/views/user/__tests__/SubscriptionsView.spec.ts`，mock active subscription 的 `daily_usage_usd=12.34`、`group.daily_limit_usd=15`，断言订阅页展示 `12.34 / 15.00` 口径。

运行：

```bash
pnpm test:run -- SubscriptionsView
```

期望：通过。

## Task 6: 今天 Shadow 用量运行态回补

**Files:**
- Create: `docs/ai/context/YYYYMMDD-HHMMSS-openai-shadow-usage-backfill-plan_CN.md`
- Create: `docs/ai/context/YYYYMMDD-HHMMSS-openai-shadow-usage-backfill-result_CN.md`

本任务在代码测试通过、部署前或部署窗口内执行；它会让今天已经发生的 Shadow OpenAI 用量进入 `user_subscriptions.daily_usage_usd`，从今天开始按新上限拦截。

实时流量处理原则：

- 本地改代码期间，正在运行的 `sub2api-candidate` 不受影响，会继续把用户请求保存到 `usage_logs` 和 `usage_facts`；这些记录是后续回补的事实源。
- 回补只能覆盖写入 `user_subscriptions.daily_usage_usd`，不能删除或改写 `usage_logs`、`usage_facts`。
- 回补 SQL 必须是幂等的：每次都按今天事实重新计算并覆盖订阅日用量，不能用累加方式，否则用户持续请求时会重复计费。
- 部署后要以“新请求不再产生 Shadow”为完成标准；如果部署后仍有 Shadow，则先修代码/配置，再执行下一轮增量回补。

- [ ] **Step 0: 建立实时计费水位**

部署前先记录当前时间和最近 10 分钟写入状态，作为后续确认“修改期间没有漏记”的水位：

```bash
docker exec -i sub2api-candidate-postgres psql -U sub2api -d sub2api <<'SQL'
SELECT now() AS billing_fix_started_at;

WITH recent_logs AS (
  SELECT date_trunc('minute', created_at) AS minute, billing_type, COUNT(*) AS cnt, COALESCE(SUM(actual_cost), 0) AS actual_cost
  FROM usage_logs
  WHERE created_at >= now() - interval '10 minutes'
  GROUP BY 1, 2
),
recent_facts AS (
  SELECT
    date_trunc('minute', completed_at) AS minute,
    COALESCE(NULLIF(payload #>> '{usage_log,BillingType}', '')::int, NULLIF(payload #>> '{usage_log,billing_type}', '')::int) AS billing_type,
    COUNT(*) AS cnt,
    COALESCE(SUM(COALESCE(
      NULLIF(payload #>> '{usage_log,ActualCost}', '')::numeric,
      NULLIF(payload #>> '{usage_log,actual_cost}', '')::numeric,
      NULLIF(payload #>> '{effects,actual_cost}', '')::numeric,
      0
    )), 0) AS actual_cost
  FROM usage_facts
  WHERE completed_at >= now() - interval '10 minutes'
  GROUP BY 1, 2
)
SELECT 'usage_logs' AS source, minute, billing_type, cnt, ROUND(actual_cost::numeric, 6) AS actual_cost FROM recent_logs
UNION ALL
SELECT 'usage_facts' AS source, minute, billing_type, cnt, ROUND(actual_cost::numeric, 6) AS actual_cost FROM recent_facts
ORDER BY minute DESC, source, billing_type
LIMIT 40;
SQL
```

期望：持续请求期间 `usage_logs` 与 `usage_facts` 均有写入；如果某一分钟只有 log 或只有 fact，需要优先用 `usage_facts + usage_logs fallback` 的去重口径回补。

- [ ] **Step 1: 备份候选数据库**

运行：

```bash
mkdir -p deploy/candidate/dumps
docker exec sub2api-candidate-postgres pg_dump -U sub2api -d sub2api -Fc > deploy/candidate/dumps/sub2api-candidate-before-openai-shadow-backfill-$(date +%Y%m%d-%H%M%S).dump
pg_restore --list deploy/candidate/dumps/sub2api-candidate-before-openai-shadow-backfill-*.dump | head
shasum -a 256 deploy/candidate/dumps/sub2api-candidate-before-openai-shadow-backfill-*.dump
```

期望：`pg_restore --list` 能列出 TOC，保存 SHA-256 到 result 文档。

- [ ] **Step 2: dry-run 统计今天应回补用量**

运行只读 SQL：

```bash
docker exec -i sub2api-candidate-postgres psql -U sub2api -d sub2api <<'SQL'
WITH bounds AS (
  SELECT
    date_trunc('day', timezone('Asia/Shanghai', now())) AT TIME ZONE 'Asia/Shanghai' AS day_start,
    now() AS now_ts
),
active_subs AS (
  SELECT us.id AS subscription_id, us.user_id, us.group_id, g.daily_limit_usd
  FROM user_subscriptions us
  JOIN groups g ON g.id = us.group_id
  CROSS JOIN bounds b
  WHERE us.deleted_at IS NULL
    AND us.status = 'active'
    AND us.starts_at <= b.now_ts
    AND us.expires_at > b.now_ts
    AND g.platform = 'openai'
    AND g.subscription_type = 'subscription'
),
fact_costs AS (
  SELECT
    uf.request_id,
    COALESCE(NULLIF(uf.payload #>> '{usage_log,APIKeyID}', '')::bigint, NULLIF(uf.payload #>> '{effects,api_key_id}', '')::bigint) AS api_key_id,
    COALESCE(NULLIF(uf.payload #>> '{usage_log,UserID}', '')::bigint, NULLIF(uf.payload #>> '{effects,user_id}', '')::bigint) AS user_id,
    COALESCE(NULLIF(uf.payload #>> '{usage_log,GroupID}', '')::bigint, NULLIF(uf.payload #>> '{effects,group_id}', '')::bigint) AS group_id,
    COALESCE(
      NULLIF(uf.payload #>> '{usage_log,ActualCost}', '')::numeric,
      NULLIF(uf.payload #>> '{usage_log,actual_cost}', '')::numeric,
      NULLIF(uf.payload #>> '{effects,actual_cost}', '')::numeric,
      0
    ) AS actual_cost
  FROM usage_facts uf
  CROSS JOIN bounds b
  WHERE uf.completed_at >= b.day_start
    AND uf.completed_at < b.now_ts
    AND uf.billing_status IN ('pending', 'settling', 'settled', 'debt')
    AND COALESCE(NULLIF(uf.payload #>> '{usage_log,BillingType}', '')::int, NULLIF(uf.payload #>> '{usage_log,billing_type}', '')::int) IN (1, 3)
),
log_costs AS (
  SELECT ul.request_id, ul.api_key_id, ul.user_id, COALESCE(ul.group_id, ak.group_id) AS group_id, ul.actual_cost::numeric AS actual_cost
  FROM usage_logs ul
  LEFT JOIN api_keys ak ON ak.id = ul.api_key_id
  CROSS JOIN bounds b
  WHERE ul.created_at >= b.day_start
    AND ul.created_at < b.now_ts
    AND ul.actual_cost > 0
    AND ul.billing_type IN (1, 3)
    AND NOT EXISTS (
      SELECT 1
      FROM fact_costs fc
      WHERE fc.request_id = ul.request_id
        AND fc.api_key_id = ul.api_key_id
    )
),
usage_source AS (
  SELECT user_id, group_id, actual_cost FROM fact_costs WHERE actual_cost > 0
  UNION ALL
  SELECT user_id, group_id, actual_cost FROM log_costs
),
usage_today AS (
  SELECT
    s.subscription_id,
    COALESCE(SUM(src.actual_cost), 0) AS usage_usd
  FROM active_subs s
  JOIN usage_source src ON src.user_id = s.user_id AND src.group_id = s.group_id
  GROUP BY s.subscription_id
)
SELECT
  s.subscription_id,
  s.user_id,
  s.group_id,
  s.daily_limit_usd,
  COALESCE(u.usage_usd, 0) AS computed_today_usage,
  COALESCE(u.usage_usd, 0) >= s.daily_limit_usd AS over_new_daily_limit
FROM active_subs s
LEFT JOIN usage_today u ON u.subscription_id = s.subscription_id
ORDER BY computed_today_usage DESC, s.subscription_id
LIMIT 80;
SQL
```

期望：列出今天实际用量、是否超过新日限额；把汇总行数、超限人数写入 result 文档。

- [ ] **Step 3: 单事务回补订阅日窗口**

执行前确认 Step 2 输出符合预期，然后运行：

```bash
docker exec -i sub2api-candidate-postgres psql -U sub2api -d sub2api <<'SQL'
BEGIN;
WITH bounds AS (
  SELECT
    date_trunc('day', timezone('Asia/Shanghai', now())) AT TIME ZONE 'Asia/Shanghai' AS day_start,
    now() AS now_ts
),
active_subs AS (
  SELECT us.id AS subscription_id, us.user_id, us.group_id
  FROM user_subscriptions us
  JOIN groups g ON g.id = us.group_id
  CROSS JOIN bounds b
  WHERE us.deleted_at IS NULL
    AND us.status = 'active'
    AND us.starts_at <= b.now_ts
    AND us.expires_at > b.now_ts
    AND g.platform = 'openai'
    AND g.subscription_type = 'subscription'
),
fact_costs AS (
  SELECT
    uf.request_id,
    COALESCE(NULLIF(uf.payload #>> '{usage_log,APIKeyID}', '')::bigint, NULLIF(uf.payload #>> '{effects,api_key_id}', '')::bigint) AS api_key_id,
    COALESCE(NULLIF(uf.payload #>> '{usage_log,UserID}', '')::bigint, NULLIF(uf.payload #>> '{effects,user_id}', '')::bigint) AS user_id,
    COALESCE(NULLIF(uf.payload #>> '{usage_log,GroupID}', '')::bigint, NULLIF(uf.payload #>> '{effects,group_id}', '')::bigint) AS group_id,
    COALESCE(
      NULLIF(uf.payload #>> '{usage_log,ActualCost}', '')::numeric,
      NULLIF(uf.payload #>> '{usage_log,actual_cost}', '')::numeric,
      NULLIF(uf.payload #>> '{effects,actual_cost}', '')::numeric,
      0
    ) AS actual_cost
  FROM usage_facts uf
  CROSS JOIN bounds b
  WHERE uf.completed_at >= b.day_start
    AND uf.completed_at < b.now_ts
    AND uf.billing_status IN ('pending', 'settling', 'settled', 'debt')
    AND COALESCE(NULLIF(uf.payload #>> '{usage_log,BillingType}', '')::int, NULLIF(uf.payload #>> '{usage_log,billing_type}', '')::int) IN (1, 3)
),
log_costs AS (
  SELECT ul.request_id, ul.api_key_id, ul.user_id, COALESCE(ul.group_id, ak.group_id) AS group_id, ul.actual_cost::numeric AS actual_cost
  FROM usage_logs ul
  LEFT JOIN api_keys ak ON ak.id = ul.api_key_id
  CROSS JOIN bounds b
  WHERE ul.created_at >= b.day_start
    AND ul.created_at < b.now_ts
    AND ul.actual_cost > 0
    AND ul.billing_type IN (1, 3)
    AND NOT EXISTS (
      SELECT 1
      FROM fact_costs fc
      WHERE fc.request_id = ul.request_id
        AND fc.api_key_id = ul.api_key_id
    )
),
usage_source AS (
  SELECT user_id, group_id, actual_cost FROM fact_costs WHERE actual_cost > 0
  UNION ALL
  SELECT user_id, group_id, actual_cost FROM log_costs
),
usage_today AS (
  SELECT
    s.subscription_id,
    COALESCE(SUM(src.actual_cost), 0) AS usage_usd
  FROM active_subs s
  JOIN usage_source src ON src.user_id = s.user_id AND src.group_id = s.group_id
  GROUP BY s.subscription_id
),
updated AS (
  UPDATE user_subscriptions us
  SET daily_usage_usd = COALESCE(u.usage_usd, 0),
      daily_window_start = b.day_start,
      updated_at = b.now_ts
  FROM active_subs s
  CROSS JOIN bounds b
  LEFT JOIN usage_today u ON u.subscription_id = s.subscription_id
  WHERE us.id = s.subscription_id
  RETURNING us.id, us.user_id, us.group_id, us.daily_usage_usd, us.daily_window_start
)
SELECT COUNT(*) AS updated_count, COALESCE(SUM(daily_usage_usd), 0) AS total_daily_usage
FROM updated;
COMMIT;
SQL
```

期望：只更新 active OpenAI 订阅的 `daily_usage_usd`、`daily_window_start`、`updated_at`。

- [ ] **Step 4: 清理相关 Redis 缓存**

运行：

```bash
docker exec sub2api-candidate-redis redis-cli --scan --pattern 'billing:sub:*' | while read key; do docker exec sub2api-candidate-redis redis-cli DEL "$key" >/dev/null; done
docker exec sub2api-candidate-redis redis-cli --scan --pattern 'apikey:auth:*' | while read key; do docker exec sub2api-candidate-redis redis-cli DEL "$key" >/dev/null; done
docker exec sub2api-candidate-redis redis-cli PUBLISH auth:cache:invalidate '*'
```

期望：下一次请求重新读取新额度和回补后的用量。

- [ ] **Step 4.5: 增量确认部署后没有继续产生 Shadow**

清缓存后连续观察至少 2 分钟：

```bash
docker exec -i sub2api-candidate-postgres psql -U sub2api -d sub2api <<'SQL'
SELECT
  CASE billing_type WHEN 1 THEN 'subscription' WHEN 2 THEN 'traffic_credit' WHEN 3 THEN 'shadow' ELSE billing_type::text END AS billing_type,
  COUNT(*) AS request_count,
  ROUND(COALESCE(SUM(actual_cost), 0)::numeric, 6) AS actual_cost
FROM usage_logs
WHERE created_at >= now() - interval '2 minutes'
GROUP BY billing_type
ORDER BY billing_type;
SQL
```

期望：OpenAI 新请求不再出现 `shadow`；如果仍有 `shadow`，先停止验收，修部署配置或代码，再重新执行 Step 2、Step 3 和 Step 4。

- [ ] **Step 5: 验证超限用户被拦截**

选 dry-run 中 `over_new_daily_limit=true` 的用户，使用其测试 API Key 发一个低成本 OpenAI 请求。

期望：
- 若无有效流量卡：返回 429/402 中项目已定义的额度不足错误，不能继续走订阅。
- 若有有效流量卡：请求来源应切到流量卡，`usage_logs.billing_type=2`，不能继续写订阅。

## Task 7: 部署和完整验证

**Files:**
- Create: `docs/ai/context/YYYYMMDD-HHMMSS-openai-quota-billing-display-limit-result_CN.md`

- [ ] **Step 1: 后端测试**

运行：

```bash
go test -count=1 -tags=unit ./internal/service -run 'OpenAIImages|OpenAIBillingAuthorization|OpenAIGatewayServiceBuildUsageFact'
go test -count=1 ./internal/repository -run 'UserSubscriptionRepository.*Calibrate|UsageBilling'
go test -count=1 ./internal/config
```

期望：全部通过。

- [ ] **Step 2: 前端测试**

运行：

```bash
pnpm test:run -- SubscriptionsView DashboardView UserDashboardStats
```

期望：全部通过。

- [ ] **Step 3: 全量质量门**

运行：

```bash
pnpm typecheck
pnpm lint:check
pnpm test:run
pnpm build
```

期望：全部通过。

- [ ] **Step 4: 部署候选应用容器**

不重启 Docker daemon；只按现有候选环境流程替换 `sub2api-candidate` 应用容器。部署前确认：

```bash
docker ps --format 'table {{.Names}}\t{{.Image}}\t{{.Ports}}' | grep 'sub2api-candidate'
docker inspect sub2api-candidate --format '{{json .Mounts}}'
```

期望：PostgreSQL/Redis volume 不变，Nginx 指向仍是 `127.0.0.1:18084`。

- [ ] **Step 5: 健康检查和 UI 验证**

运行：

```bash
curl -sS http://127.0.0.1:18084/health
```

期望：

```json
{"status":"ok"}
```

打开管理端订阅列表，确认截图中 `$0.00 / $15.00` 这类行变为今天真实用量。打开用户控制面板，确认今日额度显示为新上限，今日使用量与管理端同一用户一致。

- [ ] **Step 6: 生图预算验证**

使用一个带 `size=auto`、`quality=auto` 的生图请求和一个 1024x1024 输入图编辑请求做预授权日志验证。

期望：
- budget snapshot 中 output upper bound 不再是 `23719`。
- multipart 输入图有维度时 input upper bound 不再是 `23719`。
- 成功响应最终费用仍按 usage token 中的 `image_input_tokens`、`image_output_tokens` 计算。

## 回滚边界

1. 代码回滚：切回上一版 `sub2api-candidate` 镜像或对应 git commit；不回滚 PostgreSQL/Redis volume。
2. 配置回滚：若显式改过 `/app/data/config.yaml`，恢复部署前备份；但这会重新进入 Shadow，只有紧急止血时使用。
3. DB 回补回滚：优先使用 Step 1 的 custom dump 完整恢复；如果只回滚本次窗口回补，按备份中对应订阅的 `daily_usage_usd`、`daily_window_start`、`updated_at` 写回。
4. 缓存回滚：任何回滚后都清理 `billing:sub:*` 和 `apikey:auth:*`，并发布 `auth:cache:invalidate`。

## 不做的事

- 不修改套餐价格、有效天数、订阅起止时间、订单、余额、流量卡面额或 API Key。
- 不把生图改回按张扣费。
- 不把 OpenAI 模型请求切回账户余额扣费。
- 不重启 Docker daemon；代码或配置上线只需要替换/重启 Sub2API 应用容器。

## 外部核对

- OpenAI 官方图片与视觉指南说明图片输入会按 token 计费，可用于预算公式核对：https://developers.openai.com/api/docs/guides/images-vision
- OpenAI 官方模型文档列出 `gpt-image-1` 的文本 token、图片 input token、图片 output token 计费项，可作为“最终按 usage token 结算、不是按张结算”的外部口径：https://developers.openai.com/api/docs/models/gpt-image-1
