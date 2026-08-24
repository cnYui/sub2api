package handler

import (
	"strconv"
	"strings"
	"time"

	dbent "github.com/Wei-Shaw/sub2api/ent"
	"github.com/Wei-Shaw/sub2api/internal/payment"
	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
	"github.com/Wei-Shaw/sub2api/internal/pkg/response"
	middleware2 "github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"

	"github.com/gin-gonic/gin"
)

// PaymentHandler handles user-facing payment requests.
type PaymentHandler struct {
	paymentService *service.PaymentService
	configService  *service.PaymentConfigService
}

// NewPaymentHandler creates a new PaymentHandler.
func NewPaymentHandler(paymentService *service.PaymentService, configService *service.PaymentConfigService) *PaymentHandler {
	return &PaymentHandler{
		paymentService: paymentService,
		configService:  configService,
	}
}

// GetPaymentConfig returns the payment system configuration.
// GET /api/v1/payment/config
func (h *PaymentHandler) GetPaymentConfig(c *gin.Context) {
	cfg, err := h.configService.GetPaymentConfig(c.Request.Context())
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, cfg)
}

// GetCheckoutInfo returns all data the payment page needs in a single call.
// GET /api/v1/payment/checkout-info
func (h *PaymentHandler) GetCheckoutInfo(c *gin.Context) {
	ctx := c.Request.Context()

	// Fetch limits (methods + global range)
	limitsResp, err := h.configService.GetAvailableMethodLimits(ctx)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}

	// Fetch payment config
	cfg, err := h.configService.GetPaymentConfig(ctx)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	alipayMobilePrecreateDeepLink := false
	if cfg.AlipayMobilePrecreateDeepLink {
		alipayMobilePrecreateDeepLink, err = h.configService.UsesOfficialAlipayVisibleMethod(ctx)
		if err != nil {
			response.ErrorFrom(c, err)
			return
		}
	}

	balancePackages, err := h.paymentService.ListBalancePackagePlansForSale(ctx)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	trafficPacks, err := h.paymentService.ListTrafficPacksForSale(ctx)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	var trafficCreditSummary *service.TrafficCreditSummary
	if subject, ok := middleware2.GetAuthSubjectFromContext(c); ok {
		trafficCreditSummary, _ = h.paymentService.GetTrafficCreditSummary(ctx, subject.UserID)
	}

	response.Success(c, checkoutInfoResponse{
		Methods:                       limitsResp.Methods,
		GlobalMin:                     limitsResp.GlobalMin,
		GlobalMax:                     limitsResp.GlobalMax,
		BalancePackages:               balancePackages,
		TrafficPacks:                  trafficPacks,
		TrafficCreditSummary:          trafficCreditSummary,
		RechargeFeeRate:               cfg.RechargeFeeRate,
		HelpText:                      cfg.HelpText,
		HelpImageURL:                  cfg.HelpImageURL,
		StripePublishableKey:          cfg.StripePublishableKey,
		AlipayForceQRCode:             cfg.AlipayForceQRCode,
		AlipayMobilePrecreateDeepLink: alipayMobilePrecreateDeepLink,
	})
}

// GetMyBalancePackages 返回当前用户已购买的余额套餐及到账进度。
// GET /api/v1/payment/balance-packages
func (h *PaymentHandler) GetMyBalancePackages(c *gin.Context) {
	subject, ok := middleware2.GetAuthSubjectFromContext(c)
	if !ok {
		response.Unauthorized(c, "User not found in context")
		return
	}

	packages, err := h.paymentService.ListUserBalancePackages(c.Request.Context(), subject.UserID)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, packages)
}

