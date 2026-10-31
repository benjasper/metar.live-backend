package importer

import (
	"bytes"
	"compress/gzip"
	"context"
	"encoding/xml"
	"fmt"
	"io"
	"strconv"
	"strings"
	"time"

	"entgo.io/ent/dialect/sql"
	"github.com/google/uuid"
	"github.com/segmentio/fasthash/fnv1a"
	"metar.live/ent"
	"metar.live/ent/airport"
	"metar.live/ent/forecast"
	"metar.live/ent/metar"
	"metar.live/ent/skycondition"
	"metar.live/ent/taf"
	"metar.live/ent/weatherstation"
	"metar.live/logging"
	"metar.live/utils"
)

const (
	// Number of feed records that are processed together.
	weatherImportChunkSize = 500
	// Keeps bulk inserts well below MySQL's limit of 65535 placeholders per statement.
	insertBatchSize = 500
)

type NoaaWeatherImporter struct {
	db     *ent.Client
	logger *logging.Logger
	stats  *ImportStatistics
}

func NewNoaaWeatherImporter(db *ent.Client, logger *logging.Logger) *NoaaWeatherImporter {
	return &NoaaWeatherImporter{
		db:     db,
		logger: logger,
	}
}

func (i *NoaaWeatherImporter) ImportMetars(url string, ctx context.Context) error {
	i.stats = NewImportStatistics("METAR", i.logger)
	i.stats.Start()

	err := importFeed(ctx, url, "METAR", i.importMetarChunk)
	if err != nil {
		return err
	}

	i.stats.End()

	return nil
}

func (i *NoaaWeatherImporter) ImportTafs(url string, ctx context.Context) error {
	i.stats = NewImportStatistics("TAF", i.logger)
	i.stats.Start()

	err := importFeed(ctx, url, "TAF", i.importTafChunk)
	if err != nil {
		return err
	}

	i.stats.End()

	return nil
}

// importFeed downloads a gzipped XML feed and streams every element with the given name to process in chunks.
// The chunk passed to process is reused afterward, so it must not be retained.
func importFeed[T any](ctx context.Context, url string, element string, process func(ctx context.Context, chunk []T) error) error {
	downloadCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	data, err := utils.Download(downloadCtx, url)
	if err != nil {
		return err
	}

	gz, err := gzip.NewReader(bytes.NewReader(data))
	if err != nil {
		return err
	}
	defer func() {
		_ = gz.Close()
	}()

	decoder := xml.NewDecoder(gz)
	chunk := make([]T, 0, weatherImportChunkSize)

	for {
		token, err := decoder.Token()
		if err == io.EOF {
			break
		}
		if err != nil {
			return err
		}

		start, ok := token.(xml.StartElement)
		if !ok || start.Name.Local != element {
			continue
		}

		var record T
		if err := decoder.DecodeElement(&record, &start); err != nil {
			return err
		}

		chunk = append(chunk, record)
		if len(chunk) == weatherImportChunkSize {
			if err := process(ctx, chunk); err != nil {
				return err
			}
			chunk = chunk[:0]
		}
	}

	if len(chunk) == 0 {
		return nil
	}

	return process(ctx, chunk)
}

