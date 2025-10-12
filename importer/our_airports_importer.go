package importer

import (
	"context"
	"encoding/csv"
	"fmt"
	"github.com/ringsaturn/tzf"
	tzfrel "github.com/ringsaturn/tzf-rel"
	"github.com/ringsaturn/tzf/pb"
	"github.com/segmentio/fasthash/fnv1a"
	"golang.org/x/sync/errgroup"
	"google.golang.org/protobuf/proto"
	"io"
	"metar.live/ent"
	"metar.live/ent/airport"
	"metar.live/ent/country"
	"metar.live/ent/frequency"
	"metar.live/ent/region"
	"metar.live/ent/runway"
	"metar.live/environment"
	"metar.live/logging"
	"metar.live/utils"
	"os"
	"regexp"
	"strconv"
	"strings"
	"time"
)

type Importer struct {
	db             *ent.Client
	logger         *logging.Logger
	stats          *ImportStatistics
	timezoneFinder tzf.F
}

// NewImporter creates a new importer instance. Make sure to call Cleanup() when done.
func NewImporter(db *ent.Client, logger *logging.Logger) *Importer {
	importer := &Importer{
		db:     db,
		logger: logger,
		stats:  NewImportStatistics("", logger),
	}

	err := importer.initializeTimezoneCache()
	if err != nil {
		logger.Fatal(err)
	}

	return importer
}

type ImportLineFunction func(ctx context.Context, data []string) error
type CleanupImportFunction func(ctx context.Context) error

func (i *Importer) initializeTimezoneCache() error {
	dataFile := tzfrel.FullData

	input := &pb.Timezones{}
	if err := proto.Unmarshal(dataFile, input); err != nil {
		panic(err)
	}

	finder, _ := tzf.NewFinderFromPB(input)

	i.timezoneFinder = finder

	return nil
}

func (i *Importer) ImportAirports(ctx context.Context, url string) error {
	i.stats = NewImportStatistics("AIRPORTS", i.logger)

	err := i.importModelType(ctx, url, i.importAirportLine, i.cleanupAirports)
	if err != nil {
		return err
	}

	return nil
}

func (i *Importer) ImportRunways(ctx context.Context, url string) error {
	i.stats = NewImportStatistics("RUNWAYS", i.logger)

	err := i.importModelType(ctx, url, i.importRunwayLine, i.cleanupRunways)
	if err != nil {
		return err
	}

	return nil
}

func (i *Importer) ImportFrequencies(ctx context.Context, url string) error {
	i.stats = NewImportStatistics("FREQUENCIES", i.logger)

	err := i.importModelType(ctx, url, i.importFrequencyLine, i.cleanupFrequencies)
	if err != nil {
		return err
	}

	return nil
}

func (i *Importer) ImportCountries(ctx context.Context, url string) error {
	i.stats = NewImportStatistics("COUNTRIES", i.logger)

	err := i.importModelType(ctx, url, i.importCountryLine, i.cleanupCountries)
	if err != nil {
		return err
	}

	return nil
}

func (i *Importer) ImportRegions(ctx context.Context, url string) error {
	i.stats = NewImportStatistics("REGIONS", i.logger)

	err := i.importModelType(ctx, url, i.importRegionLine, i.cleanupRegions)
	if err != nil {
		return err
	}

	return nil
}

func (i *Importer) importModelType(ctx context.Context, url string, importFunction ImportLineFunction, cleanupFunction CleanupImportFunction) error {
	i.stats.Start()

	filepath := "file.csv"
	err := utils.DownloadFile(url, filepath, false)
	if err != nil {
		return err
	}

	// Read csv file and map to Airport struct.
	f, err := os.Open(filepath)
	if err != nil {
		return err
	}
	defer func(f *os.File) {
		_ = f.Close()
	}(f)

	csvReader := csv.NewReader(f)

	maxGoroutines := environment.Global.MaxConcurrentImports
	guard := make(chan struct{}, maxGoroutines)

	wg := errgroup.Group{}

	lineCounter := 0
	for {
		record, err := csvReader.Read()
		if err == io.EOF {
			break
		}

		if err != nil {
			return err
		}
		lineCounter++

		if lineCounter == 1 {
			continue
		}

		guard <- struct{}{} // would block if guard channel is already filled
		i.stats.AddTotal()
		wg.Go(func() error {
			record := record

			defer func() { <-guard }()
			return importFunction(ctx, record)
		})
	}

	err = wg.Wait()
	if err != nil {
		return err
	}

	err = cleanupFunction(ctx)
	if err != nil {
		return err
	}

	// Delete the file after import.
	err = os.Remove(filepath)

	i.stats.End()

	return err
}

