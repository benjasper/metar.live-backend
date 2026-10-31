package schema

import (
	"entgo.io/contrib/entgql"
	"entgo.io/ent"
	"entgo.io/ent/schema/edge"
	"entgo.io/ent/schema/field"
)

// TemperatureData holds the schema definition for the TemperatureData entity.
type TemperatureData struct {
	ent.Schema
}

// Fields of the TemperatureData.
func (TemperatureData) Fields() []ent.Field {
	return []ent.Field{
		field.Time("valid_time").Comment("The time the temperature data is valid."),
		field.Float("temperature").Comment("The surface temperature in degrees Celsius.").Annotations(entgql.Skip(entgql.SkipType)),
		field.Float("min_temperature").Optional().Nillable().Comment("The minimum temperature in degrees Celsius.").Annotations(entgql.Skip(entgql.SkipType)),
		field.Float("max_temperature").Optional().Nillable().Comment("The maximum temperature in degrees Celsius.").Annotations(entgql.Skip(entgql.SkipType)),
	}
}

// Edges of the TemperatureData.
func (TemperatureData) Edges() []ent.Edge {
	return []ent.Edge{
		edge.From("forecast", Forecast.Type).Ref("temperature_data").Unique().Annotations(entgql.Skip()),
	}
}

// Mixin of the TemperatureData.
func (TemperatureData) Mixin() []ent.Mixin {
	return []ent.Mixin{
		IDMixin{},
	}
}
