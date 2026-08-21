package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"strings"
	"time"

	// Embed the timezone database, since the Docker images for this tool are built FROM scratch and
	// therefore have no system zoneinfo for time.LoadLocation to read.
	_ "time/tzdata"

	"github.com/arran4/golang-ical"
	"github.com/avast/retry-go"
)

// ProductVersion is the application version, set during the build process by the Makefile.
var ProductVersion = "<dev>"

// ProductID identifies this software in User-Agents and iCal fields.
const ProductID = "github.com/cdzombak/wxcal"

// MaxSunDays is the largest accepted -sunDays value. Sunrise/sunset times are cheap to calculate,
// so this is only a guard against a mistyped value producing an enormous calendar.
const MaxSunDays = 36500

// CalendarForecastPeriod represents one period (daytime or nighttime) of a forecast entry on the calendar.
type CalendarForecastPeriod struct {
	IsPopulated      bool
	Name             string
	ShortForecast    string
	DetailedForecast string
	Temperature      int
	TemperatureUnit  string
}

// SummaryLine returns a brief, <1 line summary of the forecast period.
func (p CalendarForecastPeriod) SummaryLine() string {
	if !p.IsPopulated {
		return ""
	}
	sf := strings.Replace(p.ShortForecast, "Slight ", "", -1)
	sf = strings.Replace(sf, " then ", "; ", -1)
	sf = strings.Replace(sf, "Areas Of ", "", -1)
	return fmt.Sprintf("%dº%s %s", p.Temperature, p.TemperatureUnit, sf)
}

// CalendarForecastDay represents one day's forecast entry on the calendar.
type CalendarForecastDay struct {
	Start           time.Time
	DaytimePeriod   CalendarForecastPeriod
	NighttimePeriod CalendarForecastPeriod
	Sun             SunDay
}

// SummaryLine returns a brief, 1 line summary of the day's forecast.
func (d CalendarForecastDay) SummaryLine() string {
	daySummary := d.DaytimePeriod.SummaryLine()
	nightSummary := d.NighttimePeriod.SummaryLine()

	if len(daySummary) > 0 && len(nightSummary) > 0 {
		return fmt.Sprintf("%s | %s", daySummary, nightSummary)
	} else if len(nightSummary) > 0 {
		return fmt.Sprintf("%s: %s", d.NighttimePeriod.Name, nightSummary)
	}
	return daySummary
}

// DetailedForecast returns a more detailed version of the day's forecast.
func (d CalendarForecastDay) DetailedForecast() string {
	if d.DaytimePeriod.IsPopulated && d.NighttimePeriod.IsPopulated {
		return fmt.Sprintf("%s\\n\\nOvernight: %s", d.DaytimePeriod.DetailedForecast, d.NighttimePeriod.DetailedForecast)
	} else if d.NighttimePeriod.IsPopulated {
		return fmt.Sprintf("%s: %s", d.NighttimePeriod.Name, d.NighttimePeriod.DetailedForecast)
	}
	return d.DaytimePeriod.DetailedForecast
}

// DatesEqual returns true if the two given times are on the same day; false otherwise.
func DatesEqual(date1, date2 time.Time) bool {
	y1, m1, d1 := date1.Date()
	y2, m2, d2 := date2.Date()
	return y1 == y2 && m1 == m2 && d1 == d2
}

// CalendarForecast represents a collection of daily forecasts, to be rendered to calendar entries.
type CalendarForecast []CalendarForecastDay

// IndexForTime returns the index of the CalendarForecastDay for the given date, or -1 if the forecast calendar
// does not yet include the given date. The boolean return value indicates whether the date was found.
func (cf CalendarForecast) IndexForTime(t time.Time) (int, bool) {
	for i, p := range cf {
		if DatesEqual(p.Start, t) {
			return i, true
		}
	}
	return -1, false
}

