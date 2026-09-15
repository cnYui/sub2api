package repository

import (
	"context"
	"fmt"
	"strings"
	"time"

	dbent "github.com/Wei-Shaw/sub2api/ent"
	"github.com/Wei-Shaw/sub2api/ent/reimbursementrequest"
	"github.com/Wei-Shaw/sub2api/ent/user"
	"github.com/Wei-Shaw/sub2api/internal/pkg/pagination"
	"github.com/Wei-Shaw/sub2api/internal/service"

	entsql "entgo.io/ent/dialect/sql"
)

type reimbursementRepository struct {
	client *dbent.Client
}

func NewReimbursementRepository(client *dbent.Client) service.ReimbursementRepository {
	return &reimbursementRepository{client: client}
}

func (r *reimbursementRepository) Create(ctx context.Context, req *service.ReimbursementRequest) error {
	client := clientFromContext(ctx, r.client)
	status := req.Status
	if status == "" {
		status = service.ReimbursementStatusPending
	}
	created, err := client.ReimbursementRequest.Create().
		SetUserID(req.UserID).
		SetRawText(req.RawText).
		SetCompanyName(req.CompanyName).
		SetTaxID(req.TaxID).
		SetBankAccount(req.BankAccount).
		SetBankName(req.BankName).
		SetAddress(req.Address).
		SetAmount(req.Amount).
		SetStatus(status).
		Save(ctx)
	if err != nil {
		return translatePersistenceError(err, nil, nil)
	}
	req.ID = created.ID
	req.Status = created.Status
	req.CreatedAt = created.CreatedAt
	req.UpdatedAt = created.UpdatedAt
	return nil
}

func (r *reimbursementRepository) GetByID(ctx context.Context, id int64) (*service.ReimbursementRequest, error) {
	client := clientFromContext(ctx, r.client)
	m, err := client.ReimbursementRequest.Query().
		Where(reimbursementrequest.IDEQ(id)).
		WithUser().
		Only(ctx)
	if err != nil {
		return nil, translatePersistenceError(err, service.ErrReimbursementNotFound, nil)
	}
	return reimbursementEntityToService(m), nil
}

func (r *reimbursementRepository) ListByUser(ctx context.Context, userID int64, params pagination.PaginationParams) ([]service.ReimbursementRequest, *pagination.PaginationResult, error) {
	q := r.client.ReimbursementRequest.Query().
		Where(reimbursementrequest.UserIDEQ(userID))
	total, err := q.Count(ctx)
	if err != nil {
		return nil, nil, err
	}
	items, err := q.
		Offset(params.Offset()).
		Limit(params.Limit()).
		Order(dbent.Desc(reimbursementrequest.FieldCreatedAt), dbent.Desc(reimbursementrequest.FieldID)).
		All(ctx)
	if err != nil {
		return nil, nil, err
	}
	return reimbursementEntitiesToService(items), paginationResultFromTotal(int64(total), params), nil
}

func (r *reimbursementRepository) List(ctx context.Context, params pagination.PaginationParams, filters service.ReimbursementListFilters) ([]service.ReimbursementRequest, *pagination.PaginationResult, error) {
	q := r.client.ReimbursementRequest.Query()
	if filters.Status != "" {
		q = q.Where(reimbursementrequest.StatusEQ(filters.Status))
	}
	if search := strings.TrimSpace(filters.Search); search != "" {
		q = q.Where(reimbursementrequest.Or(
			reimbursementrequest.CompanyNameContainsFold(search),
			reimbursementrequest.TaxIDContainsFold(search),
			reimbursementrequest.HasUserWith(user.EmailContainsFold(search)),
		))
	}
	total, err := q.Count(ctx)
	if err != nil {
		return nil, nil, err
	}
	itemsQuery := q.
		WithUser().
		Offset(params.Offset()).
		Limit(params.Limit())
	for _, order := range reimbursementListOrders(params) {
		itemsQuery = itemsQuery.Order(order)
	}
	items, err := itemsQuery.All(ctx)
	if err != nil {
		return nil, nil, err
	}
	return reimbursementEntitiesToService(items), paginationResultFromTotal(int64(total), params), nil
}