type checkoutInfoResponse struct {
	Methods                       map[string]service.MethodLimits `json:"methods"`
	GlobalMin                     float64                         `json:"global_min"`
	GlobalMax                     float64                         `json:"global_max"`
	BalancePackages               []*dbent.BalancePackagePlan     `json:"balance_packages"`
	TrafficPacks                  []service.TrafficPack           `json:"traffic_packs"`
	TrafficCreditSummary          *service.TrafficCreditSummary   `json:"traffic_credit_summary,omitempty"`
	RechargeFeeRate               float64                         `json:"recharge_fee_rate"`
	HelpText                      string                          `json:"help_text"`
	HelpImageURL                  string                          `json:"help_image_url"`
	StripePublishableKey          string                          `json:"stripe_publishable_key"`
	AlipayForceQRCode             bool                            `json:"alipay_force_qrcode"`
	AlipayMobilePrecreateDeepLink bool                            `json:"alipay_mobile_precreate_deep_link"`
}

// GetLimits returns per-payment-type limits derived from enabled provider instances.
// GET /api/v1/payment/limits
func (h *PaymentHandler) GetLimits(c *gin.Context) {
	resp, err := h.configService.GetAvailableMethodLimits(c.Request.Context())
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, resp)
}

// CreateOrderRequest is the request body for creating a payment order.
type CreateOrderRequest struct {
	Amount               float64 `json:"amount"`
	PaymentType          string  `json:"payment_type" binding:"required"`
	OpenID               string  `json:"openid"`
	WechatResumeToken    string  `json:"wechat_resume_token"`
	ReturnURL            string  `json:"return_url"`
	PaymentSource        string  `json:"payment_source"`
	OrderType            string  `json:"order_type"`
	BalancePackagePlanID int64   `json:"balance_package_plan_id"`
	TrafficPackID        int64   `json:"traffic_pack_id"`
	// IsMobile lets the frontend declare its mobile status directly. When
	// nil we fall back to User-Agent heuristics (which miss iPadOS / some
	// embedded browsers that strip the "Mobile" keyword).
	IsMobile *bool `json:"is_mobile,omitempty"`
}

// CreateOrder creates a new payment order.
// POST /api/v1/payment/orders
func (h *PaymentHandler) CreateOrder(c *gin.Context) {
	subject, ok := requireAuth(c)
	if !ok {
		return
	}

	var req CreateOrderRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request: "+err.Error())
		return
	}
	if strings.TrimSpace(req.WechatResumeToken) != "" {
		claims, err := h.paymentService.ParseWeChatPaymentResumeToken(req.WechatResumeToken)
		if err != nil {
			response.ErrorFrom(c, err)
			return
		}
		if err := applyWeChatPaymentResumeClaims(&req, claims); err != nil {
			response.ErrorFrom(c, err)
			return
		}
	}

	mobile := isMobile(c)
	if req.IsMobile != nil {
		mobile = *req.IsMobile
	}
	result, err := h.paymentService.CreateOrder(c.Request.Context(), service.CreateOrderRequest{
		UserID:               subject.UserID,
		Amount:               req.Amount,
		PaymentType:          req.PaymentType,
		OpenID:               req.OpenID,
		ClientIP:             c.ClientIP(),
		IsMobile:             mobile,
		IsWeChatBrowser:      isWeChatBrowser(c),
		SrcHost:              c.Request.Host,
		SrcURL:               c.Request.Referer(),
		ReturnURL:            req.ReturnURL,
		PaymentSource:        req.PaymentSource,
		OrderType:            req.OrderType,
		BalancePackagePlanID: req.BalancePackagePlanID,
		TrafficPackID:        req.TrafficPackID,
		Locale:               c.GetHeader("Accept-Language"),
	})
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, result)
}