func (i *NoaaWeatherImporter) importMetarChunk(ctx context.Context, chunk []XmlMetar) error {
	i.stats.AddMultipleTotal(len(chunk))

	stations := make([]stationInput, len(chunk))
	hashes := make([]string, len(chunk))
	for idx := range chunk {
		x := &chunk[idx]
		stations[idx] = newStationInput(x.StationId, x.Latitude, x.Longitude, x.Elevation)
		hashes[idx] = x.Hash()
	}

	stationIDs, err := i.syncStations(ctx, stations)
	if err != nil {
		return err
	}

	existing, err := i.db.Metar.Query().Where(metar.HashIn(hashes...)).Select(metar.FieldHash).Strings(ctx)
	if err != nil {
		return err
	}

	seen := make(map[string]struct{}, len(chunk))
	for _, hash := range existing {
		seen[hash] = struct{}{}
	}

	var fresh []int
	var freshStations []uuid.UUID
	for idx := range chunk {
		if _, ok := seen[hashes[idx]]; ok {
			continue
		}
		seen[hashes[idx]] = struct{}{}

		fresh = append(fresh, idx)
		freshStations = append(freshStations, stationIDs[chunk[idx].StationId])
	}

	if len(fresh) == 0 {
		return nil
	}

	importTime := time.Now()

	latest, err := i.latestMetarTimes(ctx, freshStations, importTime)
	if err != nil {
		return err
	}

	predictions := make(map[int]*time.Time, len(fresh))
	for _, idx := range fresh {
		x := &chunk[idx]
		last, found := latest[stationIDs[x.StationId]]

		prediction, err := predictNextImport(last, found, importTime, x.ObservationTime)
		if err != nil {
			// Ignore error, because it could be that we don't have enough data for a prediction
			i.logger.Warn(fmt.Sprintf("Failed to make next import prediction for station %s: %s", x.StationId, err))
		}
		predictions[idx] = prediction
	}

	return i.insertWithFallback(ctx, "METAR", fresh, func(records []int) error {
		return withTx(ctx, i.db, func(tx *ent.Tx) error {
			metars := make([]*ent.MetarCreate, 0, len(records))
			var skyConditions []*ent.SkyConditionCreate

			for _, idx := range records {
				x := &chunk[idx]

				m, skies, err := buildMetar(tx, x, stationIDs[x.StationId], hashes[idx], importTime, predictions[idx])
				if err != nil {
					i.logger.Error(fmt.Sprintf("[IMPORT] Skipping METAR for station %s: %s", x.StationId, err))
					continue
				}

				metars = append(metars, m)
				skyConditions = append(skyConditions, skies...)
			}

			err := execInBatches(ctx, metars, func(ctx context.Context, batch []*ent.MetarCreate) error {
				return tx.Metar.CreateBulk(batch...).Exec(ctx)
			})
			if err != nil {
				return err
			}

			err = execInBatches(ctx, skyConditions, func(ctx context.Context, batch []*ent.SkyConditionCreate) error {
				return tx.SkyCondition.CreateBulk(batch...).Exec(ctx)
			})
			if err != nil {
				return err
			}

			i.stats.AddMultipleUpdated(len(metars))

			return nil
		})
	})
}

