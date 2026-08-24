package admin

import (
	"context"
	"strconv"
	"time"

	dbent "github.com/Wei-Shaw/sub2api/ent"
	"github.com/Wei-Shaw/sub2api/internal/payment"
	"github.com/Wei-Shaw/sub2api/internal/pkg/response"
	"github.com/Wei-Shaw/sub2api/internal/service"

	"github.com/gin-gonic/gin"
)

// PaymentHandler handles admin payment management.
type PaymentHandler struct {
	paymentService *service.PaymentService
	configService  *service.PaymentConfigService
}

type GrantBalancePackageRequest struct {
	UserID               int64 `json:"user_id" binding:"required"`
	BalancePackagePlanID int64 `json:"balance_package_plan_id" binding:"required"`
}

// ListBalancePackages 返回当前购买页可售的余额套餐。
// GET /api/v1/admin/payment/balance-packages
func (h *PaymentHandler) ListBalancePackages(c *gin.Context) {
	plans, err := h.paymentService.ListBalancePackagePlansForSale(c.Request.Context())
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, plans)
}

// GrantBalancePackage 为用户手动发放一个购买页余额套餐。
// POST /api/v1/admin/payment/balance-packages/grant
func (h *PaymentHandler) GrantBalancePackage(c *gin.Context) {
	var req GrantBalancePackageRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request: "+err.Error())
		return
	}
	adminID := getAdminIDFromContext(c)
	executeAdminIdempotentJSON(c, "admin.balance_packages.grant", req, service.DefaultWriteIdempotencyTTL(), func(ctx context.Context) (any, error) {
		return h.paymentService.GrantBalancePackage(ctx, service.GrantBalancePackageInput{
			UserID:               req.UserID,
			BalancePackagePlanID: req.BalancePackagePlanID,
			AdminUserID:          adminID,
		})
	})
}

// ResumeDebtPausedBalancePackage 在用户还清欠费后恢复余额套餐后续周额度。
func (h *PaymentHandler) ResumeDebtPausedBalancePackage(c *gin.Context) {
	packageID, ok := parseIDParam(c, "id")
	if !ok {
		return
	}
	adminID := getAdminIDFromContext(c)
	if err := h.paymentService.ResumeDebtPausedBalancePackage(c.Request.Context(), packageID, adminID); err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, gin.H{"message": "balance package resumed"})
}

// NewPaymentHandler creates a new admin PaymentHandler.
func NewPaymentHandler(paymentService *service.PaymentService, configService *service.PaymentConfigService) *PaymentHandler {
	return &PaymentHandler{
		paymentService: paymentService,
		configService:  configService,
	}
}

// --- Dashboard ---

// GetDashboard returns payment dashboard statistics.
// GET /api/v1/admin/payment/dashboard
func (h *PaymentHandler) GetDashboard(c *gin.Context) {
	days := 30
	if d := c.Query("days"); d != "" {
		if v, err := strconv.Atoi(d); err == nil && v > 0 {
			days = v
		}
	}
	stats, err := h.paymentService.GetDashboardStats(c.Request.Context(), days)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, stats)
}

// --- Orders ---

// ListOrders returns a paginated list of all payment orders.
// GET /api/v1/admin/payment/orders
func (h *PaymentHandler) ListOrders(c *gin.Context) {
	page, pageSize := response.ParsePagination(c)
	var userID int64
	if uid := c.Query("user_id"); uid != "" {
		if v, err := strconv.ParseInt(uid, 10, 64); err == nil {
			userID = v
		}
	}
	orders, total, err := h.paymentService.AdminListOrders(c.Request.Context(), userID, service.OrderListParams{
		Page:        page,
		PageSize:    pageSize,
		Status:      c.Query("status"),
		OrderType:   c.Query("order_type"),
		PaymentType: c.Query("payment_type"),
		Keyword:     c.Query("keyword"),
	})
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	items := sanitizeAdminPaymentOrdersForResponse(orders)
	if err := h.setCancellableBalancePackageOrders(c.Request.Context(), items); err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Paginated(c, items, int64(total), page, pageSize)
}