func stringContinentToEnumContinent(continent string) country.Continent {
	switch continent {
	case country.ContinentAfrica.String():
		return country.ContinentAfrica
	case country.ContinentAntarctica.String():
		return country.ContinentAntarctica
	case country.ContinentAsia.String():
		return country.ContinentAsia
	case country.ContinentOceania.String():
		return country.ContinentOceania
	case country.ContinentSouthAmerica.String():
		return country.ContinentSouthAmerica
	case country.ContinentNorthAmerica.String():
		return country.ContinentNorthAmerica
	case country.ContinentEurope.String():
		return country.ContinentEurope
	}

	return ""
}

// CSV file format: "id","ident","type","name","latitude_deg","longitude_deg","elevation_ft","continent","iso_country","iso_region","municipality","scheduled_service","gps_code","iata_code","local_code","home_link","wikipedia_link","keywords"
func (i *Importer) importAirportLine(ctx context.Context, data []string) error {
	// Hash the current line via md5
	line := strings.Join(data, "")
	hash := strconv.FormatUint(fnv1a.HashString64(line), 10)
	ourAirportID, _ := strconv.ParseInt(data[0], 10, 64)

	found, err := i.db.Airport.Update().Where(
		airport.Hash(hash),
		airport.ImportID(int(ourAirportID)),
	).
		SetImportFlag(true).
		Save(ctx)
	if err != nil {
		return err
	}

	if found == 1 {
		return nil
	}

	// Upsert airport, because we know it doesn't exist yet, or it changed

	lat, _ := strconv.ParseFloat(data[4], 64)
	lon, _ := strconv.ParseFloat(data[5], 64)

	tz := ""

	if lat != 0 && lon != 0 {
		zone := i.timezoneFinder.GetTimezoneName(lon, lat)
		if zone == "" {
			i.logger.Warn(fmt.Sprintf("[IMPORT] No timezone found for airport %s with lat: %f and lng: %f", data[1], lat, lon))
		} else {
			tz = zone
		}
	}

	elevation, _ := strconv.ParseInt(data[6], 10, 64)

	scheduledService := false
	if data[11] == "yes" {
		scheduledService = true
	}

	// Explode keywords into array.
	keywords := make([]string, 0)
	if data[17] != "" {
		keywords = strings.Split(data[17], ", ")
	}

	importance := 0

	airportType := airport.TypeSmallAirport
	switch data[2] {
	case airport.TypeLargeAirport.String():
		airportType = airport.TypeLargeAirport
		importance = 5
	case airport.TypeMediumAirport.String():
		airportType = airport.TypeMediumAirport
		importance = 4
	case airport.TypeSmallAirport.String():
		airportType = airport.TypeSmallAirport
		importance = 3
	case airport.TypeHeliport.String():
		airportType = airport.TypeHeliport
		importance = 2
	case airport.TypeSeaplaneBase.String():
		airportType = airport.TypeSeaplaneBase
		importance = 1
	case airport.TypeClosedAirport.String():
		airportType = airport.TypeClosedAirport
		importance = 0
	}

	// Fetch country
	c, err := i.db.Country.Query().Where(country.Code(data[8])).Only(ctx)
	if err != nil {
		i.logger.Error(fmt.Sprintf("[IMPORT] Could not find country with code %s", data[8]))
	}

	// Fetch region
	r, err := i.db.Region.Query().Where(region.Code(data[9])).Only(ctx)
	if err != nil {
		i.logger.Error(fmt.Sprintf("[IMPORT] Could not find region with code %s", data[9]))
	}

	createAirport := i.db.Airport.Create().
		SetImportFlag(true).
		SetHash(hash).
		SetImportID(int(ourAirportID)).
		SetIdentifier(data[1]).
		SetType(airportType).
		SetName(data[3]).
		SetLatitude(lat).
		SetLongitude(lon).
		SetNillableTimezone(utils.NillableString(tz)).
		SetNillableElevation(utils.NillableWithInput(data[6], int(elevation))).
		SetNillableMunicipality(utils.NillableString(data[10])).
		SetScheduledService(scheduledService).
		SetNillableGpsCode(utils.NillableString(data[12])).
		SetIataCode(data[13]).
		SetNillableLocalCode(utils.NillableString(data[14])).
		SetNillableWebsite(utils.NillableString(data[15])).
		SetNillableWikipedia(utils.NillableString(data[16])).
		SetKeywords(keywords).
		SetLastUpdated(time.Now())

	if c != nil {
		createAirport.SetCountryID(c.ID)
	}

	if r != nil {
		createAirport.SetRegionID(r.ID)
	}

	// Check if identifier is all letters and only 4 characters long.
	potentialIcao := strings.Trim(data[1], " ")
	if len(potentialIcao) == 4 && regexp.MustCompile(`^[a-zA-Z]+$`).MatchString(potentialIcao) {
		createAirport.SetIcaoCode(strings.ToUpper(potentialIcao))

		importance += 1
	}

	// If we have an IATA code, increase importance by 1.
	if data[13] != "" {
		importance += 1
	}

	createAirport.SetImportance(importance)

	err = createAirport.OnConflict().
		UpdateNewValues().Exec(ctx)
	if err != nil {
		return err
	}

	i.stats.AddUpdated()

	return nil
}

