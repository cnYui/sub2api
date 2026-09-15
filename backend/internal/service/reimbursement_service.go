package service

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"html"
	"io"
	"log/slog"
	"os"
	"path"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"
	"unicode/utf8"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/pkg/pagination"
)

const (
	reimbursementPDFSubdir       = "reimbursement"
	reimbursementPDFMagic        = "%PDF-"
	reimbursementNotifyTimeout   = 30 * time.Second
	reimbursementDownloadPagePth = "/reimbursement"
)

// ReimbursementService 承载报销/开票申请的业务：解析、提交、列表、PDF 上传/下载、通知。
type ReimbursementService struct {
	repo         ReimbursementRepository
	parser       *ReimbursementParser
	userRepo     UserRepository
	emailService *EmailService
	settingRepo  SettingRepository
	cfg          *config.Config

	now func() time.Time
	// notifyDone 非 nil 时，每次异步通知结束后回调，单测用来等待 goroutine。
	notifyDone func()

	// parseQuota 是按用户的当日解析次数（UTC 日期归零）。
	// 这是进程内内存限额：生产只跑单实例，重启清零可接受，不值得为此引入 Redis 依赖。
	parseQuotaMu sync.Mutex
	parseQuota   map[int64]reimbursementParseQuota
}

// reimbursementParseQuota 记录某用户在 day（UTC，YYYY-MM-DD）内已消耗的解析次数。
type reimbursementParseQuota struct {
	day   string
	count int
}

func NewReimbursementService(
	repo ReimbursementRepository,
	parser *ReimbursementParser,
	userRepo UserRepository,
	emailService *EmailService,
	settingRepo SettingRepository,
	cfg *config.Config,
) *ReimbursementService {
	return &ReimbursementService{
		repo:         repo,
		parser:       parser,
		userRepo:     userRepo,
		emailService: emailService,
		settingRepo:  settingRepo,
		cfg:          cfg,
		now:          time.Now,
		parseQuota:   map[int64]reimbursementParseQuota{},
	}
}

// CreateReimbursementInput 是用户提交申请的输入，字段由服务端再次归一化与校验。
type CreateReimbursementInput struct {
	Fields  ReimbursementFields
	RawText string
}

// AttachReimbursementPDFInput 是管理端上传 PDF 的输入。
type AttachReimbursementPDFInput struct {
	OriginalName string
	// DeclaredSize 是 multipart 头里声明的大小，超限直接拒绝，避免白读 20MB。
	DeclaredSize int64
	Reader       io.Reader
	AdminID      int64
}

// ReimbursementPDFStream 是下载用的文件流，调用方负责 Close。
type ReimbursementPDFStream struct {
	Reader   io.ReadCloser
	Size     int64
	FileName string
}

// Parse 校验文本后调 LLM；previous 为上一轮结果时做补充解析。
// 每个用户每 UTC 日最多 ReimbursementParseDailyLimit 次，超出返回 ErrReimbursementParseQuotaExceeded；
// 管理端 TestLLMConfig 不走这里、不计数。
func (s *ReimbursementService) Parse(ctx context.Context, userID int64, text string, previous *ReimbursementFields) (*ReimbursementParseResult, error) {
	text = strings.TrimSpace(text)
	if text == "" {
		return nil, ErrReimbursementTextRequired
	}
	if utf8.RuneCountInString(text) > ReimbursementMaxParseTextRunes {
		return nil, ErrReimbursementTextTooLong
	}
	if s.parser == nil {
		return nil, ErrReimbursementLLMNotConfigured
	}
	// 在真正调 LLM 之前计数：失败的调用同样消耗上游成本，也应计入。
	if err := s.consumeParseQuota(userID); err != nil {
		return nil, err
	}
	return s.parser.Parse(ctx, text, previous)
}