// GetOrderDetail returns detailed information about a single order.
// GET /api/v1/admin/payment/orders/:id
func (h *PaymentHandler) GetOrderDetail(c *gin.Context) {
	orderID, ok := parseIDParam(c, "id")
	if !ok {
		return
	}
	order, err := h.paymentService.GetOrderByID(c.Request.Context(), orderID)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	item := sanitizeAdminPaymentOrderForResponse(order)
	if err := h.setCancellableBalancePackageOrders(c.Request.Context(), []*AdminPaymentOrderResult{item}); err != nil {
		response.ErrorFrom(c, err)
		return
	}
	auditLogs, _ := h.paymentService.GetOrderAuditLogs(c.Request.Context(), orderID)
	response.Success(c, gin.H{"order": item, "auditLogs": auditLogs})
}

// CancelOrder cancels a pending order (admin).
// POST /api/v1/admin/payment/orders/:id/cancel
func (h *PaymentHandler) CancelOrder(c *gin.Context) {
	orderID, ok := parseIDParam(c, "id")
	if !ok {
		return
	}
	msg, err := h.paymentService.AdminCancelOrder(c.Request.Context(), orderID)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, gin.H{"message": msg})
}

// CancelBalancePackage 停止已生效余额套餐的后续权益，不向支付服务商发起退款。
// POST /api/v1/admin/payment/orders/:id/cancel-balance-package
func (h *PaymentHandler) CancelBalancePackage(c *gin.Context) {
	orderID, ok := parseIDParam(c, "id")
	if !ok {
		return
	}
	adminID := getAdminIDFromContext(c)
	executeAdminIdempotentJSON(c, "admin.balance_packages.cancel", gin.H{"order_id": orderID}, service.DefaultWriteIdempotencyTTL(), func(ctx context.Context) (any, error) {
		if err := h.paymentService.CancelBalancePackageForOrder(ctx, orderID, adminID); err != nil {
			return nil, err
		}
		return gin.H{"message": "balance package cancelled"}, nil
	})
}

// RetryFulfillment retries fulfillment for a paid order.
// POST /api/v1/admin/payment/orders/:id/retry
func (h *PaymentHandler) RetryFulfillment(c *gin.Context) {
	orderID, ok := parseIDParam(c, "id")
	if !ok {
		return
	}
	if err := h.paymentService.RetryFulfillment(c.Request.Context(), orderID); err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, gin.H{"message": "fulfillment retried"})
}

type AdminPaymentOrderResult struct {
	ID                      int64      `json:"id"`
	UserID                  int64      `json:"user_id"`
	UserEmail               string     `json:"user_email,omitempty"`
	UserName                string     `json:"user_name,omitempty"`
	UserNotes               *string    `json:"user_notes,omitempty"`
	Amount                  float64    `json:"amount"`
	PayAmount               float64    `json:"pay_amount"`
	FeeRate                 float64    `json:"fee_rate"`
	Currency                string     `json:"currency"`
	RechargeCode            string     `json:"recharge_code,omitempty"`
	OutTradeNo              string     `json:"out_trade_no"`
	PaymentType             string     `json:"payment_type"`
	PaymentTradeNo          string     `json:"payment_trade_no,omitempty"`
	PayURL                  *string    `json:"pay_url,omitempty"`
	QRCode                  *string    `json:"qr_code,omitempty"`
	QRCodeImg               *string    `json:"qr_code_img,omitempty"`
	OrderType               string     `json:"order_type"`
	BalancePackagePlanID    *int64     `json:"balance_package_plan_id,omitempty"`
	CanCancelBalancePackage bool       `json:"can_cancel_balance_package"`
	TrafficPackID           *int64     `json:"traffic_pack_id,omitempty"`
	TrafficPackName         string     `json:"traffic_pack_name,omitempty"`
	TrafficPackCreditUSD    *float64   `json:"traffic_pack_credit_usd,omitempty"`
	TrafficPackValidity     *int       `json:"traffic_pack_validity_days,omitempty"`
	TrafficPackPlatform     string     `json:"traffic_pack_platform,omitempty"`
	ProviderInstanceID      *string    `json:"provider_instance_id,omitempty"`
	ProviderKey             *string    `json:"provider_key,omitempty"`
	Status                  string     `json:"status"`
	RefundAmount            float64    `json:"refund_amount"`
	RefundReason            *string    `json:"refund_reason,omitempty"`
	RefundAt                *time.Time `json:"refund_at,omitempty"`
	RefundRequestedAt       *time.Time `json:"refund_requested_at,omitempty"`
	RefundRequestReason     *string    `json:"refund_request_reason,omitempty"`
	RefundRequestedBy       *string    `json:"refund_requested_by,omitempty"`
	ExpiresAt               time.Time  `json:"expires_at"`
	PaidAt                  *time.Time `json:"paid_at,omitempty"`
	CompletedAt             *time.Time `json:"completed_at,omitempty"`
	FailedAt                *time.Time `json:"failed_at,omitempty"`
	FailedReason            *string    `json:"failed_reason,omitempty"`
	ClientIP                string     `json:"client_ip,omitempty"`
	SrcHost                 string     `json:"src_host,omitempty"`
	SrcURL                  *string    `json:"src_url,omitempty"`
	CreatedAt               time.Time  `json:"created_at"`
	UpdatedAt               time.Time  `json:"updated_at"`
}

