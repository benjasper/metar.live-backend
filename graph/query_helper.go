package graph

import (
	"fmt"
	"math"
	"metar.live/utils"
)

const R = 6371.0 // Radius of the earth in kilometers.

func GetMinMaxLatLongForGeoQuery(latitude float64, longitude float64, distance float64) (float64, float64, float64, float64) {
	// Convert the distance to radians.
	radius := distance / 6371.0

	// Calculate the min/max lat/long.
	maxLat := latitude + utils.RadiansToDegrees(radius)
	minLat := latitude - utils.RadiansToDegrees(radius)
	maxLon := longitude + utils.RadiansToDegrees(math.Asin(radius)/math.Cos(utils.DegreesToRadians(longitude)))
	minLon := longitude - utils.RadiansToDegrees(math.Asin(radius)/math.Cos(utils.DegreesToRadians(latitude)))

	return minLat, maxLat, minLon, maxLon
}

// GetGeoQuerySQL This query returns the distance in kilometers.
func GetGeoQuerySQL(latitude float64, longitude float64, latitudeField string, longitudeField string) string {
	return fmt.Sprintf("acos(sin(radians(%f)) * sin(radians(%s)) + cos(radians(%f)) * cos(radians(%s)) * cos(radians(%s) - radians(%f))) * %f", latitude, latitudeField, latitude, latitudeField, longitudeField, longitude, R)
}

// BoundsForPagination for first and last arguments
func BoundsForPagination(first *int, last *int) (*int, *int) {
	if first == nil && last == nil {
		first = Pointer(5)
		return first, last
	}

	if first != nil && *first < 0 {
		first = Pointer(5)
	}

	if last != nil && *last < 0 {
		last = Pointer(5)
	}

	if first != nil && *first > 10 {
		first = Pointer(10)
	}

	if last != nil && *last > 10 {
		last = Pointer(10)
	}

	return first, last
}

func Pointer[T any](v T) *T {
	return &v
}
