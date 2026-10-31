package importer

import (
	"context"
	"fmt"
	"time"

	"entgo.io/ent/dialect/sql"
	"github.com/google/uuid"
	"metar.live/ent/metar"
)

const predictionMaxDifference = 12 * time.Hour

type metarTimes struct {
	StationID       uuid.UUID `sql:"weather_station_metars"`
	ImportTime      time.Time `sql:"import_time"`
	ObservationTime time.Time `sql:"observation_time"`
}

// latestMetarTimes returns the import and observation time of the most recently imported METAR per station.
func (i *NoaaWeatherImporter) latestMetarTimes(ctx context.Context, stationIDs []uuid.UUID, importTime time.Time) (map[uuid.UUID]metarTimes, error) {
	latest := make(map[uuid.UUID]metarTimes, len(stationIDs))
	if len(stationIDs) == 0 {
		return latest, nil
	}

	ids := make([]any, len(stationIDs))
	for idx, id := range stationIDs {
		ids[idx] = id
	}

	var rows []metarTimes
	err := i.db.Metar.Query().
		Modify(func(s *sql.Selector) {
			s.Select(s.C(metar.StationColumn), s.C(metar.FieldImportTime), s.C(metar.FieldObservationTime)).
				Where(sql.And(
					sql.In(s.C(metar.StationColumn), ids...),
					sql.GT(s.C(metar.FieldImportTime), importTime.Add(-predictionMaxDifference)),
				))
		}).
		Scan(ctx, &rows)
	if err != nil {
		return nil, err
	}

	for _, row := range rows {
		if current, ok := latest[row.StationID]; !ok || row.ImportTime.After(current.ImportTime) {
			latest[row.StationID] = row
		}
	}

	return latest, nil
}

func predictNextImport(last metarTimes, found bool, currentImportTime time.Time, currentObservationTime time.Time) (*time.Time, error) {
	if !found {
		return nil, fmt.Errorf("no METAR imported in the last %s", predictionMaxDifference)
	}

	importTimeDifference := currentImportTime.Sub(last.ImportTime)
	if importTimeDifference.Abs() > predictionMaxDifference {
		return nil, fmt.Errorf("last import was %s ago, which is more than 12 hours", importTimeDifference.String())
	}

	observationTimeDifference := currentObservationTime.Sub(last.ObservationTime)
	if observationTimeDifference.Abs() > predictionMaxDifference {
		return nil, fmt.Errorf("last observation was %s ago, which is more than 12 hours", observationTimeDifference.String())
	}

	averageDifference := (importTimeDifference + observationTimeDifference) / 2

	nextImportTime := currentImportTime.Add(averageDifference)

	return &nextImportTime, nil
}