func applyWeChatPaymentResumeClaims(req *CreateOrderRequest, claims *service.WeChatPaymentResumeClaims) error {
	if req == nil || claims == nil {
		return infraerrors.BadRequest("INVALID_WECHAT_PAYMENT_RESUME_TOKEN", "wechat payment resume context is missing")
	}
	openid := strings.TrimSpace(claims.OpenID)
	if openid == "" {
		return infraerrors.BadRequest("INVALID_WECHAT_PAYMENT_RESUME_TOKEN", "wechat payment resume token missing openid")
	}

	paymentType := service.NormalizeVisibleMethod(claims.PaymentType)
	if paymentType == "" {
		paymentType = payment.TypeWxpay
	}
	if req.PaymentType != "" {
		requestPaymentType := service.NormalizeVisibleMethod(req.PaymentType)
		if requestPaymentType != "" && requestPaymentType != paymentType {
			return infraerrors.BadRequest("INVALID_WECHAT_PAYMENT_RESUME_TOKEN", "wechat payment resume token payment type mismatch")
		}
	}
	req.PaymentType = paymentType
	req.OpenID = openid

	if claims.OrderType != "" {
		req.OrderType = claims.OrderType
	}
	if claims.BalancePackagePlanID > 0 {
		req.BalancePackagePlanID = claims.BalancePackagePlanID
	}
	if claims.TrafficPackID > 0 {
		req.TrafficPackID = claims.TrafficPackID
	}
	return nil
}

// GetMyOrders returns the authenticated user's orders.
// GET /api/v1/payment/orders/my
func (h *PaymentHandler) GetMyOrders(c *gin.Context) {
	subject, ok := requireAuth(c)
	if !ok {
		return
	}

	page, pageSize := response.ParsePagination(c)
	orders, total, err := h.paymentService.GetUserOrders(c.Request.Context(), subject.UserID, service.OrderListParams{
		Page:        page,
		PageSize:    pageSize,
		Status:      c.Query("status"),
		OrderType:   c.Query("order_type"),
		PaymentType: c.Query("payment_type"),
	})
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Paginated(c, sanitizePaymentOrdersForResponse(orders), int64(total), page, pageSize)
}

// GetOrder returns a single order for the authenticated user.
// GET /api/v1/payment/orders/:id
func (h *PaymentHandler) GetOrder(c *gin.Context) {
	subject, ok := requireAuth(c)
	if !ok {
		return
	}

	orderID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		response.BadRequest(c, "Invalid order ID")
		return
	}

	order, err := h.paymentService.GetOrder(c.Request.Context(), orderID, subject.UserID)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, sanitizePaymentOrderForResponse(order))
}

// CancelOrder cancels a pending order for the authenticated user.
// POST /api/v1/payment/orders/:id/cancel
func (h *PaymentHandler) CancelOrder(c *gin.Context) {
	subject, ok := requireAuth(c)
	if !ok {
		return
	}

	orderID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		response.BadRequest(c, "Invalid order ID")
		return
	}

	msg, err := h.paymentService.CancelOrder(c.Request.Context(), orderID, subject.UserID)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, gin.H{"message": msg})
}

// RefundRequestBody is the request body for requesting a refund.
type RefundRequestBody struct {
	Reason string `json:"reason"`
}

// GetRefundQuote 返回余额套餐的只读退款报价，提交退款时服务端会重新计算。
// GET /api/v1/payment/orders/:id/refund-quote
func (h *PaymentHandler) GetRefundQuote(c *gin.Context) {
	subject, ok := requireAuth(c)
	if !ok {
		return
	}
	orderID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		response.BadRequest(c, "Invalid order ID")
		return
	}
	order, err := h.paymentService.GetOrder(c.Request.Context(), orderID, subject.UserID)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	if order.OrderType != payment.OrderTypeBalanceSubscription {
		response.ErrorFrom(c, infraerrors.BadRequest("INVALID_ORDER_TYPE", "only balance package orders can be refunded"))
		return
	}
	quote, err := h.paymentService.GetBalancePackageRefundQuote(c.Request.Context(), orderID, subject.UserID)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, quote)
}

// RequestRefund submits a refund request for a completed order.
// POST /api/v1/payment/orders/:id/refund-request
func (h *PaymentHandler) RequestRefund(c *gin.Context) {
	subject, ok := requireAuth(c)
	if !ok {
		return
	}

	orderID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		response.BadRequest(c, "Invalid order ID")
		return
	}

	var req RefundRequestBody
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request: "+err.Error())
		return
	}

	if err := h.paymentService.RequestRefund(c.Request.Context(), orderID, subject.UserID, req.Reason); err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, gin.H{"message": "refund requested"})
}

