package service

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"strings"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/payment"
	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
)

const (
	offlinePaymentBackfillSource            = "offline_paid_backfill_20260716"
	offlinePaymentBackfillPlanID      int64 = 1
	offlinePaymentBackfillGroupID     int64 = 2
	offlinePaymentBackfillAmount            = 29.00
	offlinePaymentBackfillDays              = 30
	offlinePaymentBackfillCurrency          = "CNY"
	offlinePaymentBackfillAuditAction       = "OFFLINE_PAYMENT_RECORDED"
)

var offlinePaymentBackfillRequiredColumns = []string{
	"subscription_id",
	"funding_mode",
	"balance_amount",
	"gateway_amount",
	"provider_snapshot",
	"refund_request_id",
	"refund_gateway_status",
	"refund_entitlement_status",
	"refund_provider_ref",
	"refund_balance_amount",
	"refund_gateway_amount",
	"refund_balance_status",
}

var offlinePaymentBackfillRequiredAuditLogColumns = []string{
	"id",
	"order_id",
	"action",
	"detail",
	"operator",
	"created_at",
}

var offlinePaymentBackfillRequiredUniqueIndexes = []struct {
	table     string
	name      string
	columns   string
	predicate string
}{
	{
		table:     "payment_orders",
		name:      "paymentorder_out_trade_no",
		columns:   "out_trade_no",
		predicate: "out_trade_no <> ''",
	},
	{
		table:   "payment_audit_logs",
		name:    "idx_payment_audit_logs_order_action_uniq",
		columns: "order_id,action",
	},
}

type offlinePaymentBackfillEntry struct {
	SubscriptionID int64
	UserID         int64
	PaidAt         time.Time
	ExpectedExpiry time.Time
}

type offlinePaymentBackfillBatch struct {
	Source  string
	PlanID  int64
	GroupID int64
	Entries []offlinePaymentBackfillEntry
}

type OfflinePaymentBackfillResult struct {
	Created  int
	Planned  int
	Existing int
	DryRun   bool
	Noop     bool
}

type offlinePaymentBackfillLockedEntry struct {
	Entry    offlinePaymentBackfillEntry
	Email    string
	Username string
	Notes    sql.NullString
}

type offlinePaymentBackfillExistingOrder struct {
	ID                  int64
	UserID              int64
	Amount              float64
	PayAmount           float64
	FeeRate             float64
	FundingMode         string
	BalanceAmount       float64
	GatewayAmount       float64
	OutTradeNo          string
	PaymentType         string
	PaymentTradeNo      string
	OrderType           string
	PlanID              sql.NullInt64
	SubscriptionGroupID sql.NullInt64
	SubscriptionDays    sql.NullInt64
	SubscriptionID      sql.NullInt64
	ProviderInstanceID  sql.NullString
	ProviderKey         sql.NullString
	ProviderSnapshot    sql.NullString
	Status              string
	ExpiresAt           time.Time
	PaidAt              sql.NullTime
	CompletedAt         sql.NullTime
	CreatedAt           time.Time
	UpdatedAt           time.Time
}

func RunOfflinePaymentBackfill(ctx context.Context, db *sql.DB, operator string, execute bool) (OfflinePaymentBackfillResult, error) {
	return runOfflinePaymentBackfillBatch(ctx, db, defaultOfflinePaymentBackfillBatch(), operator, execute)
}

