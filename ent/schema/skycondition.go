package schema

import (
	"entgo.io/contrib/entgql"
	"entgo.io/ent"
	"entgo.io/ent/schema/edge"
	"entgo.io/ent/schema/field"
)

// SkyCondition holds the schema definition for the SkyCondition entity.
type SkyCondition struct {
	ent.Schema
}

// Fields of the SkyCondition.
func (SkyCondition) Fields() []ent.Field {
	return []ent.Field{
		field.Enum("sky_cover").NamedValues("Sky Clear", "SKC", "Few", "FEW", "Scattered", "SCT", "Clear", "CLR", "No significant clouds", "NSC", "Broken", "BKN", "Overcast", "OVC", "Overcast X", "OVCX", "Vertical Visibility", "OVX", "Ceiling and Visibility OK", "CAVOK"),
		field.Int("cloud_base").Optional().Nillable().Comment("Cloud base in feet.").Annotations(entgql.Skip(entgql.SkipType)),
		field.Enum("cloud_type").Optional().Nillable().NamedValues("Cumulonimbus", "CB", "Cumulus", "CU", "Towering Cumulus", "TCU").Comment("Cloud type. Only present in TAFs."),
	}
}

// Edges of the SkyCondition.
func (SkyCondition) Edges() []ent.Edge {
	return []ent.Edge{
		edge.From("metar", Metar.Type).Ref("sky_conditions").Unique().Annotations(entgql.Skip()),
		edge.From("forecast", Forecast.Type).Ref("sky_conditions").Unique().Annotations(entgql.Skip()),
	}
}

// Mixin of the SkyCondition.
func (SkyCondition) Mixin() []ent.Mixin {
	return []ent.Mixin{
		IDMixin{},
	}
}