func buildCalendarID(calLocation string, calDomain string, lat float64, lon float64, isSunCal bool) string {
	calLocation = strings.Replace(calLocation, " ", "-", -1)
	calLocation = strings.Replace(calLocation, ",", "", -1)
	if isSunCal {
		calLocation += "-Sun"
	}
	return fmt.Sprintf("%s{%.2f,%.2f}@%s",
		strings.ToLower(calLocation),
		lat, lon,
		strings.ToLower(calDomain))
}

// ICalOpts holds options for iCal generation
type ICalOpts struct {
	CalLocation    string
	CalDomain      string
	EvtTitlePrefix string
}

// OutputOpts holds output options for the program
type OutputOpts struct {
	ICalOutfile    string
	SunICalOutfile string
}

// Opts represents the command-line options for the wxcal program
type Opts struct {
	Lat      float64
	Lon      float64
	Timezone string
	SunDays  int
	ICal     ICalOpts
	Out      OutputOpts
	WxAPI    WxGovAPIOpts
}

func (o Opts) iCalFmtProductID() string {
	return fmt.Sprintf("-//%s-%s//EN", ProductID, ProductVersion)
}

func (o Opts) forecastLink() string {
	return fmt.Sprintf("https://forecast.weather.gov/MapClick.php?textField1=%.2f&textField2=%.2f", o.Lat, o.Lon)
}

// Main implements the wxcal program.
func Main(opts Opts) error {
	var forecastResp *ForecastResponse
	pointsTZ := ""

	if opts.Out.ICalOutfile != "" {
		err := retry.Do(
			func() (err error) {
				forecastResp, pointsTZ, err = GetForecast(&opts.WxAPI, opts.Lat, opts.Lon)
				return
			},
			retry.Attempts(3),
			retry.Delay(20*time.Second),
		)
		if err != nil {
			return fmt.Errorf("failed to get forecast: %w", err)
		}
	}

	loc, err := ResolveTimezone(opts.Timezone, pointsTZ, opts.Lat, opts.Lon)
	if err != nil {
		return err
	}

	if forecastResp != nil {
		if err := writeForecastCalendar(opts, forecastResp, loc); err != nil {
			return err
		}
	}

	if opts.Out.SunICalOutfile != "" {
		if err := writeSunCalendar(opts, loc); err != nil {
			return err
		}
	}

	return nil
}

// buildCalendarForecast summarizes the given forecast response as it will be used to build a calendar.
func buildCalendarForecast(forecastResp *ForecastResponse, lat float64, lon float64, loc *time.Location) CalendarForecast {
	cf := CalendarForecast{}
	for _, forecastPeriod := range forecastResp.Properties.ForecastPeriods {
		calDay := CalendarForecastDay{}
		i, existed := cf.IndexForTime(forecastPeriod.StartTime)
		if existed {
			calDay = cf[i]
		}
		calDay.Start = time.Date(forecastPeriod.StartTime.Year(), forecastPeriod.StartTime.Month(), forecastPeriod.StartTime.Day(), 0, 0, 0, 0, forecastPeriod.StartTime.Location())
		calPeriod := CalendarForecastPeriod{
			IsPopulated:      true,
			Name:             forecastPeriod.Name,
			ShortForecast:    forecastPeriod.ShortForecast,
			DetailedForecast: forecastPeriod.DetailedForecast,
			Temperature:      mustInt(forecastPeriod.Temperature),
			TemperatureUnit:  forecastPeriod.TemperatureUnit,
		}
		if forecastPeriod.Daytime {
			calDay.DaytimePeriod = calPeriod
		} else {
			calDay.NighttimePeriod = calPeriod
		}
		if !existed {
			// Interpret the API's calendar date in loc directly. Handing SunDayFor the instant
			// instead would convert it, shifting the date whenever loc is west of the UTC offset
			// the API reported (which -timezone can arrange).
			calDay.Sun = SunDayFor(lat, lon, loc,
				time.Date(calDay.Start.Year(), calDay.Start.Month(), calDay.Start.Day(), 12, 0, 0, 0, loc))
		}
		if existed {
			cf[i] = calDay
		} else {
			cf = append(cf, calDay)
		}
	}
	return cf
}