func runOfflinePaymentBackfillBatch(ctx context.Context, db *sql.DB, batch offlinePaymentBackfillBatch, operator string, execute bool) (OfflinePaymentBackfillResult, error) {
	if db == nil {
		return OfflinePaymentBackfillResult{}, infraerrors.BadRequest("OFFLINE_PAYMENT_BACKFILL_DATABASE_REQUIRED", "database is required")
	}
	operator = strings.TrimSpace(operator)
	if operator == "" {
		return OfflinePaymentBackfillResult{}, infraerrors.BadRequest("OFFLINE_PAYMENT_BACKFILL_OPERATOR_REQUIRED", "operator is required")
	}
	if err := validateOfflinePaymentBackfillBatch(batch); err != nil {
		return OfflinePaymentBackfillResult{}, err
	}
	if err := ensureOfflinePaymentBackfillSchema(ctx, db); err != nil {
		return OfflinePaymentBackfillResult{}, err
	}

	tx, err := db.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelSerializable})
	if err != nil {
		return OfflinePaymentBackfillResult{}, infraerrors.InternalServer("OFFLINE_PAYMENT_BACKFILL_TRANSACTION_FAILED", "failed to start offline payment backfill transaction").WithCause(err)
	}
	defer func() { _ = tx.Rollback() }()

	if _, err := tx.ExecContext(ctx, "SELECT pg_advisory_xact_lock(hashtext('offline_paid_backfill_20260716'))"); err != nil {
		return OfflinePaymentBackfillResult{}, offlinePaymentBackfillTransactionError(err)
	}
	if _, err := tx.ExecContext(ctx, "LOCK TABLE payment_orders IN SHARE ROW EXCLUSIVE MODE"); err != nil {
		return OfflinePaymentBackfillResult{}, offlinePaymentBackfillTransactionError(err)
	}

	lockedEntries, err := lockOfflinePaymentBackfillEntries(ctx, tx, batch)
	if err != nil {
		return OfflinePaymentBackfillResult{}, err
	}
	if err := validateOfflinePaymentBackfillPlan(ctx, tx, batch); err != nil {
		return OfflinePaymentBackfillResult{}, err
	}

	existingOrders, err := loadOfflinePaymentBackfillExistingOrders(ctx, tx, batch)
	if err != nil {
		return OfflinePaymentBackfillResult{}, err
	}

	result := OfflinePaymentBackfillResult{DryRun: !execute}
	switch len(existingOrders) {
	case 0:
		if err := ensureNoOtherSubscriptionOrders(ctx, tx, batch, nil); err != nil {
			return OfflinePaymentBackfillResult{}, err
		}
		if !execute {
			result.Planned = len(lockedEntries)
			return result, nil
		}
		for _, lockedEntry := range lockedEntries {
			if err := insertOfflinePaymentBackfillOrder(ctx, tx, batch, lockedEntry, operator); err != nil {
				return OfflinePaymentBackfillResult{}, err
			}
			result.Created++
		}
	case len(batch.Entries):
		if err := ensureNoOtherSubscriptionOrders(ctx, tx, batch, existingOrders); err != nil {
			return OfflinePaymentBackfillResult{}, err
		}
		for _, entry := range batch.Entries {
			order := existingOrders[offlinePaymentBackfillOutTradeNo(batch.Source, entry.SubscriptionID)]
			if err := verifyOfflinePaymentBackfillExistingOrder(ctx, tx, batch, entry, order); err != nil {
				return OfflinePaymentBackfillResult{}, err
			}
		}
		result.Existing = len(batch.Entries)
		result.Noop = true
	default:
		return OfflinePaymentBackfillResult{}, infraerrors.Conflict("OFFLINE_PAYMENT_BACKFILL_PRECONDITION_FAILED", "offline payment backfill has a partial existing batch")
	}

	if !execute {
		return result, nil
	}
	if err := tx.Commit(); err != nil {
		return OfflinePaymentBackfillResult{}, offlinePaymentBackfillTransactionError(err)
	}
	return result, nil
}

func defaultOfflinePaymentBackfillBatch() offlinePaymentBackfillBatch {
	return offlinePaymentBackfillBatch{
		Source:  offlinePaymentBackfillSource,
		PlanID:  offlinePaymentBackfillPlanID,
		GroupID: offlinePaymentBackfillGroupID,
		Entries: []offlinePaymentBackfillEntry{
			{SubscriptionID: 2, UserID: 3, PaidAt: mustParseOfflinePaymentBackfillTime("2026-07-16T12:08:33.371+08:00"), ExpectedExpiry: mustParseOfflinePaymentBackfillTime("2026-08-16T00:00:00+08:00")},
			{SubscriptionID: 4, UserID: 6, PaidAt: mustParseOfflinePaymentBackfillTime("2026-07-16T12:06:25.442+08:00"), ExpectedExpiry: mustParseOfflinePaymentBackfillTime("2026-08-16T00:00:00+08:00")},
			{SubscriptionID: 7, UserID: 12, PaidAt: mustParseOfflinePaymentBackfillTime("2026-07-16T12:05:16.893+08:00"), ExpectedExpiry: mustParseOfflinePaymentBackfillTime("2026-08-16T00:00:00+08:00")},
			{SubscriptionID: 9, UserID: 15, PaidAt: mustParseOfflinePaymentBackfillTime("2026-07-16T11:49:52.625+08:00"), ExpectedExpiry: mustParseOfflinePaymentBackfillTime("2026-10-15T00:00:00+08:00")},
			{SubscriptionID: 13, UserID: 21, PaidAt: mustParseOfflinePaymentBackfillTime("2026-07-16T13:30:29.288+08:00"), ExpectedExpiry: mustParseOfflinePaymentBackfillTime("2026-10-15T00:00:00+08:00")},
		},
	}
}

