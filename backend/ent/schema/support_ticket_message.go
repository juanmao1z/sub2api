package schema

import (
	"entgo.io/ent"
	"entgo.io/ent/dialect"
	"entgo.io/ent/dialect/entsql"
	"entgo.io/ent/schema"
	"entgo.io/ent/schema/field"
	"time"
)

// SupportTicketMessage stores a reply in a support ticket conversation.
type SupportTicketMessage struct{ ent.Schema }

func (SupportTicketMessage) Annotations() []schema.Annotation {
	return []schema.Annotation{entsql.Annotation{Table: "support_ticket_messages"}}
}

func (SupportTicketMessage) Fields() []ent.Field {
	return []ent.Field{
		field.Int64("ticket_id"),
		field.Int64("sender_id"),
		field.String("sender_type").MaxLen(20),
		field.String("body").SchemaType(map[string]string{dialect.Postgres: "text"}),
		field.Time("created_at").Immutable().Default(time.Now).SchemaType(map[string]string{dialect.Postgres: "timestamptz"}),
	}
}
