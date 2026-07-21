package service

import (
	"context"
	stdsql "database/sql"
	"errors"
	"fmt"
	"math"
	"time"

	"entgo.io/ent/dialect"
	entsql "entgo.io/ent/dialect/sql"
	dbent "github.com/Wei-Shaw/sub2api/ent"
	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
)

// SubscriptionRefundQuote 是用户可见报价与提交退款共用的不可变计算输入。
type SubscriptionRefundQuote struct {
	Eligible              bool      `json:"eligible"`
	ManualReviewRequired  bool      `json:"manual_review_required"`
	EntitlementPeriodID   int64     `json:"entitlement_period_id,omitempty"`
	PurchaseBaseAmount    float64   `json:"purchase_base_amount"`
	NonRefundableFee      float64   `json:"non_refundable_fee"`
	PeriodTotalQuotaUSD   float64   `json:"period_total_quota_usd"`
	UsedQuotaUSD          float64   `json:"used_quota_usd"`
	UsageRatio            float64   `json:"usage_ratio"`
	EstimatedRefundAmount float64   `json:"estimated_refund_amount"`
	CalculatedAt          time.Time `json:"calculated_at"`
}

var ErrRefundManualReviewRequired = infraerrors.Conflict("REFUND_MANUAL_REVIEW_REQUIRED", "refund requires manual review because historical usage cannot be allocated unambiguously")

func (s *PaymentService) GetSubscriptionRefundQuote(ctx context.Context, orderID, userID int64) (*SubscriptionRefundQuote, error) {
	o, err := s.entClient.PaymentOrder.Get(ctx, orderID)
	if err != nil {
		return nil, infraerrors.NotFound("NOT_FOUND", "order not found")
	}
	if o.UserID != userID {
		return nil, infraerrors.Forbidden("FORBIDDEN", "no permission")
	}
	return s.calculateSubscriptionRefundQuote(ctx, o)
}

func (s *PaymentService) AdminGetSubscriptionRefundQuote(ctx context.Context, orderID int64) (*SubscriptionRefundQuote, error) {
	o, err := s.entClient.PaymentOrder.Get(ctx, orderID)
	if err != nil {
		return nil, infraerrors.NotFound("NOT_FOUND", "order not found")
	}
	return s.calculateSubscriptionRefundQuote(ctx, o)
}

func (s *PaymentService) calculateSubscriptionRefundQuote(ctx context.Context, o *dbent.PaymentOrder) (*SubscriptionRefundQuote, error) {
	return s.calculateSubscriptionRefundQuoteWithClient(ctx, s.entClient, o, false)
}

func (s *PaymentService) calculateSubscriptionRefundQuoteWithClient(ctx context.Context, client *dbent.Client, o *dbent.PaymentOrder, lockFacts bool) (*SubscriptionRefundQuote, error) {
	if o == nil || o.OrderType != "subscription" {
		return nil, infraerrors.BadRequest("INVALID_ORDER_TYPE", "only subscription orders have a refund quote")
	}
	if client == nil {
		return nil, infraerrors.InternalServer("PAYMENT_REPOSITORY_UNAVAILABLE", "payment repository is unavailable")
	}
	quote := &SubscriptionRefundQuote{PurchaseBaseAmount: o.Amount, NonRefundableFee: math.Max(o.PayAmount-o.Amount, 0), CalculatedAt: time.Now()}
	periodID, totalQuota, found, err := s.findRefundEntitlementWithClient(ctx, client, o.ID, lockFacts)
	if err != nil {
		if errors.Is(err, ErrRefundManualReviewRequired) {
			quote.ManualReviewRequired = true
			return quote, nil
		}
		return nil, err
	}
	if !found || totalQuota <= 0 {
		quote.ManualReviewRequired = true
		return quote, nil
	}
	quote.EntitlementPeriodID = periodID
	quote.PeriodTotalQuotaUSD = totalQuota
	used, allocatedFacts, err := s.sumRefundEntitlementUsageFactsWithClient(ctx, client, periodID, lockFacts)
	if err != nil {
		return nil, err
	}
	if !allocatedFacts {
		hasUnallocated, err := s.hasUnallocatedRefundUsageFactsWithClient(ctx, client, periodID, lockFacts)
		if err != nil {
			return nil, err
		}
		if hasUnallocated {
			quote.ManualReviewRequired = true
			return quote, nil
		}
		ambiguous, err := s.hasAmbiguousRefundEntitlementWithClient(ctx, client, periodID)
		if err != nil {
			return nil, err
		}
		if ambiguous {
			quote.ManualReviewRequired = true
			return quote, nil
		}
		used, err = s.sumRefundLegacyUsageLogs(ctx, periodID)
		if err != nil {
			return nil, err
		}
	}
	quote.UsedQuotaUSD = math.Max(used, 0)
	quote.UsageRatio = math.Min(math.Max(quote.UsedQuotaUSD/quote.PeriodTotalQuotaUSD, 0), 1)
	quote.EstimatedRefundAmount = math.Max(quote.PurchaseBaseAmount*(1-quote.UsageRatio), 0)
	quote.Eligible = quote.EstimatedRefundAmount > 0
	return quote, nil
}