func validateOfflinePaymentBackfillBatch(batch offlinePaymentBackfillBatch) error {
	if batch.Source == "" || strings.TrimSpace(batch.Source) != batch.Source || len(batch.Source) > 42 {
		return infraerrors.BadRequest("OFFLINE_PAYMENT_BACKFILL_INVALID_BATCH", "offline payment backfill source is invalid")
	}
	if batch.PlanID <= 0 || batch.GroupID <= 0 || len(batch.Entries) != 5 {
		return infraerrors.BadRequest("OFFLINE_PAYMENT_BACKFILL_INVALID_BATCH", "offline payment backfill batch is invalid")
	}

	subscriptionIDs := make(map[int64]struct{}, len(batch.Entries))
	userIDs := make(map[int64]struct{}, len(batch.Entries))
	for _, entry := range batch.Entries {
		if entry.SubscriptionID <= 0 || entry.UserID <= 0 || entry.PaidAt.IsZero() || entry.ExpectedExpiry.IsZero() {
			return infraerrors.BadRequest("OFFLINE_PAYMENT_BACKFILL_INVALID_BATCH", "offline payment backfill entry is invalid")
		}
		if _, exists := subscriptionIDs[entry.SubscriptionID]; exists {
			return infraerrors.BadRequest("OFFLINE_PAYMENT_BACKFILL_INVALID_BATCH", "offline payment backfill subscription is duplicated")
		}
		if _, exists := userIDs[entry.UserID]; exists {
			return infraerrors.BadRequest("OFFLINE_PAYMENT_BACKFILL_INVALID_BATCH", "offline payment backfill user is duplicated")
		}
		subscriptionIDs[entry.SubscriptionID] = struct{}{}
		userIDs[entry.UserID] = struct{}{}
	}
	return nil
}

