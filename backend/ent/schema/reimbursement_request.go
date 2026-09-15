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

// ReimbursementRequest 保存用户提交的报销/开票信息申请。
//
// 用户粘贴的纯文本经 LLM 解析成 6 个结构化字段后才允许入库，所以这里的
// 业务字段全部 NOT NULL；raw_text 保留原文供管理员核对解析结果。
// 管理员上传发票 PDF 后状态从 pending 变为 completed，PDF 本体落在数据目录，
// 数据库只记录相对路径与摘要信息。
//
// 删除策略：硬删除（随用户级联），申请本身没有需要长期追溯的资金语义。
type ReimbursementRequest struct {
	ent.Schema
}

func (ReimbursementRequest) Annotations() []schema.Annotation {
	return []schema.Annotation{
		entsql.Annotation{Table: "reimbursement_requests"},
	}
}

func (ReimbursementRequest) Fields() []ent.Field {
	return []ent.Field{
		field.Int64("user_id"),
		field.String("raw_text").
			SchemaType(map[string]string{dialect.Postgres: "text"}).
			Default(""),
		field.String("company_name").
			MaxLen(255),
		field.String("tax_id").
			MaxLen(64),
		field.String("bank_account").
			MaxLen(64),
		field.String("bank_name").
			MaxLen(255),
		field.String("address").
			MaxLen(500),
		field.Float("amount").
			SchemaType(map[string]string{dialect.Postgres: "decimal(20,2)"}),
		field.String("status").
			MaxLen(20).
			Default("pending"),
		field.String("pdf_path").
			MaxLen(500).
			Default(""),
		field.String("pdf_file_name").
			MaxLen(255).
			Default(""),
		field.Int64("pdf_size").
			Default(0),
		field.String("pdf_sha256").
			MaxLen(64).
			Default(""),
		field.Time("pdf_uploaded_at").
			Optional().
			Nillable().
			SchemaType(map[string]string{dialect.Postgres: "timestamptz"}),
		field.Int64("handled_by").
			Optional().
			Nillable(),
		field.Time("completed_at").
			Optional().
			Nillable().
			SchemaType(map[string]string{dialect.Postgres: "timestamptz"}),
		field.Time("notified_at").
			Optional().
			Nillable().
			SchemaType(map[string]string{dialect.Postgres: "timestamptz"}),
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

func (ReimbursementRequest) Edges() []ent.Edge {
	return []ent.Edge{
		edge.From("user", User.Type).
			Ref("reimbursement_requests").
			Field("user_id").
			Unique().
			Required(),
	}
}

func (ReimbursementRequest) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("user_id", "created_at"),
		index.Fields("status", "created_at"),
	}
}