func (s *PaymentService) findRefundEntitlement(ctx context.Context, orderID int64) (int64, float64, bool, error) {
	return s.findRefundEntitlementWithClient(ctx, s.entClient, orderID, false)
}

func (s *PaymentService) findRefundEntitlementWithClient(ctx context.Context, client *dbent.Client, orderID int64, lockRow bool) (int64, float64, bool, error) {
	var rows entsql.Rows
	query := `SELECT id, period_total_quota_usd FROM subscription_entitlement_periods WHERE source_type = 'payment_order' AND source_id = $1 AND status = 'active' ORDER BY id LIMIT 2`
	if lockRow && isPostgresEntClient(client) {
		query += ` FOR UPDATE`
	}
	err := client.Driver().Query(ctx, query, []any{fmt.Sprint(orderID)}, &rows)
	if err != nil {
		return 0, 0, false, err
	}
	defer rows.Close()
	if !rows.Next() {
		return 0, 0, false, rows.Err()
	}
	var id int64
	var total stdsql.NullFloat64
	if err := rows.Scan(&id, &total); err != nil {
		return 0, 0, false, err
	}
	if rows.Next() {
		return 0, 0, false, ErrRefundManualReviewRequired
	}
	if err := rows.Err(); err != nil {
		return 0, 0, false, err
	}
	return id, total.Float64, total.Valid, nil
}

func (s *PaymentService) sumRefundEntitlementUsageFacts(ctx context.Context, periodID int64) (float64, bool, error) {
	return s.sumRefundEntitlementUsageFactsWithClient(ctx, s.entClient, periodID, false)
}

func (s *PaymentService) sumRefundEntitlementUsageFactsWithClient(ctx context.Context, client *dbent.Client, periodID int64, lockRows bool) (float64, bool, error) {
	var rows entsql.Rows
	costExpr := refundUsageFactCostExpression(client, "payload")
	query := fmt.Sprintf(`SELECT COUNT(*), COALESCE(SUM(COALESCE(%s, 0)), 0) FROM usage_facts WHERE entitlement_period_id = $1 AND billing_status IN ('pending','settling','settled','debt')`, costExpr)
	if lockRows && isPostgresEntClient(client) {
		query = fmt.Sprintf(`WITH locked_usage_facts AS (
			SELECT payload
			FROM usage_facts
			WHERE entitlement_period_id = $1
				AND billing_status IN ('pending','settling','settled','debt')
			FOR UPDATE
		)
		SELECT COUNT(*), COALESCE(SUM(COALESCE(%s, 0)), 0)
		FROM locked_usage_facts`, costExpr)
	}
	err := client.Driver().Query(ctx, query, []any{periodID}, &rows)
	if err != nil {
		return 0, false, err
	}
	defer rows.Close()
	if !rows.Next() {
		return 0, false, rows.Err()
	}
	var count int64
	var total float64
	if err := rows.Scan(&count, &total); err != nil {
		return 0, false, err
	}
	return total, count > 0, nil
}