func (i *Importer) cleanupAirports(ctx context.Context) error {
	deleted, err := i.db.Airport.Delete().Where(
		airport.ImportFlag(false),
	).Exec(ctx)
	if err != nil {
		return err
	}

	i.stats.AddMultipleDeleted(deleted)

	_, err = i.db.Airport.Update().Where(
		airport.ImportFlag(true),
	).SetImportFlag(false).Save(ctx)
	if err != nil {
		return err
	}

	return nil
}

// The raw data is in the following order: "id","airport_ref","airport_ident","length_ft","width_ft","surface",
// "lighted","closed","le_ident","le_latitude_deg","le_longitude_deg","le_elevation_ft","le_heading_degT",
// "le_displaced_threshold_ft","he_ident","he_latitude_deg","he_longitude_deg","he_elevation_ft","he_heading_degT",
// "he_displaced_threshold_ft"
func (i *Importer) importRunwayLine(ctx context.Context, data []string) error {
	// Hash the current line via md5
	line := strings.Join(data, "")
	hash := strconv.FormatUint(fnv1a.HashString64(line), 10)
	runwayID, _ := strconv.ParseInt(data[0], 10, 64)

	found, err := i.db.Runway.Update().Where(
		runway.Hash(hash),
		runway.ImportID(int(runwayID)),
	).
		SetImportFlag(true).
		Save(ctx)
	if err != nil {
		return err
	}

	if found == 1 {
		return nil
	}

	// Find the airport
	a, err := i.db.Airport.Query().Where(
		airport.Identifier(data[2]),
	).First(ctx)
	if err != nil {
		return err
	}

	// Upsert airport, because we know it doesn't exist yet, or it changed

	length, _ := strconv.ParseInt(data[3], 10, 64)
	width, _ := strconv.ParseInt(data[4], 10, 64)
	isLighted, _ := strconv.ParseBool(data[6])
	isClosed, _ := strconv.ParseBool(data[7])

	leElevation, _ := strconv.ParseInt(data[11], 10, 64)
	leHeading, _ := strconv.ParseFloat(data[12], 64)
	leDisplacedThreshold, _ := strconv.ParseInt(data[13], 10, 64)
	leLatitude, _ := strconv.ParseFloat(data[9], 64)
	leLongitude, _ := strconv.ParseFloat(data[10], 64)

	heElevation, _ := strconv.ParseInt(data[17], 10, 64)
	heHeading, _ := strconv.ParseFloat(data[18], 64)
	heDisplacedThreshold, _ := strconv.ParseInt(data[19], 10, 64)
	heLatitude, _ := strconv.ParseFloat(data[15], 64)
	heLongitude, _ := strconv.ParseFloat(data[16], 64)

	err = i.db.Runway.Create().
		SetImportFlag(true).
		SetHash(hash).
		SetImportID(int(runwayID)).
		SetAirport(a).
		SetLength(int(length)).
		SetWidth(int(width)).
		SetNillableSurface(utils.NillableString(data[5])).
		SetLighted(isLighted).
		SetClosed(isClosed).
		SetLowRunwayIdentifier(data[8]).
		SetNillableLowRunwayLatitude(utils.NillableWithInput(data[9], leLatitude)).
		SetNillableLowRunwayLongitude(utils.NillableWithInput(data[10], leLongitude)).
		SetNillableLowRunwayElevation(utils.NillableWithInput(data[11], int(leElevation))).
		SetNillableLowRunwayHeading(utils.NillableWithInput(data[12], leHeading)).
		SetNillableLowRunwayDisplacedThreshold(utils.NillableWithInput(data[13], int(leDisplacedThreshold))).
		SetHighRunwayIdentifier(data[14]).
		SetNillableHighRunwayLatitude(utils.NillableWithInput(data[15], heLatitude)).
		SetNillableHighRunwayLongitude(utils.NillableWithInput(data[16], heLongitude)).
		SetNillableHighRunwayElevation(utils.NillableWithInput(data[17], int(heElevation))).
		SetNillableHighRunwayHeading(utils.NillableWithInput(data[18], heHeading)).
		SetNillableHighRunwayDisplacedThreshold(utils.NillableWithInput(data[19], int(heDisplacedThreshold))).
		SetLastUpdated(time.Now()).
		OnConflict().
		UpdateNewValues().
		Exec(ctx)
	if err != nil {
		return err
	}

	i.stats.AddUpdated()

	return nil
}