func buildMetar(tx *ent.Tx, x *XmlMetar, stationID uuid.UUID, hash string, importTime time.Time, prediction *time.Time) (*ent.MetarCreate, []*ent.SkyConditionCreate, error) {
	if x.MetarType == "" {
		return nil, nil, fmt.Errorf("empty metar type")
	}

	metarType := metar.MetarTypeMETAR
	if x.MetarType == "SPECI" {
		metarType = metar.MetarTypeSPECI
	}

	var flightCategory *metar.FlightCategory
	if x.FlightCategory != nil {
		var f metar.FlightCategory

		switch *x.FlightCategory {
		case "VFR":
			f = metar.FlightCategoryVFR
		case "MVFR":
			f = metar.FlightCategoryMVFR
		case "IFR":
			f = metar.FlightCategoryIFR
		case "LIFR":
			f = metar.FlightCategoryLIFR
		}

		if f != "" {
			flightCategory = &f
		}
	}

	windDirDegrees, windIsVariable := parseWindDirection(x.WindDirDegrees)

	visibility, visibilityIsMoreThan, err := parseVisibility(x.VisibilityStatuteMi)
	if err != nil {
		return nil, nil, err
	}

	id := uuid.New()

	skyConditions := make([]*ent.SkyConditionCreate, 0, len(x.SkyCondition))
	for _, condition := range x.SkyCondition {
		skyCover, err := getSkyCoverFromString(condition.Coverage)
		if err != nil {
			return nil, nil, err
		}

		skyConditions = append(skyConditions, tx.SkyCondition.Create().
			SetSkyCover(skyCover).
			SetNillableCloudBase(condition.Altitude).
			SetMetarID(id))
	}

	m := tx.Metar.Create().
		SetID(id).
		SetStationID(stationID).
		SetRawText(x.RawText).
		SetImportTime(importTime).
		SetObservationTime(x.ObservationTime).
		SetNillableTemperature(x.TempC).
		SetNillableDewpoint(x.DewpointC).
		SetNillableWindDirection(windDirDegrees).
		SetWindDirectionVariable(windIsVariable).
		SetNillableWindSpeed(x.WindSpeedKt).
		SetNillableWindGust(x.WindGustKt).
		SetVisibility(utils.Nillable(visibility)).
		SetVisibilityIsMoreThan(visibilityIsMoreThan).
		SetNillableAltimeter(x.AltimeterInHg).
		SetNillableSeaLevelPressure(x.SeaLevelPressureMb).
		SetQualityControlAutoStation(x.QualityControlFlags.Auto).
		SetQualityControlCorrected(x.QualityControlFlags.Corrected).
		SetQualityControlMaintenanceIndicatorOn(x.QualityControlFlags.MaintenanceIndicatorOn).
		SetQualityControlNoSignal(x.QualityControlFlags.NoSignal).
		SetQualityControlLightningSensorOff(x.QualityControlFlags.LightningSensorOff).
		SetQualityControlFreezingRainSensorOff(x.QualityControlFlags.FreezingRainSensorOff).
		SetQualityControlPresentWeatherSensorOff(x.QualityControlFlags.PresentWeatherSensorOff).
		SetNillablePresentWeather(x.WxString).
		SetNillableFlightCategory(flightCategory).
		SetNillablePressureTendency(x.ThreeHrPressureTendencyMb).
		SetNillableMaxTemp6(x.MaxTempC).
		SetNillableMinTemp6(x.MinTempC).
		SetNillableMaxTemp24(x.MaxTemp24HrC).
		SetNillableMinTemp24(x.MinTemp24HrC).
		SetNillablePrecipitation(x.PrecipIn).
		SetNillablePrecipitation3(x.Precip3HrIn).
		SetNillablePrecipitation6(x.Precip6HrIn).
		SetNillablePrecipitation24(x.Precip24HrIn).
		SetNillableSnowDepth(x.SnowIn).
		SetNillableVertVis(x.VertVisFt).
		SetNillableNextImportTimePrediction(prediction).
		SetMetarType(metarType).
		SetHash(hash)

	return m, skyConditions, nil
}

func (i *NoaaWeatherImporter) importTafChunk(ctx context.Context, chunk []XmlTaf) error {
	i.stats.AddMultipleTotal(len(chunk))

	stations := make([]stationInput, len(chunk))
	hashes := make([]string, len(chunk))
	for idx := range chunk {
		x := &chunk[idx]
		stations[idx] = newStationInput(x.StationId, x.Latitude, x.Longitude, x.Elevation)
		hashes[idx] = x.Hash()
	}

	stationIDs, err := i.syncStations(ctx, stations)
	if err != nil {
		return err
	}

	existing, err := i.db.Taf.Query().Where(taf.HashIn(hashes...)).Select(taf.FieldHash).Strings(ctx)
	if err != nil {
		return err
	}

	seen := make(map[string]struct{}, len(chunk))
	for _, hash := range existing {
		seen[hash] = struct{}{}
	}

	var fresh []int
	for idx := range chunk {
		if _, ok := seen[hashes[idx]]; ok {
			continue
		}
		seen[hashes[idx]] = struct{}{}

		fresh = append(fresh, idx)
	}

	if len(fresh) == 0 {
		return nil
	}

	return i.insertWithFallback(ctx, "TAF", fresh, func(records []int) error {
		return withTx(ctx, i.db, func(tx *ent.Tx) error {
			var rows tafRows

			for _, idx := range records {
				x := &chunk[idx]

				if err := rows.add(tx, i.logger, x, stationIDs[x.StationId], hashes[idx]); err != nil {
					i.logger.Error(fmt.Sprintf("[IMPORT] Skipping TAF for station %s: %s", x.StationId, err))
				}
			}

			if err := rows.exec(ctx, tx); err != nil {
				return err
			}

			i.stats.AddMultipleUpdated(len(rows.tafs))

			return nil
		})
	})
}