func refundUsageFactCostExpression(client *dbent.Client, payloadExpr string) string {
	if isPostgresEntClient(client) {
		return fmt.Sprintf(
			"NULLIF(%[1]s #>> '{usage_log,ActualCost}', '')::numeric, NULLIF(%[1]s #>> '{usage_log,actual_cost}', '')::numeric, NULLIF(%[1]s #>> '{effects,actual_cost}', '')::numeric, NULLIF(%[1]s #>> '{billing_command,subscription_cost}', '')::numeric",
			payloadExpr,
		)
	}
	return fmt.Sprintf(
		"CAST(NULLIF(json_extract(%[1]s, '$.usage_log.ActualCost'), '') AS REAL), CAST(NULLIF(json_extract(%[1]s, '$.usage_log.actual_cost'), '') AS REAL), CAST(NULLIF(json_extract(%[1]s, '$.effects.actual_cost'), '') AS REAL), CAST(NULLIF(json_extract(%[1]s, '$.billing_command.subscription_cost'), '') AS REAL)",
		payloadExpr,
	)
}

func isPostgresEntClient(client *dbent.Client) bool {
	return client != nil && client.Driver() != nil && client.Driver().Dialect() == dialect.Postgres
}

func (s *PaymentService) hasAmbiguousRefundEntitlement(ctx context.Context, periodID int64) (bool, error) {
	return s.hasAmbiguousRefundEntitlementWithClient(ctx, s.entClient, periodID)
}

func (s *PaymentService) hasAmbiguousRefundEntitlementWithClient(ctx context.Context, client *dbent.Client, periodID int64) (bool, error) {
	var rows entsql.Rows
	err := client.Driver().Query(ctx, `SELECT EXISTS(SELECT 1 FROM subscription_entitlement_periods target JOIN subscription_entitlement_periods other ON other.subscription_id = target.subscription_id AND other.id <> target.id AND other.status = 'active' AND other.starts_at < target.expires_at AND target.starts_at < other.expires_at WHERE target.id = $1)`, []any{periodID}, &rows)
	if err != nil {
		return false, err
	}
	defer rows.Close()
	if !rows.Next() {
		return false, rows.Err()
	}
	var ambiguous bool
	if err := rows.Scan(&ambiguous); err != nil {
		return false, err
	}
	return ambiguous, nil
}

func (s *PaymentService) hasUnallocatedRefundUsageFactsWithClient(ctx context.Context, client *dbent.Client, periodID int64, lockRows bool) (bool, error) {
	var rows entsql.Rows
	query := `
		SELECT EXISTS(
			SELECT 1
			FROM usage_facts uf
			JOIN subscription_entitlement_periods sep ON sep.id = $1
			WHERE uf.entitlement_period_id IS NULL
				AND uf.billing_status IN ('pending','settling','settled','debt')
				AND uf.completed_at >= sep.starts_at
				AND uf.completed_at < sep.expires_at
				AND uf.user_id = sep.user_id
				AND sep.subscription_id::text IN (
					NULLIF(uf.payload #>> '{billing_command,SubscriptionID}', ''),
					NULLIF(uf.payload #>> '{billing_command,subscription_id}', ''),
					NULLIF(uf.payload #>> '{usage_log,SubscriptionID}', ''),
					NULLIF(uf.payload #>> '{usage_log,subscription_id}', '')
				)
		`
	if lockRows && isPostgresEntClient(client) {
		query += ` FOR UPDATE`
	}
	query += `)`
	if !isPostgresEntClient(client) {
		query = `
			SELECT EXISTS(
				SELECT 1
				FROM usage_facts uf
				JOIN subscription_entitlement_periods sep ON sep.id = ?
				WHERE uf.entitlement_period_id IS NULL
					AND uf.billing_status IN ('pending','settling','settled','debt')
					AND uf.completed_at >= sep.starts_at
					AND uf.completed_at < sep.expires_at
					AND uf.user_id = sep.user_id
					AND CAST(sep.subscription_id AS TEXT) IN (
						CAST(json_extract(uf.payload, '$.billing_command.SubscriptionID') AS TEXT),
						CAST(json_extract(uf.payload, '$.billing_command.subscription_id') AS TEXT),
						CAST(json_extract(uf.payload, '$.usage_log.SubscriptionID') AS TEXT),
						CAST(json_extract(uf.payload, '$.usage_log.subscription_id') AS TEXT)
					)
			)`
	}
	err := client.Driver().Query(ctx, query, []any{periodID}, &rows)
	if err != nil {
		return false, err
	}
	defer rows.Close()
	if !rows.Next() {
		return false, rows.Err()
	}
	var exists bool
	if err := rows.Scan(&exists); err != nil {
		return false, err
	}
	return exists, nil
}

