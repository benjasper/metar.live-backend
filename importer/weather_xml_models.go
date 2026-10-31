package importer

import (
	"crypto/md5"
	"encoding/hex"
	"encoding/xml"
	"strings"
	"time"
)

type XmlQualityControlFlags struct {
	Auto                    bool `xml:"auto"`
	Corrected               bool `xml:"corrected"`
	MaintenanceIndicatorOn  bool `xml:"maintenance_indicator"`
	NoSignal                bool `xml:"no_signal"`
	LightningSensorOff      bool `xml:"lightning_sensor_off"`
	FreezingRainSensorOff   bool `xml:"freezing_rain_sensor_off"`
	PresentWeatherSensorOff bool `xml:"present_weather_sensor_off"`
}

type XmlSkyCondition struct {
	Coverage string `xml:"sky_cover,attr"`
	Altitude *int   `xml:"cloud_base_ft_agl,attr"`
}

type XmlMetar struct {
	RawText                   string                 `xml:"raw_text"`
	StationId                 string                 `xml:"station_id"`
	ObservationTime           time.Time              `xml:"observation_time"`
	Latitude                  *float64               `xml:"latitude"`
	Longitude                 *float64               `xml:"longitude"`
	TempC                     *float64               `xml:"temp_c"`
	DewpointC                 *float64               `xml:"dewpoint_c"`
	WindDirDegrees            *string                `xml:"wind_dir_degrees"`
	WindSpeedKt               *int                   `xml:"wind_speed_kt"`
	WindGustKt                *int                   `xml:"wind_gust_kt"`
	VisibilityStatuteMi       *string                `xml:"visibility_statute_mi"`
	AltimeterInHg             *float64               `xml:"altim_in_hg"`
	SeaLevelPressureMb        *float64               `xml:"sea_level_pressure_mb"`
	QualityControlFlags       XmlQualityControlFlags `xml:"quality_control_flags"`
	WxString                  *string                `xml:"wx_string"`
	SkyCondition              []XmlSkyCondition      `xml:"sky_condition"`
	FlightCategory            *string                `xml:"flight_category"`
	ThreeHrPressureTendencyMb *float64               `xml:"three_hr_pressure_tendency_mb"`
	MaxTempC                  *float64               `xml:"maxT_c"`
	MinTempC                  *float64               `xml:"minT_c"`
	MaxTemp24HrC              *float64               `xml:"maxT24hr_c"`
	MinTemp24HrC              *float64               `xml:"minT24hr_c"`
	PrecipIn                  *float64               `xml:"precip_in"`
	Precip3HrIn               *float64               `xml:"precip_3hr_in"`
	Precip6HrIn               *float64               `xml:"precip_6hr_in"`
	Precip24HrIn              *float64               `xml:"precip24hr_in"`
	SnowIn                    *float64               `xml:"snow_in"`
	VertVisFt                 *float64               `xml:"vert_vis_ft"`
	MetarType                 string                 `xml:"metar_type"`
	Elevation                 *float64               `xml:"elevation_m"`
}

func (x *XmlMetar) Hash() string {
	return rawTextHash(x.RawText)
}

type XmlMetarResponse struct {
	XMLName                   xml.Name `xml:"response"`
	Text                      string   `xml:",chardata"`
	Xsd                       string   `xml:"xsd,attr"`
	Xsi                       string   `xml:"xsi,attr"`
	Version                   string   `xml:"version,attr"`
	NoNamespaceSchemaLocation string   `xml:"noNamespaceSchemaLocation,attr"`
	RequestIndex              string   `xml:"request_index"`
	DataSource                struct {
		Text string `xml:",chardata"`
		Name string `xml:"name,attr"`
	} `xml:"data_source"`
	Request struct {
		Text string `xml:",chardata"`
		Type string `xml:"type,attr"`
	} `xml:"request"`
	Errors      string `xml:"errors"`
	Warnings    string `xml:"warnings"`
	TimeTakenMs string `xml:"time_taken_ms"`
	Data        struct {
		Text       string     `xml:",chardata"`
		NumResults string     `xml:"num_results,attr"`
		METAR      []XmlMetar `xml:"METAR"`
	} `xml:"data"`
}

