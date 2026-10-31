package schema

import (
	"entgo.io/contrib/entgql"
	"entgo.io/ent"
	"entgo.io/ent/dialect/entsql"
	"entgo.io/ent/schema/edge"
	"entgo.io/ent/schema/field"
)

// Forecast holds the schema definition for the Forecast entity.
type Forecast struct {
	ent.Schema
}

// Fields of the Forecast.
func (Forecast) Fields() []ent.Field {
	return []ent.Field{
		field.Time("from_time").Comment("The start time of the forecast period."),
		field.Time("to_time").Comment("The end time of the forecast period."),
		field.Enum("change_indicator").Nillable().Optional().Values("BECMG", "FM", "TEMPO", "PROB").Comment("The change indicator."),
		field.Time("change_time").Optional().Nillable().Comment("The time of the change."),
		field.Int("change_probability").Optional().Nillable().Comment("The probability of the change."),
		field.Int("wind_direction").Optional().Nillable().Comment("The wind direction in degrees."),
		field.Bool("wind_direction_variable").Comment("Whether the wind direction is variable (VRB)"),
		field.Int("wind_speed").Optional().Nillable().Comment("The wind speed in knots.").Annotations(entgql.Skip(entgql.SkipType)),
		field.Int("wind_gust").Optional().Nillable().Comment("The wind gust in knots.").Annotations(entgql.Skip(entgql.SkipType)),
		field.Int("wind_shear_height").Optional().Nillable().Comment("The height of the wind shear in feet above ground level.").Annotations(entgql.Skip(entgql.SkipType)),
		field.Int("wind_shear_direction").Optional().Nillable().Comment("The wind shear direction in degrees."),
		field.Int("wind_shear_speed").Optional().Nillable().Comment("The wind shear speed in knots.").Annotations(entgql.Skip(entgql.SkipType)),
		field.Float("visibility_horizontal").Optional().Nillable().Comment("The visibility in statute miles.").Annotations(entgql.Skip(entgql.SkipType)),
		field.Bool("visibility_horizontal_is_more_than").Comment("Whether the visibility is more than it's assigned value (+)"),
		field.Int("visibility_vertical").Optional().Nillable().Comment("The vertical visibility in feet.").Annotations(entgql.Skip(entgql.SkipType)),
		field.Float("altimeter").Optional().Nillable().Comment("The altimeter in inches of mercury.").Annotations(entgql.Skip(entgql.SkipType)),
		field.String("weather").Optional().Comment("The weather string."),
		field.String("not_decoded").Optional().Comment("The not decoded string."),
	}
}

// Edges of the Forecast.
func (Forecast) Edges() []ent.Edge {
	return []ent.Edge{
		edge.From("taf", Taf.Type).Ref("forecast").Unique().Annotations(entgql.Skip()),
		edge.To("sky_conditions", SkyCondition.Type).Comment("The sky conditions.").Annotations(entsql.Annotation{
			OnDelete: entsql.Cascade,
		}),
		edge.To("turbulence_conditions", TurbulenceCondition.Type).Comment("The turbulence conditions.").Annotations(
			entsql.Annotation{
				OnDelete: entsql.Cascade,
			}),
		edge.To("icing_conditions", IcingCondition.Type).Comment("The icing conditions.").Annotations(
			entsql.Annotation{
				OnDelete: entsql.Cascade,
			}),
		edge.To("temperature_data", TemperatureData.Type).Comment("The temperature data.").Annotations(
			entsql.Annotation{
				OnDelete: entsql.Cascade,
			}),
	}
}

// Mixin of the Forecast.
func (Forecast) Mixin() []ent.Mixin {
	return []ent.Mixin{
		IDMixin{},
	}
}
