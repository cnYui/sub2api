package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"strconv"
	"strings"
	"time"

	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
)

// 管理员把一张未使用的兑换码直接发到用户的注册邮箱（事件 redeem.code_delivery），码以明文写在邮件里。
//
// 和到账通知不一样：这是管理员当场触发的一次性投递，成败要回给调用方，所以同步发送，
// 也不受「到账成功通知」开关控制。同一张码只认第一个收件人——发给第二个人只会有一个人兑换得了。

const redeemCodeDeliveryKeyPrefix = "redeem_code_delivery:"

var (
	ErrRedeemCodeDeliveryTarget      = infraerrors.BadRequest("REDEEM_CODE_DELIVERY_TARGET_REQUIRED", "exactly one of user_id or email is required")
	ErrRedeemCodeDeliveryUnsupported = infraerrors.BadRequest("REDEEM_CODE_DELIVERY_UNSUPPORTED", "only positive balance codes and balance package codes can be emailed")
	ErrRedeemCodeDeliveryNotUsable   = infraerrors.Conflict("REDEEM_CODE_NOT_USABLE", "only unused redeem codes can be emailed")
	ErrRedeemCodeDeliveryNoEmail     = infraerrors.BadRequest("REDEEM_CODE_DELIVERY_NO_EMAIL", "user has no email address")
	ErrRedeemCodeDeliveryUnavailable = infraerrors.ServiceUnavailable("REDEEM_CODE_DELIVERY_UNAVAILABLE", "email delivery is not configured")
)

type RedeemCodeDeliveryInput struct {
	CodeID int64
	// UserID 与 Email 二选一，Email 按注册邮箱查用户，不接受站外地址。
	UserID int64
	Email  string
	// Resend 允许给同一个收件人再发一次（对方说没收到）；换收件人一律拒绝，与它无关。
	Resend bool
}

type RedeemCodeDeliveryResult struct {
	CodeID      int64     `json:"code_id"`
	UserID      int64     `json:"user_id"`
	Email       string    `json:"email"`
	SentAt      time.Time `json:"sent_at"`
	AlreadySent bool      `json:"already_sent"`
}

type redeemCodeDeliveryRecord struct {
	UserID int64     `json:"user_id"`
	Email  string    `json:"email"`
	SentAt time.Time `json:"sent_at"`
}

// DeliverByEmail 校验兑换码和收件人后，把码发到该用户的注册邮箱。
func (s *RedeemService) DeliverByEmail(ctx context.Context, input RedeemCodeDeliveryInput) (*RedeemCodeDeliveryResult, error) {
	if s.purchaseNotify == nil {
		return nil, ErrRedeemCodeDeliveryUnavailable
	}
	email := strings.TrimSpace(input.Email)
	if (input.UserID > 0) == (email != "") {
		return nil, ErrRedeemCodeDeliveryTarget
	}

	code, err := s.redeemRepo.GetByID(ctx, input.CodeID)
	if err != nil {
		return nil, err
	}
	switch {
	case code.IsExpired():
		return nil, ErrRedeemCodeExpired
	case code.IsUsed():
		return nil, ErrRedeemCodeUsed
	case !code.CanUse():
		return nil, ErrRedeemCodeDeliveryNotUsable
	}
	switch {
	case code.Type == RedeemTypeBalance && code.Value > 0:
	case code.Type == RedeemTypeBalancePackage && code.BalancePackagePlan != nil:
	default:
		// 负数余额码是后台补扣，其余类型邮件里说不清到账内容，都不允许发。
		return nil, ErrRedeemCodeDeliveryUnsupported
	}

	var target *User
	if input.UserID > 0 {
		target, err = s.userRepo.GetByID(ctx, input.UserID)
	} else {
		target, err = s.userRepo.GetByEmail(ctx, email)
	}
	if err != nil {
		return nil, err
	}
	if target.Status != StatusActive {
		return nil, infraerrors.Forbidden("USER_INACTIVE", "user account is disabled")
	}
	if strings.TrimSpace(target.Email) == "" {
		return nil, ErrRedeemCodeDeliveryNoEmail
	}

	return s.purchaseNotify.deliverRedeemCode(ctx, code, target, input.Resend)
}

