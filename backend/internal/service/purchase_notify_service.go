package service

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strconv"
	"strings"
	"time"

	dbent "github.com/Wei-Shaw/sub2api/ent"
	"github.com/Wei-Shaw/sub2api/ent/balancepackageplan"
	"github.com/Wei-Shaw/sub2api/ent/userbalancepackage"
	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/payment"
)

// 到账成功通知：余额套餐、流量卡、兑换码加余额共用同一套发送逻辑。
//
// 邮件一律 best-effort：订单和兑换在调用本服务之前就已经落库完成，
// 这里的任何失败都只记日志，绝不回滚业务或让调用方报错。

const (
	// 支付回调和兑换接口都在等这条链路返回，所以发信只能异步；这个超时只约束后台 goroutine。
	purchaseNotifyTimeout = 30 * time.Second

	purchaseKindNew        = "new"
	purchaseKindRenewal    = "renewal"
	purchaseKindAdminGrant = "admin_grant"
	purchaseKindRedeem     = "redeem"

	// 前端路由，见 frontend/src/router/index.ts。
	balancePackageDashboardPath = "/subscriptions"
	trafficPackDashboardPath    = "/subscriptions"
	redeemDashboardPath         = "/redeem"
)

// PurchaseNotifyService 发送购买/到账成功通知邮件。
type PurchaseNotifyService struct {
	notificationEmail *NotificationEmailService
	settingRepo       SettingRepository
	userRepo          UserRepository
	entClient         *dbent.Client
	cfg               *config.Config

	// notifyDone 仅供测试等待后台发送 goroutine 结束。
	notifyDone func()
}

func NewPurchaseNotifyService(
	notificationEmail *NotificationEmailService,
	settingRepo SettingRepository,
	userRepo UserRepository,
	entClient *dbent.Client,
	cfg *config.Config,
) *PurchaseNotifyService {
	return &PurchaseNotifyService{
		notificationEmail: notificationEmail,
		settingRepo:       settingRepo,
		userRepo:          userRepo,
		entClient:         entClient,
		cfg:               cfg,
	}
}

// NotifyBalancePackage 在余额套餐首期到账后异步通知用户（新购、续费、管理员发放共用）。
func (s *PurchaseNotifyService) NotifyBalancePackage(order *dbent.PaymentOrder) {
	if s == nil || order == nil {
		return
	}
	s.async(func(ctx context.Context) error { return s.sendBalancePackageNotice(ctx, order) },
		"balance_package", order.ID)
}

// NotifyTrafficPack 在流量卡到账后异步通知用户。
func (s *PurchaseNotifyService) NotifyTrafficPack(order *dbent.PaymentOrder) {
	if s == nil || order == nil {
		return
	}
	s.async(func(ctx context.Context) error { return s.sendTrafficPackNotice(ctx, order) },
		"traffic_pack", order.ID)
}

// NotifyRedeemBalance 在兑换码加余额成功后异步通知用户。
func (s *PurchaseNotifyService) NotifyRedeemBalance(userID, redeemCodeID int64, code string, amountUSD float64) {
	if s == nil {
		return
	}
	s.async(func(ctx context.Context) error {
		return s.sendRedeemBalanceNotice(ctx, userID, redeemCodeID, code, amountUSD)
	}, "redeem_balance", redeemCodeID)
}

func (s *PurchaseNotifyService) async(fn func(context.Context) error, kind string, sourceID int64) {
	go func() {
		defer func() {
			if s.notifyDone != nil {
				s.notifyDone()
			}
		}()
		ctx, cancel := context.WithTimeout(context.Background(), purchaseNotifyTimeout)
		defer cancel()
		if err := fn(ctx); err != nil {
			if errors.Is(err, ErrEmailNotConfigured) {
				slog.Debug("purchase_notify.skipped_smtp_not_configured", "kind", kind, "source_id", sourceID)
				return
			}
			slog.Warn("purchase_notify.failed", "kind", kind, "source_id", sourceID, "error", err.Error())
		}
	}()
}

