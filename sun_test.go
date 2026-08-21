package main

import (
	"testing"
	"time"
)

func mustLoadLocation(t *testing.T, name string) *time.Location {
	t.Helper()
	loc, err := time.LoadLocation(name)
	if err != nil {
		t.Fatalf("failed to load location %s: %s", name, err)
	}
	return loc
}

// TestSunDayForDST verifies that sunrise/sunset are reported in the local time actually in effect
// on the given day, including on days when a DST transition occurs. Calculating with a single UTC
// offset taken from the start of the day (as wxcal did before) reports these an hour off.
func TestSunDayForDST(t *testing.T) {
	loc := mustLoadLocation(t, "America/Detroit")
	lat, lon := 42.27, -83.74

	tests := []struct {
		date        string
		wantSunrise string
		wantSunset  string
		wantZone    string
	}{
		{"2026-03-07", "7:01 AM", "6:32 PM", "EST"}, // day before "spring forward"
		{"2026-03-08", "7:59 AM", "7:33 PM", "EDT"}, // "spring forward"
		{"2026-06-21", "5:59 AM", "9:15 PM", "EDT"}, // solstice, no transition
		{"2026-11-01", "7:08 AM", "5:29 PM", "EST"}, // "fall back"
		{"2026-11-02", "7:09 AM", "5:28 PM", "EST"}, // day after "fall back"
	}

	for _, tt := range tests {
		day, err := time.ParseInLocation("2006-01-02", tt.date, loc)
		if err != nil {
			t.Fatalf("bad test date %s: %s", tt.date, err)
		}
		got := SunDayFor(lat, lon, loc, day)
		if !got.HasRiseSet() {
			t.Errorf("%s: expected the sun to rise and set", tt.date)
			continue
		}
		gotSunrise := got.Sunrise.Round(time.Minute).Format("3:04 PM")
		gotSunset := got.Sunset.Round(time.Minute).Format("3:04 PM")
		if gotSunrise != tt.wantSunrise || gotSunset != tt.wantSunset {
			t.Errorf("%s: got sunrise %s / sunset %s; want %s / %s",
				tt.date, gotSunrise, gotSunset, tt.wantSunrise, tt.wantSunset)
		}
		if gotZone, _ := got.Sunrise.Zone(); gotZone != tt.wantZone {
			t.Errorf("%s: sunrise reported in %s; want %s", tt.date, gotZone, tt.wantZone)
		}
		if gotZone, _ := got.Sunset.Zone(); gotZone != tt.wantZone {
			t.Errorf("%s: sunset reported in %s; want %s", tt.date, gotZone, tt.wantZone)
		}
	}
}

// TestSunDayForLandsOnLocalDay verifies that each day's sunrise and sunset fall on the local
// calendar day the entry represents. Passing a local date straight through to go-sunrise (which
// expects a UTC date) puts every event a day off in zones far from their solar time.
func TestSunDayForLandsOnLocalDay(t *testing.T) {
	places := []struct {
		name     string
		lat, lon float64
		tz       string
	}{
		{"Ann Arbor", 42.27, -83.74, "America/Detroit"},
		{"Kiritimati", 1.87, -157.43, "Pacific/Kiritimati"}, // UTC+14
		{"Apia", -13.83, -171.77, "Pacific/Apia"},           // UTC+13
		{"Kashgar", 39.47, 75.99, "Asia/Shanghai"},          // civil time far from solar time
		{"Honolulu", 21.31, -157.86, "Pacific/Honolulu"},
		{"Sydney", -33.87, 151.21, "Australia/Sydney"},
		{"Madrid", 40.42, -3.70, "Europe/Madrid"},
	}

	for _, p := range places {
		loc := mustLoadLocation(t, p.tz)
		start := time.Date(2026, 1, 1, 0, 0, 0, 0, loc)
		for _, d := range SunDays(p.lat, p.lon, loc, start, 365) {
			if !d.HasRiseSet() {
				continue
			}
			wantY, wantM, wantD := d.Date.Date()
			for label, got := range map[string]time.Time{"sunrise": d.Sunrise, "sunset": d.Sunset} {
				y, m, dd := got.Date()
				if y != wantY || m != wantM || dd != wantD {
					t.Errorf("%s: %s for %s fell on %s",
						p.name, label, d.Date.Format("2006-01-02"), got.Format("2006-01-02 15:04 MST"))
				}
			}
		}
	}
}

