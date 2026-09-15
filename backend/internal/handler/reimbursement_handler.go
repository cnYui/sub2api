package handler

import (
	"io"
	"net/http"
	"strconv"

	"github.com/Wei-Shaw/sub2api/internal/handler/dto"
	"github.com/Wei-Shaw/sub2api/internal/pkg/pagination"
	"github.com/Wei-Shaw/sub2api/internal/pkg/response"
	middleware2 "github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"

	"github.com/gin-gonic/gin"
)

// ReimbursementHandler 是用户侧报销/开票申请接口。
type ReimbursementHandler struct {
	reimbursementService *service.ReimbursementService
}

func NewReimbursementHandler(reimbursementService *service.ReimbursementService) *ReimbursementHandler {
	return &ReimbursementHandler{reimbursementService: reimbursementService}
}

// ReimbursementParseRequest 是 POST /reimbursement/parse 的请求体。
type ReimbursementParseRequest struct {
	Text     string                       `json:"text"`
	Previous *service.ReimbursementFields `json:"previous"`
}

// ReimbursementCreateRequest 是 POST /reimbursement/requests 的请求体；
// 字段允许为 null，由服务端统一判缺失并返回 REIMBURSEMENT_INCOMPLETE。
type ReimbursementCreateRequest struct {
	CompanyName *string  `json:"company_name"`
	TaxID       *string  `json:"tax_id"`
	BankAccount *string  `json:"bank_account"`
	BankName    *string  `json:"bank_name"`
	Address     *string  `json:"address"`
	Amount      *float64 `json:"amount"`
	RawText     string   `json:"raw_text"`
}

// Parse 调 LLM 解析文本，不入库。
// POST /api/v1/reimbursement/parse
func (h *ReimbursementHandler) Parse(c *gin.Context) {
	subject, ok := middleware2.GetAuthSubjectFromContext(c)
	if !ok {
		response.Unauthorized(c, "User not authenticated")
		return
	}
	var req ReimbursementParseRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request: "+err.Error())
		return
	}
	result, err := h.reimbursementService.Parse(c.Request.Context(), subject.UserID, req.Text, req.Previous)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, result)
}

// Create 提交六项齐全的申请。
// POST /api/v1/reimbursement/requests
func (h *ReimbursementHandler) Create(c *gin.Context) {
	subject, ok := middleware2.GetAuthSubjectFromContext(c)
	if !ok {
		response.Unauthorized(c, "User not authenticated")
		return
	}
	var req ReimbursementCreateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request: "+err.Error())
		return
	}
	record, err := h.reimbursementService.Create(c.Request.Context(), subject.UserID, service.CreateReimbursementInput{
		Fields: service.ReimbursementFields{
			CompanyName: req.CompanyName,
			TaxID:       req.TaxID,
			BankAccount: req.BankAccount,
			BankName:    req.BankName,
			Address:     req.Address,
			Amount:      req.Amount,
		},
		RawText: req.RawText,
	})
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, dto.ReimbursementRequestFromService(record))
}

// List 返回自己的申请，最新在前。
// GET /api/v1/reimbursement/requests
func (h *ReimbursementHandler) List(c *gin.Context) {
	subject, ok := middleware2.GetAuthSubjectFromContext(c)
	if !ok {
		response.Unauthorized(c, "User not authenticated")
		return
	}
	page, pageSize := response.ParsePagination(c)
	params := pagination.PaginationParams{Page: page, PageSize: pageSize}
	items, result, err := h.reimbursementService.ListForUser(c.Request.Context(), subject.UserID, params)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	out := make([]dto.ReimbursementRequest, 0, len(items))
	for i := range items {
		out = append(out, *dto.ReimbursementRequestFromService(&items[i]))
	}
	response.Paginated(c, out, result.Total, page, pageSize)
}

// GetByID 取自己的单条申请；他人的当不存在。
// GET /api/v1/reimbursement/requests/:id
func (h *ReimbursementHandler) GetByID(c *gin.Context) {
	subject, ok := middleware2.GetAuthSubjectFromContext(c)
	if !ok {
		response.Unauthorized(c, "User not authenticated")
		return
	}
	id, ok := parseReimbursementID(c)
	if !ok {
		return
	}
	record, err := h.reimbursementService.GetForUser(c.Request.Context(), subject.UserID, id)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, dto.ReimbursementRequestFromService(record))
}

// DownloadPDF 下载自己已完成申请的发票 PDF。
// GET /api/v1/reimbursement/requests/:id/pdf
func (h *ReimbursementHandler) DownloadPDF(c *gin.Context) {
	subject, ok := middleware2.GetAuthSubjectFromContext(c)
	if !ok {
		response.Unauthorized(c, "User not authenticated")
		return
	}
	id, ok := parseReimbursementID(c)
	if !ok {
		return
	}
	stream, err := h.reimbursementService.OpenPDFForUser(c.Request.Context(), subject.UserID, id)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	WriteReimbursementPDF(c, id, stream)
}

func parseReimbursementID(c *gin.Context) (int64, bool) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || id <= 0 {
		response.BadRequest(c, "Invalid reimbursement request ID")
		return 0, false
	}
	return id, true
}

// WriteReimbursementPDF 以附件形式流式输出 PDF，用户端与管理端共用同一组响应头。
func WriteReimbursementPDF(c *gin.Context, id int64, stream *service.ReimbursementPDFStream) {
	defer func() { _ = stream.Reader.Close() }()
	c.Header("Content-Type", "application/pdf")
	c.Header("Content-Disposition", service.ReimbursementPDFContentDisposition(id, stream.FileName))
	c.Header("X-Content-Type-Options", "nosniff")
	c.Header("Cache-Control", "private, max-age=0")
	if stream.Size >= 0 {
		c.Header("Content-Length", strconv.FormatInt(stream.Size, 10))
	}
	c.Status(http.StatusOK)
	// 写入失败通常是客户端断开，响应头已发出，无法再改状态码。
	_, _ = io.Copy(c.Writer, stream.Reader)
}
