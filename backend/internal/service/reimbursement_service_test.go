//go:build unit

package service

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
	"unicode/utf8"

	"github.com/Wei-Shaw/sub2api/internal/config"
	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
	"github.com/Wei-Shaw/sub2api/internal/pkg/pagination"
	"github.com/stretchr/testify/require"
)

// fakeReimbursementRepo 是内存仓储，只覆盖 service 用到的语义（含 completed_at 首次保持）。
type fakeReimbursementRepo struct {
	mu     sync.Mutex
	nextID int64
	items  map[int64]*ReimbursementRequest
	// lastListFilters 记录最近一次 List 收到的过滤条件，用来断言服务层的归一化。
	lastListFilters ReimbursementListFilters
	// attachErr 非 nil 时 AttachPDF 直接失败，模拟写库失败。
	attachErr error
}

func newFakeReimbursementRepo() *fakeReimbursementRepo {
	return &fakeReimbursementRepo{nextID: 1, items: map[int64]*ReimbursementRequest{}}
}

func (r *fakeReimbursementRepo) Create(_ context.Context, req *ReimbursementRequest) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	req.ID = r.nextID
	r.nextID++
	now := time.Now()
	req.CreatedAt = now
	req.UpdatedAt = now
	clone := *req
	r.items[req.ID] = &clone
	return nil
}

func (r *fakeReimbursementRepo) GetByID(_ context.Context, id int64) (*ReimbursementRequest, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	item, ok := r.items[id]
	if !ok {
		return nil, ErrReimbursementNotFound
	}
	clone := *item
	return &clone, nil
}

func (r *fakeReimbursementRepo) ListByUser(_ context.Context, userID int64, params pagination.PaginationParams) ([]ReimbursementRequest, *pagination.PaginationResult, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	out := make([]ReimbursementRequest, 0)
	for _, item := range r.items {
		if item.UserID == userID {
			out = append(out, *item)
		}
	}
	return out, &pagination.PaginationResult{Total: int64(len(out)), Page: params.Page, PageSize: params.Limit(), Pages: 1}, nil
}

func (r *fakeReimbursementRepo) List(_ context.Context, params pagination.PaginationParams, filters ReimbursementListFilters) ([]ReimbursementRequest, *pagination.PaginationResult, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.lastListFilters = filters
	out := make([]ReimbursementRequest, 0, len(r.items))
	for _, item := range r.items {
		out = append(out, *item)
	}
	return out, &pagination.PaginationResult{Total: int64(len(out)), Page: params.Page, PageSize: params.Limit(), Pages: 1}, nil
}

func (r *fakeReimbursementRepo) AttachPDF(_ context.Context, id int64, update ReimbursementPDFUpdate) (*ReimbursementRequest, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.attachErr != nil {
		return nil, r.attachErr
	}
	item, ok := r.items[id]
	if !ok {
		return nil, ErrReimbursementNotFound
	}
	if item.CompletedAt == nil {
		at := update.UploadedAt
		item.CompletedAt = &at
	}
	uploadedAt := update.UploadedAt
	item.Status = ReimbursementStatusCompleted
	item.PDFPath = update.PDFPath
	item.PDFFileName = update.PDFFileName
	item.PDFSize = update.PDFSize
	item.PDFSha256 = update.PDFSha256
	item.PDFUploadedAt = &uploadedAt
	handledBy := update.HandledBy
	item.HandledBy = &handledBy
	item.UpdatedAt = update.UploadedAt
	clone := *item
	return &clone, nil
}

func (r *fakeReimbursementRepo) MarkNotified(_ context.Context, id int64, at time.Time) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	item, ok := r.items[id]
	if !ok {
		return ErrReimbursementNotFound
	}
	item.NotifiedAt = &at
	return nil
}

