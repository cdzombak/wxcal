package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func testForecastResponse(t *testing.T, loc *time.Location, day time.Time) *ForecastResponse {
	t.Helper()
	next := day.AddDate(0, 0, 1)
	resp := &ForecastResponse{}
	resp.Properties.Updated = day.UTC()
	resp.Properties.ForecastPeriods = []ForecastPeriod{
		{
			Number: 1, Name: "Today", Daytime: true,
			StartTime:        time.Date(day.Year(), day.Month(), day.Day(), 6, 0, 0, 0, loc),
			Temperature:      "58",
			TemperatureUnit:  "F",
			ShortForecast:    "Slight Chance Rain Showers",
			DetailedForecast: "A slight chance of rain showers.",
		},
		{
			Number: 2, Name: "Tonight", Daytime: false,
			StartTime:        time.Date(day.Year(), day.Month(), day.Day(), 18, 0, 0, 0, loc),
			Temperature:      "41",
			TemperatureUnit:  "F",
			ShortForecast:    "Mostly Cloudy",
			DetailedForecast: "Mostly cloudy, with a low around 41.",
		},
		{
			Number: 3, Name: "Monday", Daytime: true,
			StartTime:        time.Date(next.Year(), next.Month(), next.Day(), 6, 0, 0, 0, loc),
			Temperature:      "60",
			TemperatureUnit:  "F",
			ShortForecast:    "Sunny",
			DetailedForecast: "Sunny, with a high near 60.",
		},
	}
	return resp
}

func writeTestForecast(t *testing.T, opts Opts, apiLoc *time.Location, loc *time.Location, day time.Time) string {
	t.Helper()
	outfile := filepath.Join(t.TempDir(), "wx.ics")
	opts.Out.ICalOutfile = outfile
	if err := writeForecastCalendar(opts, testForecastResponse(t, apiLoc, day), loc); err != nil {
		t.Fatalf("writeForecastCalendar: %s", err)
	}
	content, err := os.ReadFile(outfile)
	if err != nil {
		t.Fatalf("reading output: %s", err)
	}
	return unfoldICS(string(content))
}

// unfoldICS undoes RFC 5545 line folding, so that assertions can match against content which the
// serializer may have split across lines.
func unfoldICS(s string) string {
	s = strings.ReplaceAll(s, "\r\n ", "")
	return strings.ReplaceAll(s, "\n ", "")
}

// TestForecastCalendarIncludesSunTimes verifies that the forecast calendar's event descriptions
// carry sunrise/sunset times for the correct local day, in the timezone actually in effect.
func TestForecastCalendarIncludesSunTimes(t *testing.T) {
	loc := mustLoadLocation(t, "America/Detroit")
	opts := Opts{
		Lat: 42.27, Lon: -83.74,
		ICal: ICalOpts{CalLocation: "Ann Arbor, MI", CalDomain: "ics.dzombak.com"},
	}

	// 2026-11-01 is the "fall back" DST transition.
	content := writeTestForecast(t, opts, loc, loc, time.Date(2026, 11, 1, 0, 0, 0, 0, loc))

	if got := strings.Count(content, "BEGIN:VEVENT"); got != 2 {
		t.Errorf("got %d events; want 2 (one per forecast day)", got)
	}
	// These are the correct EST times for the two days.
	for _, want := range []string{
		"Sunrise: 7:08:00 AM\\nSunset: 5:28:57 PM",
		"Sunrise: 7:09:15 AM\\nSunset: 5:27:41 PM",
	} {
		if !strings.Contains(content, want) {
			t.Errorf("output is missing %q", want)
		}
	}
	// The forecast calendar keeps its hourly refresh interval.
	for _, want := range []string{"X-PUBLISHED-TTL:PT1H", "REFRESH-INTERVAL;VALUE=DURATION:PT1H"} {
		if !strings.Contains(content, want) {
			t.Errorf("output is missing %q", want)
		}
	}
	// Day and night periods for the same day are merged into one event.
	if !strings.Contains(content, "SUMMARY:58ºF Chance Rain Showers | 41ºF Mostly Cloudy") {
		t.Error("output is missing the merged day/night summary")
	}
}

// TestForecastCalendarPolar verifies that a forecast day on which the sun does not rise is
// described in words rather than as a zero time.
func TestForecastCalendarPolar(t *testing.T) {
	loc := mustLoadLocation(t, "America/Anchorage")
	opts := Opts{
		Lat: 71.29, Lon: -156.79, // Utqiagvik, AK
		ICal: ICalOpts{CalLocation: "Utqiagvik, AK", CalDomain: "ics.dzombak.com"},
	}

	// Utqiagvik's polar night runs from roughly mid-November to late January.
	content := writeTestForecast(t, opts, loc, loc, time.Date(2026, 12, 21, 0, 0, 0, 0, loc))

	if !strings.Contains(content, "The sun does not rise today.") {
		t.Error("output is missing the polar night description")
	}
	if strings.Contains(content, "Sunrise: 12:00:00 AM") {
		t.Error("output contains a zero sunrise time")
	}
}

// TestForecastCalendarRespectsTimezoneOverride verifies that each forecast event's sunrise/sunset
// lines describe that event's own date. The weather.gov API returns times in the forecast
// location's own UTC offset; converting that instant into a -timezone zone further west lands on
// the previous day, which would attach the wrong day's sun times to the event.
func TestForecastCalendarRespectsTimezoneOverride(t *testing.T) {
	// The forecast is for Michigan, but the user asked for Pacific times.
	apiLoc := mustLoadLocation(t, "America/Detroit")
	loc := mustLoadLocation(t, "America/Los_Angeles")
	opts := Opts{
		Lat: 42.27, Lon: -83.74,
		ICal: ICalOpts{CalLocation: "Ann Arbor, MI", CalDomain: "ics.dzombak.com"},
	}

	content := writeTestForecast(t, opts, apiLoc, loc, time.Date(2026, 7, 15, 0, 0, 0, 0, apiLoc))

	// Sunrise in Ann Arbor on 2026-07-15, expressed in Pacific time. The event for the 15th must
	// carry the 15th's sun times, not the 14th's.
	want := SunDayFor(opts.Lat, opts.Lon, loc, time.Date(2026, 7, 15, 12, 0, 0, 0, loc))
	if !strings.Contains(content, want.DetailLines()) {
		t.Errorf("the 2026-07-15 event is missing that day's sun times (%q)", want.DetailLines())
	}
	notWant := SunDayFor(opts.Lat, opts.Lon, loc, time.Date(2026, 7, 14, 12, 0, 0, 0, loc))
	if strings.Contains(content, notWant.DetailLines()) {
		t.Errorf("the calendar contains 2026-07-14's sun times (%q)", notWant.DetailLines())
	}
}
