package schema

import (
	"entgo.io/contrib/entgql"
	"entgo.io/ent"
	"entgo.io/ent/schema/edge"
	"entgo.io/ent/schema/field"
)

// TurbulenceCondition holds the schema definition for the TurbulenceCondition entity.
type TurbulenceCondition struct {
	ent.Schema
}

// Fields of the TurbulenceCondition.
func (TurbulenceCondition) Fields() []ent.Field {
	return []ent.Field{
		field.String("intensity").Comment("The intensity of the turbulence."),
		field.Int("min_altitude").Comment("The minimum altitude in feet that the turbulence is present.").Annotations(entgql.Skip(entgql.SkipType)),
		field.Int("max_altitude").Comment("The maximum altitude in feet that the turbulence is present.").Annotations(entgql.Skip(entgql.SkipType)),
	}
}

// Edges of the TurbulenceCondition.
func (TurbulenceCondition) Edges() []ent.Edge {
	return []ent.Edge{
		edge.From("forecast", Forecast.Type).Ref("turbulence_conditions").Unique().Annotations(entgql.Skip()),
	}
}

// Mixin of the TurbulenceCondition.
func (TurbulenceCondition) Mixin() []ent.Mixin {
	return []ent.Mixin{
		IDMixin{},
	}
}