func newReimbursementTestService(t *testing.T, repo *fakeReimbursementRepo, cfgMut func(*config.Config)) (*ReimbursementService, string) {
	t.Helper()
	dataDir := t.TempDir()
	cfg := &config.Config{}
	cfg.Pricing.DataDir = dataDir
	if cfgMut != nil {
		cfgMut(cfg)
	}
	// emailService 为 nil：异步通知直接跳过，用 notifyDone 等待 goroutine 退出，避免与 TempDir 清理竞争。
	svc := NewReimbursementService(repo, nil, nil, nil, newStubSettingRepo(), cfg)
	return svc, dataDir
}

func completeReimbursementFields() ReimbursementFields {
	return ReimbursementFields{
		CompanyName: reimbStr("上海熠视智能科技有限公司"),
		TaxID:       reimbStr("9131 0115 makf gd5n xy"),
		BankAccount: reimbStr("1219 9577-1710001"),
		BankName:    reimbStr("招商银行股份有限公司上海张杨支行"),
		Address:     reimbStr("上海市浦东新区张杨路500号华润时代广场12楼 "),
		Amount:      reimbFloat(414.104),
	}
}

func TestReimbursementService_CreateRejectsIncomplete(t *testing.T) {
	repo := newFakeReimbursementRepo()
	svc, _ := newReimbursementTestService(t, repo, nil)

	fields := completeReimbursementFields()
	fields.Address = reimbStr("   ")
	fields.Amount = reimbFloat(0)
	_, err := svc.Create(context.Background(), 5, CreateReimbursementInput{Fields: fields})
	require.Error(t, err)
	var appErr *infraerrors.ApplicationError
	require.True(t, errors.As(err, &appErr))
	require.Equal(t, 400, int(appErr.Code))
	require.Equal(t, "REIMBURSEMENT_INCOMPLETE", appErr.Reason)
	require.Equal(t, "missing fields: address, amount", appErr.Message)
	require.Empty(t, repo.items)
}

func TestReimbursementService_CreateNormalizesAndStores(t *testing.T) {
	repo := newFakeReimbursementRepo()
	svc, _ := newReimbursementTestService(t, repo, nil)

	record, err := svc.Create(context.Background(), 5, CreateReimbursementInput{Fields: completeReimbursementFields(), RawText: "  原文  "})
	require.NoError(t, err)
	require.Equal(t, int64(1), record.ID)
	require.Equal(t, int64(5), record.UserID)
	require.Equal(t, ReimbursementStatusPending, record.Status)
	require.Equal(t, "91310115MAKFGD5NXY", record.TaxID)
	require.Equal(t, "121995771710001", record.BankAccount)
	require.Equal(t, "上海市浦东新区张杨路500号华润时代广场12楼", record.Address)
	require.Equal(t, 414.1, record.Amount)
	require.Equal(t, "原文", record.RawText)
	require.False(t, record.PDFAvailable())

	_, err = svc.Create(context.Background(), 5, CreateReimbursementInput{Fields: completeReimbursementFields(), RawText: strings.Repeat("字", ReimbursementMaxRawTextRunes+1)})
	require.ErrorIs(t, err, ErrReimbursementTextTooLong)
}

func TestReimbursementService_ParseValidatesText(t *testing.T) {
	svc, _ := newReimbursementTestService(t, newFakeReimbursementRepo(), nil)
	_, err := svc.Parse(context.Background(), 5, "   ", nil)
	require.ErrorIs(t, err, ErrReimbursementTextRequired)
	_, err = svc.Parse(context.Background(), 5, strings.Repeat("字", ReimbursementMaxParseTextRunes+1), nil)
	require.ErrorIs(t, err, ErrReimbursementTextTooLong)
	// parser 未注入等同未配置。
	_, err = svc.Parse(context.Background(), 5, "x", nil)
	require.ErrorIs(t, err, ErrReimbursementLLMNotConfigured)
	// 校验失败与未配置都不占当日额度。
	require.Empty(t, svc.parseQuota)
}