// insertWithFallback inserts all records in one go. If the database rejects that, it inserts them one by one,
// so that a single invalid record doesn't prevent the others from being imported.
func (i *NoaaWeatherImporter) insertWithFallback(ctx context.Context, kind string, records []int, insert func(records []int) error) error {
	err := insert(records)
	if err == nil || ctx.Err() != nil {
		return err
	}

	failed := 0
	for _, record := range records {
		if err = insert([]int{record}); err != nil {
			failed++
			i.logger.Error(fmt.Sprintf("[IMPORT] Failed to insert %s: %s", kind, err))
		}
	}

	i.logger.Warn(fmt.Sprintf("[IMPORT] Bulk insert of %d %ss failed, inserted them one by one instead, %d failed", len(records), kind, failed))

	return nil
}

// tafRows collects the rows of all TAFs of a chunk, so that every table can be written with bulk inserts.
type tafRows struct {
	tafs                 []*ent.TafCreate
	forecasts            []*ent.ForecastCreate
	skyConditions        []*ent.SkyConditionCreate
	turbulenceConditions []*ent.TurbulenceConditionCreate
	icingConditions      []*ent.IcingConditionCreate
	temperatureData      []*ent.TemperatureDataCreate
}

// add appends all rows of the TAF, or none of them if the TAF is invalid.
func (r *tafRows) add(tx *ent.Tx, logger *logging.Logger, x *XmlTaf, stationID uuid.UUID, hash string) error {
	var rows tafRows

	tafID := uuid.New()

	rows.tafs = append(rows.tafs, tx.Taf.Create().
		SetID(tafID).
		SetStationID(stationID).
		SetRawText(x.RawText).
		SetIssueTime(x.IssueTime).
		SetBulletinTime(x.BulletinTime).
		SetValidFromTime(x.ValidTimeFrom).
		SetValidToTime(x.ValidTimeTo).
		SetRemarks(x.Remarks).
		SetHash(hash))

	for _, xmlForecast := range x.Forecasts {
		forecastID := uuid.New()

		fc := tx.Forecast.Create().
			SetID(forecastID).
			SetTafID(tafID).
			SetFromTime(xmlForecast.TimeFrom).
			SetToTime(xmlForecast.TimeTo).
			SetNillableChangeTime(optionalTime(xmlForecast.TimeBecoming)).
			SetNillableChangeProbability(xmlForecast.Probability).
			SetNillableWindSpeed(xmlForecast.WindSpeed).
			SetNillableWindGust(xmlForecast.WindGust).
			SetNillableWindShearHeight(xmlForecast.WindShear).
			SetNillableWindShearDirection(xmlForecast.WindShearDir).
			SetNillableWindShearSpeed(xmlForecast.WindShearSpd).
			SetNillableAltimeter(xmlForecast.Altimeter).
			SetNillableVisibilityVertical(xmlForecast.VertVis).
			SetWeather(xmlForecast.Weather).
			SetNotDecoded(xmlForecast.NotDecoded)

		if xmlForecast.Change != nil {
			switch *xmlForecast.Change {
			case "TEMPO":
				fc.SetChangeIndicator(forecast.ChangeIndicatorTEMPO)
			case "BECMG":
				fc.SetChangeIndicator(forecast.ChangeIndicatorBECMG)
			case "FM":
				fc.SetChangeIndicator(forecast.ChangeIndicatorFM)
			case "PROB":
				fc.SetChangeIndicator(forecast.ChangeIndicatorPROB)
			default:
				logger.Error(fmt.Sprintf("unknown forecast change indicator %s", *xmlForecast.Change))
			}
		}

		windDirDegrees, windIsVariable := parseWindDirection(xmlForecast.WindDir)
		fc.SetNillableWindDirection(windDirDegrees)
		fc.SetWindDirectionVariable(windIsVariable)

		visibility, visibilityIsMoreThan, err := parseVisibility(xmlForecast.Visibility)
		if err != nil {
			return err
		}
		fc.SetNillableVisibilityHorizontal(visibility)
		fc.SetVisibilityHorizontalIsMoreThan(visibilityIsMoreThan)

		rows.forecasts = append(rows.forecasts, fc)

		for _, condition := range xmlForecast.SkyCondition {
			skyCover, err := getSkyCoverFromString(condition.Coverage)
			if err != nil {
				return err
			}

			rows.skyConditions = append(rows.skyConditions, tx.SkyCondition.Create().
				SetSkyCover(skyCover).
				SetNillableCloudBase(condition.Altitude).
				SetForecastID(forecastID))
		}

		for _, xmlTurbulence := range xmlForecast.TurbulenceCondition {
			if xmlTurbulence.Intensity == "" && xmlTurbulence.MinAlt == 0 && xmlTurbulence.MaxAlt == 0 {
				continue
			}

			rows.turbulenceConditions = append(rows.turbulenceConditions, tx.TurbulenceCondition.Create().
				SetIntensity(xmlTurbulence.Intensity).
				SetMinAltitude(xmlTurbulence.MinAlt).
				SetMaxAltitude(xmlTurbulence.MaxAlt).
				SetForecastID(forecastID))
		}

		for _, xmlIcing := range xmlForecast.IcingCondition {
			if xmlIcing.Intensity == "" && xmlIcing.MinAlt == 0 && xmlIcing.MaxAlt == 0 {
				continue
			}

			rows.icingConditions = append(rows.icingConditions, tx.IcingCondition.Create().
				SetIntensity(xmlIcing.Intensity).
				SetMinAltitude(xmlIcing.MinAlt).
				SetMaxAltitude(xmlIcing.MaxAlt).
				SetForecastID(forecastID))
		}

		for _, xmlTemperature := range xmlForecast.Temperature {
			if xmlTemperature.SurfaceTempC == 0 && xmlTemperature.ValidTime.IsZero() && xmlTemperature.MinTempC == nil && xmlTemperature.MaxTempC == nil {
				continue
			}

			rows.temperatureData = append(rows.temperatureData, tx.TemperatureData.Create().
				SetTemperature(xmlTemperature.SurfaceTempC).
				SetNillableMinTemperature(xmlTemperature.MinTempC).
				SetNillableMaxTemperature(xmlTemperature.MaxTempC).
				SetValidTime(xmlTemperature.ValidTime).
				SetForecastID(forecastID))
		}
	}

	r.tafs = append(r.tafs, rows.tafs...)
	r.forecasts = append(r.forecasts, rows.forecasts...)
	r.skyConditions = append(r.skyConditions, rows.skyConditions...)
	r.turbulenceConditions = append(r.turbulenceConditions, rows.turbulenceConditions...)
	r.icingConditions = append(r.icingConditions, rows.icingConditions...)
	r.temperatureData = append(r.temperatureData, rows.temperatureData...)

	return nil
}