// GetRefundEligibleProviders returns provider instance IDs that allow user refund.
func (h *PaymentHandler) GetRefundEligibleProviders(c *gin.Context) {
	ids, err := h.configService.GetUserRefundEligibleInstanceIDs(c.Request.Context())
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, gin.H{"provider_instance_ids": ids})
}

// VerifyOrderRequest is the request body for verifying a payment order.
type VerifyOrderRequest struct {
	OutTradeNo string `json:"out_trade_no" binding:"required"`
}

type ResolveOrderByResumeTokenRequest struct {
	ResumeToken string `json:"resume_token" binding:"required"`
}

// VerifyOrder actively queries the upstream payment provider to check
// if payment was made, and processes it if so.
// POST /api/v1/payment/orders/verify
func (h *PaymentHandler) VerifyOrder(c *gin.Context) {
	subject, ok := requireAuth(c)
	if !ok {
		return
	}

	var req VerifyOrderRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request: "+err.Error())
		return
	}

	order, err := h.paymentService.VerifyOrderByOutTradeNo(c.Request.Context(), req.OutTradeNo, subject.UserID)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, sanitizePaymentOrderForResponse(order))
}

// PublicOrderResult is returned after a signed resume-token lookup. The token
// proves possession of the checkout session, so the result keeps the legacy
// frontend contract needed by payment result pages.
type PublicOrderResult struct {
	ID                   int64      `json:"id"`
	OutTradeNo           string     `json:"out_trade_no"`
	Amount               float64    `json:"amount"`
	PayAmount            float64    `json:"pay_amount"`
	FeeRate              float64    `json:"fee_rate"`
	Currency             string     `json:"currency"`
	PaymentType          string     `json:"payment_type"`
	OrderType            string     `json:"order_type"`
	Status               string     `json:"status"`
	CreatedAt            time.Time  `json:"created_at"`
	ExpiresAt            time.Time  `json:"expires_at"`
	PaidAt               *time.Time `json:"paid_at,omitempty"`
	CompletedAt          *time.Time `json:"completed_at,omitempty"`
	RefundAmount         float64    `json:"refund_amount"`
	RefundReason         *string    `json:"refund_reason,omitempty"`
	RefundRequestedAt    *time.Time `json:"refund_requested_at,omitempty"`
	RefundRequestedBy    *string    `json:"refund_requested_by,omitempty"`
	RefundRequestReason  *string    `json:"refund_request_reason,omitempty"`
	BalancePackagePlanID *int64     `json:"balance_package_plan_id,omitempty"`
	TrafficPackID        *int64     `json:"traffic_pack_id,omitempty"`
	TrafficPackName      string     `json:"traffic_pack_name,omitempty"`
	TrafficPackCreditUSD *float64   `json:"traffic_pack_credit_usd,omitempty"`
	TrafficPackValidity  *int       `json:"traffic_pack_validity_days,omitempty"`
	TrafficPackPlatform  string     `json:"traffic_pack_platform,omitempty"`
}

// PublicOrderVerifyResult is returned by the legacy anonymous out_trade_no
// lookup. Keep this intentionally minimal because out_trade_no is not secret.
type PublicOrderVerifyResult struct {
	OutTradeNo  string     `json:"out_trade_no"`
	Status      string     `json:"status"`
	Paid        bool       `json:"paid"`
	CreatedAt   time.Time  `json:"created_at"`
	ExpiresAt   time.Time  `json:"expires_at"`
	PaidAt      *time.Time `json:"paid_at,omitempty"`
	CompletedAt *time.Time `json:"completed_at,omitempty"`
}