type XmlTaf struct {
	RawText       string        `xml:"raw_text"`
	StationId     string        `xml:"station_id"`
	IssueTime     time.Time     `xml:"issue_time"`
	BulletinTime  time.Time     `xml:"bulletin_time"`
	ValidTimeFrom time.Time     `xml:"valid_time_from"`
	ValidTimeTo   time.Time     `xml:"valid_time_to"`
	Remarks       string        `xml:"remarks"`
	Latitude      *float64      `xml:"latitude"`
	Longitude     *float64      `xml:"longitude"`
	Elevation     *float64      `xml:"elevation_m"`
	Forecasts     []XmlForecast `xml:"forecast"`
}

func (x *XmlTaf) Hash() string {
	return rawTextHash(x.RawText)
}

// rawTextHash must stay equal to MySQL's MD5(TRIM(raw_text)), see MigrateWeatherHashes.
// NOAA sometimes republishes a report with a trailing space, which would otherwise be imported twice.
func rawTextHash(rawText string) string {
	sum := md5.Sum([]byte(strings.Trim(rawText, " ")))
	return hex.EncodeToString(sum[:])
}

type XmlForecast struct {
	TimeFrom            time.Time                `xml:"fcst_time_from"`
	TimeTo              time.Time                `xml:"fcst_time_to"`
	Change              *string                  `xml:"change_indicator"`
	TimeBecoming        *time.Time               `xml:"time_becoming"`
	Probability         *int                     `xml:"probability"`
	WindDir             *string                  `xml:"wind_dir_degrees"`
	WindSpeed           *int                     `xml:"wind_speed_kt"`
	WindGust            *int                     `xml:"wind_gust_kt"`
	WindShear           *int                     `xml:"wind_shear_hgt_ft_agl"`
	WindShearDir        *int                     `xml:"wind_shear_dir_degrees"`
	WindShearSpd        *int                     `xml:"wind_shear_speed_kt"`
	Visibility          *string                  `xml:"visibility_statute_mi"`
	Altimeter           *float64                 `xml:"altim_in_hg"`
	VertVis             *int                     `xml:"vert_vis_ft"`
	Weather             string                   `xml:"wx_string"`
	NotDecoded          string                   `xml:"not_decoded"`
	SkyCondition        []XmlSkyCondition        `xml:"sky_condition"`
	TurbulenceCondition []XmlTurbulenceCondition `xml:"turbulence_condition"`
	IcingCondition      []XmlIcingCondition      `xml:"icing_condition"`
	Temperature         []XmlTemperature         `xml:"temperature"`
}

type XmlTurbulenceCondition struct {
	Intensity string `xml:"turbulence_intensity"`
	MinAlt    int    `xml:"min_alt_ft_agl"`
	MaxAlt    int    `xml:"max_alt_ft_agl"`
}

type XmlIcingCondition struct {
	Intensity string `xml:"icing_intensity"`
	MinAlt    int    `xml:"min_alt_ft_agl"`
	MaxAlt    int    `xml:"max_alt_ft_agl"`
}

type XmlTemperature struct {
	ValidTime    time.Time `xml:"valid_time"`
	SurfaceTempC float64   `xml:"sfc_temp_c"`
	MaxTempC     *float64  `xml:"max_temp_c"`
	MinTempC     *float64  `xml:"min_temp_c"`
}

type XmlTafResponse struct {
	XMLName                   xml.Name `xml:"response"`
	Text                      string   `xml:",chardata"`
	Xsd                       string   `xml:"xsd,attr"`
	Xsi                       string   `xml:"xsi,attr"`
	Version                   string   `xml:"version,attr"`
	NoNamespaceSchemaLocation string   `xml:"noNamespaceSchemaLocation,attr"`
	RequestIndex              string   `xml:"request_index"`
	DataSource                struct {
		Text string `xml:",chardata"`
		Name string `xml:"name,attr"`
	} `xml:"data_source"`
	Request struct {
		Text string `xml:",chardata"`
		Type string `xml:"type,attr"`
	} `xml:"request"`
	Errors      string `xml:"errors"`
	Warnings    string `xml:"warnings"`
	TimeTakenMs string `xml:"time_taken_ms"`
	Data        struct {
		Text       string   `xml:",chardata"`
		NumResults string   `xml:"num_results,attr"`
		TAF        []XmlTaf `xml:"TAF"`
	} `xml:"data"`
}