// writeForecastCalendar renders the given forecast to the iCal file named by opts.
func writeForecastCalendar(opts Opts, forecastResp *ForecastResponse, loc *time.Location) error {
	cf := buildCalendarForecast(forecastResp, opts.Lat, opts.Lon, loc)

	nowTime := time.Now()
	forecastLink := opts.forecastLink()
	calID := buildCalendarID(opts.ICal.CalLocation, opts.ICal.CalDomain, opts.Lat, opts.Lon, false)

	cal := ics.NewCalendar()
	cal.SetName(fmt.Sprintf("%s Weather", opts.ICal.CalLocation))
	cal.SetXWRCalName(fmt.Sprintf("%s Weather", opts.ICal.CalLocation))
	cal.SetDescription(fmt.Sprintf("Weather forecast for the next week in %s, provided by weather.gov.", opts.ICal.CalLocation))
	cal.SetXWRCalDesc(fmt.Sprintf("Weather forecast for the next week in %s, provided by weather.gov.", opts.ICal.CalLocation))
	cal.SetLastModified(forecastResp.Properties.Updated)
	cal.SetMethod(ics.MethodPublish)
	cal.SetProductId(opts.iCalFmtProductID())
	cal.SetVersion("2.0")
	cal.SetXPublishedTTL("PT1H")
	cal.SetRefreshInterval("PT1H")

	for _, d := range cf {
		event := cal.AddEvent(fmt.Sprintf("%s-%s", d.Start.Format("20060102"), calID))
		event.SetDtStampTime(nowTime)
		event.SetModifiedAt(forecastResp.Properties.Updated)
		event.SetAllDayStartAt(d.Start)
		event.SetAllDayEndAt(d.Start) // one-day all-day event ends the same day it started
		event.SetLocation(opts.ICal.CalLocation)
		event.SetURL(forecastLink)
		evtSummary := d.SummaryLine()
		if len(opts.ICal.EvtTitlePrefix) > 0 {
			evtSummary = fmt.Sprintf("%s %s", opts.ICal.EvtTitlePrefix, evtSummary)
		}
		event.SetSummary(evtSummary)
		event.SetDescription(fmt.Sprintf("%s\\n\\n%s\\n\\nForecast Detail: %s",
			d.DetailedForecast(),
			d.Sun.DetailLines(),
			forecastLink,
		))
	}

	// TODO(cdzombak): make perm configurable
	if err := os.WriteFile(opts.Out.ICalOutfile, []byte(cal.Serialize()), 0644); err != nil {
		return fmt.Errorf("failed to write output file '%s': %w", opts.Out.ICalOutfile, err)
	}
	return nil
}

