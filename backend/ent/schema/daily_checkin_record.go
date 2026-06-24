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

// DailyCheckinRecord holds the schema definition for a user's daily check-in.
//
// 删除策略：硬删除
// 签到记录是余额奖励流水的业务来源，默认作为追加型历史记录保存；如需清理历史数据，
// 可以按时间范围批量删除，无需软删除状态参与查询。
type DailyCheckinRecord struct {
	ent.Schema
}

func (DailyCheckinRecord) Annotations() []schema.Annotation {
	return []schema.Annotation{
		entsql.Annotation{Table: "daily_checkin_records"},
	}
}

func (DailyCheckinRecord) Mixin() []ent.Mixin {
	return []ent.Mixin{
		mixins.TimeMixin{},
	}
}

func (DailyCheckinRecord) Fields() []ent.Field {
	return []ent.Field{
		field.Int64("user_id"),
		field.Time("checkin_date").
			SchemaType(map[string]string{dialect.Postgres: "date"}).
			Comment("Check-in date in the user's selected timezone"),
		field.String("timezone").
			MaxLen(64).
			Default("UTC").
			Comment("Timezone used to derive checkin_date"),
		field.Float("base_reward").
			SchemaType(map[string]string{dialect.Postgres: "decimal(20,8)"}).
			Default(0).
			Comment("Base reward credited for this check-in"),
		field.Float("bonus_reward").
			SchemaType(map[string]string{dialect.Postgres: "decimal(20,8)"}).
			Default(0).
			Comment("Extra streak reward credited for this check-in"),
		field.Float("total_reward").
			SchemaType(map[string]string{dialect.Postgres: "decimal(20,8)"}).
			Default(0).
			Comment("Total reward credited for this check-in"),
		field.Int("streak_days").
			Default(1).
			Comment("Consecutive check-in days after this check-in"),
	}
}

func (DailyCheckinRecord) Edges() []ent.Edge {
	return []ent.Edge{
		edge.From("user", User.Type).
			Ref("daily_checkin_records").
			Field("user_id").
			Unique().
			Required(),
	}
}

func (DailyCheckinRecord) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("user_id", "checkin_date").Unique(),
		index.Fields("user_id"),
		index.Fields("checkin_date"),
		index.Fields("created_at"),
	}
}
