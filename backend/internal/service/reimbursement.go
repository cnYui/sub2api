package service

import (
	"context"
	"fmt"
	"math"
	"net/url"
	"strings"
	"time"
	"unicode"

	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
	"github.com/Wei-Shaw/sub2api/internal/pkg/pagination"
)

// 报销/开票申请状态：只有两态，PDF 上传即完成。
const (
	ReimbursementStatusPending   = "pending"
	ReimbursementStatusCompleted = "completed"
)

// 六个业务字段的 JSON 键名，顺序固定（missing 数组按此顺序输出）。
const (
	ReimbursementFieldCompanyName = "company_name"
	ReimbursementFieldTaxID       = "tax_id"
	ReimbursementFieldBankAccount = "bank_account"
	ReimbursementFieldBankName    = "bank_name"
	ReimbursementFieldAddress     = "address"
	ReimbursementFieldAmount      = "amount"
)

// ReimbursementFieldKeys 是六个字段的固定顺序。
var ReimbursementFieldKeys = []string{
	ReimbursementFieldCompanyName,
	ReimbursementFieldTaxID,
	ReimbursementFieldBankAccount,
	ReimbursementFieldBankName,
	ReimbursementFieldAddress,
	ReimbursementFieldAmount,
}

// 字段长度与金额上限，与 ent schema / 迁移的列定义对齐。
const (
	reimbursementMaxCompanyNameLen = 255
	reimbursementMaxTaxIDLen       = 64
	reimbursementMaxBankAccountLen = 64
	reimbursementMaxBankNameLen    = 255
	reimbursementMaxAddressLen     = 500
	reimbursementMaxAmount         = 1e9
	// ReimbursementMaxParseTextRunes 是单次解析文本的上限（字符数）。
	ReimbursementMaxParseTextRunes = 5000
	// ReimbursementMaxRawTextRunes 是入库原文的上限（字符数）。
	ReimbursementMaxRawTextRunes = 20000
	// ReimbursementMaxPDFSize 是发票 PDF 的大小上限。
	ReimbursementMaxPDFSize        int64 = 20 << 20
	reimbursementMaxPDFFileNameLen       = 255
	// ReimbursementParseDailyLimit 是单用户每 UTC 日的 LLM 解析次数上限；
	// 每次解析都要付上游 token 费，按分钟限流挡不住一整天的慢速刷量。
	ReimbursementParseDailyLimit = 200
)

var (
	ErrReimbursementTextRequired     = infraerrors.BadRequest("REIMBURSEMENT_TEXT_REQUIRED", "text is required")
	ErrReimbursementTextTooLong      = infraerrors.BadRequest("REIMBURSEMENT_TEXT_TOO_LONG", "text is too long")
	ErrReimbursementLLMNotConfigured = infraerrors.ServiceUnavailable("REIMBURSEMENT_LLM_NOT_CONFIGURED", "解析服务未配置，请联系管理员")
	ErrReimbursementLLMFailed        = infraerrors.ServiceUnavailable("REIMBURSEMENT_LLM_FAILED", "解析服务暂时不可用，请稍后重试")
	ErrReimbursementNotFound         = infraerrors.NotFound("REIMBURSEMENT_NOT_FOUND", "reimbursement request not found")
	ErrReimbursementPDFNotReady      = infraerrors.NotFound("REIMBURSEMENT_PDF_NOT_READY", "invoice pdf is not ready")
	ErrReimbursementPDFInvalid       = infraerrors.BadRequest("REIMBURSEMENT_PDF_INVALID", "file must be a valid pdf")
	ErrReimbursementPDFTooLarge      = infraerrors.BadRequest("REIMBURSEMENT_PDF_TOO_LARGE", "pdf exceeds 20MB limit")
	ErrReimbursementConfigInvalid    = infraerrors.BadRequest("REIMBURSEMENT_CONFIG_INVALID", "invalid reimbursement llm config")
	// ErrReimbursementParseQuotaExceeded 是当日解析次数用尽。
	ErrReimbursementParseQuotaExceeded = infraerrors.TooManyRequests("REIMBURSEMENT_PARSE_QUOTA_EXCEEDED", "今日解析次数已达上限，请明天再试")
)

// NewReimbursementIncompleteError 生成带缺失字段清单的 400 错误，message 形如
// "missing fields: address, amount"，前端直接展示。
func NewReimbursementIncompleteError(missing []string) *infraerrors.ApplicationError {
	return infraerrors.BadRequest("REIMBURSEMENT_INCOMPLETE", "missing fields: "+strings.Join(missing, ", "))
}

// ReimbursementFields 是 LLM 抽取 / 用户提交的六个字段，缺失用 nil 表示，
// 序列化时六个键总是全部存在（缺失为 null）。
type ReimbursementFields struct {
	CompanyName *string  `json:"company_name"`
	TaxID       *string  `json:"tax_id"`
	BankAccount *string  `json:"bank_account"`
	BankName    *string  `json:"bank_name"`
	Address     *string  `json:"address"`
	Amount      *float64 `json:"amount"`
}

