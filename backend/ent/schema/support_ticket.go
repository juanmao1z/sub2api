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

// SupportTicket stores a user support request (refund or suggestion).
type SupportTicket struct{ ent.Schema }

func (SupportTicket) Annotations() []schema.Annotation {
	return []schema.Annotation{entsql.Annotation{Table: "support_tickets"}}
}

func (SupportTicket) Fields() []ent.Field {
	return []ent.Field{
		field.Int64("user_id"),
		field.String("type").MaxLen(20),
		field.String("status").MaxLen(20).Default("OPEN"),
		field.String("subject").MaxLen(200),
		field.String("description").SchemaType(map[string]string{dialect.Postgres: "text"}),
		field.Int64("order_id").Optional().Nillable(),
		field.Time("created_at").Immutable().Default(time.Now).SchemaType(map[string]string{dialect.Postgres: "timestamptz"}),
		field.Time("updated_at").Default(time.Now).UpdateDefault(time.Now).SchemaType(map[string]string{dialect.Postgres: "timestamptz"}),
	}
}

func (SupportTicket) Indexes() []ent.Index {
	return []ent.Index{index.Fields("user_id"), index.Fields("status"), index.Fields("type"), index.Fields("order_id"), index.Fields("created_at")}
}