// consumeParseQuota 占用一次当日解析额度；额度已满时不占用并返回错误。
func (s *ReimbursementService) consumeParseQuota(userID int64) error {
	day := s.now().UTC().Format("2006-01-02")
	s.parseQuotaMu.Lock()
	defer s.parseQuotaMu.Unlock()
	if s.parseQuota == nil {
		s.parseQuota = map[int64]reimbursementParseQuota{}
	}
	quota := s.parseQuota[userID]
	if quota.day != day {
		// 跨日时顺手清掉其它用户的过期条目，避免 map 随历史用户数无限增长。
		for id, q := range s.parseQuota {
			if q.day != day {
				delete(s.parseQuota, id)
			}
		}
		quota = reimbursementParseQuota{day: day}
	}
	if quota.count >= ReimbursementParseDailyLimit {
		return ErrReimbursementParseQuotaExceeded
	}
	quota.count++
	s.parseQuota[userID] = quota
	return nil
}

// Create 归一化并要求六项齐全后入库，状态 pending。
func (s *ReimbursementService) Create(ctx context.Context, userID int64, input CreateReimbursementInput) (*ReimbursementRequest, error) {
	if userID <= 0 {
		return nil, ErrReimbursementNotFound
	}
	fields := NormalizeReimbursementFields(input.Fields)
	if missing := MissingReimbursementFields(fields); len(missing) > 0 {
		return nil, NewReimbursementIncompleteError(missing)
	}
	rawText := strings.TrimSpace(input.RawText)
	if utf8.RuneCountInString(rawText) > ReimbursementMaxRawTextRunes {
		return nil, ErrReimbursementTextTooLong
	}
	record := &ReimbursementRequest{
		UserID:      userID,
		RawText:     rawText,
		CompanyName: *fields.CompanyName,
		TaxID:       *fields.TaxID,
		BankAccount: *fields.BankAccount,
		BankName:    *fields.BankName,
		Address:     *fields.Address,
		Amount:      *fields.Amount,
		Status:      ReimbursementStatusPending,
	}
	if err := s.repo.Create(ctx, record); err != nil {
		return nil, fmt.Errorf("create reimbursement request: %w", err)
	}
	return record, nil
}

// ListForUser 返回用户自己的申请，最新在前。
func (s *ReimbursementService) ListForUser(ctx context.Context, userID int64, params pagination.PaginationParams) ([]ReimbursementRequest, *pagination.PaginationResult, error) {
	if userID <= 0 {
		return []ReimbursementRequest{}, &pagination.PaginationResult{Page: params.Page, PageSize: params.Limit(), Pages: 1}, nil
	}
	return s.repo.ListByUser(ctx, userID, params)
}

// GetForUser 取单条；不是本人的记录一律当不存在，不泄露 ID 是否存在。
func (s *ReimbursementService) GetForUser(ctx context.Context, userID, id int64) (*ReimbursementRequest, error) {
	if userID <= 0 || id <= 0 {
		return nil, ErrReimbursementNotFound
	}
	record, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if record.UserID != userID {
		return nil, ErrReimbursementNotFound
	}
	return record, nil
}

// OpenPDFForUser 仅本人且已完成且文件存在时返回文件流。
func (s *ReimbursementService) OpenPDFForUser(ctx context.Context, userID, id int64) (*ReimbursementPDFStream, error) {
	record, err := s.GetForUser(ctx, userID, id)
	if err != nil {
		return nil, err
	}
	return s.openPDF(record)
}

// ListAll 是管理端列表。
func (s *ReimbursementService) ListAll(ctx context.Context, params pagination.PaginationParams, filters ReimbursementListFilters) ([]ReimbursementRequest, *pagination.PaginationResult, error) {
	filters.Status = strings.ToLower(strings.TrimSpace(filters.Status))
	filters.Search = strings.TrimSpace(filters.Search)
	if utf8.RuneCountInString(filters.Search) > 200 {
		filters.Search = truncateRunes(filters.Search, 200)
	}
	return s.repo.List(ctx, params, filters)
}

// GetByID 是管理端取单条。
func (s *ReimbursementService) GetByID(ctx context.Context, id int64) (*ReimbursementRequest, error) {
	if id <= 0 {
		return nil, ErrReimbursementNotFound
	}
	return s.repo.GetByID(ctx, id)
}

// OpenPDF 是管理端下载，不限本人。
func (s *ReimbursementService) OpenPDF(ctx context.Context, id int64) (*ReimbursementPDFStream, error) {
	record, err := s.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	return s.openPDF(record)
}

