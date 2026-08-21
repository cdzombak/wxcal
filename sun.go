package main

import (
	"fmt"
	"time"

	sunrise "github.com/nathan-osman/go-sunrise"
)

// SunDay represents the sunrise/sunset times for a single local calendar day.
//
// Above the Arctic Circle and below the Antarctic Circle, there are days on which the sun neither
// rises nor sets. On those days Sunrise and Sunset are zero and exactly one of AlwaysUp (polar day)
// or AlwaysDown (polar night) is true.
type SunDay struct {
	Date       time.Time
	Sunrise    time.Time
	Sunset     time.Time
	AlwaysUp   bool
	AlwaysDown bool
}

// HasRiseSet returns true if the sun both rises and sets on this day.
func (d SunDay) HasRiseSet() bool {
	return !d.AlwaysUp && !d.AlwaysDown
}

// SummaryLine returns a brief, 1 line summary of the day's sunrise/sunset times.
func (d SunDay) SummaryLine() string {
	if d.AlwaysUp {
		return "☼ Sun up all day"
	}
	if d.AlwaysDown {
		return "☼ Sun down all day"
	}
	return fmt.Sprintf("☼ ↑ %s | ↓ %s",
		d.Sunrise.Round(time.Minute).Format("3:04 PM"),
		d.Sunset.Round(time.Minute).Format("3:04 PM"),
	)
}

// DetailLines returns a more detailed description of the day's sunrise/sunset times.
func (d SunDay) DetailLines() string {
	if d.AlwaysUp {
		return "The sun does not set today."
	}
	if d.AlwaysDown {
		return "The sun does not rise today."
	}
	return fmt.Sprintf("Sunrise: %s\\nSunset: %s",
		d.Sunrise.Format("3:04:05 PM"),
		d.Sunset.Format("3:04:05 PM"),
	)
}

// SunDayFor calculates the sunrise/sunset times for the local calendar day containing the given
// time, at the given latitude/longitude. Returned times are in the given location.
func SunDayFor(lat, lon float64, loc *time.Location, day time.Time) SunDay {
	day = day.In(loc)
	result := SunDay{
		Date: time.Date(day.Year(), day.Month(), day.Day(), 0, 0, 0, 0, loc),
	}

	// go-sunrise interprets its date arguments as a UTC calendar date. Anchoring on the UTC instant
	// of local noon keeps each result on the local day it belongs to, including in zones whose civil
	// time is far from their solar time (eg. UTC+13/+14, where using the local date directly places
	// every event on the wrong day).
	anchor := time.Date(day.Year(), day.Month(), day.Day(), 12, 0, 0, 0, loc).UTC()

	rise, set := sunrise.SunriseSunset(lat, lon, anchor.Year(), anchor.Month(), anchor.Day())
	if rise.IsZero() || set.IsZero() {
		// The sun neither rises nor sets today, so it is either above or below the horizon for the
		// whole day; its elevation at any moment of the day tells us which.
		if sunrise.Elevation(lat, lon, anchor) > 0 {
			result.AlwaysUp = true
		} else {
			result.AlwaysDown = true
		}
		return result
	}

	result.Sunrise = rise.In(loc)
	result.Sunset = set.In(loc)
	return result
}

// SunDays calculates the sunrise/sunset times for count consecutive local calendar days, beginning
// with the local day containing start.
func SunDays(lat, lon float64, loc *time.Location, start time.Time, count int) []SunDay {
	start = start.In(loc)
	first := time.Date(start.Year(), start.Month(), start.Day(), 0, 0, 0, 0, loc)

	days := make([]SunDay, 0, count)
	for i := 0; i < count; i++ {
		// AddDate, not Add(24*time.Hour): the latter drifts across DST transitions.
		days = append(days, SunDayFor(lat, lon, loc, first.AddDate(0, 0, i)))
	}
	return days
}