func sanitizeAdminPaymentOrdersForResponse(orders []*dbent.PaymentOrder) []*AdminPaymentOrderResult {
	out := make([]*AdminPaymentOrderResult, 0, len(orders))
	for _, order := range orders {
		if item := sanitizeAdminPaymentOrderForResponse(order); item != nil {
			out = append(out, item)
		}
	}
	return out
}

func sanitizeAdminPaymentOrderForResponse(order *dbent.PaymentOrder) *AdminPaymentOrderResult {
	if order == nil {
		return nil
	}
	trafficPack := service.GetTrafficPackOrderInfo(order)
	var trafficPackID *int64
	var trafficPackCreditUSD *float64
	var trafficPackValidity *int
	trafficPackName, trafficPackPlatform := "", ""
	if trafficPack != nil {
		trafficPackID = &trafficPack.ID
		trafficPackCreditUSD = &trafficPack.CreditUSD
		trafficPackValidity = &trafficPack.ValidityDays
		trafficPackName = trafficPack.Name
		trafficPackPlatform = trafficPack.Platform
	}
	return &AdminPaymentOrderResult{
		ID:                   order.ID,
		UserID:               order.UserID,
		UserEmail:            order.UserEmail,
		UserName:             order.UserName,
		UserNotes:            order.UserNotes,
		Amount:               order.Amount,
		PayAmount:            order.PayAmount,
		FeeRate:              order.FeeRate,
		Currency:             service.PaymentOrderCurrency(order),
		RechargeCode:         order.RechargeCode,
		OutTradeNo:           order.OutTradeNo,
		PaymentType:          order.PaymentType,
		PaymentTradeNo:       order.PaymentTradeNo,
		PayURL:               order.PayURL,
		QRCode:               order.QrCode,
		QRCodeImg:            order.QrCodeImg,
		OrderType:            order.OrderType,
		BalancePackagePlanID: order.BalancePackagePlanID,
		TrafficPackID:        trafficPackID,
		TrafficPackName:      trafficPackName,
		TrafficPackCreditUSD: trafficPackCreditUSD,
		TrafficPackValidity:  trafficPackValidity,
		TrafficPackPlatform:  trafficPackPlatform,
		ProviderInstanceID:   order.ProviderInstanceID,
		ProviderKey:          order.ProviderKey,
		Status:               order.Status,
		RefundAmount:         order.RefundAmount,
		RefundReason:         order.RefundReason,
		RefundAt:             order.RefundAt,
		RefundRequestedAt:    order.RefundRequestedAt,
		RefundRequestReason:  order.RefundRequestReason,
		RefundRequestedBy:    order.RefundRequestedBy,
		ExpiresAt:            order.ExpiresAt,
		PaidAt:               order.PaidAt,
		CompletedAt:          order.CompletedAt,
		FailedAt:             order.FailedAt,
		FailedReason:         order.FailedReason,
		ClientIP:             order.ClientIP,
		SrcHost:              order.SrcHost,
		SrcURL:               order.SrcURL,
		CreatedAt:            order.CreatedAt,
		UpdatedAt:            order.UpdatedAt,
	}
}

func (h *PaymentHandler) setCancellableBalancePackageOrders(ctx context.Context, orders []*AdminPaymentOrderResult) error {
	orderIDs := make([]int64, 0, len(orders))
	for _, order := range orders {
		if order == nil || order.OrderType != payment.OrderTypeBalanceSubscription || (order.Status != service.OrderStatusCompleted && order.Status != service.OrderStatusRefundFailed) {
			continue
		}
		orderIDs = append(orderIDs, order.ID)
	}
	if len(orderIDs) == 0 {
		return nil
	}
	cancellable, err := h.paymentService.CancellableBalancePackageOrderIDs(ctx, orderIDs)
	if err != nil {
		return err
	}
	for _, order := range orders {
		if order != nil {
			order.CanCancelBalancePackage = cancellable[order.ID]
		}
	}
	return nil
}