// newReimbursementTestServiceWithParser 接一个总是返回合法内容的伪 LLM，用来测计数类逻辑。
func newReimbursementTestServiceWithParser(t *testing.T) (*ReimbursementService, *atomic.Int32) {
	t.Helper()
	srv, calls, _ := newReimbursementLLMServer(t, func(_ int, w http.ResponseWriter, _ *http.Request) {
		writeReimbursementChatContent(w, `{"company_name":null,"tax_id":null,"bank_account":null,"bank_name":null,"address":null,"amount":null,"notes":null}`)
	})
	parser, settingRepo := newReimbursementTestParser(t, srv, "sk-test-key-1234")
	cfg := &config.Config{}
	cfg.Pricing.DataDir = t.TempDir()
	svc := NewReimbursementService(newFakeReimbursementRepo(), parser, nil, nil, settingRepo, cfg)
	return svc, calls
}

func TestReimbursementService_ParseDailyQuota(t *testing.T) {
	svc, calls := newReimbursementTestServiceWithParser(t)
	day1 := time.Date(2026, 9, 15, 23, 30, 0, 0, time.UTC)
	svc.now = func() time.Time { return day1 }
	ctx := context.Background()

	for i := 0; i < ReimbursementParseDailyLimit; i++ {
		_, err := svc.Parse(ctx, 5, "开票信息", nil)
		require.NoError(t, err, "call %d", i+1)
	}
	// 第 201 次：拒绝且不再打上游。
	_, err := svc.Parse(ctx, 5, "开票信息", nil)
	require.ErrorIs(t, err, ErrReimbursementParseQuotaExceeded)
	require.EqualValues(t, ReimbursementParseDailyLimit, calls.Load())
	var appErr *infraerrors.ApplicationError
	require.True(t, errors.As(err, &appErr))
	require.Equal(t, 429, int(appErr.Code))
	require.Equal(t, "REIMBURSEMENT_PARSE_QUOTA_EXCEEDED", appErr.Reason)

	// 额度按用户隔离。
	_, err = svc.Parse(ctx, 6, "开票信息", nil)
	require.NoError(t, err)

	// 跨 UTC 日归零（day1 是 23:30，+1h 已是次日）。
	svc.now = func() time.Time { return day1.Add(time.Hour) }
	_, err = svc.Parse(ctx, 5, "开票信息", nil)
	require.NoError(t, err)
	require.Equal(t, 1, svc.parseQuota[5].count)
	require.Equal(t, "2026-09-16", svc.parseQuota[5].day)
	// 跨日时其它用户的旧条目被顺手清理。
	_, stale := svc.parseQuota[6]
	require.False(t, stale)
}

// 管理端测试解析不走 Parse，不占用户额度。
func TestReimbursementService_TestLLMConfigDoesNotConsumeQuota(t *testing.T) {
	svc, calls := newReimbursementTestServiceWithParser(t)
	_, err := svc.TestLLMConfig(context.Background(), "")
	require.NoError(t, err)
	require.EqualValues(t, 1, calls.Load())
	require.Empty(t, svc.parseQuota)
}

// 管理端搜索词按 rune 截断：67 个以上汉字（>200 字节）不能切出半个字符，否则 PostgreSQL 报 22021。
func TestReimbursementService_ListAllTruncatesSearchByRune(t *testing.T) {
	repo := newFakeReimbursementRepo()
	svc, _ := newReimbursementTestService(t, repo, nil)
	search := strings.Repeat("发", 250)
	_, _, err := svc.ListAll(context.Background(), pagination.PaginationParams{Page: 1, PageSize: 20}, ReimbursementListFilters{Search: search})
	require.NoError(t, err)
	got := repo.lastListFilters.Search
	require.True(t, utf8.ValidString(got))
	require.Equal(t, 200, utf8.RuneCountInString(got))
	require.Equal(t, strings.Repeat("发", 200), got)

	// 恰好 67 个汉字 = 201 字节，按 rune 计不超限、原样透传。
	short := strings.Repeat("票", 67)
	_, _, err = svc.ListAll(context.Background(), pagination.PaginationParams{Page: 1, PageSize: 20}, ReimbursementListFilters{Search: short})
	require.NoError(t, err)
	require.Equal(t, short, repo.lastListFilters.Search)
}

