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

// SubscriptionEntitlementPeriod 保存一次不可变的套餐权益周期和当时的额度快照。
type SubscriptionEntitlementPeriod struct {
	ent.Schema
}

func (SubscriptionEntitlementPeriod) Annotations() []schema.Annotation {
	return []schema.Annotation{
		entsql.Annotation{
			Table: "subscription_entitlement_periods",
			Checks: map[string]string{
				"subscription_entitlement_periods_days_check":              "period_days > 0",
				"subscription_entitlement_periods_range_check":             "expires_at > starts_at",
				"subscription_entitlement_periods_limit_check":             "daily_limit_usd IS NULL OR daily_limit_usd >= 0",
				"subscription_entitlement_periods_weekly_limit_check":      "weekly_limit_usd IS NULL OR weekly_limit_usd >= 0",
				"subscription_entitlement_periods_total_quota_check":       "period_total_quota_usd IS NULL OR period_total_quota_usd >= 0",
				"subscription_entitlement_periods_quota_window_unit_check": "quota_window_unit IN ('day', 'week', 'month', 'none')",
				"subscription_entitlement_periods_quota_window_days_check": "quota_window_days > 0",
				"subscription_entitlement_periods_status_check":            "status IN ('active', 'revoked')",
			},
		},
	}
}

func (SubscriptionEntitlementPeriod) Mixin() []ent.Mixin {
	return []ent.Mixin{
		mixins.TimeMixin{},
	}
}

func (SubscriptionEntitlementPeriod) Fields() []ent.Field {
	return []ent.Field{
		field.Int64("user_id"),
		field.Int64("subscription_id"),
		field.Int64("group_id"),
		field.String("source_type").MaxLen(40).NotEmpty(),
		field.String("source_id").MaxLen(128).NotEmpty(),
		field.Time("starts_at").
			SchemaType(map[string]string{dialect.Postgres: "timestamptz"}),
		field.Time("expires_at").
			SchemaType(map[string]string{dialect.Postgres: "timestamptz"}),
		field.Time("quota_window_anchor_at").
			Optional().
			Nillable().
			SchemaType(map[string]string{dialect.Postgres: "timestamptz"}),
		field.Int("period_days"),
		field.Float("daily_limit_usd").
			Optional().
			Nillable().
			SchemaType(map[string]string{dialect.Postgres: "numeric(20,10)"}),
		field.Float("weekly_limit_usd").
			Optional().
			Nillable().
			SchemaType(map[string]string{dialect.Postgres: "numeric(20,10)"}),
		field.Float("period_total_quota_usd").
			Optional().
			Nillable().
			SchemaType(map[string]string{dialect.Postgres: "numeric(20,10)"}),
		field.String("quota_window_unit").MaxLen(20).Default("day"),
		field.Int("quota_window_days").Default(1),
		field.String("status").MaxLen(20).Default("active"),
		field.Time("revoked_at").
			Optional().
			Nillable().
			SchemaType(map[string]string{dialect.Postgres: "timestamptz"}),
		field.String("revoked_reason").
			SchemaType(map[string]string{dialect.Postgres: "text"}).
			Default(""),
	}
}

func (SubscriptionEntitlementPeriod) Edges() []ent.Edge {
	return []ent.Edge{
		edge.From("user", User.Type).
			Ref("subscription_entitlement_periods").
			Field("user_id").
			Unique().
			Required(),
		edge.From("subscription", UserSubscription.Type).
			Ref("entitlement_periods").
			Field("subscription_id").
			Unique().
			Required(),
		edge.From("group", Group.Type).
			Ref("subscription_entitlement_periods").
			Field("group_id").
			Unique().
			Required(),
	}
}

func (SubscriptionEntitlementPeriod) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("source_type", "source_id").
			Unique().
			StorageKey("idx_subscription_entitlement_periods_source"),
		index.Fields("user_id", "expires_at", "starts_at").
			StorageKey("idx_subscription_entitlement_periods_active_user_expiry").
			Annotations(
				entsql.IndexWhere("status = 'active'"),
				entsql.DescColumns("starts_at"),
			),
	}
}
