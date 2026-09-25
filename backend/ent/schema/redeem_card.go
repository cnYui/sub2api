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

// RedeemCard 是管理员为兑换码制作的分享卡片，用户通过 /card/<token> 打开看到 3D 兑换卡。
//
// token 随机生成、与兑换码无关：兑换码不会出现在链接和访问日志里，撤销链接也只删卡片、不动兑换码。
// 一个兑换码最多一张卡，重新编辑只改卡面，链接保持不变。
// 「主理人」信息和二维码是全站共用的，存在 settings（redeem_card_profile）而不是每张卡里，
// 这样换一次微信群二维码（约 7 天过期），所有已发出的卡片都会显示新的。
type RedeemCard struct {
	ent.Schema
}

func (RedeemCard) Annotations() []schema.Annotation {
	return []schema.Annotation{
		entsql.Annotation{Table: "redeem_cards"},
	}
}

func (RedeemCard) Fields() []ent.Field {
	return []ent.Field{
		field.String("token").
			MaxLen(64).
			NotEmpty().
			Unique(),
		field.Int64("redeem_code_id").
			Unique(),
		// dark / light，对应站长设计的黑色版与白色版卡面。
		field.String("theme").
			MaxLen(16).
			Default("dark"),
		// 卡面上与兑换码相关的文字：面值、套餐、用量、有效期、编号、热力图种子。
		field.JSON("content", map[string]any{}).
			Default(func() map[string]any { return map[string]any{} }).
			SchemaType(map[string]string{dialect.Postgres: "jsonb"}),
		field.Int64("created_by").
			Optional().
			Nillable(),
		field.Int("view_count").
			Default(0),
		field.Time("last_viewed_at").
			Optional().
			Nillable().
			SchemaType(map[string]string{dialect.Postgres: "timestamptz"}),
		field.Time("created_at").
			Immutable().
			Default(time.Now).
			SchemaType(map[string]string{dialect.Postgres: "timestamptz"}),
		// 只在卡面内容变化时更新；刻意不用 UpdateDefault，否则每次有人打开卡片累加浏览数都会改掉它。
		field.Time("updated_at").
			Default(time.Now).
			SchemaType(map[string]string{dialect.Postgres: "timestamptz"}),
	}
}

func (RedeemCard) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("created_at"),
	}
}