func ensureOfflinePaymentBackfillSchema(ctx context.Context, db *sql.DB) error {
	requiredMigrations := []string{"162_refund_state_machine.sql", "163_alipay_balance_hybrid_payment.sql"}
	rows, err := db.QueryContext(ctx, `
		SELECT filename
		FROM schema_migrations
		WHERE filename IN ($1, $2)
	`, requiredMigrations[0], requiredMigrations[1])
	if err != nil {
		return offlinePaymentBackfillSchemaNotReady(err)
	}
	defer func() { _ = rows.Close() }()

	applied := make(map[string]struct{}, len(requiredMigrations))
	for rows.Next() {
		var filename string
		if err := rows.Scan(&filename); err != nil {
			return offlinePaymentBackfillSchemaNotReady(err)
		}
		applied[filename] = struct{}{}
	}
	if err := rows.Err(); err != nil {
		return offlinePaymentBackfillSchemaNotReady(err)
	}
	for _, filename := range requiredMigrations {
		if _, exists := applied[filename]; !exists {
			return offlinePaymentBackfillSchemaNotReady(fmt.Errorf("required migration %s is missing", filename))
		}
	}

	columnRows, err := db.QueryContext(ctx, `
		SELECT column_name
		FROM information_schema.columns
		WHERE table_schema = current_schema()
		  AND table_name = 'payment_orders'
		  AND column_name IN (
				'subscription_id', 'funding_mode', 'balance_amount', 'gateway_amount', 'provider_snapshot',
				'refund_request_id', 'refund_gateway_status', 'refund_entitlement_status', 'refund_provider_ref',
				'refund_balance_amount', 'refund_gateway_amount', 'refund_balance_status'
			)
	`)
	if err != nil {
		return offlinePaymentBackfillSchemaNotReady(err)
	}
	defer func() { _ = columnRows.Close() }()

	present := make(map[string]struct{}, len(offlinePaymentBackfillRequiredColumns))
	for columnRows.Next() {
		var column string
		if err := columnRows.Scan(&column); err != nil {
			return offlinePaymentBackfillSchemaNotReady(err)
		}
		present[column] = struct{}{}
	}
	if err := columnRows.Err(); err != nil {
		return offlinePaymentBackfillSchemaNotReady(err)
	}
	for _, column := range offlinePaymentBackfillRequiredColumns {
		if _, exists := present[column]; !exists {
			return offlinePaymentBackfillSchemaNotReady(fmt.Errorf("required payment_orders column %s is missing", column))
		}
	}

	var auditTable string
	err = db.QueryRowContext(ctx, `
		SELECT table_name
		FROM information_schema.tables
		WHERE table_schema = current_schema()
		  AND table_name = $1
	`, "payment_audit_logs").Scan(&auditTable)
	if errors.Is(err, sql.ErrNoRows) {
		return offlinePaymentBackfillSchemaNotReady(fmt.Errorf("required table payment_audit_logs is missing"))
	}
	if err != nil {
		return offlinePaymentBackfillSchemaNotReady(err)
	}
	if auditTable != "payment_audit_logs" {
		return offlinePaymentBackfillSchemaNotReady(fmt.Errorf("required table payment_audit_logs is missing"))
	}

	auditColumnRows, err := db.QueryContext(ctx, `
		SELECT column_name
		FROM information_schema.columns
		WHERE table_schema = current_schema()
		  AND table_name = 'payment_audit_logs'
		  AND column_name IN ('id', 'order_id', 'action', 'detail', 'operator', 'created_at')
	`)
	if err != nil {
		return offlinePaymentBackfillSchemaNotReady(err)
	}
	defer func() { _ = auditColumnRows.Close() }()

	auditColumns := make(map[string]struct{}, len(offlinePaymentBackfillRequiredAuditLogColumns))
	for auditColumnRows.Next() {
		var column string
		if err := auditColumnRows.Scan(&column); err != nil {
			return offlinePaymentBackfillSchemaNotReady(err)
		}
		auditColumns[column] = struct{}{}
	}
	if err := auditColumnRows.Err(); err != nil {
		return offlinePaymentBackfillSchemaNotReady(err)
	}
	for _, column := range offlinePaymentBackfillRequiredAuditLogColumns {
		if _, exists := auditColumns[column]; !exists {
			return offlinePaymentBackfillSchemaNotReady(fmt.Errorf("required payment_audit_logs column %s is missing", column))
		}
	}

	for _, index := range offlinePaymentBackfillRequiredUniqueIndexes {
		if err := ensureOfflinePaymentBackfillUniqueIndex(ctx, db, index.table, index.name, index.columns, index.predicate); err != nil {
			return err
		}
	}
	return nil
}

func ensureOfflinePaymentBackfillUniqueIndex(ctx context.Context, db *sql.DB, table, name, columns, predicate string) error {
	var unique bool
	var actualColumns, actualPredicate string
	err := db.QueryRowContext(ctx, `
		SELECT
			i.indisunique,
			COALESCE(index_columns.columns, ''),
			COALESCE(pg_get_expr(i.indpred, i.indrelid), '')
		FROM pg_class AS index_class
		JOIN pg_index AS i ON i.indexrelid = index_class.oid
		JOIN pg_class AS table_class ON table_class.oid = i.indrelid
		JOIN pg_namespace AS table_schema ON table_schema.oid = table_class.relnamespace
		JOIN LATERAL (
			SELECT string_agg(attribute.attname::text, ',' ORDER BY key_attnum.ordinality) AS columns
			FROM unnest(i.indkey::smallint[]) WITH ORDINALITY AS key_attnum(attnum, ordinality)
			JOIN pg_attribute AS attribute
				ON attribute.attrelid = table_class.oid
			   AND attribute.attnum = key_attnum.attnum
		) AS index_columns ON true
		WHERE table_schema.nspname = current_schema()
		  AND table_class.relname = $1
		  AND index_class.relname = $2
	`, table, name).Scan(&unique, &actualColumns, &actualPredicate)
	if errors.Is(err, sql.ErrNoRows) {
		return offlinePaymentBackfillSchemaNotReady(fmt.Errorf("required unique index %s on %s is missing", name, table))
	}
	if err != nil {
		return offlinePaymentBackfillSchemaNotReady(err)
	}
	if !unique || actualColumns != columns || normalizeOfflinePaymentBackfillIndexPredicate(actualPredicate) != normalizeOfflinePaymentBackfillIndexPredicate(predicate) {
		return offlinePaymentBackfillSchemaNotReady(fmt.Errorf("required unique index %s on %s is invalid", name, table))
	}
	return nil
}

