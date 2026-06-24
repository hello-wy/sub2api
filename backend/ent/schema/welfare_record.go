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

// WelfareRecord holds the schema definition for the WelfareRecord entity.
type WelfareRecord struct {
	ent.Schema
}

func (WelfareRecord) Annotations() []schema.Annotation {
	return []schema.Annotation{
		entsql.Annotation{Table: "welfare_records"},
	}
}

func (WelfareRecord) Mixin() []ent.Mixin {
	return []ent.Mixin{
		mixins.TimeMixin{},
	}
}

func (WelfareRecord) Fields() []ent.Field {
	return []ent.Field{
		field.Int64("user_id").
			Comment("User ID"),
		field.String("user_email").
			MaxLen(255).
			Comment("User Email"),
		field.Float("amount").
			SchemaType(map[string]string{dialect.Postgres: "decimal(20,8)"}).
			Comment("Welfare Bonus Amount"),
		field.String("remarks").
			SchemaType(map[string]string{dialect.Postgres: "text"}).
			Default("").
			Comment("Remarks (template: Date + Leaderboard spending + #Rank)"),
		field.String("status").
			MaxLen(30).
			Default("success").
			Comment("Status (success / revoked)"),
	}
}

func (WelfareRecord) Edges() []ent.Edge {
	return []ent.Edge{
		edge.From("user", User.Type).
			Ref("welfare_records").
			Field("user_id").
			Unique().
			Required(),
	}
}

func (WelfareRecord) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("user_id"),
		index.Fields("user_email"),
		index.Fields("status"),
		index.Fields("created_at"),
		index.Fields("remarks").
			Unique().
			Annotations(entsql.IndexWhere("status = 'success'")),
	}
}