func (s *ReimbursementService) openPDF(record *ReimbursementRequest) (*ReimbursementPDFStream, error) {
	if !record.PDFAvailable() {
		return nil, ErrReimbursementPDFNotReady
	}
	fullPath, ok := s.resolvePDFPath(record.ID, record.PDFPath)
	if !ok {
		slog.Warn("reimbursement.pdf_path_rejected", "request_id", record.ID)
		return nil, ErrReimbursementPDFNotReady
	}
	f, err := os.Open(fullPath)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			slog.Warn("reimbursement.pdf_file_missing", "request_id", record.ID)
			return nil, ErrReimbursementPDFNotReady
		}
		return nil, fmt.Errorf("open reimbursement pdf: %w", err)
	}
	info, err := f.Stat()
	if err != nil {
		_ = f.Close()
		return nil, fmt.Errorf("stat reimbursement pdf: %w", err)
	}
	fileName := strings.TrimSpace(record.PDFFileName)
	if fileName == "" {
		fileName = DefaultReimbursementPDFFileName(record.ID)
	}
	return &ReimbursementPDFStream{Reader: f, Size: info.Size(), FileName: fileName}, nil
}

// AttachPDF 校验并落盘 PDF，更新记录为 completed，随后异步通知用户。
func (s *ReimbursementService) AttachPDF(ctx context.Context, id int64, input AttachReimbursementPDFInput) (*ReimbursementRequest, error) {
	if id <= 0 {
		return nil, ErrReimbursementNotFound
	}
	if input.Reader == nil {
		return nil, ErrReimbursementPDFInvalid
	}
	if input.DeclaredSize > ReimbursementMaxPDFSize {
		return nil, ErrReimbursementPDFTooLarge
	}
	if !strings.EqualFold(filepath.Ext(strings.TrimSpace(input.OriginalName)), ".pdf") {
		return nil, ErrReimbursementPDFInvalid
	}
	// 先确认记录存在，再动文件系统。
	if _, err := s.repo.GetByID(ctx, id); err != nil {
		return nil, err
	}

	dir, storedPath := s.pdfLocation(id)
	if err := os.MkdirAll(dir, 0o750); err != nil {
		return nil, fmt.Errorf("create reimbursement pdf dir: %w", err)
	}
	// 顺序：写临时文件 → 写库 → 改名到正式路径。
	// 库写失败时旧 PDF 原封不动、临时文件删掉；不能先改名再写库，否则写库失败会留下
	// 「文件已被新版覆盖、库里摘要还是旧版」的不一致。
	tmpName, size, sum, err := writeReimbursementPDFTemp(dir, input.Reader)
	if err != nil {
		return nil, err
	}

	now := s.now()
	updated, err := s.repo.AttachPDF(ctx, id, ReimbursementPDFUpdate{
		PDFPath:     storedPath,
		PDFFileName: SanitizeReimbursementPDFFileName(input.OriginalName, id),
		PDFSize:     size,
		PDFSha256:   sum,
		UploadedAt:  now,
		HandledBy:   input.AdminID,
	})
	if err != nil {
		_ = os.Remove(tmpName)
		return nil, err
	}
	target := filepath.Join(dir, reimbursementPDFFileName(id))
	if err := os.Rename(tmpName, target); err != nil {
		// 极端情况：库已记录新摘要，磁盘上仍是旧文件（或没有文件）。
		// 事务已提交无法回退，只能记 Error 让运维介入（重新上传即可自愈）。
		_ = os.Remove(tmpName)
		slog.Error("reimbursement.pdf_rename_failed_after_db_commit", "request_id", id, "error", err.Error())
		return nil, fmt.Errorf("rename reimbursement pdf: %w", err)
	}
	s.notifyCompletedAsync(updated)
	return updated, nil
}

// reimbursementPDFFileName 是落盘文件名，读写两侧共用，读取侧据此校验数据库路径。
func reimbursementPDFFileName(id int64) string {
	return strconv.FormatInt(id, 10) + ".pdf"
}