func (r *tafRows) exec(ctx context.Context, tx *ent.Tx) error {
	err := execInBatches(ctx, r.tafs, func(ctx context.Context, batch []*ent.TafCreate) error {
		return tx.Taf.CreateBulk(batch...).Exec(ctx)
	})
	if err != nil {
		return err
	}

	err = execInBatches(ctx, r.forecasts, func(ctx context.Context, batch []*ent.ForecastCreate) error {
		return tx.Forecast.CreateBulk(batch...).Exec(ctx)
	})
	if err != nil {
		return err
	}

	err = execInBatches(ctx, r.skyConditions, func(ctx context.Context, batch []*ent.SkyConditionCreate) error {
		return tx.SkyCondition.CreateBulk(batch...).Exec(ctx)
	})
	if err != nil {
		return err
	}

	err = execInBatches(ctx, r.turbulenceConditions, func(ctx context.Context, batch []*ent.TurbulenceConditionCreate) error {
		return tx.TurbulenceCondition.CreateBulk(batch...).Exec(ctx)
	})
	if err != nil {
		return err
	}

	err = execInBatches(ctx, r.icingConditions, func(ctx context.Context, batch []*ent.IcingConditionCreate) error {
		return tx.IcingCondition.CreateBulk(batch...).Exec(ctx)
	})
	if err != nil {
		return err
	}

	return execInBatches(ctx, r.temperatureData, func(ctx context.Context, batch []*ent.TemperatureDataCreate) error {
		return tx.TemperatureData.CreateBulk(batch...).Exec(ctx)
	})
}