// ReimbursementParseResult 是一次解析的完整结果。
type ReimbursementParseResult struct {
	Fields   ReimbursementFields `json:"fields"`
	Missing  []string            `json:"missing"`
	Complete bool                `json:"complete"`
	Notes    *string             `json:"notes"`
}

// ReimbursementRequest 是领域对象；User 仅在管理端列表 WithUser 预加载时非 nil。
type ReimbursementRequest struct {
	ID            int64
	UserID        int64
	RawText       string
	CompanyName   string
	TaxID         string
	BankAccount   string
	BankName      string
	Address       string
	Amount        float64
	Status        string
	PDFPath       string
	PDFFileName   string
	PDFSize       int64
	PDFSha256     string
	PDFUploadedAt *time.Time
	HandledBy     *int64
	CompletedAt   *time.Time
	NotifiedAt    *time.Time
	CreatedAt     time.Time
	UpdatedAt     time.Time

	User *User
}

// PDFAvailable 返回用户是否可以下载 PDF：必须已完成且文件路径已落库。
func (r *ReimbursementRequest) PDFAvailable() bool {
	return r != nil && r.Status == ReimbursementStatusCompleted && strings.TrimSpace(r.PDFPath) != ""
}

// ReimbursementListFilters 是管理端列表过滤条件。
type ReimbursementListFilters struct {
	Status string
	Search string
}

// ReimbursementPDFUpdate 是上传 PDF 后要写回数据库的字段。
type ReimbursementPDFUpdate struct {
	PDFPath     string
	PDFFileName string
	PDFSize     int64
	PDFSha256   string
	UploadedAt  time.Time
	HandledBy   int64
}

// ReimbursementRepository 是申请单的持久化接口，由 repository 包用 ent 实现。
type ReimbursementRepository interface {
	Create(ctx context.Context, r *ReimbursementRequest) error
	// GetByID 预加载用户；不存在返回 ErrReimbursementNotFound。
	GetByID(ctx context.Context, id int64) (*ReimbursementRequest, error)
	// ListByUser 只返回该用户自己的申请，按 created_at 降序。
	ListByUser(ctx context.Context, userID int64, params pagination.PaginationParams) ([]ReimbursementRequest, *pagination.PaginationResult, error)
	// List 是管理端列表，预加载用户，排序白名单见实现。
	List(ctx context.Context, params pagination.PaginationParams, filters ReimbursementListFilters) ([]ReimbursementRequest, *pagination.PaginationResult, error)
	// AttachPDF 写入 PDF 元数据并置为 completed；已完成的记录视为重新上传，
	// completed_at 保持首次值。返回更新后的完整记录（含用户）。
	AttachPDF(ctx context.Context, id int64, update ReimbursementPDFUpdate) (*ReimbursementRequest, error)
	// MarkNotified 记录通知邮件发送时间，失败由调用方忽略。
	MarkNotified(ctx context.Context, id int64, at time.Time) error
}

// NormalizeReimbursementFields 按契约做服务端归一化：
// 字符串 trim 并限长，税号去空白/连字符转大写，账号只留数字，金额两位小数；
// 归一化后为空 / 非正 / 超上限的字段置 nil 表示缺失。
func NormalizeReimbursementFields(in ReimbursementFields) ReimbursementFields {
	var out ReimbursementFields
	out.CompanyName = normalizeReimbursementText(in.CompanyName, reimbursementMaxCompanyNameLen)
	out.TaxID = normalizeReimbursementTaxID(in.TaxID)
	out.BankAccount = normalizeReimbursementBankAccount(in.BankAccount)
	out.BankName = normalizeReimbursementText(in.BankName, reimbursementMaxBankNameLen)
	out.Address = normalizeReimbursementText(in.Address, reimbursementMaxAddressLen)
	out.Amount = normalizeReimbursementAmount(in.Amount)
	return out
}

// MissingReimbursementFields 返回缺失字段键名，顺序固定为 ReimbursementFieldKeys。
// 调用方应先归一化。
func MissingReimbursementFields(f ReimbursementFields) []string {
	missing := make([]string, 0, len(ReimbursementFieldKeys))
	if f.CompanyName == nil {
		missing = append(missing, ReimbursementFieldCompanyName)
	}
	if f.TaxID == nil {
		missing = append(missing, ReimbursementFieldTaxID)
	}
	if f.BankAccount == nil {
		missing = append(missing, ReimbursementFieldBankAccount)
	}
	if f.BankName == nil {
		missing = append(missing, ReimbursementFieldBankName)
	}
	if f.Address == nil {
		missing = append(missing, ReimbursementFieldAddress)
	}
	if f.Amount == nil {
		missing = append(missing, ReimbursementFieldAmount)
	}
	return missing
}