// TestSunDayForPolar verifies that days with no sunrise or sunset are reported as polar day or
// polar night rather than as garbage times.
func TestSunDayForPolar(t *testing.T) {
	// Utqiagvik, AK
	loc := mustLoadLocation(t, "America/Anchorage")
	lat, lon := 71.29, -156.79

	summer := SunDayFor(lat, lon, loc, time.Date(2026, 6, 21, 0, 0, 0, 0, loc))
	if !summer.AlwaysUp || summer.AlwaysDown || summer.HasRiseSet() {
		t.Errorf("June solstice: expected polar day; got %+v", summer)
	}
	if got, want := summer.SummaryLine(), "☼ Sun up all day"; got != want {
		t.Errorf("June solstice: got summary %q; want %q", got, want)
	}

	winter := SunDayFor(lat, lon, loc, time.Date(2026, 12, 21, 0, 0, 0, 0, loc))
	if !winter.AlwaysDown || winter.AlwaysUp || winter.HasRiseSet() {
		t.Errorf("December solstice: expected polar night; got %+v", winter)
	}
	if got, want := winter.SummaryLine(), "☼ Sun down all day"; got != want {
		t.Errorf("December solstice: got summary %q; want %q", got, want)
	}

	// A day on which the sun does rise and set, for contrast:
	spring := SunDayFor(lat, lon, loc, time.Date(2026, 4, 15, 0, 0, 0, 0, loc))
	if !spring.HasRiseSet() {
		t.Errorf("April 15: expected the sun to rise and set; got %+v", spring)
	}
}

// TestSunDaysCoversConsecutiveLocalDays verifies that the requested number of days is returned, and
// that days advance by one local calendar day even across a DST transition.
func TestSunDaysCoversConsecutiveLocalDays(t *testing.T) {
	loc := mustLoadLocation(t, "America/Detroit")
	// Start a few days ahead of the "spring forward" transition:
	start := time.Date(2026, 3, 5, 22, 30, 0, 0, loc)

	days := SunDays(42.27, -83.74, loc, start, 400)
	if len(days) != 400 {
		t.Fatalf("got %d days; want 400", len(days))
	}

	want := time.Date(2026, 3, 5, 0, 0, 0, 0, loc)
	for i, d := range days {
		if !d.Date.Equal(want) {
			t.Fatalf("day %d: got %s; want %s", i, d.Date.Format(time.RFC3339), want.Format(time.RFC3339))
		}
		if h, m, s := d.Date.Clock(); h != 0 || m != 0 || s != 0 {
			t.Errorf("day %d: got %s; want local midnight", i, d.Date.Format(time.RFC3339))
		}
		want = want.AddDate(0, 0, 1)
	}
}

// TestResolveTimezone covers the -timezone flag, the weather.gov-provided timezone, and the
// lat/lon lookup fallback.
func TestResolveTimezone(t *testing.T) {
	tests := []struct {
		name     string
		tzFlag   string
		pointsTZ string
		lat, lon float64
		want     string
	}{
		{"flag wins", "America/New_York", "America/Detroit", 42.27, -83.74, "America/New_York"},
		{"points used when no flag", "", "America/Detroit", 21.31, -157.86, "America/Detroit"},
		{"lookup when neither given", "", "", 42.27, -83.74, "America/Detroit"},
		{"lookup honolulu", "", "", 21.31, -157.86, "Pacific/Honolulu"},
		{"lookup sydney", "", "", -33.87, 151.21, "Australia/Sydney"},
		{"lookup london", "", "", 51.51, -0.13, "Europe/London"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			loc, err := ResolveTimezone(tt.tzFlag, tt.pointsTZ, tt.lat, tt.lon)
			if err != nil {
				t.Fatalf("unexpected error: %s", err)
			}
			if loc.String() != tt.want {
				t.Errorf("got %s; want %s", loc, tt.want)
			}
		})
	}

	if _, err := ResolveTimezone("Not/AZone", "", 42.27, -83.74); err == nil {
		t.Error("expected an error for an invalid -timezone value")
	}
}
