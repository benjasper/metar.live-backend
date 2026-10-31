package schema

import (
	"entgo.io/contrib/entgql"
	"entgo.io/ent"
	"entgo.io/ent/schema/edge"
	"entgo.io/ent/schema/field"
)

// IcingCondition holds the schema definition for the IcingCondition entity.
type IcingCondition struct {
	ent.Schema
}

// Fields of the IcingCondition.
func (IcingCondition) Fields() []ent.Field {
	return []ent.Field{
		field.String("intensity").Comment("The intensity of the icing."),
		field.Int("min_altitude").Optional().Nillable().Comment("The minimum altitude in feet that the icing is present.").Annotations(entgql.Skip(entgql.SkipType)),
		field.Int("max_altitude").Optional().Nillable().Comment("The maximum altitude in feet that the icing is present.").Annotations(entgql.Skip(entgql.SkipType)),
	}
}

// Edges of the IcingCondition.
func (IcingCondition) Edges() []ent.Edge {
	return []ent.Edge{
		edge.From("forecast", Forecast.Type).Ref("icing_conditions").Unique().Annotations(entgql.Skip()),
	}
}

// Mixin of the IcingCondition.
func (IcingCondition) Mixin() []ent.Mixin {
	return []ent.Mixin{
		IDMixin{},
	}
}
