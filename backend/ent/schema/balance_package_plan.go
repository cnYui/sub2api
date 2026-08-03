package schema

import (
	"time"

	"entgo.io/ent"
	"entgo.io/ent/dialect"
	"entgo.io/ent/dialect/entsql"
	"entgo.io/ent/schema"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"
)

// BalancePackagePlan 定义以人民币购买、按周到账美元余额的套餐。
// 它独立于模型分组，避免购买行为改变 API Key 的可用渠道。
type BalancePackagePlan struct {
	ent.Schema
}

func (BalancePackagePlan) Annotations() []schema.Annotation {
	return []schema.Annotation{entsql.Annotation{Table: "balance_package_plans"}}
}

func (BalancePackagePlan) Fields() []ent.Field {
	return []ent.Field{
		field.String("code").MaxLen(64).NotEmpty().Unique(),
		field.String("name").MaxLen(100).NotEmpty(),
		field.Float("price_cny").SchemaType(map[string]string{dialect.Postgres: "decimal(20,2)"}),
		field.Float("weekly_credit_usd").SchemaType(map[string]string{dialect.Postgres: "decimal(20,8)"}),
		field.Int("validity_days").Default(28),
		field.Int("refresh_count").Default(4),
		field.Int("refresh_interval_days").Default(7),
		field.Bool("for_sale").Default(true),
		field.Int("sort_order").Default(0),
		field.Time("created_at").Immutable().Default(time.Now).SchemaType(map[string]string{dialect.Postgres: "timestamptz"}),
		field.Time("updated_at").Default(time.Now).UpdateDefault(time.Now).SchemaType(map[string]string{dialect.Postgres: "timestamptz"}),
	}
}

func (BalancePackagePlan) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("for_sale", "sort_order"),
	}
}