// optionalTime drops the 1969-12-31T23:59:59Z that NOAA sends for missing times, which MySQL can't store.
func optionalTime(t *time.Time) *time.Time {
	if t == nil || t.Unix() <= 0 {
		return nil
	}

	return t
}

func parseWindDirection(input *string) (*int, bool) {
	if input == nil {
		return nil, false
	}

	if *input == "VRB" {
		return nil, true
	}

	direction, err := strconv.Atoi(*input)
	if err != nil {
		return nil, false
	}

	return &direction, false
}

func parseVisibility(input *string) (*float64, bool, error) {
	if input == nil {
		return nil, false, nil
	}

	isMoreThan := strings.Contains(*input, "+")

	visibility, err := strconv.ParseFloat(strings.ReplaceAll(*input, "+", ""), 64)
	if err != nil {
		return nil, false, err
	}

	return &visibility, isMoreThan, nil
}

func getSkyCoverFromString(input string) (skycondition.SkyCover, error) {
	switch input {
	case "SCT":
		return skycondition.SkyCoverScattered, nil
	case "CAVOK":
		return skycondition.SkyCoverCeilingAndVisibilityOK, nil
	case "NSC":
		return skycondition.SkyCoverNoSignificantClouds, nil
	case "SKC":
		return skycondition.SkyCoverSkyClear, nil
	case "CLR":
		return skycondition.SkyCoverClear, nil
	case "BKN":
		return skycondition.SkyCoverBroken, nil
	case "FEW":
		return skycondition.SkyCoverFew, nil
	case "OVC":
		return skycondition.SkyCoverOvercast, nil
	case "OVX":
		return skycondition.SkyCoverVerticalVisibility, nil
	}

	return "", fmt.Errorf("unknown sky cover %s", input)
}

type stationInput struct {
	stationID string
	latitude  float64
	longitude float64
	elevation float64
}

func newStationInput(stationID string, latitude *float64, longitude *float64, elevation *float64) stationInput {
	return stationInput{
		stationID: stationID,
		latitude:  utils.Nillable(latitude),
		longitude: utils.Nillable(longitude),
		elevation: utils.Nillable(elevation),
	}
}

