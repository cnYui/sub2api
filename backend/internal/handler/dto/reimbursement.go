package dto

import (
	"time"

	"github.com/Wei-Shaw/sub2api/internal/service"
)

// ReimbursementRequest 是用户侧 DTO：不暴露 raw_text / pdf_path / handled_by / pdf_sha256。
type ReimbursementRequest struct {
	ID           int64      `json:"id"`
	CompanyName  string     `json:"company_name"`
	TaxID        string     `json:"tax_id"`
	BankAccount  string     `json:"bank_account"`
	BankName     string     `json:"bank_name"`
	Address      string     `json:"address"`
	Amount       float64    `json:"amount"`
	Status       string     `json:"status"`
	PDFAvailable bool       `json:"pdf_available"`
	PDFFileName  string     `json:"pdf_file_name"`
	CreatedAt    time.Time  `json:"created_at"`
	UpdatedAt    time.Time  `json:"updated_at"`
	CompletedAt  *time.Time `json:"completed_at"`
}

// AdminReimbursementRequest 是管理侧 DTO：用户 DTO 全部字段 + 用户信息与处理元数据。
type AdminReimbursementRequest struct {
	ReimbursementRequest
	UserID        int64      `json:"user_id"`
	UserEmail     string     `json:"user_email"`
	Username      string     `json:"username"`
	RawText       string     `json:"raw_text"`
	PDFSize       int64      `json:"pdf_size"`
	PDFUploadedAt *time.Time `json:"pdf_uploaded_at"`
	HandledBy     *int64     `json:"handled_by"`
	NotifiedAt    *time.Time `json:"notified_at"`
}

func ReimbursementRequestFromService(r *service.ReimbursementRequest) *ReimbursementRequest {
	if r == nil {
		return nil
	}
	fileName := ""
	if r.PDFAvailable() {
		fileName = r.PDFFileName
		if fileName == "" {
			fileName = service.DefaultReimbursementPDFFileName(r.ID)
		}
	}
	return &ReimbursementRequest{
		ID:           r.ID,
		CompanyName:  r.CompanyName,
		TaxID:        r.TaxID,
		BankAccount:  r.BankAccount,
		BankName:     r.BankName,
		Address:      r.Address,
		Amount:       r.Amount,
		Status:       r.Status,
		PDFAvailable: r.PDFAvailable(),
		PDFFileName:  fileName,
		CreatedAt:    r.CreatedAt,
		UpdatedAt:    r.UpdatedAt,
		CompletedAt:  r.CompletedAt,
	}
}

func AdminReimbursementRequestFromService(r *service.ReimbursementRequest) *AdminReimbursementRequest {
	if r == nil {
		return nil
	}
	out := &AdminReimbursementRequest{
		ReimbursementRequest: *ReimbursementRequestFromService(r),
		UserID:               r.UserID,
		RawText:              r.RawText,
		PDFSize:              r.PDFSize,
		PDFUploadedAt:        r.PDFUploadedAt,
		HandledBy:            r.HandledBy,
		NotifiedAt:           r.NotifiedAt,
	}
	// 用户被软删或未预加载时留空串，前端兜底显示 #user_id。
	if r.User != nil {
		out.UserEmail = r.User.Email
		out.Username = r.User.Username
	}
	return out
}
