package importer

import (
	"context"
	"fmt"
	"metar.live/ent"
	"metar.live/ent/metar"
	"metar.live/ent/weatherstation"
	"time"
)

func (i *NoaaWeatherImporter) MakeNextImportPrediction(ctx context.Context, stationID string, currentImportTime *time.Time, currentObservationTime *time.Time) (*time.Time, error) {

	// Query the last metar for this station
	m, err := i.db.Metar.Query().
		Where(
			metar.HasStationWith(weatherstation.StationID(stationID)),
		).
		Order(ent.Desc(metar.FieldImportTime)).First(ctx)

	if err != nil {
		return nil, err
	}

	importTimeDifference := currentImportTime.Sub(m.ImportTime)
	if importTimeDifference.Abs() > time.Hour*12 {
		return nil, fmt.Errorf("last import for station %s was %s ago, which is more than 12 hours", stationID, importTimeDifference.String())
	}

	observationTimeDifference := currentObservationTime.Sub(m.ObservationTime)
	if observationTimeDifference.Abs() > time.Hour*12 {
		return nil, fmt.Errorf("last observation for station %s was %s ago, which is more than 12 hours", stationID, observationTimeDifference.String())
	}

	averageDifference := (importTimeDifference + observationTimeDifference) / 2

	nextImportTime := currentImportTime.Add(averageDifference)

	return &nextImportTime, nil
}
