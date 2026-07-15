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

// PaymentBalanceHold holds the schema definition for reserved user balance in mixed payments.
type PaymentBalanceHold struct {
	ent.Schema
}

func (PaymentBalanceHold) Annotations() []schema.Annotation {
	return []schema.Annotation{
		entsql.Annotation{Table: "payment_balance_holds"},
	}
}

func (PaymentBalanceHold) Fields() []ent.Field {
	return []ent.Field{
		field.Int64("order_id"),
		field.Int64("user_id"),
		field.Float("amount").
			SchemaType(map[string]string{dialect.Postgres: "decimal(20,2)"}),
		field.String("status").
			MaxLen(20).
			Default("RESERVED"),
		field.Time("expires_at").
			SchemaType(map[string]string{dialect.Postgres: "timestamptz"}),
		field.Time("captured_at").
			Optional().
			Nillable().
			SchemaType(map[string]string{dialect.Postgres: "timestamptz"}),
		field.Time("released_at").
			Optional().
			Nillable().
			SchemaType(map[string]string{dialect.Postgres: "timestamptz"}),
		field.String("release_reason").
			Optional().
			Nillable().
			SchemaType(map[string]string{dialect.Postgres: "text"}),
		field.Time("created_at").
			Immutable().
			Default(time.Now).
			SchemaType(map[string]string{dialect.Postgres: "timestamptz"}),
		field.Time("updated_at").
			Default(time.Now).
			UpdateDefault(time.Now).
			SchemaType(map[string]string{dialect.Postgres: "timestamptz"}),
	}
}

func (PaymentBalanceHold) Edges() []ent.Edge {
	return []ent.Edge{
		edge.From("order", PaymentOrder.Type).
			Ref("balance_hold").
			Field("order_id").
			Unique().
			Required(),
		edge.From("user", User.Type).
			Ref("payment_balance_holds").
			Field("user_id").
			Unique().
			Required(),
	}
}

func (PaymentBalanceHold) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("order_id").
			Unique(),
		index.Fields("user_id"),
		index.Fields("status"),
		index.Fields("expires_at"),
	}
}