func (s *PurchaseNotifyService) sendBalancePackageNotice(ctx context.Context, order *dbent.PaymentOrder) error {
	if !s.ready(ctx) {
		return nil
	}
	recipient, userID := s.recipientFor(ctx, order.UserID, order.UserEmail)
	if recipient == "" {
		return nil
	}
	pkg, err := s.entClient.UserBalancePackage.Query().
		Where(userbalancepackage.PaymentOrderIDEQ(order.ID)).
		Only(ctx)
	if err != nil {
		return fmt.Errorf("load balance package for order %d: %w", order.ID, err)
	}

	locale := s.notificationEmail.ResolveRecipientLocale(ctx, userID, recipient)
	kind := balancePackagePurchaseKind(order, pkg)

	variables := buildBalancePackageNoticeVariables(
		order, pkg, kind, locale,
		s.balancePackagePlanName(ctx, pkg.PlanID, locale),
		s.dashboardURL(ctx, balancePackageDashboardPath),
	)
	return s.send(ctx, NotificationEmailSendInput{
		Event:          NotificationEmailEventBalancePackageCredited,
		Locale:         locale,
		RecipientEmail: recipient,
		RecipientName:  emailRecipientName(recipient),
		UserID:         userID,
		SourceType:     "payment_order",
		SourceID:       strconv.FormatInt(order.ID, 10),
		Variables:      variables,
	})
}

func (s *PurchaseNotifyService) sendTrafficPackNotice(ctx context.Context, order *dbent.PaymentOrder) error {
	if !s.ready(ctx) {
		return nil
	}
	recipient, userID := s.recipientFor(ctx, order.UserID, order.UserEmail)
	if recipient == "" {
		return nil
	}
	info := GetTrafficPackOrderInfo(order)
	if info == nil {
		return fmt.Errorf("order %d traffic pack snapshot is missing", order.ID)
	}

	// 与 trafficPackCreditInputFromOrder 保持同一口径，邮件里的有效期才和实际入账一致。
	creditedAt := time.Now().UTC()
	if order.PaidAt != nil {
		creditedAt = order.PaidAt.UTC()
	}
	expiresAt := creditedAt.AddDate(0, 0, info.ValidityDays)

	locale := s.notificationEmail.ResolveRecipientLocale(ctx, userID, recipient)
	variables := buildTrafficPackNoticeVariables(
		order, info, expiresAt, locale,
		s.dashboardURL(ctx, trafficPackDashboardPath),
	)
	return s.send(ctx, NotificationEmailSendInput{
		Event:          NotificationEmailEventTrafficPackCredited,
		Locale:         locale,
		RecipientEmail: recipient,
		RecipientName:  emailRecipientName(recipient),
		UserID:         userID,
		SourceType:     "payment_order",
		SourceID:       strconv.FormatInt(order.ID, 10),
		Variables:      variables,
	})
}

func (s *PurchaseNotifyService) sendRedeemBalanceNotice(ctx context.Context, userID, redeemCodeID int64, code string, amountUSD float64) error {
	if !s.ready(ctx) {
		return nil
	}
	// 后台手工补扣走的是负数 admin_balance 兑换码（AGENTS.md 坑 30），绝不能给用户发"到账"。
	if amountUSD <= 0 {
		return nil
	}
	if s.userRepo == nil || userID <= 0 {
		return nil
	}
	user, err := s.userRepo.GetByID(ctx, userID)
	if err != nil || user == nil {
		return fmt.Errorf("load redeem user %d: %w", userID, err)
	}
	recipient := strings.TrimSpace(user.Email)
	if recipient == "" {
		return nil
	}

	locale := s.notificationEmail.ResolveRecipientLocale(ctx, userID, recipient)
	variables := buildRedeemBalanceNoticeVariables(
		code, amountUSD, user.Balance,
		s.dashboardURL(ctx, redeemDashboardPath),
	)
	return s.send(ctx, NotificationEmailSendInput{
		Event:          NotificationEmailEventRedeemBalanceCredited,
		Locale:         locale,
		RecipientEmail: recipient,
		RecipientName:  emailRecipientName(recipient),
		UserID:         userID,
		SourceType:     "redeem_code",
		SourceID:       strconv.FormatInt(redeemCodeID, 10),
		Variables:      variables,
	})
}