func TestReimbursementService_GetForUserHidesOthers(t *testing.T) {
	repo := newFakeReimbursementRepo()
	svc, _ := newReimbursementTestService(t, repo, nil)
	record, err := svc.Create(context.Background(), 5, CreateReimbursementInput{Fields: completeReimbursementFields()})
	require.NoError(t, err)

	got, err := svc.GetForUser(context.Background(), 5, record.ID)
	require.NoError(t, err)
	require.Equal(t, record.ID, got.ID)

	_, err = svc.GetForUser(context.Background(), 6, record.ID)
	require.ErrorIs(t, err, ErrReimbursementNotFound)
	_, err = svc.OpenPDFForUser(context.Background(), 6, record.ID)
	require.ErrorIs(t, err, ErrReimbursementNotFound)
	_, err = svc.GetForUser(context.Background(), 5, 999)
	require.ErrorIs(t, err, ErrReimbursementNotFound)

	// 本人但尚未上传 PDF。
	_, err = svc.OpenPDFForUser(context.Background(), 5, record.ID)
	require.ErrorIs(t, err, ErrReimbursementPDFNotReady)
}

func pdfBytes(payload string) []byte {
	return []byte("%PDF-1.4\n" + payload + "\n%%EOF\n")
}

func waitReimbursementNotify(t *testing.T, svc *ReimbursementService) func() {
	t.Helper()
	done := make(chan struct{}, 8)
	svc.notifyDone = func() { done <- struct{}{} }
	return func() {
		select {
		case <-done:
		case <-time.After(5 * time.Second):
			t.Fatal("notify goroutine did not finish")
		}
	}
}

func TestReimbursementService_AttachPDFValidation(t *testing.T) {
	repo := newFakeReimbursementRepo()
	svc, dataDir := newReimbursementTestService(t, repo, nil)
	record, err := svc.Create(context.Background(), 5, CreateReimbursementInput{Fields: completeReimbursementFields()})
	require.NoError(t, err)
	ctx := context.Background()

	// 扩展名不是 .pdf
	_, err = svc.AttachPDF(ctx, record.ID, AttachReimbursementPDFInput{OriginalName: "invoice.txt", DeclaredSize: 10, Reader: bytes.NewReader(pdfBytes("x")), AdminID: 1})
	require.ErrorIs(t, err, ErrReimbursementPDFInvalid)

	// 魔数不对（不信 Content-Type / 扩展名）
	_, err = svc.AttachPDF(ctx, record.ID, AttachReimbursementPDFInput{OriginalName: "invoice.pdf", DeclaredSize: 10, Reader: bytes.NewReader([]byte("<html>not a pdf</html>")), AdminID: 1})
	require.ErrorIs(t, err, ErrReimbursementPDFInvalid)

	// 声明大小超限，直接拒绝
	_, err = svc.AttachPDF(ctx, record.ID, AttachReimbursementPDFInput{OriginalName: "invoice.pdf", DeclaredSize: ReimbursementMaxPDFSize + 1, Reader: bytes.NewReader(pdfBytes("x")), AdminID: 1})
	require.ErrorIs(t, err, ErrReimbursementPDFTooLarge)

	// 声明大小合法但实际流超限：边写边判，超限中止并清理临时文件
	oversized := io.MultiReader(bytes.NewReader([]byte("%PDF-1.4\n")), io.LimitReader(zeroReader{}, ReimbursementMaxPDFSize))
	_, err = svc.AttachPDF(ctx, record.ID, AttachReimbursementPDFInput{OriginalName: "invoice.pdf", DeclaredSize: 100, Reader: oversized, AdminID: 1})
	require.ErrorIs(t, err, ErrReimbursementPDFTooLarge)

	// 记录不存在
	_, err = svc.AttachPDF(ctx, 999, AttachReimbursementPDFInput{OriginalName: "invoice.pdf", DeclaredSize: 10, Reader: bytes.NewReader(pdfBytes("x")), AdminID: 1})
	require.ErrorIs(t, err, ErrReimbursementNotFound)

	// 失败路径不能留下任何文件
	entries, _ := os.ReadDir(filepath.Join(dataDir, "reimbursement"))
	require.Empty(t, entries)
	got, err := repo.GetByID(ctx, record.ID)
	require.NoError(t, err)
	require.Equal(t, ReimbursementStatusPending, got.Status)
}