func normalizeOfflinePaymentBackfillIndexPredicate(value string) string {
	normalized := strings.ToLower(strings.Join(strings.Fields(value), ""))
	return strings.NewReplacer(
		`"`, "",
		"(", "",
		")", "",
		"::charactervarying", "",
		"::varchar", "",
		"::text", "",
	).Replace(normalized)
}

func lockOfflinePaymentBackfillEntries(ctx context.Context, tx *sql.Tx, batch offlinePaymentBackfillBatch) ([]offlinePaymentBackfillLockedEntry, error) {
	lockedEntries := make([]offlinePaymentBackfillLockedEntry, 0, len(batch.Entries))
	for _, entry := range batch.Entries {
		var subscriptionID, userID, groupID int64
		var status string
		var expiresAt time.Time
		var subscriptionDeletedAt, userDeletedAt sql.NullTime
		var email string
		var username, notes sql.NullString
		err := tx.QueryRowContext(ctx, `
			SELECT us.id, us.user_id, us.group_id, us.status, us.expires_at, us.deleted_at,
				u.email, u.username, u.notes, u.deleted_at
			FROM user_subscriptions AS us
			JOIN users AS u ON u.id = us.user_id
			WHERE us.id = $1
			  AND us.user_id = $2
			FOR UPDATE OF us, u
		`, entry.SubscriptionID, entry.UserID).Scan(
			&subscriptionID, &userID, &groupID, &status, &expiresAt, &subscriptionDeletedAt,
			&email, &username, &notes, &userDeletedAt,
		)
		if errors.Is(err, sql.ErrNoRows) {
			return nil, offlinePaymentBackfillPreconditionError()
		}
		if err != nil {
			return nil, offlinePaymentBackfillTransactionError(err)
		}
		if subscriptionID != entry.SubscriptionID || userID != entry.UserID || groupID != batch.GroupID || status != "active" || subscriptionDeletedAt.Valid || userDeletedAt.Valid || !expiresAt.Equal(entry.ExpectedExpiry) {
			return nil, offlinePaymentBackfillPreconditionError()
		}
		lockedEntries = append(lockedEntries, offlinePaymentBackfillLockedEntry{
			Entry:    entry,
			Email:    email,
			Username: username.String,
			Notes:    notes,
		})
	}
	return lockedEntries, nil
}

func validateOfflinePaymentBackfillPlan(ctx context.Context, tx *sql.Tx, batch offlinePaymentBackfillBatch) error {
	var id, groupID int64
	var price float64
	var validityDays int
	err := tx.QueryRowContext(ctx, `
		SELECT id, group_id, price, validity_days
		FROM subscription_plans
		WHERE id = $1
		FOR UPDATE
	`, batch.PlanID).Scan(&id, &groupID, &price, &validityDays)
	if errors.Is(err, sql.ErrNoRows) {
		return offlinePaymentBackfillPreconditionError()
	}
	if err != nil {
		return offlinePaymentBackfillTransactionError(err)
	}
	if id != batch.PlanID || groupID != batch.GroupID || math.Abs(price-offlinePaymentBackfillAmount) > 0.000001 || validityDays != offlinePaymentBackfillDays {
		return offlinePaymentBackfillPreconditionError()
	}
	return nil
}