// balancePackagePurchaseKind 判定邮件里那一行"类型"。零金额的两种发放各有说法，
// 其余按是否续费区分。
func balancePackagePurchaseKind(order *dbent.PaymentOrder, pkg *dbent.UserBalancePackage) string {
	switch {
	case order.PaymentType == payment.PaymentTypeAdminGrant:
		return purchaseKindAdminGrant
	case order.PaymentType == payment.PaymentTypeRedeemCode:
		return purchaseKindRedeem
	case pkg != nil && pkg.RenewalCount > 0:
		return purchaseKindRenewal
	default:
		return purchaseKindNew
	}
}

// buildBalancePackageNoticeVariables 等三个构造器是纯函数，便于用回归测试钉死"占位符必须传满"。
func buildBalancePackageNoticeVariables(order *dbent.PaymentOrder, pkg *dbent.UserBalancePackage, kind, locale, planName, dashboardURL string) map[string]string {
	return map[string]string{
		"purchase_kind":         purchaseKindLabel(kind, locale),
		"plan_name":             planName,
		"pay_amount":            purchasePayAmountText(order, kind, locale),
		"weekly_credit_usd":     formatPurchaseUSD(pkg.WeeklyCreditUsd),
		"remaining_usd":         formatPurchaseUSD(pkg.RemainingUsd),
		"refresh_count":         strconv.Itoa(pkg.RefreshCount),
		"refresh_interval_days": strconv.Itoa(pkg.RefreshIntervalDays),
		"next_credit_at":        formatPurchaseTime(pkg.NextCreditAt),
		"expires_at":            formatPurchaseTime(&pkg.ExpiresAt),
		"order_no":              purchaseOrderNo(order),
		"dashboard_url":         dashboardURL,
	}
}

func buildTrafficPackNoticeVariables(order *dbent.PaymentOrder, info *TrafficPackOrderInfo, expiresAt time.Time, locale, dashboardURL string) map[string]string {
	return map[string]string{
		"pack_name":     info.Name,
		"pay_amount":    purchasePayAmountText(order, purchaseKindNew, locale),
		"credit_usd":    formatPurchaseUSD(info.CreditUSD),
		"validity_days": strconv.Itoa(info.ValidityDays),
		"expires_at":    formatPurchaseTime(&expiresAt),
		"order_no":      purchaseOrderNo(order),
		"dashboard_url": dashboardURL,
	}
}

func buildRedeemBalanceNoticeVariables(code string, amountUSD, currentBalance float64, dashboardURL string) map[string]string {
	return map[string]string{
		"redeem_code":     maskRedeemCode(code),
		"credit_usd":      formatPurchaseUSD(amountUSD),
		"current_balance": formatPurchaseUSD(currentBalance),
		"dashboard_url":   dashboardURL,
	}
}

// send 统一补齐通知框架的公共占位符后投递。
//
// 注意：NotificationEmailService.runtimeVariables 会先铺一层"预览示例值"再覆盖调用方传入的变量，
// 事件声明了却没传的占位符会把示例值（"张三"、"https://example.com/..."）发给真实用户，
// 所以这里必须把事件声明的占位符全部显式传满。
func (s *PurchaseNotifyService) send(ctx context.Context, input NotificationEmailSendInput) error {
	if s.notificationEmail == nil {
		return ErrEmailNotConfigured
	}
	return s.notificationEmail.Send(ctx, input)
}

// ready 检查全局开关与依赖是否具备。开关缺省视为开启。
func (s *PurchaseNotifyService) ready(ctx context.Context) bool {
	if s == nil || s.notificationEmail == nil || s.entClient == nil {
		return false
	}
	if s.settingRepo == nil {
		return true
	}
	value, err := s.settingRepo.GetValue(ctx, SettingKeyPurchaseNotifyEnabled)
	if err != nil {
		return true
	}
	return !isFalseSettingValue(value)
}