// writeReimbursementPDFTemp 校验魔数后写入同目录临时文件，边写边算 sha256 与大小；
// 超限中途终止并清理，绝不留下半截文件。成功时返回临时文件路径，由调用方在写库成功后改名。
func writeReimbursementPDFTemp(dir string, r io.Reader) (tmpName string, size int64, sum string, err error) {
	head := make([]byte, len(reimbursementPDFMagic))
	if _, err := io.ReadFull(r, head); err != nil || string(head) != reimbursementPDFMagic {
		return "", 0, "", ErrReimbursementPDFInvalid
	}
	tmp, err := os.CreateTemp(dir, "upload-*.tmp")
	if err != nil {
		return "", 0, "", fmt.Errorf("create reimbursement pdf temp: %w", err)
	}
	tmpName = tmp.Name()
	cleanup := func() {
		_ = tmp.Close()
		_ = os.Remove(tmpName)
	}
	hasher := sha256.New()
	w := io.MultiWriter(tmp, hasher)
	if _, err := w.Write(head); err != nil {
		cleanup()
		return "", 0, "", fmt.Errorf("write reimbursement pdf: %w", err)
	}
	// 多读 1 字节即可判定超限，避免依赖 multipart 声明的大小。
	n, err := io.Copy(w, io.LimitReader(r, ReimbursementMaxPDFSize-int64(len(head))+1))
	if err != nil {
		cleanup()
		return "", 0, "", fmt.Errorf("write reimbursement pdf: %w", err)
	}
	total := n + int64(len(head))
	if total > ReimbursementMaxPDFSize {
		cleanup()
		return "", 0, "", ErrReimbursementPDFTooLarge
	}
	if err := tmp.Sync(); err != nil {
		cleanup()
		return "", 0, "", fmt.Errorf("sync reimbursement pdf: %w", err)
	}
	if err := tmp.Close(); err != nil {
		_ = os.Remove(tmpName)
		return "", 0, "", fmt.Errorf("close reimbursement pdf: %w", err)
	}
	return tmpName, total, hex.EncodeToString(hasher.Sum(nil)), nil
}

// pdfLocation 返回落盘目录与写入数据库的路径。
// 默认目录在 DataDir 下，数据库存相对路径（换机器/换挂载点不用改库）；
// 被 PDFDir 覆盖时存绝对路径，读时用 filepath.IsAbs 区分。
func (s *ReimbursementService) pdfLocation(id int64) (dir string, storedPath string) {
	fileName := reimbursementPDFFileName(id)
	if custom := s.customPDFDir(); custom != "" {
		return custom, filepath.Join(custom, fileName)
	}
	return filepath.Join(s.dataDir(), reimbursementPDFSubdir), path.Join(reimbursementPDFSubdir, fileName)
}

func (s *ReimbursementService) customPDFDir() string {
	if s.cfg == nil {
		return ""
	}
	dir := strings.TrimSpace(s.cfg.Reimbursement.PDFDir)
	if dir == "" {
		return ""
	}
	if abs, err := filepath.Abs(dir); err == nil {
		return abs
	}
	return dir
}

func (s *ReimbursementService) dataDir() string {
	if s.cfg != nil && strings.TrimSpace(s.cfg.Pricing.DataDir) != "" {
		return strings.TrimSpace(s.cfg.Pricing.DataDir)
	}
	return "./data"
}

// resolvePDFPath 把数据库里的路径换成本机绝对路径，并校验它确实是我们自己写出的文件：
// 文件名必须等于 <id>.pdf（与 pdfLocation 的命名一致），相对路径还必须落在 DataDir/reimbursement 内。
// 绝对路径来自历史上生效过的 PDFDir，故意不与当前 customPDFDir() 绑定——否则日后改 pdf_dir
// 会让旧记录读不到；只靠文件名约束就足以挡住指向任意文件的篡改路径。
func (s *ReimbursementService) resolvePDFPath(id int64, stored string) (string, bool) {
	stored = strings.TrimSpace(stored)
	if stored == "" || id <= 0 {
		return "", false
	}
	var full string
	if filepath.IsAbs(stored) {
		full = filepath.Clean(stored)
	} else {
		base := filepath.Clean(filepath.Join(s.dataDir(), reimbursementPDFSubdir))
		full = filepath.Clean(filepath.Join(s.dataDir(), filepath.FromSlash(stored)))
		if !isPathWithinReimbursementDir(full, base) {
			return "", false
		}
	}
	if filepath.Base(full) != reimbursementPDFFileName(id) {
		return "", false
	}
	return full, true
}