func loadOfflinePaymentBackfillExistingOrders(ctx context.Context, tx *sql.Tx, batch offlinePaymentBackfillBatch) (map[string]*offlinePaymentBackfillExistingOrder, error) {
	orders := make(map[string]*offlinePaymentBackfillExistingOrder, len(batch.Entries))
	for _, entry := range batch.Entries {
		outTradeNo := offlinePaymentBackfillOutTradeNo(batch.Source, entry.SubscriptionID)
		order, err := loadOfflinePaymentBackfillExistingOrder(ctx, tx, outTradeNo)
		if errors.Is(err, sql.ErrNoRows) {
			continue
		}
		if err != nil {
			if infraerrors.Reason(err) == "OFFLINE_PAYMENT_BACKFILL_EXISTING_RECORD_MISMATCH" {
				return nil, err
			}
			return nil, offlinePaymentBackfillTransactionError(err)
		}
		orders[outTradeNo] = order
	}
	return orders, nil
}

func loadOfflinePaymentBackfillExistingOrder(ctx context.Context, tx *sql.Tx, outTradeNo string) (*offlinePaymentBackfillExistingOrder, error) {
	order := &offlinePaymentBackfillExistingOrder{}
	rows, err := tx.QueryContext(ctx, `
		SELECT id, user_id, amount, pay_amount, fee_rate, funding_mode, balance_amount, gateway_amount,
			out_trade_no, payment_type, payment_trade_no, order_type, plan_id, subscription_group_id,
			subscription_days, subscription_id, provider_instance_id, provider_key, provider_snapshot, status,
			expires_at, paid_at, completed_at, created_at, updated_at
		FROM payment_orders
		WHERE out_trade_no = $1
		ORDER BY id ASC
		LIMIT 2
		FOR UPDATE
	`, outTradeNo)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	count := 0
	for rows.Next() {
		count++
		if err := rows.Scan(
			&order.ID, &order.UserID, &order.Amount, &order.PayAmount, &order.FeeRate, &order.FundingMode, &order.BalanceAmount, &order.GatewayAmount,
			&order.OutTradeNo, &order.PaymentType, &order.PaymentTradeNo, &order.OrderType, &order.PlanID, &order.SubscriptionGroupID,
			&order.SubscriptionDays, &order.SubscriptionID, &order.ProviderInstanceID, &order.ProviderKey, &order.ProviderSnapshot, &order.Status,
			&order.ExpiresAt, &order.PaidAt, &order.CompletedAt, &order.CreatedAt, &order.UpdatedAt,
		); err != nil {
			return nil, err
		}
		if count > 1 {
			return nil, offlinePaymentBackfillExistingRecordMismatchError()
		}
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	if count == 0 {
		return nil, sql.ErrNoRows
	}
	return order, nil
}

func ensureNoOtherSubscriptionOrders(ctx context.Context, tx *sql.Tx, batch offlinePaymentBackfillBatch, expected map[string]*offlinePaymentBackfillExistingOrder) error {
	expectedOutTradeNos := make(map[string]struct{}, len(expected))
	for outTradeNo := range expected {
		expectedOutTradeNos[outTradeNo] = struct{}{}
	}
	for _, entry := range batch.Entries {
		rows, err := tx.QueryContext(ctx, `
			SELECT out_trade_no
			FROM payment_orders
			WHERE user_id = $1
			  AND order_type = $2
			FOR UPDATE
		`, entry.UserID, payment.OrderTypeSubscription)
		if err != nil {
			return offlinePaymentBackfillTransactionError(err)
		}
		for rows.Next() {
			var outTradeNo string
			if err := rows.Scan(&outTradeNo); err != nil {
				_ = rows.Close()
				return offlinePaymentBackfillTransactionError(err)
			}
			if _, exists := expectedOutTradeNos[outTradeNo]; !exists {
				_ = rows.Close()
				return offlinePaymentBackfillPreconditionError()
			}
		}
		if err := rows.Err(); err != nil {
			_ = rows.Close()
			return offlinePaymentBackfillTransactionError(err)
		}
		if err := rows.Close(); err != nil {
			return offlinePaymentBackfillTransactionError(err)
		}
	}
	return nil
}

func verifyOfflinePaymentBackfillExistingOrder(ctx context.Context, tx *sql.Tx, batch offlinePaymentBackfillBatch, entry offlinePaymentBackfillEntry, order *offlinePaymentBackfillExistingOrder) error {
	if order == nil || !offlinePaymentBackfillOrderMatches(batch, entry, order) {
		return offlinePaymentBackfillExistingRecordMismatchError()
	}
	detail, err := offlinePaymentBackfillAuditDetail(batch.Source, entry)
	if err != nil {
		return offlinePaymentBackfillTransactionError(err)
	}
	rows, err := tx.QueryContext(ctx, `
		SELECT order_id, action, detail, operator
		FROM payment_audit_logs
		WHERE order_id = $1
		  AND action = $2
		ORDER BY id ASC
		LIMIT 2
		FOR UPDATE
	`, fmt.Sprint(order.ID), offlinePaymentBackfillAuditAction)
	if err != nil {
		return offlinePaymentBackfillTransactionError(err)
	}
	defer func() { _ = rows.Close() }()

	var orderID, action, existingDetail, operator string
	count := 0
	for rows.Next() {
		count++
		if err := rows.Scan(&orderID, &action, &existingDetail, &operator); err != nil {
			return offlinePaymentBackfillTransactionError(err)
		}
		if count > 1 {
			return offlinePaymentBackfillExistingRecordMismatchError()
		}
	}
	if err := rows.Err(); err != nil {
		return offlinePaymentBackfillTransactionError(err)
	}
	if count != 1 {
		return offlinePaymentBackfillExistingRecordMismatchError()
	}
	if orderID != fmt.Sprint(order.ID) || action != offlinePaymentBackfillAuditAction || strings.TrimSpace(operator) == "" || existingDetail != detail {
		return offlinePaymentBackfillExistingRecordMismatchError()
	}
	return nil
}

func offlinePaymentBackfillOrderMatches(batch offlinePaymentBackfillBatch, entry offlinePaymentBackfillEntry, order *offlinePaymentBackfillExistingOrder) bool {
	return order.UserID == entry.UserID &&
		math.Abs(order.Amount-offlinePaymentBackfillAmount) <= 0.000001 &&
		math.Abs(order.PayAmount-offlinePaymentBackfillAmount) <= 0.000001 &&
		math.Abs(order.FeeRate) <= 0.000001 &&
		order.FundingMode == payment.TypeOffline &&
		math.Abs(order.BalanceAmount) <= 0.000001 &&
		math.Abs(order.GatewayAmount) <= 0.000001 &&
		order.OutTradeNo == offlinePaymentBackfillOutTradeNo(batch.Source, entry.SubscriptionID) &&
		order.PaymentType == payment.TypeOffline &&
		order.PaymentTradeNo == "" &&
		order.OrderType == payment.OrderTypeSubscription &&
		order.PlanID.Valid && order.PlanID.Int64 == batch.PlanID &&
		order.SubscriptionGroupID.Valid && order.SubscriptionGroupID.Int64 == batch.GroupID &&
		order.SubscriptionDays.Valid && order.SubscriptionDays.Int64 == offlinePaymentBackfillDays &&
		order.SubscriptionID.Valid && order.SubscriptionID.Int64 == entry.SubscriptionID &&
		!order.ProviderInstanceID.Valid && !order.ProviderKey.Valid && !order.ProviderSnapshot.Valid &&
		order.Status == OrderStatusCompleted &&
		order.ExpiresAt.Equal(entry.PaidAt) &&
		order.PaidAt.Valid && order.PaidAt.Time.Equal(entry.PaidAt) &&
		order.CompletedAt.Valid && order.CompletedAt.Time.Equal(entry.PaidAt) &&
		order.CreatedAt.Equal(entry.PaidAt) &&
		order.UpdatedAt.Equal(entry.PaidAt)
}

func insertOfflinePaymentBackfillOrder(ctx context.Context, tx *sql.Tx, batch offlinePaymentBackfillBatch, lockedEntry offlinePaymentBackfillLockedEntry, operator string) error {
	entry := lockedEntry.Entry
	var orderID int64
	err := tx.QueryRowContext(ctx, `
		INSERT INTO payment_orders (
			user_id, user_email, user_name, user_notes,
			amount, pay_amount, fee_rate, funding_mode, balance_amount, gateway_amount,
			recharge_code, out_trade_no, payment_type, payment_trade_no, order_type,
			plan_id, subscription_group_id, subscription_days, subscription_id,
			provider_instance_id, provider_key, provider_snapshot, status,
			expires_at, paid_at, completed_at, client_ip, src_host, src_url, created_at, updated_at
		) VALUES (
			$1, $2, $3, $4,
			29.00, 29.00, 0, 'offline', 0, 0,
			'', $5, 'offline', '', 'subscription',
			$6, $7, 30, $8,
			NULL, NULL, NULL, 'COMPLETED',
			$9, $9, $9, '', 'offline-payment-backfill', NULL, $9, $9
		)
		RETURNING id
	`, entry.UserID, lockedEntry.Email, lockedEntry.Username, nullableOfflinePaymentBackfillString(lockedEntry.Notes),
		offlinePaymentBackfillOutTradeNo(batch.Source, entry.SubscriptionID), batch.PlanID, batch.GroupID, entry.SubscriptionID, entry.PaidAt).Scan(&orderID)
	if err != nil {
		return offlinePaymentBackfillTransactionError(err)
	}
	detail, err := offlinePaymentBackfillAuditDetail(batch.Source, entry)
	if err != nil {
		return offlinePaymentBackfillTransactionError(err)
	}
	if _, err := tx.ExecContext(ctx, `
		INSERT INTO payment_audit_logs (order_id, action, detail, operator, created_at)
		VALUES ($1, $2, $3, $4, $5)
	`, fmt.Sprint(orderID), offlinePaymentBackfillAuditAction, detail, operator, time.Now().UTC()); err != nil {
		return offlinePaymentBackfillTransactionError(err)
	}
	return nil
}

func offlinePaymentBackfillAuditDetail(source string, entry offlinePaymentBackfillEntry) (string, error) {
	detail := struct {
		Source         string      `json:"source"`
		SubscriptionID int64       `json:"subscription_id"`
		UserID         int64       `json:"user_id"`
		PaidAt         string      `json:"paid_at"`
		Amount         json.Number `json:"amount"`
		Currency       string      `json:"currency"`
		RefundPolicy   string      `json:"refund_policy"`
	}{
		Source:         source,
		SubscriptionID: entry.SubscriptionID,
		UserID:         entry.UserID,
		PaidAt:         entry.PaidAt.Format(time.RFC3339Nano),
		Amount:         json.Number("29.00"),
		Currency:       offlinePaymentBackfillCurrency,
		RefundPolicy:   "manual_only",
	}
	encoded, err := json.Marshal(detail)
	if err != nil {
		return "", err
	}
	return string(encoded), nil
}

func nullableOfflinePaymentBackfillString(value sql.NullString) any {
	if !value.Valid {
		return nil
	}
	return value.String
}

func offlinePaymentBackfillOutTradeNo(source string, subscriptionID int64) string {
	return fmt.Sprintf("%s_s%d", source, subscriptionID)
}

func offlinePaymentBackfillSchemaNotReady(cause error) error {
	return infraerrors.Conflict("OFFLINE_PAYMENT_BACKFILL_SCHEMA_NOT_READY", "offline payment backfill schema is not ready").WithCause(cause)
}

func offlinePaymentBackfillPreconditionError() error {
	return infraerrors.Conflict("OFFLINE_PAYMENT_BACKFILL_PRECONDITION_FAILED", "offline payment backfill precondition failed")
}

func offlinePaymentBackfillExistingRecordMismatchError() error {
	return infraerrors.Conflict("OFFLINE_PAYMENT_BACKFILL_EXISTING_RECORD_MISMATCH", "offline payment backfill existing record does not match")
}

func offlinePaymentBackfillTransactionError(cause error) error {
	return infraerrors.InternalServer("OFFLINE_PAYMENT_BACKFILL_TRANSACTION_FAILED", "offline payment backfill transaction failed").WithCause(cause)
}

func mustParseOfflinePaymentBackfillTime(raw string) time.Time {
	parsed, err := time.Parse(time.RFC3339Nano, raw)
	if err != nil {
		panic(err)
	}
	return parsed
}