// recipientFor 优先用下单时的邮箱快照，缺失再回源用户表。
func (s *PurchaseNotifyService) recipientFor(ctx context.Context, userID int64, snapshotEmail string) (string, int64) {
	if email := strings.TrimSpace(snapshotEmail); email != "" {
		return email, userID
	}
	if s.userRepo == nil || userID <= 0 {
		return "", userID
	}
	user, err := s.userRepo.GetByID(ctx, userID)
	if err != nil || user == nil {
		return "", userID
	}
	return strings.TrimSpace(user.Email), user.ID
}

func (s *PurchaseNotifyService) balancePackagePlanName(ctx context.Context, planID int64, locale string) string {
	if s.entClient == nil || planID <= 0 {
		return purchaseFallbackPlanName(locale)
	}
	plan, err := s.entClient.BalancePackagePlan.Query().Where(balancepackageplan.IDEQ(planID)).Only(ctx)
	if err != nil || strings.TrimSpace(plan.Name) == "" {
		return purchaseFallbackPlanName(locale)
	}
	return strings.TrimSpace(plan.Name)
}

// purchasePayAmountText 管理员发放是零金额订单，写金额会误导用户，这里改写成"赠送"。
func purchasePayAmountText(order *dbent.PaymentOrder, kind, locale string) string {
	// 兑换码同样是零金额订单，但码本身可能是用户在别处花钱买的，写"赠送"是替对方下结论，所以留空。
	if kind == purchaseKindRedeem {
		return "—"
	}
	if kind == purchaseKindAdminGrant || order.PayAmount <= 0 {
		if isChinesePurchaseLocale(locale) {
			return "赠送"
		}
		return "Complimentary"
	}
	return fmt.Sprintf("%.2f %s", order.PayAmount, PaymentOrderCurrency(order))
}

// dashboardURL 取站点前端地址拼页面路径；settings 优先，其次 config，都没有则留空。
func (s *PurchaseNotifyService) dashboardURL(ctx context.Context, path string) string {
	base := ""
	if s.settingRepo != nil {
		if v, err := s.settingRepo.GetValue(ctx, SettingKeyFrontendURL); err == nil {
			base = strings.TrimSpace(v)
		}
	}
	if base == "" && s.cfg != nil {
		base = strings.TrimSpace(s.cfg.Server.FrontendURL)
	}
	if base == "" {
		return ""
	}
	return strings.TrimRight(base, "/") + path
}

func purchaseOrderNo(order *dbent.PaymentOrder) string {
	if code := strings.TrimSpace(order.RechargeCode); code != "" {
		return code
	}
	return strconv.FormatInt(order.ID, 10)
}

func purchaseKindLabel(kind, locale string) string {
	zh := isChinesePurchaseLocale(locale)
	switch kind {
	case purchaseKindRenewal:
		if zh {
			return "续费"
		}
		return "Renewal"
	case purchaseKindAdminGrant:
		if zh {
			return "管理员发放"
		}
		return "Admin grant"
	case purchaseKindRedeem:
		if zh {
			return "兑换码兑换"
		}
		return "Code redemption"
	default:
		if zh {
			return "新购"
		}
		return "New purchase"
	}
}

func purchaseFallbackPlanName(locale string) string {
	if isChinesePurchaseLocale(locale) {
		return "余额套餐"
	}
	return "Balance package"
}

func isChinesePurchaseLocale(locale string) bool {
	return normalizeNotificationLocale(locale) == notificationEmailLocaleChinese
}

func formatPurchaseUSD(value float64) string {
	return strconv.FormatFloat(value, 'f', 2, 64)
}

func formatPurchaseTime(value *time.Time) string {
	if value == nil || value.IsZero() {
		return "—"
	}
	return value.UTC().Format("2006-01-02 15:04") + " UTC"
}

// maskRedeemCode 邮件里只保留尾部字符，避免把完整兑换码留在收件箱里。
func maskRedeemCode(code string) string {
	trimmed := strings.TrimSpace(code)
	if trimmed == "" {
		return "—"
	}
	runes := []rune(trimmed)
	if len(runes) <= 4 {
		return strings.Repeat("*", len(runes))
	}
	return strings.Repeat("*", len(runes)-4) + string(runes[len(runes)-4:])
}
