package schema

import (
	"time"

	"entgo.io/ent"
	"entgo.io/ent/dialect"
	"entgo.io/ent/dialect/entsql"
	"entgo.io/ent/schema"
	"entgo.io/ent/schema/edge"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"
)

// UserBalancePackage 保存已支付套餐的不可变到账快照。
// payment_order_id 唯一约束让支付回调重试无法重复给首次余额到账。
type UserBalancePackage struct {
	ent.Schema
}

func (UserBalancePackage) Annotations() []schema.Annotation {
	return []schema.Annotation{entsql.Annotation{Table: "user_balance_packages"}}
}

func (UserBalancePackage) Fields() []ent.Field {
	return []ent.Field{
		field.Int64("user_id"),
		field.Int64("plan_id"),
		field.Int64("payment_order_id").Unique(),
		field.Float("weekly_credit_usd").SchemaType(map[string]string{dialect.Postgres: "decimal(20,8)"}),
		field.Int("credited_count").Default(0),
		field.Int("refresh_count"),
		field.Int("refresh_interval_days"),
		field.Time("starts_at").SchemaType(map[string]string{dialect.Postgres: "timestamptz"}),
		field.Time("next_credit_at").Optional().Nillable().SchemaType(map[string]string{dialect.Postgres: "timestamptz"}),
		field.Time("expires_at").SchemaType(map[string]string{dialect.Postgres: "timestamptz"}),
		field.String("status").MaxLen(20).Default("active"),
		field.Time("created_at").Immutable().Default(time.Now).SchemaType(map[string]string{dialect.Postgres: "timestamptz"}),
		field.Time("updated_at").Default(time.Now).UpdateDefault(time.Now).SchemaType(map[string]string{dialect.Postgres: "timestamptz"}),
	}
}

func (UserBalancePackage) Edges() []ent.Edge {
	return []ent.Edge{
		edge.From("user", User.Type).Ref("balance_packages").Field("user_id").Unique().Required(),
	}
}

func (UserBalancePackage) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("user_id", "status"),
		index.Fields("status", "next_credit_at"),
	}
}