// AdminProcessRefundRequest is the request body for admin refund processing.
type AdminProcessRefundRequest struct {
	Reason string `json:"reason"`
}

// ProcessRefund processes a refund for an order (admin).
// POST /api/v1/admin/payment/orders/:id/refund
func (h *PaymentHandler) ProcessRefund(c *gin.Context) {
	orderID, ok := parseIDParam(c, "id")
	if !ok {
		return
	}

	var req AdminProcessRefundRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request: "+err.Error())
		return
	}

	plan, err := h.paymentService.PrepareRefund(c.Request.Context(), orderID, req.Reason)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}

	result, err := h.paymentService.ExecuteRefund(c.Request.Context(), plan)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, result)
}

// QueryAndFinalizeRefund queries the provider refund status and finalizes a pending refund.
// POST /api/v1/admin/payment/orders/:id/refund/query
func (h *PaymentHandler) QueryAndFinalizeRefund(c *gin.Context) {
	orderID, ok := parseIDParam(c, "id")
	if !ok {
		return
	}

	result, err := h.paymentService.QueryAndFinalizeRefund(c.Request.Context(), orderID)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, result)
}

// --- Provider Instances ---

// ListProviders returns all payment provider instances.
// GET /api/v1/admin/payment/providers
func (h *PaymentHandler) ListProviders(c *gin.Context) {
	providers, err := h.configService.ListProviderInstancesWithConfig(c.Request.Context())
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, providers)
}

// CreateProvider creates a new payment provider instance.
// POST /api/v1/admin/payment/providers
func (h *PaymentHandler) CreateProvider(c *gin.Context) {
	var req service.CreateProviderInstanceRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request: "+err.Error())
		return
	}
	inst, err := h.configService.CreateProviderInstance(c.Request.Context(), req)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	h.paymentService.RefreshProviders(c.Request.Context())
	response.Created(c, inst)
}

// UpdateProvider updates an existing payment provider instance.
// PUT /api/v1/admin/payment/providers/:id
func (h *PaymentHandler) UpdateProvider(c *gin.Context) {
	id, ok := parseIDParam(c, "id")
	if !ok {
		return
	}
	var req service.UpdateProviderInstanceRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request: "+err.Error())
		return
	}
	inst, err := h.configService.UpdateProviderInstance(c.Request.Context(), id, req)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	h.paymentService.RefreshProviders(c.Request.Context())
	response.Success(c, inst)
}

// DeleteProvider deletes a payment provider instance.
// DELETE /api/v1/admin/payment/providers/:id
func (h *PaymentHandler) DeleteProvider(c *gin.Context) {
	id, ok := parseIDParam(c, "id")
	if !ok {
		return
	}
	if err := h.configService.DeleteProviderInstance(c.Request.Context(), id); err != nil {
		response.ErrorFrom(c, err)
		return
	}
	h.paymentService.RefreshProviders(c.Request.Context())
	response.Success(c, gin.H{"message": "deleted"})
}

// parseIDParam parses an int64 path parameter.
// Returns the parsed ID and true on success; on failure it writes a BadRequest response and returns false.
func parseIDParam(c *gin.Context, paramName string) (int64, bool) {
	id, err := strconv.ParseInt(c.Param(paramName), 10, 64)
	if err != nil {
		response.BadRequest(c, "Invalid "+paramName)
		return 0, false
	}
	return id, true
}

// --- Config ---

// GetConfig returns the payment configuration (admin view).
// GET /api/v1/admin/payment/config
func (h *PaymentHandler) GetConfig(c *gin.Context) {
	cfg, err := h.configService.GetPaymentConfig(c.Request.Context())
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, cfg)
}

// UpdateConfig updates the payment configuration.
// PUT /api/v1/admin/payment/config
func (h *PaymentHandler) UpdateConfig(c *gin.Context) {
	var req service.UpdatePaymentConfigRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request: "+err.Error())
		return
	}
	if err := h.configService.UpdatePaymentConfig(c.Request.Context(), req); err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, gin.H{"message": "updated"})
}