func (i *Importer) cleanupRunways(ctx context.Context) error {
	deleted, err := i.db.Runway.Delete().Where(
		runway.ImportFlag(false),
	).Exec(ctx)
	if err != nil {
		return err
	}

	i.stats.AddMultipleDeleted(deleted)

	_, err = i.db.Runway.Update().Where(
		runway.ImportFlag(true),
	).SetImportFlag(false).Save(ctx)
	if err != nil {
		return err
	}

	return nil
}

// The raw data is in the following order: "id","airport_ref","airport_ident","type","description","frequency_mhz"
func (i *Importer) importFrequencyLine(ctx context.Context, data []string) error {
	// Hash the current line via md5
	line := strings.Join(data, "")
	hash := strconv.FormatUint(fnv1a.HashString64(line), 10)
	frequencyID, _ := strconv.ParseInt(data[0], 10, 64)

	found, err := i.db.Frequency.Update().Where(
		frequency.Hash(hash),
		frequency.ImportID(int(frequencyID)),
	).
		SetImportFlag(true).
		Save(ctx)
	if err != nil {
		return err
	}

	if found == 1 {
		return nil
	}

	// Upsert airport, because we know it doesn't exist yet, or it changed
	a, err := i.db.Airport.Query().Where(
		airport.Identifier(data[2]),
	).First(ctx)
	if err != nil {
		return err
	}

	f, _ := strconv.ParseFloat(data[5], 64)

	err = i.db.Frequency.Create().
		SetImportFlag(true).
		SetHash(hash).
		SetImportID(int(frequencyID)).
		SetAirport(a).
		SetType(data[3]).
		SetDescription(data[4]).
		SetFrequency(f).
		SetLastUpdated(time.Now()).
		OnConflict().
		UpdateNewValues().
		Exec(ctx)
	if err != nil {
		return err
	}

	i.stats.AddUpdated()

	return nil
}