func buildPublicOrderResult(order *dbent.PaymentOrder) PublicOrderResult {
	trafficPack := trafficPackResponseFieldsForOrder(order)
	return PublicOrderResult{
		ID:                   order.ID,
		OutTradeNo:           order.OutTradeNo,
		Amount:               order.Amount,
		PayAmount:            order.PayAmount,
		FeeRate:              order.FeeRate,
		Currency:             service.PaymentOrderCurrency(order),
		PaymentType:          order.PaymentType,
		OrderType:            order.OrderType,
		Status:               order.Status,
		CreatedAt:            order.CreatedAt,
		ExpiresAt:            order.ExpiresAt,
		PaidAt:               order.PaidAt,
		CompletedAt:          order.CompletedAt,
		RefundAmount:         order.RefundAmount,
		RefundReason:         order.RefundReason,
		RefundRequestedAt:    order.RefundRequestedAt,
		RefundRequestedBy:    order.RefundRequestedBy,
		RefundRequestReason:  order.RefundRequestReason,
		BalancePackagePlanID: order.BalancePackagePlanID,
		TrafficPackID:        trafficPack.ID,
		TrafficPackName:      trafficPack.Name,
		TrafficPackCreditUSD: trafficPack.CreditUSD,
		TrafficPackValidity:  trafficPack.Validity,
		TrafficPackPlatform:  trafficPack.Platform,
	}
}

func buildPublicOrderVerifyResult(order *dbent.PaymentOrder) PublicOrderVerifyResult {
	return PublicOrderVerifyResult{
		OutTradeNo:  order.OutTradeNo,
		Status:      order.Status,
		Paid:        publicOrderStatusPaid(order.Status),
		CreatedAt:   order.CreatedAt,
		ExpiresAt:   order.ExpiresAt,
		PaidAt:      order.PaidAt,
		CompletedAt: order.CompletedAt,
	}
}

func publicOrderStatusPaid(status string) bool {
	switch status {
	case service.OrderStatusPaid,
		service.OrderStatusCompleted,
		service.OrderStatusRefundRequested,
		service.OrderStatusRefunding,
		service.OrderStatusRefundPending,
		service.OrderStatusPartiallyRefunded,
		service.OrderStatusRefunded,
		service.OrderStatusRefundFailed:
		return true
	default:
		return false
	}
}

// VerifyOrderPublic keeps the legacy anonymous out_trade_no lookup available as
// a compatibility path for older result pages and staggered deploys.
// POST /api/v1/payment/public/orders/verify
func (h *PaymentHandler) VerifyOrderPublic(c *gin.Context) {
	var req VerifyOrderRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request: "+err.Error())
		return
	}

	order, err := h.paymentService.VerifyOrderPublic(c.Request.Context(), req.OutTradeNo)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, buildPublicOrderVerifyResult(order))
}

// ResolveOrderPublicByResumeToken resolves a payment order from a signed resume token.
// POST /api/v1/payment/public/orders/resolve
func (h *PaymentHandler) ResolveOrderPublicByResumeToken(c *gin.Context) {
	var req ResolveOrderByResumeTokenRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request: "+err.Error())
		return
	}

	order, err := h.paymentService.GetPublicOrderByResumeToken(c.Request.Context(), req.ResumeToken)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, buildPublicOrderResult(order))
}

// requireAuth extracts the authenticated subject from the context.
// Returns the subject and true on success; on failure it writes an Unauthorized response and returns false.
func requireAuth(c *gin.Context) (middleware2.AuthSubject, bool) {
	subject, ok := middleware2.GetAuthSubjectFromContext(c)
	if !ok {
		response.Unauthorized(c, "User not authenticated")
		return middleware2.AuthSubject{}, false
	}
	return subject, true
}

// isMobile detects mobile user agents.
func isMobile(c *gin.Context) bool {
	ua := strings.ToLower(c.GetHeader("User-Agent"))
	for _, kw := range []string{"mobile", "android", "iphone", "ipad", "ipod"} {
		if strings.Contains(ua, kw) {
			return true
		}
	}
	return false
}