func (s *PaymentService) sumRefundLegacyUsageLogs(ctx context.Context, periodID int64) (float64, error) {
	var rows entsql.Rows
	err := s.entClient.Driver().Query(ctx, `SELECT COALESCE(SUM(ul.actual_cost), 0) FROM subscription_entitlement_periods sep LEFT JOIN usage_logs ul ON ul.subscription_id = sep.subscription_id AND ul.created_at >= sep.starts_at AND ul.created_at < sep.expires_at WHERE sep.id = $1`, []any{periodID}, &rows)
	if err != nil {
		return 0, err
	}
	defer rows.Close()
	if !rows.Next() {
		return 0, rows.Err()
	}
	var total float64
	if err := rows.Scan(&total); err != nil {
		return 0, err
	}
	return total, nil
}

func (q *SubscriptionRefundQuote) Basis() map[string]any {
	if q == nil {
		return nil
	}
	return map[string]any{"entitlement_period_id": q.EntitlementPeriodID, "period_total_quota_usd": q.PeriodTotalQuotaUSD, "used_quota_usd": q.UsedQuotaUSD, "usage_ratio": q.UsageRatio, "purchase_base_amount": q.PurchaseBaseAmount, "non_refundable_fee": q.NonRefundableFee, "calculated_at": q.CalculatedAt.UTC().Format(time.RFC3339Nano)}
}

func (s *PaymentService) persistSubscriptionRefundBasis(ctx context.Context, orderID int64, quote *SubscriptionRefundQuote) error {
	return s.persistSubscriptionRefundBasisWithClient(ctx, s.entClient, orderID, quote)
}

func (s *PaymentService) persistSubscriptionRefundBasisWithClient(ctx context.Context, client *dbent.Client, orderID int64, quote *SubscriptionRefundQuote) error {
	if quote == nil {
		return nil
	}
	if client == nil {
		return infraerrors.InternalServer("PAYMENT_REPOSITORY_UNAVAILABLE", "payment repository is unavailable")
	}
	_, err := client.PaymentOrder.UpdateOneID(orderID).SetRefundBasis(quote.Basis()).Save(ctx)
	return err
}

func (s *PaymentService) requireSubscriptionRefundQuote(ctx context.Context, o *dbent.PaymentOrder) (*SubscriptionRefundQuote, error) {
	return s.requireSubscriptionRefundQuoteWithClient(ctx, s.entClient, o, false)
}

func (s *PaymentService) requireSubscriptionRefundQuoteWithClient(ctx context.Context, client *dbent.Client, o *dbent.PaymentOrder, lockFacts bool) (*SubscriptionRefundQuote, error) {
	quote, err := s.calculateSubscriptionRefundQuoteWithClient(ctx, client, o, lockFacts)
	if err != nil {
		return nil, err
	}
	if quote.ManualReviewRequired {
		return nil, ErrRefundManualReviewRequired
	}
	if !quote.Eligible {
		return nil, infraerrors.BadRequest("NO_REFUNDABLE_QUOTA", "subscription quota has been fully used")
	}
	return quote, nil
}