func isPathWithinReimbursementDir(target, base string) bool {
	rel, err := filepath.Rel(base, target)
	if err != nil {
		return false
	}
	return rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator))
}

// notifyCompletedAsync 在事务外异步发邮件，失败只记日志、不影响上传结果。
func (s *ReimbursementService) notifyCompletedAsync(record *ReimbursementRequest) {
	if record == nil {
		return
	}
	snapshot := *record
	go func() {
		defer func() {
			if s.notifyDone != nil {
				s.notifyDone()
			}
		}()
		ctx, cancel := context.WithTimeout(context.Background(), reimbursementNotifyTimeout)
		defer cancel()
		if err := s.sendCompletedEmail(ctx, &snapshot); err != nil {
			if errors.Is(err, ErrEmailNotConfigured) {
				slog.Debug("reimbursement.notify_skipped_smtp_not_configured", "request_id", snapshot.ID)
				return
			}
			slog.Warn("reimbursement.notify_failed", "request_id", snapshot.ID, "error", err.Error())
			return
		}
		if s.repo != nil {
			if err := s.repo.MarkNotified(ctx, snapshot.ID, s.now()); err != nil {
				slog.Warn("reimbursement.mark_notified_failed", "request_id", snapshot.ID, "error", err.Error())
			}
		}
	}()
}

func (s *ReimbursementService) sendCompletedEmail(ctx context.Context, record *ReimbursementRequest) error {
	if s.emailService == nil {
		return ErrEmailNotConfigured
	}
	recipient, userID := s.recipientFor(ctx, record)
	if recipient == "" {
		slog.Debug("reimbursement.notify_skipped_no_recipient", "request_id", record.ID)
		return nil
	}
	siteName := s.siteName(ctx)
	variables := s.completedEmailVariables(ctx, record, siteName, recipient)
	if s.emailService.notificationEmailService != nil {
		err := s.emailService.notificationEmailService.Send(ctx, NotificationEmailSendInput{
			Event:          NotificationEmailEventReimbursementCompleted,
			RecipientEmail: recipient,
			RecipientName:  emailRecipientName(recipient),
			UserID:         userID,
			SourceType:     "reimbursement",
			SourceID:       strconv.FormatInt(record.ID, 10),
			ReminderKey:    reimbursementReminderKey(record),
			Variables:      variables,
		})
		if err == nil {
			return nil
		}
		if !shouldFallbackNotificationEmail(err) {
			return err
		}
		slog.Warn("reimbursement.template_email_failed_fallback", "request_id", record.ID, "error", err.Error())
	}
	subject := fmt.Sprintf("[%s] 发票已上传 / Invoice ready", sanitizeEmailHeader(siteName))
	body := buildReimbursementCompletedEmailBody(variables)
	return s.emailService.SendEmail(ctx, recipient, subject, body)
}

// reimbursementReminderKey 用上传时间做去重键：同一申请重新上传 PDF 时可以再次通知。
func reimbursementReminderKey(record *ReimbursementRequest) string {
	if record.PDFUploadedAt != nil {
		return strconv.FormatInt(record.PDFUploadedAt.Unix(), 10)
	}
	return ""
}

func (s *ReimbursementService) recipientFor(ctx context.Context, record *ReimbursementRequest) (string, int64) {
	if record.User != nil && strings.TrimSpace(record.User.Email) != "" {
		return strings.TrimSpace(record.User.Email), record.User.ID
	}
	if s.userRepo == nil || record.UserID <= 0 {
		return "", 0
	}
	user, err := s.userRepo.GetByID(ctx, record.UserID)
	if err != nil || user == nil {
		return "", 0
	}
	return strings.TrimSpace(user.Email), user.ID
}

