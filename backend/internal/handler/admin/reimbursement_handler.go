package admin

import (
	"errors"
	"io"
	"net/http"
	"strconv"
	"strings"

	"github.com/Wei-Shaw/sub2api/internal/handler/dto"
	"github.com/Wei-Shaw/sub2api/internal/pkg/pagination"
	"github.com/Wei-Shaw/sub2api/internal/pkg/response"
	middleware2 "github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"

	"github.com/gin-gonic/gin"
)

// ReimbursementHandler 是管理端报销/开票申请接口。
type ReimbursementHandler struct {
	reimbursementService *service.ReimbursementService
}

func NewReimbursementHandler(reimbursementService *service.ReimbursementService) *ReimbursementHandler {
	return &ReimbursementHandler{reimbursementService: reimbursementService}
}

// reimbursementConfigRequest 是 PUT /admin/reimbursement/config 的请求体；指针为 nil 表示不修改。
type reimbursementConfigRequest struct {
	BaseURL     *string `json:"base_url"`
	Model       *string `json:"model"`
	TimeoutMS   *int    `json:"timeout_ms"`
	APIKey      *string `json:"api_key"`
	ClearAPIKey bool    `json:"clear_api_key"`
}

type reimbursementConfigTestRequest struct {
	Text string `json:"text"`
}

// List 管理端列表；默认按提交时间升序。
// GET /api/v1/admin/reimbursement/requests
func (h *ReimbursementHandler) List(c *gin.Context) {
	page, pageSize := response.ParsePagination(c)
	status := strings.TrimSpace(c.Query("status"))
	// 长度截断交给服务层按 rune 做；这里按字节切会把多字节字符切坏，PostgreSQL 直接报 22021。
	search := strings.TrimSpace(c.Query("search"))
	params := pagination.PaginationParams{
		Page:      page,
		PageSize:  pageSize,
		SortBy:    c.DefaultQuery("sort_by", "created_at"),
		SortOrder: c.DefaultQuery("sort_order", pagination.SortOrderAsc),
	}
	items, result, err := h.reimbursementService.ListAll(c.Request.Context(), params, service.ReimbursementListFilters{
		Status: status,
		Search: search,
	})
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	out := make([]dto.AdminReimbursementRequest, 0, len(items))
	for i := range items {
		out = append(out, *dto.AdminReimbursementRequestFromService(&items[i]))
	}
	response.Paginated(c, out, result.Total, page, pageSize)
}

// GetByID 管理端取单条。
// GET /api/v1/admin/reimbursement/requests/:id
func (h *ReimbursementHandler) GetByID(c *gin.Context) {
	id, ok := parseReimbursementID(c)
	if !ok {
		return
	}
	record, err := h.reimbursementService.GetByID(c.Request.Context(), id)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, dto.AdminReimbursementRequestFromService(record))
}

// UploadPDF 上传发票 PDF（multipart 字段名 file），成功后申请置为 completed。
// POST /api/v1/admin/reimbursement/requests/:id/pdf
func (h *ReimbursementHandler) UploadPDF(c *gin.Context) {
	subject, ok := middleware2.GetAuthSubjectFromContext(c)
	if !ok {
		response.Unauthorized(c, "User not authenticated")
		return
	}
	id, ok := parseReimbursementID(c)
	if !ok {
		return
	}
	middleware2.SetAuditAction(c, "admin.reimbursement.pdf.upload")
	fh, err := c.FormFile("file")
	if err != nil {
		// 路由上挂了 RequestBodyLimit(21<<20)：整个 multipart 超限时 FormFile 读到的是
		// MaxBytesError，要报「太大」而不是「不是 PDF」，否则用户拿不到正确的纠正提示。
		if errors.As(err, new(*http.MaxBytesError)) {
			response.ErrorFrom(c, service.ErrReimbursementPDFTooLarge)
			return
		}
		response.ErrorFrom(c, service.ErrReimbursementPDFInvalid)
		return
	}
	if fh.Size > service.ReimbursementMaxPDFSize {
		response.ErrorFrom(c, service.ErrReimbursementPDFTooLarge)
		return
	}
	f, err := fh.Open()
	if err != nil {
		response.ErrorFrom(c, service.ErrReimbursementPDFInvalid)
		return
	}
	defer func() { _ = f.Close() }()

	record, err := h.reimbursementService.AttachPDF(c.Request.Context(), id, service.AttachReimbursementPDFInput{
		OriginalName: fh.Filename,
		DeclaredSize: fh.Size,
		Reader:       f,
		AdminID:      subject.UserID,
	})
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, dto.AdminReimbursementRequestFromService(record))
}

// DownloadPDF 管理端下载 PDF，不限本人。
// GET /api/v1/admin/reimbursement/requests/:id/pdf
func (h *ReimbursementHandler) DownloadPDF(c *gin.Context) {
	id, ok := parseReimbursementID(c)
	if !ok {
		return
	}
	stream, err := h.reimbursementService.OpenPDF(c.Request.Context(), id)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	defer func() { _ = stream.Reader.Close() }()
	c.Header("Content-Type", "application/pdf")
	c.Header("Content-Disposition", service.ReimbursementPDFContentDisposition(id, stream.FileName))
	c.Header("X-Content-Type-Options", "nosniff")
	c.Header("Cache-Control", "private, max-age=0")
	if stream.Size >= 0 {
		c.Header("Content-Length", strconv.FormatInt(stream.Size, 10))
	}
	c.Status(http.StatusOK)
	_, _ = io.Copy(c.Writer, stream.Reader)
}

// GetConfig 返回 LLM 配置掩码视图。
// GET /api/v1/admin/reimbursement/config
func (h *ReimbursementHandler) GetConfig(c *gin.Context) {
	view, err := h.reimbursementService.GetLLMConfig(c.Request.Context())
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, view)
}

// UpdateConfig 更新 settings 里的 LLM 配置。
// PUT /api/v1/admin/reimbursement/config
func (h *ReimbursementHandler) UpdateConfig(c *gin.Context) {
	var req reimbursementConfigRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request: "+err.Error())
		return
	}
	view, err := h.reimbursementService.UpdateLLMConfig(c.Request.Context(), service.UpdateReimbursementLLMConfigInput{
		BaseURL:     req.BaseURL,
		Model:       req.Model,
		TimeoutMS:   req.TimeoutMS,
		APIKey:      req.APIKey,
		ClearAPIKey: req.ClearAPIKey,
	})
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, view)
}

// TestConfig 用当前生效配置真实调用一次 LLM。
// POST /api/v1/admin/reimbursement/config/test
func (h *ReimbursementHandler) TestConfig(c *gin.Context) {
	var req reimbursementConfigTestRequest
	// 请求体可省略（用缺省样例），只有真的发了非法 JSON 才报 400。
	if c.Request.ContentLength != 0 {
		if err := c.ShouldBindJSON(&req); err != nil && !errors.Is(err, io.EOF) {
			response.BadRequest(c, "Invalid request: "+err.Error())
			return
		}
	}
	result, err := h.reimbursementService.TestLLMConfig(c.Request.Context(), req.Text)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, result)
}

func parseReimbursementID(c *gin.Context) (int64, bool) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || id <= 0 {
		response.BadRequest(c, "Invalid reimbursement request ID")
		return 0, false
	}
	return id, true
}