// writeSunCalendar renders a sunrise/sunset calendar covering opts.SunDays days, beginning today,
// to the iCal file named by opts.
func writeSunCalendar(opts Opts, loc *time.Location) error {
	nowTime := time.Now()
	sunDays := SunDays(opts.Lat, opts.Lon, loc, nowTime, opts.SunDays)
	calID := buildCalendarID(opts.ICal.CalLocation, opts.ICal.CalDomain, opts.Lat, opts.Lon, true)

	calDesc := fmt.Sprintf("Sunrise/sunset times for the next %d days in %s.", opts.SunDays, opts.ICal.CalLocation)

	cal := ics.NewCalendar()
	cal.SetName(fmt.Sprintf("%s Sunrise/Sunset", opts.ICal.CalLocation))
	cal.SetXWRCalName(fmt.Sprintf("%s Sunrise/Sunset", opts.ICal.CalLocation))
	cal.SetDescription(calDesc)
	cal.SetXWRCalDesc(calDesc)
	cal.SetLastModified(nowTime)
	cal.SetMethod(ics.MethodPublish)
	cal.SetProductId(opts.iCalFmtProductID())
	cal.SetVersion("2.0")
	cal.SetXPublishedTTL("P1D")
	cal.SetRefreshInterval("P1D")

	for _, d := range sunDays {
		event := cal.AddEvent(fmt.Sprintf("%s-%s", d.Date.Format("20060102"), calID))
		event.SetDtStampTime(nowTime)
		event.SetModifiedAt(nowTime)
		event.SetAllDayStartAt(d.Date)
		event.SetAllDayEndAt(d.Date) // one-day all-day event ends the same day it started
		event.SetLocation(opts.ICal.CalLocation)
		evtSummary := d.SummaryLine()
		if len(opts.ICal.EvtTitlePrefix) > 0 {
			evtSummary = fmt.Sprintf("%s %s", opts.ICal.EvtTitlePrefix, evtSummary)
		}
		event.SetSummary(evtSummary)
		event.SetDescription(d.DetailLines())
	}

	// TODO(cdzombak): make perm configurable
	if err := os.WriteFile(opts.Out.SunICalOutfile, []byte(cal.Serialize()), 0644); err != nil {
		return fmt.Errorf("failed to write output file '%s': %w", opts.Out.SunICalOutfile, err)
	}
	return nil
}

func main() {
	var calLocation = flag.String("calLocation", "", "The name of the calendar's location (eg. \"Ann Arbor, MI\") (required)")
	var calDomain = flag.String("calDomain", "", "The calendar's domain (eg. \"ical.dzombak.com\") (required)")
	var evtTitlePrefix = flag.String("evtTitlePrefix", "", "An optional prefix to be inserted before each event's title")
	var lat = flag.Float64("lat", 42.27, "The forecast location's latitude (eg. \"42.27\")")
	var lon = flag.Float64("lon", -83.74, "The forecast location's longitude (eg. \"-83.74\")")
	var icalOutfile = flag.String("icalFile", "", "Path/filename for the weather forecast iCal output file (at least one of -icalFile/-sunIcalFile is required)")
	var sunICalOutfile = flag.String("sunIcalFile", "", "Path/filename for the sunrise/sunset iCal output file (at least one of -icalFile/-sunIcalFile is required)")
	var sunDays = flag.Int("sunDays", 7, "The number of days, counting today, to include in the sunrise/sunset calendar")
	var timezone = flag.String("timezone", "", "IANA timezone name for the sunrise/sunset times in both calendars (eg. \"America/Detroit\"); if omitted, the timezone is determined from the forecast API or the given lat/lon")
	var uaEmail = flag.String("uaEmail", "", "Email address to include in the User-Agent header for api.weather.gov requests")
	var forceIpv4 = flag.Bool("forceIpv4", false, "Force IPv4 for api.weather.gov requests")
	var printVersion = flag.Bool("version", false, "Print version and exit")
	flag.Parse()

	if *printVersion {
		fmt.Println(ProductVersion)
		os.Exit(0)
	}

	if *calLocation == "" || *calDomain == "" || (*icalOutfile == "" && *sunICalOutfile == "") {
		flag.PrintDefaults()
		os.Exit(1)
	}

	if *sunDays < 1 || *sunDays > MaxSunDays {
		fmt.Printf("-sunDays must be between 1 and %d\n", MaxSunDays)
		os.Exit(1)
	}

	if err := Main(Opts{
		Lat:      *lat,
		Lon:      *lon,
		Timezone: *timezone,
		SunDays:  *sunDays,
		ICal: ICalOpts{
			CalLocation:    *calLocation,
			CalDomain:      *calDomain,
			EvtTitlePrefix: *evtTitlePrefix,
		},
		Out: OutputOpts{
			ICalOutfile:    *icalOutfile,
			SunICalOutfile: *sunICalOutfile,
		},
		WxAPI: WxGovAPIOpts{
			ForceIpv4: *forceIpv4,
			UaEmail:   *uaEmail,
		},
	}); err != nil {
		log.Fatalf("%s", err.Error())
	}
}