type PaymentOrderResult struct {
	ID                   int64      `json:"id"`
	UserID               int64      `json:"user_id"`
	Amount               float64    `json:"amount"`
	PayAmount            float64    `json:"pay_amount"`
	FeeRate              float64    `json:"fee_rate"`
	Currency             string     `json:"currency"`
	PaymentType          string     `json:"payment_type"`
	OutTradeNo           string     `json:"out_trade_no"`
	Status               string     `json:"status"`
	OrderType            string     `json:"order_type"`
	CreatedAt            time.Time  `json:"created_at"`
	ExpiresAt            time.Time  `json:"expires_at"`
	PaidAt               *time.Time `json:"paid_at,omitempty"`
	CompletedAt          *time.Time `json:"completed_at,omitempty"`
	RefundAmount         float64    `json:"refund_amount"`
	RefundReason         *string    `json:"refund_reason,omitempty"`
	RefundRequestedAt    *time.Time `json:"refund_requested_at,omitempty"`
	RefundRequestedBy    *string    `json:"refund_requested_by,omitempty"`
	RefundRequestReason  *string    `json:"refund_request_reason,omitempty"`
	BalancePackagePlanID *int64     `json:"balance_package_plan_id,omitempty"`
	TrafficPackID        *int64     `json:"traffic_pack_id,omitempty"`
	TrafficPackName      string     `json:"traffic_pack_name,omitempty"`
	TrafficPackCreditUSD *float64   `json:"traffic_pack_credit_usd,omitempty"`
	TrafficPackValidity  *int       `json:"traffic_pack_validity_days,omitempty"`
	TrafficPackPlatform  string     `json:"traffic_pack_platform,omitempty"`
	ProviderInstanceID   *string    `json:"provider_instance_id,omitempty"`
}

type trafficPackResponseFields struct {
	ID        *int64
	Name      string
	CreditUSD *float64
	Validity  *int
	Platform  string
}

func trafficPackResponseFieldsForOrder(order *dbent.PaymentOrder) trafficPackResponseFields {
	info := service.GetTrafficPackOrderInfo(order)
	if info == nil {
		return trafficPackResponseFields{}
	}
	return trafficPackResponseFields{
		ID:        &info.ID,
		Name:      info.Name,
		CreditUSD: &info.CreditUSD,
		Validity:  &info.ValidityDays,
		Platform:  info.Platform,
	}
}

func sanitizePaymentOrdersForResponse(orders []*dbent.PaymentOrder) []PaymentOrderResult {
	out := make([]PaymentOrderResult, 0, len(orders))
	for _, order := range orders {
		if item := sanitizePaymentOrderForResponse(order); item != nil {
			out = append(out, *item)
		}
	}
	return out
}

func sanitizePaymentOrderForResponse(order *dbent.PaymentOrder) *PaymentOrderResult {
	if order == nil {
		return nil
	}
	trafficPack := trafficPackResponseFieldsForOrder(order)
	return &PaymentOrderResult{
		ID:                   order.ID,
		UserID:               order.UserID,
		Amount:               order.Amount,
		PayAmount:            order.PayAmount,
		FeeRate:              order.FeeRate,
		Currency:             service.PaymentOrderCurrency(order),
		PaymentType:          order.PaymentType,
		OutTradeNo:           order.OutTradeNo,
		Status:               order.Status,
		OrderType:            order.OrderType,
		CreatedAt:            order.CreatedAt,
		ExpiresAt:            order.ExpiresAt,
		PaidAt:               order.PaidAt,
		CompletedAt:          order.CompletedAt,
		RefundAmount:         order.RefundAmount,
		RefundReason:         order.RefundReason,
		RefundRequestedAt:    order.RefundRequestedAt,
		RefundRequestedBy:    order.RefundRequestedBy,
		RefundRequestReason:  order.RefundRequestReason,
		BalancePackagePlanID: order.BalancePackagePlanID,
		TrafficPackID:        trafficPack.ID,
		TrafficPackName:      trafficPack.Name,
		TrafficPackCreditUSD: trafficPack.CreditUSD,
		TrafficPackValidity:  trafficPack.Validity,
		TrafficPackPlatform:  trafficPack.Platform,
		ProviderInstanceID:   order.ProviderInstanceID,
	}
}

func isWeChatBrowser(c *gin.Context) bool {
	return strings.Contains(strings.ToLower(c.GetHeader("User-Agent")), "micromessenger")
}