// syncStations creates or updates the weather stations of a chunk and returns their IDs by station ID.
func (i *NoaaWeatherImporter) syncStations(ctx context.Context, inputs []stationInput) (map[string]uuid.UUID, error) {
	codes := make([]string, 0, len(inputs))
	for _, input := range inputs {
		codes = append(codes, input.stationID)
	}

	airports, err := i.db.Airport.Query().
		Where(airport.IdentifierIn(codes...)).
		Select(airport.FieldID, airport.FieldIdentifier).
		All(ctx)
	if err != nil {
		return nil, err
	}

	airportsByCode := make(map[string]*ent.Airport, len(airports))
	ambiguous := make(map[string]bool)
	for _, a := range airports {
		if _, ok := airportsByCode[a.Identifier]; ok {
			ambiguous[a.Identifier] = true
		}
		airportsByCode[a.Identifier] = a
	}
	for code := range ambiguous {
		delete(airportsByCode, code)
	}

	stations, err := i.db.WeatherStation.Query().
		Where(weatherstation.StationIDIn(codes...)).
		Select(weatherstation.FieldID, weatherstation.FieldStationID, weatherstation.FieldHash).
		All(ctx)
	if err != nil {
		return nil, err
	}

	stationsByCode := make(map[string]*ent.WeatherStation, len(stations))
	for _, s := range stations {
		stationsByCode[s.StationID] = s
	}

	ids := make(map[string]uuid.UUID, len(inputs))
	var creates []*ent.WeatherStationCreate

	for _, input := range inputs {
		if _, ok := ids[input.stationID]; ok {
			continue
		}

		a := airportsByCode[input.stationID]

		line := fmt.Sprintf("%s%f%f%f", input.stationID, input.latitude, input.longitude, input.elevation)
		if a != nil {
			line += fmt.Sprintf("%d", a.ID)
		}

		hash := strconv.FormatUint(fnv1a.HashString64(line), 10)

		if s, ok := stationsByCode[input.stationID]; ok {
			ids[input.stationID] = s.ID

			if s.Hash == hash {
				continue
			}

			update := i.db.WeatherStation.UpdateOneID(s.ID).
				SetHash(hash).
				SetLatitude(input.latitude).
				SetLongitude(input.longitude).
				SetElevation(input.elevation)
			if a != nil {
				update.SetAirportID(a.ID)
			}

			if err := update.Exec(ctx); err != nil {
				return nil, err
			}

			continue
		}

		id := uuid.New()
		ids[input.stationID] = id

		create := i.db.WeatherStation.Create().
			SetID(id).
			SetHash(hash).
			SetLatitude(input.latitude).
			SetLongitude(input.longitude).
			SetElevation(input.elevation).
			SetStationID(input.stationID)
		if a != nil {
			create.SetAirportID(a.ID)
		}

		creates = append(creates, create)
	}

	err = execInBatches(ctx, creates, func(ctx context.Context, batch []*ent.WeatherStationCreate) error {
		return i.db.WeatherStation.CreateBulk(batch...).Exec(ctx)
	})
	if err != nil {
		return nil, err
	}

	return ids, nil
}

func execInBatches[T any](ctx context.Context, builders []T, exec func(ctx context.Context, batch []T) error) error {
	for start := 0; start < len(builders); start += insertBatchSize {
		end := min(start+insertBatchSize, len(builders))
		if err := exec(ctx, builders[start:end]); err != nil {
			return err
		}
	}

	return nil
}

func withTx(ctx context.Context, db *ent.Client, fn func(tx *ent.Tx) error) error {
	tx, err := db.Tx(ctx)
	if err != nil {
		return err
	}

	if err := fn(tx); err != nil {
		_ = tx.Rollback()
		return err
	}

	return tx.Commit()
}

// MigrateWeatherHashes rewrites hashes of METARs and TAFs that were created with a previous hash function,
// so that already imported records are not imported again.
func MigrateWeatherHashes(ctx context.Context, db *ent.Client) error {
	err := db.Metar.Update().
		Where(func(s *sql.Selector) {
			s.Where(sql.ExprP(fmt.Sprintf("%s <> MD5(TRIM(%s))", s.C(metar.FieldHash), s.C(metar.FieldRawText))))
		}).
		Modify(func(u *sql.UpdateBuilder) {
			u.Set(metar.FieldHash, sql.Expr(fmt.Sprintf("MD5(TRIM(%s))", metar.FieldRawText)))
		}).
		Exec(ctx)
	if err != nil {
		return err
	}

	return db.Taf.Update().
		Where(func(s *sql.Selector) {
			s.Where(sql.ExprP(fmt.Sprintf("%s <> MD5(TRIM(%s))", s.C(taf.FieldHash), s.C(taf.FieldRawText))))
		}).
		Modify(func(u *sql.UpdateBuilder) {
			u.Set(taf.FieldHash, sql.Expr(fmt.Sprintf("MD5(TRIM(%s))", taf.FieldRawText)))
		}).
		Exec(ctx)
}