func (i *Importer) cleanupFrequencies(ctx context.Context) error {
	deleted, err := i.db.Frequency.Delete().Where(
		frequency.ImportFlag(false),
	).Exec(ctx)
	if err != nil {
		return err
	}

	i.stats.AddMultipleDeleted(deleted)

	_, err = i.db.Frequency.Update().Where(
		frequency.ImportFlag(true),
	).SetImportFlag(false).Save(ctx)
	if err != nil {
		return err
	}

	return nil
}

// The raw data is in the following order: "id","code","name","continent","wikipedia_link","keywords"
func (i *Importer) importCountryLine(ctx context.Context, data []string) error {
	// Hash the current line via md5
	line := strings.Join(data, "")
	hash := strconv.FormatUint(fnv1a.HashString64(line), 10)
	countryID, _ := strconv.ParseInt(data[0], 10, 64)

	found, err := i.db.Country.Update().Where(
		country.Hash(hash),
		country.ImportID(int(countryID)),
	).
		SetImportFlag(true).
		Save(ctx)
	if err != nil {
		return err
	}

	if found == 1 {
		return nil
	}

	keywords := make([]string, 0)
	if data[5] != "" {
		keywords = strings.Split(data[5], ", ")
	}

	err = i.db.Country.Create().
		SetImportFlag(true).
		SetHash(hash).
		SetImportID(int(countryID)).
		SetCode(data[1]).
		SetName(data[2]).
		SetContinent(stringContinentToEnumContinent(data[3])).
		SetWikipediaLink(data[4]).
		SetKeywords(keywords).
		SetLastUpdated(time.Now()).
		OnConflict().
		UpdateNewValues().
		Exec(ctx)
	if err != nil {
		return err
	}

	i.stats.AddUpdated()

	return nil
}

func (i *Importer) cleanupCountries(ctx context.Context) error {
	deleted, err := i.db.Country.Delete().Where(
		country.ImportFlag(false),
	).Exec(ctx)
	if err != nil {
		return err
	}

	i.stats.AddMultipleDeleted(deleted)

	_, err = i.db.Country.Update().Where(
		country.ImportFlag(true),
	).SetImportFlag(false).Save(ctx)
	if err != nil {
		return err
	}

	return nil
}

// CSV fields: "id","code","local_code","name","continent","iso_country","wikipedia_link","keywords"
func (i *Importer) importRegionLine(ctx context.Context, data []string) error {
	// Hash the current line via md5
	line := strings.Join(data, "")
	hash := strconv.FormatUint(fnv1a.HashString64(line), 10)
	regionID, _ := strconv.ParseInt(data[0], 10, 64)

	found, err := i.db.Region.Update().Where(
		region.Hash(hash),
		region.ImportID(int(regionID)),
	).
		SetImportFlag(true).
		Save(ctx)
	if err != nil {
		return err
	}

	if found == 1 {
		return nil
	}

	err = i.db.Region.Create().
		SetImportFlag(true).
		SetHash(hash).
		SetImportID(int(regionID)).
		SetCode(data[1]).
		SetLocalCode(data[2]).
		SetName(data[3]).
		SetWikipediaLink(data[6]).
		SetKeywords(strings.Split(data[7], ", ")).
		SetLastUpdated(time.Now()).
		OnConflict().
		UpdateNewValues().
		Exec(ctx)
	if err != nil {
		return err
	}

	i.stats.AddUpdated()

	return nil
}

func (i *Importer) cleanupRegions(ctx context.Context) error {
	deleted, err := i.db.Region.Delete().Where(
		region.ImportFlag(false),
	).Exec(ctx)
	if err != nil {
		return err
	}

	i.stats.AddMultipleDeleted(deleted)

	_, err = i.db.Region.Update().Where(
		region.ImportFlag(true),
	).SetImportFlag(false).Save(ctx)
	if err != nil {
		return err
	}

	return nil
}