type zeroReader struct{}

func (zeroReader) Read(p []byte) (int, error) {
	for i := range p {
		p[i] = 0
	}
	return len(p), nil
}

func TestReimbursementService_AttachPDFStoresAndReuploadOverwrites(t *testing.T) {
	repo := newFakeReimbursementRepo()
	svc, dataDir := newReimbursementTestService(t, repo, nil)
	fixed := time.Date(2026, 9, 15, 4, 0, 0, 0, time.UTC)
	svc.now = func() time.Time { return fixed }
	wait := waitReimbursementNotify(t, svc)
	record, err := svc.Create(context.Background(), 5, CreateReimbursementInput{Fields: completeReimbursementFields()})
	require.NoError(t, err)
	ctx := context.Background()

	first := pdfBytes("first")
	updated, err := svc.AttachPDF(ctx, record.ID, AttachReimbursementPDFInput{
		OriginalName: `../..\发票 "2026".pdf`,
		DeclaredSize: int64(len(first)),
		Reader:       bytes.NewReader(first),
		AdminID:      42,
	})
	require.NoError(t, err)
	wait()

	require.Equal(t, ReimbursementStatusCompleted, updated.Status)
	require.True(t, updated.PDFAvailable())
	require.Equal(t, "reimbursement/1.pdf", updated.PDFPath, "默认目录下存相对 DataDir 的路径")
	require.Equal(t, int64(len(first)), updated.PDFSize)
	sum := sha256.Sum256(first)
	require.Equal(t, hex.EncodeToString(sum[:]), updated.PDFSha256)
	require.NotNil(t, updated.HandledBy)
	require.Equal(t, int64(42), *updated.HandledBy)
	require.NotNil(t, updated.CompletedAt)
	require.Equal(t, fixed, *updated.CompletedAt)
	require.NotNil(t, updated.PDFUploadedAt)
	require.Equal(t, fixed, *updated.PDFUploadedAt)
	// 原名只做展示：路径分隔符与引号被替换，不含 ..
	require.NotContains(t, updated.PDFFileName, "/")
	require.NotContains(t, updated.PDFFileName, `\`)
	require.NotContains(t, updated.PDFFileName, `"`)
	require.NotContains(t, updated.PDFFileName, "..")
	require.True(t, strings.HasSuffix(updated.PDFFileName, ".pdf"))

	// 落盘位置：<DataDir>/reimbursement/<id>.pdf，且没有临时文件残留
	target := filepath.Join(dataDir, "reimbursement", "1.pdf")
	content, err := os.ReadFile(target)
	require.NoError(t, err)
	require.Equal(t, first, content)
	entries, err := os.ReadDir(filepath.Join(dataDir, "reimbursement"))
	require.NoError(t, err)
	require.Len(t, entries, 1)
	require.Equal(t, "1.pdf", entries[0].Name())

	// 用户下载拿到同样内容与文件名
	stream, err := svc.OpenPDFForUser(ctx, 5, record.ID)
	require.NoError(t, err)
	streamed, err := io.ReadAll(stream.Reader)
	require.NoError(t, err)
	require.NoError(t, stream.Reader.Close())
	require.Equal(t, first, streamed)
	require.Equal(t, int64(len(first)), stream.Size)
	require.Equal(t, updated.PDFFileName, stream.FileName)

	// 重新上传：覆盖文件与摘要，刷新 pdf_uploaded_at，completed_at 保持首次值
	later := fixed.Add(2 * time.Hour)
	svc.now = func() time.Time { return later }
	second := pdfBytes("second version with more bytes")
	reuploaded, err := svc.AttachPDF(ctx, record.ID, AttachReimbursementPDFInput{
		OriginalName: "invoice-final.PDF",
		DeclaredSize: int64(len(second)),
		Reader:       bytes.NewReader(second),
		AdminID:      43,
	})
	require.NoError(t, err)
	wait()
	require.Equal(t, fixed, *reuploaded.CompletedAt)
	require.Equal(t, later, *reuploaded.PDFUploadedAt)
	require.Equal(t, int64(43), *reuploaded.HandledBy)
	require.Equal(t, "invoice-final.PDF", reuploaded.PDFFileName)
	sum2 := sha256.Sum256(second)
	require.Equal(t, hex.EncodeToString(sum2[:]), reuploaded.PDFSha256)
	require.NotEqual(t, updated.PDFSha256, reuploaded.PDFSha256)
	content, err = os.ReadFile(target)
	require.NoError(t, err)
	require.Equal(t, second, content)
	entries, err = os.ReadDir(filepath.Join(dataDir, "reimbursement"))
	require.NoError(t, err)
	require.Len(t, entries, 1)
}

func TestReimbursementService_CustomPDFDirStoresAbsolutePath(t *testing.T) {
	repo := newFakeReimbursementRepo()
	custom := t.TempDir()
	svc, dataDir := newReimbursementTestService(t, repo, func(cfg *config.Config) {
		cfg.Reimbursement.PDFDir = custom
	})
	wait := waitReimbursementNotify(t, svc)
	record, err := svc.Create(context.Background(), 5, CreateReimbursementInput{Fields: completeReimbursementFields()})
	require.NoError(t, err)

	payload := pdfBytes("custom dir")
	updated, err := svc.AttachPDF(context.Background(), record.ID, AttachReimbursementPDFInput{
		OriginalName: "a.pdf", DeclaredSize: int64(len(payload)), Reader: bytes.NewReader(payload), AdminID: 1,
	})
	require.NoError(t, err)
	wait()
	require.True(t, filepath.IsAbs(updated.PDFPath))
	require.Equal(t, filepath.Join(custom, "1.pdf"), updated.PDFPath)
	_, err = os.Stat(filepath.Join(dataDir, "reimbursement"))
	require.True(t, os.IsNotExist(err), "覆盖目录后不应再写 DataDir")

	stream, err := svc.OpenPDF(context.Background(), record.ID)
	require.NoError(t, err)
	got, err := io.ReadAll(stream.Reader)
	require.NoError(t, err)
	require.NoError(t, stream.Reader.Close())
	require.Equal(t, payload, got)
}

func TestReimbursementService_OpenPDFRejectsPathOutsideDir(t *testing.T) {
	repo := newFakeReimbursementRepo()
	svc, dataDir := newReimbursementTestService(t, repo, nil)
	// 直接在仓储里伪造一条指向目录外的记录，模拟数据被篡改。
	outside := filepath.Join(dataDir, "secret.txt")
	require.NoError(t, os.WriteFile(outside, []byte("secret"), 0o600))
	now := time.Now()
	repo.items[7] = &ReimbursementRequest{ID: 7, UserID: 5, Status: ReimbursementStatusCompleted, PDFPath: "../secret.txt", CompletedAt: &now}

	_, err := svc.OpenPDFForUser(context.Background(), 5, 7)
	require.ErrorIs(t, err, ErrReimbursementPDFNotReady)
}

// 绝对路径分支以前只检查「在自己的父目录内」（恒真），篡改成任意绝对路径都能读出来。
func TestReimbursementService_OpenPDFRejectsTamperedAbsolutePath(t *testing.T) {
	repo := newFakeReimbursementRepo()
	svc, dataDir := newReimbursementTestService(t, repo, nil)
	pdfDir := filepath.Join(dataDir, "reimbursement")
	require.NoError(t, os.MkdirAll(pdfDir, 0o750))
	now := time.Now()

	// 目录外的任意文件（绝对路径）
	outside := filepath.Join(dataDir, "secret.txt")
	require.NoError(t, os.WriteFile(outside, []byte("secret"), 0o600))
	repo.items[7] = &ReimbursementRequest{ID: 7, UserID: 5, Status: ReimbursementStatusCompleted, PDFPath: outside, CompletedAt: &now}
	_, err := svc.OpenPDF(context.Background(), 7)
	require.ErrorIs(t, err, ErrReimbursementPDFNotReady)

	// 目录内但文件名不是 <id>.pdf（绝对路径）：不允许借别的申请的 PDF
	other := filepath.Join(pdfDir, "8.pdf")
	require.NoError(t, os.WriteFile(other, pdfBytes("someone else"), 0o600))
	repo.items[7].PDFPath = other
	_, err = svc.OpenPDF(context.Background(), 7)
	require.ErrorIs(t, err, ErrReimbursementPDFNotReady)

	// 相对路径同样校验文件名
	repo.items[7].PDFPath = "reimbursement/8.pdf"
	_, err = svc.OpenPDF(context.Background(), 7)
	require.ErrorIs(t, err, ErrReimbursementPDFNotReady)

	// 历史 PDFDir 写出的绝对路径（不等于当前配置目录）只要文件名对得上就仍能读：改 pdf_dir 不能让旧记录失效
	legacyDir := t.TempDir()
	legacy := filepath.Join(legacyDir, "7.pdf")
	require.NoError(t, os.WriteFile(legacy, pdfBytes("legacy"), 0o600))
	repo.items[7].PDFPath = legacy
	stream, err := svc.OpenPDF(context.Background(), 7)
	require.NoError(t, err)
	require.NoError(t, stream.Reader.Close())
}

// 写库失败时：旧 PDF 原样保留、临时文件被清掉、记录不变。
func TestReimbursementService_AttachPDFDBFailureKeepsOldFile(t *testing.T) {
	repo := newFakeReimbursementRepo()
	svc, dataDir := newReimbursementTestService(t, repo, nil)
	wait := waitReimbursementNotify(t, svc)
	record, err := svc.Create(context.Background(), 5, CreateReimbursementInput{Fields: completeReimbursementFields()})
	require.NoError(t, err)
	ctx := context.Background()

	first := pdfBytes("first")
	before, err := svc.AttachPDF(ctx, record.ID, AttachReimbursementPDFInput{OriginalName: "a.pdf", DeclaredSize: int64(len(first)), Reader: bytes.NewReader(first), AdminID: 1})
	require.NoError(t, err)
	wait()

	repo.attachErr = errors.New("db down")
	second := pdfBytes("second")
	_, err = svc.AttachPDF(ctx, record.ID, AttachReimbursementPDFInput{OriginalName: "b.pdf", DeclaredSize: int64(len(second)), Reader: bytes.NewReader(second), AdminID: 2})
	require.Error(t, err)
	require.ErrorContains(t, err, "db down")

	pdfDir := filepath.Join(dataDir, "reimbursement")
	content, err := os.ReadFile(filepath.Join(pdfDir, "1.pdf"))
	require.NoError(t, err)
	require.Equal(t, first, content, "写库失败不能覆盖旧 PDF")
	entries, err := os.ReadDir(pdfDir)
	require.NoError(t, err)
	require.Len(t, entries, 1, "临时文件必须被清理")
	after, err := repo.GetByID(ctx, record.ID)
	require.NoError(t, err)
	require.Equal(t, before.PDFSha256, after.PDFSha256)
	require.Equal(t, before.PDFUploadedAt, after.PDFUploadedAt)
}

func TestReimbursementPDFContentDisposition(t *testing.T) {
	got := ReimbursementPDFContentDisposition(12, `发票 "final".pdf`)
	require.True(t, strings.HasPrefix(got, `attachment; filename="invoice-12.pdf"; filename*=UTF-8''`))
	require.NotContains(t, got, `"final"`)
	require.Contains(t, got, "%E5%8F%91%E7%A5%A8")
	require.Equal(t, `attachment; filename="invoice-3.pdf"; filename*=UTF-8''invoice-3.pdf`, ReimbursementPDFContentDisposition(3, ""))
}

func TestSanitizeReimbursementPDFFileName(t *testing.T) {
	require.Equal(t, "invoice-9.pdf", SanitizeReimbursementPDFFileName("", 9))
	require.Equal(t, "invoice-9.pdf", SanitizeReimbursementPDFFileName("   ", 9))
	require.Equal(t, "a_b_c.pdf", SanitizeReimbursementPDFFileName(`a/b\c.pdf`, 9))
	require.NotContains(t, SanitizeReimbursementPDFFileName("..\\..\\x.pdf", 9), "..")
	require.Equal(t, "x.pdf", SanitizeReimbursementPDFFileName("x\x00\x01.pdf", 9))
	require.Len(t, []rune(SanitizeReimbursementPDFFileName(strings.Repeat("长", 300)+".pdf", 9)), 255)
}

func TestReimbursementService_CompletedEmailVariables(t *testing.T) {
	repo := newFakeReimbursementRepo()
	svc, _ := newReimbursementTestService(t, repo, func(cfg *config.Config) {
		cfg.Server.FrontendURL = "https://example.com/"
	})
	record := &ReimbursementRequest{ID: 12, CompanyName: "示例公司", Amount: 414.1}
	vars := svc.completedEmailVariables(context.Background(), record, "Sub2API", "alice@example.com")
	require.Equal(t, "414.10", vars["amount"])
	require.Equal(t, "12", vars["request_id"])
	require.Equal(t, "https://example.com/reimbursement", vars["download_page_url"])
	require.Equal(t, "alice", vars["recipient_name"])

	// settings 里的 frontend_url 优先于 config
	stub, ok := svc.settingRepo.(*stubSettingRepo)
	require.True(t, ok)
	stub.values[SettingKeyFrontendURL] = "https://panel.example.com"
	vars = svc.completedEmailVariables(context.Background(), record, "Sub2API", "alice@example.com")
	require.Equal(t, "https://panel.example.com/reimbursement", vars["download_page_url"])

	body := buildReimbursementCompletedEmailBody(vars)
	require.Contains(t, body, "#12")
	require.Contains(t, body, "示例公司")
	require.Contains(t, body, `href="https://panel.example.com/reimbursement"`)

	// 事件已登记：官方模板能用运行时变量渲染出来
	raw, err := json.Marshal(vars)
	require.NoError(t, err)
	require.NotEmpty(t, raw)
	info, ok := notificationEmailEventDefinitions[NotificationEmailEventReimbursementCompleted]
	require.True(t, ok)
	require.True(t, info.Optional)
	for _, key := range []string{"company_name", "amount", "request_id", "download_page_url"} {
		require.Contains(t, info.Placeholders, key)
	}
	for _, locale := range []string{notificationEmailDefaultLocale, notificationEmailLocaleChinese} {
		tmpl := notificationEmailOfficialTemplates[NotificationEmailEventReimbursementCompleted][locale]
		vars["unsubscribe_url"] = "https://example.com/unsub"
		rendered, err := renderNotificationEmail(NotificationEmailEventReimbursementCompleted, tmpl.Subject, tmpl.HTML, vars, nil)
		require.NoError(t, err, locale)
		require.Contains(t, rendered.Subject, "12")
		require.Contains(t, rendered.HTML, "示例公司")
	}
}
