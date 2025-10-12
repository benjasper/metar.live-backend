package graph

import (
	"metar.live/graph/model"
)

func TemperatureFromCelsiusToUnit(value float64, unit model.TemperatureUnit) float64 {
	switch unit {
	case model.TemperatureUnitFahrenheit:
		return value * 1.8
	default:
		// Celsius
		return value
	}
}

func PressureFromInchesOfMercuryToUnit(value float64, unit model.PressureUnit) float64 {
	switch unit {
	case model.PressureUnitInchOfMercury:
		return value
	default:
		// Hectopascals
		return value * 33.8637526
	}
}

func PressureFromHectopascalsToUnit(value float64, unit model.PressureUnit) float64 {
	switch unit {
	case model.PressureUnitInchOfMercury:
		return value / 33.8637526
	default:
		// Hectopascals
		return value
	}
}

func SpeedFromKnotsToUnit(value int, unit model.SpeedUnit) float64 {
	switch unit {
	case model.SpeedUnitKilometerPerHour:
		return float64(value) * 1.852
	default:
		// Knots
		return float64(value)
	}
}

func LengthFromStatuteMileToUnit(value float64, unit model.LengthUnit) float64 {
	switch unit {
	case model.LengthUnitStatuteMile:
		return value
	case model.LengthUnitNauticalMile:
		return value * 0.868976
	case model.LengthUnitMeter:
		return value * 1609.344
	case model.LengthUnitFoot:
		return value * 5280
	default:
		// Kilometers
		return value * 1.609344
	}
}

func LengthFromFeetToUnit(value float64, unit model.LengthUnit) float64 {
	return LengthFromStatuteMileToUnit(value/5280, unit)
}

func SmallLengthFromInchesToUnit(value float64, unit model.SmallLengthUnit) float64 {
	switch unit {
	case model.SmallLengthUnitInch:
		return value
	default:
		// Centimeters
		return value * 2.54
	}
}
