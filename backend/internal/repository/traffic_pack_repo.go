package repository

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"math"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/service"
)

type trafficPackRepository struct {
	db         *sql.DB
	isPostgres bool
}

func NewTrafficPackRepository(db *sql.DB) service.TrafficPackRepository {
	return &trafficPackRepository{db: db, isPostgres: isPostgresDriver(db)}
}

func (r *trafficPackRepository) ListForSale(ctx context.Context) ([]service.TrafficPack, error) {
	rows, err := r.db.QueryContext(ctx, `SELECT id, code, name, description, price, credit_usd, validity_days, platform, for_sale, sort_order FROM traffic_packs WHERE for_sale = TRUE ORDER BY sort_order, id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := []service.TrafficPack{}
	for rows.Next() {
		var p service.TrafficPack
		if err := rows.Scan(&p.ID, &p.Code, &p.Name, &p.Description, &p.Price, &p.CreditUSD, &p.ValidityDays, &p.Platform, &p.ForSale, &p.SortOrder); err != nil {
			return nil, err
		}
		result = append(result, p)
	}
	return result, rows.Err()
}

func (r *trafficPackRepository) GetForSaleByID(ctx context.Context, id int64) (*service.TrafficPack, error) {
	var p service.TrafficPack
	err := r.db.QueryRowContext(ctx, `SELECT id, code, name, description, price, credit_usd, validity_days, platform, for_sale, sort_order FROM traffic_packs WHERE id = $1 AND for_sale = TRUE`, id).Scan(&p.ID, &p.Code, &p.Name, &p.Description, &p.Price, &p.CreditUSD, &p.ValidityDays, &p.Platform, &p.ForSale, &p.SortOrder)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, service.ErrUserNotFound
	}
	if err != nil {
		return nil, err
	}
	return &p, nil
}

func (r *trafficPackRepository) GetSummary(ctx context.Context, userID int64, now time.Time) (*service.TrafficCreditSummary, error) {
	s := &service.TrafficCreditSummary{}
	if err := r.db.QueryRowContext(ctx, `SELECT COALESCE(SUM(initial_usd),0), COALESCE(SUM(remaining_usd),0) FROM user_traffic_credits WHERE user_id=$1 AND remaining_usd>0 AND expires_at>$2`, userID, now).Scan(&s.TotalInitialUSD, &s.TotalRemainingUSD); err != nil {
		return nil, err
	}
	if err := r.db.QueryRowContext(ctx, `SELECT COALESCE(SUM(CASE WHEN entry_type='debt' THEN amount_usd ELSE -amount_usd END),0) FROM traffic_credit_debt_ledger WHERE user_id=$1`, userID).Scan(&s.TrafficDebtUSD); err != nil {
		return nil, err
	}
	if s.TrafficDebtUSD < 0 {
		s.TrafficDebtUSD = 0
	}
	s.TotalRemainingUSD -= s.TrafficDebtUSD
	if s.TotalRemainingUSD < 0 {
		s.TotalRemainingUSD = 0
	}
	var expires time.Time
	err := r.db.QueryRowContext(ctx, `SELECT expires_at, COALESCE(SUM(remaining_usd),0) FROM user_traffic_credits WHERE user_id=$1 AND remaining_usd>0 AND expires_at>$2 GROUP BY expires_at ORDER BY expires_at LIMIT 1`, userID, now).Scan(&expires, &s.NextExpiringUSD)
	if errors.Is(err, sql.ErrNoRows) {
		return s, nil
	}
	if err != nil {
		return nil, err
	}
	s.NextExpiresAt = &expires
	return s, nil
}

func (r *trafficPackRepository) ListUserCredits(ctx context.Context, userID int64, now time.Time) ([]service.TrafficCredit, error) {
	rows, err := r.db.QueryContext(ctx, `SELECT id, order_id, pack_id, initial_usd, remaining_usd, credited_at, expires_at FROM user_traffic_credits WHERE user_id=$1 AND expires_at>$2 ORDER BY expires_at, credited_at, id`, userID, now)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := []service.TrafficCredit{}
	for rows.Next() {
		var c service.TrafficCredit
		var orderID, packID sql.NullInt64
		if err := rows.Scan(&c.ID, &orderID, &packID, &c.InitialUSD, &c.RemainingUSD, &c.CreditedAt, &c.ExpiresAt); err != nil {
			return nil, err
		}
		if orderID.Valid {
			c.OrderID = &orderID.Int64
		}
		if packID.Valid {
			c.PackID = &packID.Int64
		}
		c.AvailableUSD = math.Max(c.RemainingUSD, 0)
		result = append(result, c)
	}
	return result, rows.Err()
}

func (r *trafficPackRepository) GetCreditByOrderID(ctx context.Context, orderID int64) (*service.TrafficCredit, error) {
	var c service.TrafficCredit
	var oid, pid sql.NullInt64
	err := r.db.QueryRowContext(ctx, `SELECT id, order_id, pack_id, initial_usd, remaining_usd, credited_at, expires_at FROM user_traffic_credits WHERE order_id=$1`, orderID).Scan(&c.ID, &oid, &pid, &c.InitialUSD, &c.RemainingUSD, &c.CreditedAt, &c.ExpiresAt)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, service.ErrUserNotFound
	}
	if err != nil {
		return nil, err
	}
	if oid.Valid {
		c.OrderID = &oid.Int64
	}
	if pid.Valid {
		c.PackID = &pid.Int64
	}
	c.AvailableUSD = math.Max(c.RemainingUSD, 0)
	return &c, nil
}

func (r *trafficPackRepository) HasAvailableCredit(ctx context.Context, userID int64, now time.Time) (bool, error) {
	summary, err := r.GetSummary(ctx, userID, now)
	if err != nil {
		return false, err
	}
	return summary != nil && summary.TotalRemainingUSD > 0, nil
}

func (r *trafficPackRepository) CreditPurchase(ctx context.Context, input service.CreditTrafficPackInput) error {
	if input.UserID <= 0 || input.OrderID <= 0 || input.CreditUSD <= 0 {
		return service.ErrUserNotFound
	}
	days := input.ValidityDays
	if days <= 0 {
		days = service.TrafficPackValidityDays
	}
	credited := input.CreditedAt
	if credited.IsZero() {
		credited = time.Now().UTC()
	}
	expires := credited.AddDate(0, 0, days)
	amount := math.Round(input.CreditUSD*1e10) / 1e10
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	var lockedUserID int64
	if err := tx.QueryRowContext(ctx, `SELECT id FROM users WHERE id=$1 AND deleted_at IS NULL FOR UPDATE`, input.UserID).Scan(&lockedUserID); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return service.ErrUserNotFound
		}
		return err
	}
	var debt float64
	if err := tx.QueryRowContext(ctx, `SELECT COALESCE(SUM(CASE WHEN entry_type='debt' THEN amount_usd ELSE -amount_usd END),0) FROM traffic_credit_debt_ledger WHERE user_id=$1`, input.UserID).Scan(&debt); err != nil {
		return err
	}
	if debt < 0 {
		debt = 0
	}
	var id int64
	debtRepaid := math.Min(amount, debt)
	remaining := amount - debtRepaid
	err = tx.QueryRowContext(ctx, `INSERT INTO user_traffic_credits(user_id,order_id,pack_id,platform,initial_usd,remaining_usd,credited_at,expires_at,created_at,updated_at) VALUES($1,$2,$3,$4,$5,$6,$7,$8,$7,$7) ON CONFLICT(order_id) DO NOTHING RETURNING id`, input.UserID, input.OrderID, input.PackID, service.TrafficPackPlatformAll, amount, remaining, credited, expires).Scan(&id)
	if errors.Is(err, sql.ErrNoRows) {
		return tx.Commit()
	}
	if err != nil {
		return err
	}
	if _, err = tx.ExecContext(ctx, `INSERT INTO traffic_credit_ledger(user_id,credit_id,order_id,request_id,entry_type,amount_usd,balance_after_usd,created_at) VALUES($1,$2,$3,'',$4,$5,$6,$7)`, input.UserID, id, input.OrderID, service.TrafficCreditLedgerTypePurchase, amount, remaining, credited); err != nil {
		return err
	}
	if debtRepaid > 0 {
		debtAfter := debt - debtRepaid
		if _, err = tx.ExecContext(ctx, `INSERT INTO traffic_credit_debt_ledger(user_id,entry_type,amount_usd,balance_after_usd,source_type,source_ref,created_at) VALUES($1,'repayment',$2,$3,'traffic_pack_purchase',$4,$5)`, input.UserID, debtRepaid, debtAfter, fmt.Sprintf("order:%d", input.OrderID), credited); err != nil {
			return err
		}
	}
	return tx.Commit()
}

func (r *trafficPackRepository) Deduct(ctx context.Context, userID int64, amountUSD float64, requestID string, now time.Time) (bool, []service.TrafficCreditDeduction, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return false, nil, err
	}
	defer tx.Rollback()
	lock := ""
	if r.isPostgres {
		lock = " FOR UPDATE"
	}
	rows, err := tx.QueryContext(ctx, `SELECT id,user_id,order_id,pack_id,initial_usd,remaining_usd,credited_at,expires_at FROM user_traffic_credits WHERE user_id=$1 AND remaining_usd>0 AND expires_at>$2 ORDER BY expires_at,credited_at,id`+lock, userID, now)
	if err != nil {
		return false, nil, err
	}
	var batches []service.TrafficCreditBatch
	for rows.Next() {
		var b service.TrafficCreditBatch
		var oid, pid sql.NullInt64
		if err := rows.Scan(&b.ID, &b.UserID, &oid, &pid, &b.InitialUSD, &b.RemainingUSD, &b.CreditedAt, &b.ExpiresAt); err != nil {
			rows.Close()
			return false, nil, err
		}
		if oid.Valid {
			b.OrderID = &oid.Int64
		}
		if pid.Valid {
			b.PackID = &pid.Int64
		}
		batches = append(batches, b)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return false, nil, err
	}
	deductions, covered := service.PlanTrafficCreditDeductions(batches, amountUSD)
	if !covered {
		return false, nil, nil
	}
	for _, d := range deductions {
		var after float64
		if err := tx.QueryRowContext(ctx, `UPDATE user_traffic_credits SET remaining_usd=remaining_usd-$1,updated_at=$2 WHERE id=$3 AND remaining_usd+$4 >= $1 RETURNING remaining_usd`, d.AmountUSD, now, d.CreditID, 0.0000000001).Scan(&after); err != nil {
			return false, nil, err
		}
		if _, err := tx.ExecContext(ctx, `INSERT INTO traffic_credit_ledger(user_id,credit_id,order_id,request_id,entry_type,amount_usd,balance_after_usd,created_at) VALUES($1,$2,NULL,$3,$4,$5,$6,$7)`, userID, d.CreditID, requestID, service.TrafficCreditLedgerTypeDeduction, d.AmountUSD, after, now); err != nil {
			return false, nil, err
		}
	}
	if err := tx.Commit(); err != nil {
		return false, nil, err
	}
	return true, deductions, nil
}

func (r *trafficPackRepository) RevokePurchase(ctx context.Context, orderID int64, now time.Time) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	var userID, creditID int64
	var remaining float64
	err = tx.QueryRowContext(ctx, `UPDATE user_traffic_credits SET remaining_usd=0,updated_at=$2 WHERE order_id=$1 AND remaining_usd>0 RETURNING user_id,id,remaining_usd`, orderID, now).Scan(&userID, &creditID, &remaining)
	if errors.Is(err, sql.ErrNoRows) {
		return tx.Commit()
	}
	if err != nil {
		return err
	}
	if _, err = tx.ExecContext(ctx, `INSERT INTO traffic_credit_ledger(user_id,credit_id,order_id,request_id,entry_type,amount_usd,balance_after_usd,created_at) VALUES($1,$2,$3,'',$4,$5,0,$6)`, userID, creditID, orderID, service.TrafficCreditLedgerTypeRefund, remaining, now); err != nil {
		return err
	}
	return tx.Commit()
}