func (s *PurchaseNotifyService) deliverRedeemCode(ctx context.Context, code *RedeemCode, target *User, resend bool) (*RedeemCodeDeliveryResult, error) {
	if s == nil || s.notificationEmail == nil || s.settingRepo == nil {
		return nil, ErrRedeemCodeDeliveryUnavailable
	}
	recipient := strings.TrimSpace(target.Email)
	key := redeemCodeDeliveryKey(code.ID)

	previous, err := s.loadRedeemCodeDelivery(ctx, key)
	if err != nil {
		return nil, err
	}
	if previous != nil {
		if previous.UserID != target.ID {
			return nil, infraerrors.Conflict("REDEEM_CODE_ALREADY_DELIVERED", "redeem code has already been emailed to another user").
				WithMetadata(map[string]string{"delivered_user_id": strconv.FormatInt(previous.UserID, 10)})
		}
		if !resend {
			return &RedeemCodeDeliveryResult{CodeID: code.ID, UserID: target.ID, Email: previous.Email, SentAt: previous.SentAt, AlreadySent: true}, nil
		}
	}

	locale := s.notificationEmail.ResolveRecipientLocale(ctx, target.ID, recipient)
	reminderKey := ""
	if previous != nil {
		// 通知框架按投递键去重，重发不换键会被当成"已发过"直接跳过。
		reminderKey = "resend-" + strconv.FormatInt(time.Now().UnixNano(), 10)
	}
	err = s.notificationEmail.Send(ctx, NotificationEmailSendInput{
		Event:          NotificationEmailEventRedeemCodeDelivery,
		Locale:         locale,
		RecipientEmail: recipient,
		RecipientName:  emailRecipientName(recipient),
		UserID:         target.ID,
		SourceType:     "redeem_code_delivery",
		SourceID:       strconv.FormatInt(code.ID, 10),
		ReminderKey:    reminderKey,
		Variables:      buildRedeemCodeDeliveryVariables(code, locale),
	})
	if err != nil {
		if errors.Is(err, ErrEmailNotConfigured) {
			return nil, ErrRedeemCodeDeliveryUnavailable
		}
		return nil, infraerrors.ServiceUnavailable("REDEEM_CODE_DELIVERY_FAILED", "failed to send the redeem code email: "+err.Error())
	}

	record := redeemCodeDeliveryRecord{UserID: target.ID, Email: recipient, SentAt: time.Now().UTC()}
	payload, _ := json.Marshal(record)
	if err := s.settingRepo.Set(ctx, key, string(payload)); err != nil {
		// 信已经发出去了，记录失败只影响之后的换人拦截，不能让调用方以为没发、再发一次。
		slog.Warn("redeem_code_delivery.record_failed", "code_id", code.ID, "error", err.Error())
	}
	return &RedeemCodeDeliveryResult{CodeID: code.ID, UserID: target.ID, Email: recipient, SentAt: record.SentAt}, nil
}

func (s *PurchaseNotifyService) loadRedeemCodeDelivery(ctx context.Context, key string) (*redeemCodeDeliveryRecord, error) {
	raw, err := s.settingRepo.GetValue(ctx, key)
	if err != nil {
		if errors.Is(err, ErrSettingNotFound) {
			return nil, nil
		}
		return nil, fmt.Errorf("load redeem code delivery record: %w", err)
	}
	if strings.TrimSpace(raw) == "" {
		return nil, nil
	}
	var record redeemCodeDeliveryRecord
	if err := json.Unmarshal([]byte(raw), &record); err != nil {
		return nil, fmt.Errorf("decode redeem code delivery record: %w", err)
	}
	return &record, nil
}

func redeemCodeDeliveryKey(codeID int64) string {
	return redeemCodeDeliveryKeyPrefix + strconv.FormatInt(codeID, 10)
}

// buildRedeemCodeDeliveryVariables 按码的类型生成邮件里的到账说明；纯函数，便于回归测试钉死"占位符必须传满"。
// 余额套餐的额度取档位当前值，和兑换时 GrantByRedeemCode 读的是同一份。
func buildRedeemCodeDeliveryVariables(code *RedeemCode, locale string) map[string]string {
	zh := isChinesePurchaseLocale(locale)
	variables := map[string]string{
		"full_redeem_code": strings.TrimSpace(code.Code),
		"code_expires_at":  redeemCodeDeliveryExpiry(code.ExpiresAt, zh),
	}

	if code.Type == RedeemTypeBalancePackage && code.BalancePackagePlan != nil {
		plan := code.BalancePackagePlan
		name := strings.TrimSpace(plan.Name)
		if name == "" {
			name = purchaseFallbackPlanName(locale)
		}
		weekly := formatPurchaseUSD(plan.WeeklyCreditUSD)
		variables["reward_name"] = name
		if zh {
			variables["reward_value"] = fmt.Sprintf("每期 $%s · 共 %d 期", weekly, plan.RefreshCount)
			variables["reward_note"] = fmt.Sprintf("兑换后第 1 期 $%s 立即到账，之后每 %d 天到账一期，共 %d 期。已有生效中的余额套餐时，需要等它结束后再兑换。",
				weekly, plan.RefreshIntervalDays, plan.RefreshCount)
			if plan.RefreshCount <= 1 {
				variables["reward_note"] = fmt.Sprintf("兑换后 $%s 立即到账。已有生效中的余额套餐时，需要等它结束后再兑换。", weekly)
			}
		} else {
			variables["reward_value"] = fmt.Sprintf("$%s per period · %d periods", weekly, plan.RefreshCount)
			variables["reward_note"] = fmt.Sprintf("The first $%s is credited right after redemption, then one period every %d days, %d periods in total. If you already have an active balance package, redeem this after it ends.",
				weekly, plan.RefreshIntervalDays, plan.RefreshCount)
			if plan.RefreshCount <= 1 {
				variables["reward_note"] = fmt.Sprintf("$%s is credited right after redemption. If you already have an active balance package, redeem this after it ends.", weekly)
			}
		}
		return variables
	}

	amount := formatPurchaseUSD(code.Value)
	variables["reward_value"] = "$" + amount
	if zh {
		variables["reward_name"] = "账户余额 $" + amount
		variables["reward_note"] = "兑换后 $" + amount + " 直接加入账户余额。"
	} else {
		variables["reward_name"] = "$" + amount + " account balance"
		variables["reward_note"] = "$" + amount + " is added to your account balance right after redemption."
	}
	return variables
}

func redeemCodeDeliveryExpiry(expiresAt *time.Time, zh bool) string {
	if expiresAt == nil || expiresAt.IsZero() {
		if zh {
			return "长期有效"
		}
		return "No expiry"
	}
	return formatPurchaseTime(expiresAt)
}
