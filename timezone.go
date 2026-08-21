package main

import (
	"fmt"
	"time"

	"github.com/ringsaturn/tzf"
)

// ResolveTimezone determines the timezone to use for sunrise/sunset calculations, in this order:
//
//  1. tzFlag, the value of the -timezone flag, if given;
//  2. pointsTZ, the timezone reported by the weather.gov /points API, if a forecast was fetched;
//  3. a lookup of the given latitude/longitude in the tzf timezone boundary database.
func ResolveTimezone(tzFlag string, pointsTZ string, lat float64, lon float64) (*time.Location, error) {
	if tzFlag != "" {
		loc, err := time.LoadLocation(tzFlag)
		if err != nil {
			return nil, fmt.Errorf("invalid -timezone '%s': %w", tzFlag, err)
		}
		return loc, nil
	}

	if pointsTZ != "" {
		loc, err := time.LoadLocation(pointsTZ)
		if err != nil {
			return nil, fmt.Errorf("weather.gov returned an unusable timezone '%s': %w", pointsTZ, err)
		}
		return loc, nil
	}

	// Building the finder is comparatively expensive (~0.5s), so it's only done on this path.
	finder, err := tzf.NewDefaultFinder()
	if err != nil {
		return nil, fmt.Errorf("failed to initialize timezone lookup: %w", err)
	}
	// nb. tzf takes longitude first.
	name := finder.GetTimezoneName(lon, lat)
	if name == "" {
		return nil, fmt.Errorf("failed to determine the timezone for %.2f,%.2f; use -timezone to specify one", lat, lon)
	}
	loc, err := time.LoadLocation(name)
	if err != nil {
		return nil, fmt.Errorf("failed to load timezone '%s' for %.2f,%.2f: %w", name, lat, lon, err)
	}
	return loc, nil
}