func (s *ReimbursementService) siteName(ctx context.Context) string {
	if s.settingRepo == nil {
		return defaultSiteName
	}
	name, err := s.settingRepo.GetValue(ctx, SettingKeySiteName)
	if err != nil || strings.TrimSpace(name) == "" {
		return defaultSiteName
	}
	return strings.TrimSpace(name)
}

// downloadPageURL 取站点前端地址拼下载页；settings 优先，其次 config，都没有则留空。
func (s *ReimbursementService) downloadPageURL(ctx context.Context) string {
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
	return strings.TrimRight(base, "/") + reimbursementDownloadPagePth
}

func (s *ReimbursementService) completedEmailVariables(ctx context.Context, record *ReimbursementRequest, siteName, recipient string) map[string]string {
	return map[string]string{
		"site_name":         siteName,
		"recipient_name":    emailRecipientName(recipient),
		"recipient_email":   recipient,
		"company_name":      record.CompanyName,
		"amount":            strconv.FormatFloat(record.Amount, 'f', 2, 64),
		"request_id":        strconv.FormatInt(record.ID, 10),
		"download_page_url": s.downloadPageURL(ctx),
	}
}

// buildReimbursementCompletedEmailBody 是模板不可用时的内置 HTML 兜底。
func buildReimbursementCompletedEmailBody(vars map[string]string) string {
	link := ""
	if u := vars["download_page_url"]; u != "" {
		link = `<p><a href="` + html.EscapeString(u) + `">前往下载 / Download</a></p>`
	}
	return `<!DOCTYPE html><html><head><meta charset="UTF-8"></head><body style="font-family:-apple-system,BlinkMacSystemFont,'Segoe UI',Roboto,sans-serif;">` +
		`<h2>发票已上传 / Invoice ready</h2>` +
		`<p>` + html.EscapeString(vars["recipient_name"]) + `，您好：</p>` +
		`<p>您提交的开票申请（编号 #` + html.EscapeString(vars["request_id"]) + `，抬头 ` + html.EscapeString(vars["company_name"]) +
		`，金额 ¥` + html.EscapeString(vars["amount"]) + `）已处理完成，发票 PDF 已上传，可登录后在「报销/开票」页面下载。</p>` +
		`<p>Your invoice request #` + html.EscapeString(vars["request_id"]) + ` has been completed. The PDF is ready for download on the Reimbursement page.</p>` +
		link +
		`<p style="color:#888;font-size:12px;">` + html.EscapeString(vars["site_name"]) + `</p></body></html>`
}

// GetLLMConfig / UpdateLLMConfig / TestLLMConfig 是管理端配置接口的薄封装。
func (s *ReimbursementService) GetLLMConfig(ctx context.Context) (*ReimbursementLLMConfigView, error) {
	if s.parser == nil {
		return nil, ErrReimbursementLLMNotConfigured
	}
	return s.parser.GetConfig(ctx)
}

func (s *ReimbursementService) UpdateLLMConfig(ctx context.Context, input UpdateReimbursementLLMConfigInput) (*ReimbursementLLMConfigView, error) {
	if s.parser == nil {
		return nil, ErrReimbursementLLMNotConfigured
	}
	return s.parser.UpdateConfig(ctx, input)
}

// TestLLMConfig 缺省文本用契约案例 3。
func (s *ReimbursementService) TestLLMConfig(ctx context.Context, text string) (*ReimbursementLLMTestResult, error) {
	if s.parser == nil {
		return nil, ErrReimbursementLLMNotConfigured
	}
	text = strings.TrimSpace(text)
	if text == "" {
		text = ReimbursementSampleText
	}
	if utf8.RuneCountInString(text) > ReimbursementMaxParseTextRunes {
		return nil, ErrReimbursementTextTooLong
	}
	return s.parser.Test(ctx, text)
}

// ReimbursementSampleText 是管理端测试解析的缺省样例（契约案例 3）。
const ReimbursementSampleText = `名称: 北京和讯在线信息咨询服务有限公司
纳税人识别号: 91110105723558454P
开户行: 中国工商银行股份有限公司北京东城支行
银行账号: 0200080709024530517
地址: 北京市朝阳区朝外大街22号10层A1-3
电话: 010-85650972   金额  49`
