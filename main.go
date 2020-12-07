package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"strings"
	"time"

	"github.com/arran4/golang-ical"
	"github.com/avast/retry-go"
)

var version = "<dev>"

// ProductID identifies this software in User-Agents and iCal fields.
const ProductID = "github.com/cdzombak/wxcal"

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
}

// SummaryLine returns a brief, 1 line summary of the day's forecast.
func (d CalendarForecastDay) SummaryLine() string {
	daySummary := d.DaytimePeriod.SummaryLine()
	nightSummary := d.NighttimePeriod.SummaryLine()

	if len(daySummary) > 0 && len(nightSummary) > 0 {
		return fmt.Sprintf("%s | %s", daySummary, nightSummary)
	} else if len(nightSummary) > 0 {
		return fmt.Sprintf("%s: %s", d.NighttimePeriod.Name, nightSummary)
	} else {
		return daySummary
	}
}

// DetailedForecast returns a more detailed version of the day's forecast.
func (d CalendarForecastDay) DetailedForecast() string {
	if d.DaytimePeriod.IsPopulated && d.NighttimePeriod.IsPopulated {
		return fmt.Sprintf("%s\\n\\nOvernight: %s", d.DaytimePeriod.DetailedForecast, d.NighttimePeriod.DetailedForecast)
	} else if d.NighttimePeriod.IsPopulated {
		return fmt.Sprintf("%s: %s", d.NighttimePeriod.Name, d.NighttimePeriod.DetailedForecast)
	} else {
		return d.DaytimePeriod.DetailedForecast
	}
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

func buildCalendarID(calLocation string, calDomain string, lat float64, lon float64) string {
	calLocation = strings.Replace(calLocation, " ", "-", -1)
	calLocation = strings.Replace(calLocation, ",", "", -1)
	return fmt.Sprintf("%s{%.2f,%.2f}@%s",
		strings.ToLower(calLocation),
		lat, lon,
		strings.ToLower(calDomain))
}

func mustInt(x json.Number) int {
	xi64, err := x.Int64()
	if err != nil {
		panic(err)
	}
	return int(xi64)
}

// Main implements the wxcal program.
func Main(calLocation string, calDomain string, lat float64, lon float64, evtTitlePrefix string, icalOutfile string) error {
	var forecastResp *ForecastResponse
	err := retry.Do(
		func() (err error) {
			forecastResp, err = GetForecast(lat, lon)
			return
		},
		retry.Attempts(3),
		retry.Delay(20*time.Second),
	)
	if err != nil {
		return fmt.Errorf("failed to get forecast: %w", err)
	}

	// build a structure summarizing the data as we'll use it to build a calendar:
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
		if existed {
			cf[i] = calDay
		} else {
			cf = append(cf, calDay)
		}
	}

	forecastLink := fmt.Sprintf("https://forecast.weather.gov/MapClick.php?textField1=%.2f&textField2=%.2f", lat, lon)

	calID := buildCalendarID(calLocation, calDomain, lat, lon)
	cal := ics.NewCalendar()
	cal.SetName(fmt.Sprintf("%s Weather", calLocation))
	cal.SetXWRCalName(fmt.Sprintf("%s Weather", calLocation))
	cal.SetDescription(fmt.Sprintf("Weather forecast for the next week in %s, provided by weather.gov.", calLocation))
	cal.SetXWRCalDesc(fmt.Sprintf("Weather forecast for the next week in %s, provided by weather.gov.", calLocation))
	cal.SetLastModified(forecastResp.Updated)
	cal.SetMethod(ics.MethodPublish)
	cal.SetProductId(fmt.Sprintf("-//%s//EN", ProductID))
	cal.SetVersion("2.0")
	cal.SetXPublishedTTL("PT1H")
	cal.SetRefreshInterval("PT1H")

	for _, d := range cf {
		event := cal.AddEvent(fmt.Sprintf("%s-%s", d.Start.Format("20060102"), calID))
		event.SetDtStampTime(time.Now())
		event.SetModifiedAt(forecastResp.Updated)
		event.SetAllDayStartAt(d.Start)
		event.SetAllDayEndAt(d.Start) // one-day all-day event ends the same day it started
		event.SetLocation(calLocation)
		event.SetURL(forecastLink)
		evtSummary := d.SummaryLine()
		if len(evtTitlePrefix) > 0 {
			evtSummary = fmt.Sprintf("%s %s", evtTitlePrefix, evtSummary)
		}
		event.SetSummary(evtSummary)
		event.SetDescription(fmt.Sprintf("%s\\n\\nForecast Detail: %s", d.DetailedForecast(), forecastLink))
	}

	err = ioutil.WriteFile(icalOutfile, []byte(cal.Serialize()), 0644)
	if err != nil {
		return fmt.Errorf("failed to write output file '%s': %w", icalOutfile, err)
	}

	return nil
}

func main() {
	var calLocation = flag.String("calLocation", "", "The name of the calendar's location (eg. \"Ann Arbor, MI\") (required)")
	var calendarDomain = flag.String("calDomain", "", "The calendar's domain (eg. \"ical.dzombak.com\") (required)")
	var evtTitlePrefix = flag.String("evtTitlePrefix", "", "An optional prefix to be inserted before each event's title")
	var lat = flag.Float64("lat", 42.27, "The forecast location's latitude (eg. \"42.27\")")
	var lon = flag.Float64("lon", -83.74, "The forecast location's longitude (eg. \"-83.74\")")
	var icalOutfile = flag.String("icalFile", "", "Path/filename for iCal output file (required)")
	var printVersion = flag.Bool("version", false, "Print version and exit")
	flag.Parse()

	if *printVersion {
		fmt.Println(version)
		os.Exit(0)
	}

	if *calLocation == "" || *calendarDomain == "" || *icalOutfile == "" {
		flag.PrintDefaults()
		os.Exit(1)
	}

	if err := Main(*calLocation, *calendarDomain, *lat, *lon, *evtTitlePrefix, *icalOutfile); err != nil {
		log.Fatalf(err.Error())
	}
}