// reimbursementListOrders 排序白名单：created_at（默认）/ status / amount / id；
// 默认升序（管理端按提交时间从早到晚），次序键固定追加 id 同向保证分页稳定。
func reimbursementListOrders(params pagination.PaginationParams) []func(*entsql.Selector) {
	sortBy := strings.ToLower(strings.TrimSpace(params.SortBy))
	sortOrder := params.NormalizedSortOrder(pagination.SortOrderAsc)
	var field string
	switch sortBy {
	case "status":
		field = reimbursementrequest.FieldStatus
	case "amount":
		field = reimbursementrequest.FieldAmount
	case "id":
		field = reimbursementrequest.FieldID
	default:
		field = reimbursementrequest.FieldCreatedAt
	}
	if sortOrder == pagination.SortOrderAsc {
		if field == reimbursementrequest.FieldID {
			return []func(*entsql.Selector){dbent.Asc(field)}
		}
		return []func(*entsql.Selector){dbent.Asc(field), dbent.Asc(reimbursementrequest.FieldID)}
	}
	if field == reimbursementrequest.FieldID {
		return []func(*entsql.Selector){dbent.Desc(field)}
	}
	return []func(*entsql.Selector){dbent.Desc(field), dbent.Desc(reimbursementrequest.FieldID)}
}

// AttachPDF 把两条 UPDATE 放进同一个事务：completed_at 首次写入与 PDF 元数据刷新
// 要么都落库要么都不落，避免中途失败留下「已完成但没有 PDF」的记录。
// ctx 已带事务时直接沿用，由外层负责提交。
func (r *reimbursementRepository) AttachPDF(ctx context.Context, id int64, update service.ReimbursementPDFUpdate) (*service.ReimbursementRequest, error) {
	if dbent.TxFromContext(ctx) != nil {
		return r.attachPDFInTx(ctx, id, update)
	}
	tx, err := r.client.Tx(ctx)
	if err != nil {
		return nil, fmt.Errorf("begin reimbursement transaction: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	txCtx := dbent.NewTxContext(ctx, tx)
	updated, err := r.attachPDFInTx(txCtx, id, update)
	if err != nil {
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("commit reimbursement transaction: %w", err)
	}
	return updated, nil
}

func (r *reimbursementRepository) attachPDFInTx(ctx context.Context, id int64, update service.ReimbursementPDFUpdate) (*service.ReimbursementRequest, error) {
	client := clientFromContext(ctx, r.client)
	// completed_at 只在首次完成时写入；重新上传只刷新 PDF 元数据与 pdf_uploaded_at。
	if _, err := client.ReimbursementRequest.Update().
		Where(reimbursementrequest.IDEQ(id), reimbursementrequest.CompletedAtIsNil()).
		SetCompletedAt(update.UploadedAt).
		Save(ctx); err != nil {
		return nil, translatePersistenceError(err, service.ErrReimbursementNotFound, nil)
	}
	if _, err := client.ReimbursementRequest.UpdateOneID(id).
		SetStatus(service.ReimbursementStatusCompleted).
		SetPdfPath(update.PDFPath).
		SetPdfFileName(update.PDFFileName).
		SetPdfSize(update.PDFSize).
		SetPdfSha256(update.PDFSha256).
		SetPdfUploadedAt(update.UploadedAt).
		SetHandledBy(update.HandledBy).
		Save(ctx); err != nil {
		return nil, translatePersistenceError(err, service.ErrReimbursementNotFound, nil)
	}
	return r.GetByID(ctx, id)
}

func (r *reimbursementRepository) MarkNotified(ctx context.Context, id int64, at time.Time) error {
	client := clientFromContext(ctx, r.client)
	_, err := client.ReimbursementRequest.Update().
		Where(reimbursementrequest.IDEQ(id)).
		SetNotifiedAt(at).
		Save(ctx)
	return translatePersistenceError(err, service.ErrReimbursementNotFound, nil)
}

func reimbursementEntityToService(m *dbent.ReimbursementRequest) *service.ReimbursementRequest {
	if m == nil {
		return nil
	}
	out := &service.ReimbursementRequest{
		ID:            m.ID,
		UserID:        m.UserID,
		RawText:       m.RawText,
		CompanyName:   m.CompanyName,
		TaxID:         m.TaxID,
		BankAccount:   m.BankAccount,
		BankName:      m.BankName,
		Address:       m.Address,
		Amount:        m.Amount,
		Status:        m.Status,
		PDFPath:       m.PdfPath,
		PDFFileName:   m.PdfFileName,
		PDFSize:       m.PdfSize,
		PDFSha256:     m.PdfSha256,
		PDFUploadedAt: m.PdfUploadedAt,
		HandledBy:     m.HandledBy,
		CompletedAt:   m.CompletedAt,
		NotifiedAt:    m.NotifiedAt,
		CreatedAt:     m.CreatedAt,
		UpdatedAt:     m.UpdatedAt,
	}
	if m.Edges.User != nil {
		out.User = userEntityToService(m.Edges.User)
	}
	return out
}

func reimbursementEntitiesToService(models []*dbent.ReimbursementRequest) []service.ReimbursementRequest {
	out := make([]service.ReimbursementRequest, 0, len(models))
	for i := range models {
		if s := reimbursementEntityToService(models[i]); s != nil {
			out = append(out, *s)
		}
	}
	return out
}
