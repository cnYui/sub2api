package schema

import (
	"github.com/Wei-Shaw/sub2api/ent/schema/mixins"

	"entgo.io/ent"
	"entgo.io/ent/dialect"
	"entgo.io/ent/dialect/entsql"
	"entgo.io/ent/schema"
	"entgo.io/ent/schema/edge"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"
)

// SubscriptionQuotaDebtAdjustment 保存订阅额度切换或人工审计产生的有效期抵扣事实。
type SubscriptionQuotaDebtAdjustment struct {
	ent.Schema
}

func (SubscriptionQuotaDebtAdjustment) Annotations() []schema.Annotation {
	return []schema.Annotation{
		entsql.Annotation{
			Table: "subscription_quota_debt_adjustments",
			Checks: map[string]string{
				"subscription_quota_debt_adjustments_status_check": "application_status IN ('pending', 'applied', 'already_applied', 'manual_review')",
				"subscription_quota_debt_adjustments_days_check":   "deducted_days >= 0",
			},
		},
	}
}

func (SubscriptionQuotaDebtAdjustment) Mixin() []ent.Mixin {
	return []ent.Mixin{
		mixins.TimeMixin{},
	}
}

func (SubscriptionQuotaDebtAdjustment) Fields() []ent.Field {
	return []ent.Field{
		field.Int64("subscription_id"),
		field.Int64("user_id"),
		field.Int64("group_id"),
		field.String("source_key").
			MaxLen(160).
			NotEmpty().
			Unique(),
		field.Float("overage_usd").
			SchemaType(map[string]string{dialect.Postgres: "numeric(20,10)"}),
		field.Float("weekly_limit_usd").
			SchemaType(map[string]string{dialect.Postgres: "numeric(20,10)"}),
		field.Float("daily_equivalent_usd").
			SchemaType(map[string]string{dialect.Postgres: "numeric(20,10)"}),
		field.Float("raw_deduction_days").
			SchemaType(map[string]string{dialect.Postgres: "numeric(20,10)"}),
		field.Int("deducted_days"),
		field.Time("original_expires_at").
			SchemaType(map[string]string{dialect.Postgres: "timestamptz"}),
		field.Time("new_expires_at").
			SchemaType(map[string]string{dialect.Postgres: "timestamptz"}),
		field.String("application_status").
			MaxLen(32),
		field.Time("applied_at").
			Optional().
			Nillable().
			SchemaType(map[string]string{dialect.Postgres: "timestamptz"}),
		field.String("notes").
			SchemaType(map[string]string{dialect.Postgres: "text"}).
			Default(""),
	}
}

func (SubscriptionQuotaDebtAdjustment) Edges() []ent.Edge {
	return []ent.Edge{
		edge.From("subscription", UserSubscription.Type).
			Ref("quota_debt_adjustments").
			Field("subscription_id").
			Unique().
			Required(),
		edge.From("user", User.Type).
			Ref("subscription_quota_debt_adjustments").
			Field("user_id").
			Unique().
			Required(),
		edge.From("group", Group.Type).
			Ref("subscription_quota_debt_adjustments").
			Field("group_id").
			Unique().
			Required(),
	}
}

func (SubscriptionQuotaDebtAdjustment) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("subscription_id", "created_at").
			StorageKey("idx_subscription_quota_debt_adjustments_subscription").
			Annotations(entsql.DescColumns("created_at")),
	}
}