// MergeReimbursementFields 做防御性合并：LLM 本轮返回 null 而上一轮有值 → 沿用上一轮。
// 两边都应是归一化后的值。
func MergeReimbursementFields(current ReimbursementFields, previous *ReimbursementFields) ReimbursementFields {
	if previous == nil {
		return current
	}
	if current.CompanyName == nil {
		current.CompanyName = previous.CompanyName
	}
	if current.TaxID == nil {
		current.TaxID = previous.TaxID
	}
	if current.BankAccount == nil {
		current.BankAccount = previous.BankAccount
	}
	if current.BankName == nil {
		current.BankName = previous.BankName
	}
	if current.Address == nil {
		current.Address = previous.Address
	}
	if current.Amount == nil {
		current.Amount = previous.Amount
	}
	return current
}

// BuildReimbursementParseResult 由归一化后的字段生成完整解析结果。
func BuildReimbursementParseResult(fields ReimbursementFields, notes *string) *ReimbursementParseResult {
	missing := MissingReimbursementFields(fields)
	if notes != nil {
		trimmed := strings.TrimSpace(*notes)
		if trimmed == "" {
			notes = nil
		} else {
			notes = &trimmed
		}
	}
	return &ReimbursementParseResult{
		Fields:   fields,
		Missing:  missing,
		Complete: len(missing) == 0,
		Notes:    notes,
	}
}

func normalizeReimbursementText(v *string, maxLen int) *string {
	if v == nil {
		return nil
	}
	s := strings.TrimSpace(*v)
	if s == "" {
		return nil
	}
	s = truncateRunes(s, maxLen)
	return &s
}

func normalizeReimbursementTaxID(v *string) *string {
	if v == nil {
		return nil
	}
	var b strings.Builder
	for _, r := range *v {
		if unicode.IsSpace(r) || r == '-' || r == '－' || r == '—' || r == '_' {
			continue
		}
		_, _ = b.WriteRune(unicode.ToUpper(r))
	}
	s := b.String()
	if s == "" {
		return nil
	}
	s = truncateRunes(s, reimbursementMaxTaxIDLen)
	return &s
}

func normalizeReimbursementBankAccount(v *string) *string {
	if v == nil {
		return nil
	}
	var b strings.Builder
	for _, r := range *v {
		if r >= '0' && r <= '9' {
			_, _ = b.WriteRune(r)
		}
	}
	s := b.String()
	if s == "" {
		return nil
	}
	s = truncateRunes(s, reimbursementMaxBankAccountLen)
	return &s
}

func normalizeReimbursementAmount(v *float64) *float64 {
	if v == nil {
		return nil
	}
	amount := *v
	if math.IsNaN(amount) || math.IsInf(amount, 0) || amount <= 0 || amount > reimbursementMaxAmount {
		return nil
	}
	amount = math.Round(amount*100) / 100
	if amount <= 0 {
		return nil
	}
	return &amount
}

func truncateRunes(s string, max int) string {
	if max <= 0 {
		return s
	}
	runes := []rune(s)
	if len(runes) <= max {
		return s
	}
	return string(runes[:max])
}

// SanitizeReimbursementPDFFileName 清洗上传原名用于展示与下载：
// 去掉路径分隔符、引号与控制字符，限长 255，空则回退 invoice-<id>.pdf。
// 该名字绝不参与落盘路径拼接。
func SanitizeReimbursementPDFFileName(name string, id int64) string {
	name = strings.TrimSpace(name)
	var b strings.Builder
	for _, r := range name {
		switch {
		case r == '/' || r == '\\' || r == '"' || r == '\'' || r == ':':
			_ = b.WriteByte('_')
		case unicode.IsControl(r):
			continue
		default:
			_, _ = b.WriteRune(r)
		}
	}
	out := strings.Trim(b.String(), ". ")
	for strings.Contains(out, "..") {
		out = strings.ReplaceAll(out, "..", "_")
	}
	if out == "" || strings.EqualFold(out, ".pdf") {
		return DefaultReimbursementPDFFileName(id)
	}
	return truncateRunes(out, reimbursementMaxPDFFileNameLen)
}

// DefaultReimbursementPDFFileName 是没有可用原名时的下载文件名。
func DefaultReimbursementPDFFileName(id int64) string {
	return fmt.Sprintf("invoice-%d.pdf", id)
}

// ReimbursementPDFContentDisposition 生成下载头：ASCII 兜底名 + RFC 5987 编码的原名，
// 让浏览器能显示中文文件名，又不会因引号/分号破坏头部。
func ReimbursementPDFContentDisposition(id int64, fileName string) string {
	fallback := DefaultReimbursementPDFFileName(id)
	name := strings.TrimSpace(fileName)
	if name == "" {
		name = fallback
	}
	return fmt.Sprintf(`attachment; filename="%s"; filename*=UTF-8''%s`, fallback, url.PathEscape(name))
}
