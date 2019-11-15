package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"time"
)

// API client for a subset of https://www.weather.gov/documentation/services-web-api

// PointsResponse represents a subset of a response from the /points/{point} API
// https://www.weather.gov/documentation/services-web-api#/default/get_points__point_
type PointsResponse struct {
	Properties struct {
		ForecastURL string `json:"forecast"`
	} `json:"properties"`
}

// ForecastPeriod represents a forecast for a specific time period from the weather.gov forecast API
type ForecastPeriod struct {
	Number           int       `json:"number"`
	Name             string    `json:"name"`
	StartTime        time.Time `json:"startTime"`
	EndTime          time.Time `json:"endTime"`
	Daytime          bool      `json:"isDaytime"`
	Temperature      int       `json:"temperature"`
	TemperatureUnit  string    `json:"temperatureUnit"`
	WindSpeed        string    `json:"windSpeed"`
	WindDirection    string    `json:"windDirection"`
	ShortForecast    string    `json:"shortForecast"`
	DetailedForecast string    `json:"detailedForecast"`
}

// ForecastResponse represents a subset of a response from the /gridpoints/{x},{y}/forecast API
// https://www.weather.gov/documentation/services-web-api#/default/get_gridpoints__wfo___x___y__forecast
type ForecastResponse struct {
	Updated    time.Time `json:"updated"`
	Properties struct {
		ForecastPeriods []ForecastPeriod `json:"periods"`
	} `json:"properties"`
}

const typeGeoJSON = "application/geo+json"
const userAgent = ProductID + ";chris@dzombak.com"
const apiTimeout = 5 * time.Second

// MakeHTTPClient returns an http.Client configured for use with the weather.gov forecast API
// (including the necessary headers)
func MakeHTTPClient() *http.Client {
	httpClient := &http.Client{Timeout: apiTimeout}
	rt := withHeader(httpClient.Transport)
	rt.Set("Accept", typeGeoJSON)
	rt.Set("User-Agent", userAgent)
	httpClient.Transport = rt
	return httpClient
}

// DoJSONRequest performs the given API request, decoding the body as JSON into the given respBody.
func DoJSONRequest(httpClient *http.Client, req *http.Request, respBody interface{}) error {
	resp, err := httpClient.Do(req)
	if err != nil {
		return err
	}
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return err
	}
	err = json.Unmarshal(body, &respBody)
	if err != nil {
		return err
	}
	return nil
}

// GetForecast returns a ForecastResponse representing the weather.gov forecast for the given latitude/longitude.
func GetForecast(lat float64, lon float64) (*ForecastResponse, error) {
	httpClient := MakeHTTPClient()

	reqURL := fmt.Sprintf("https://api.weather.gov/points/%.2f,%.2f", lat, lon)
	req, err := http.NewRequest(http.MethodGet, reqURL, nil)
	if err != nil {
		return nil, err
	}

	pointsResp := PointsResponse{}
	if err = DoJSONRequest(httpClient, req, &pointsResp); err != nil {
		return nil, err
	}

	reqURL = pointsResp.Properties.ForecastURL
	req, err = http.NewRequest(http.MethodGet, reqURL, nil)
	if err != nil {
		return nil, err
	}

	forecastResp := ForecastResponse{}
	if err = DoJSONRequest(httpClient, req, &forecastResp); err != nil {
		return nil, err
	}

	return &forecastResp, nil
}

// headerSettingRoundTripper is a RoundTripper transport which automatically sets headers on every request.
type headerSettingRoundTripper struct {
	http.Header
	rt http.RoundTripper
}

func withHeader(rt http.RoundTripper) headerSettingRoundTripper {
	if rt == nil {
		rt = http.DefaultTransport
	}
	return headerSettingRoundTripper{
		Header: make(http.Header),
		rt:     rt,
	}
}

// RoundTrip conforms headerSettingRoundTripper to http.RoundTripper.
func (h headerSettingRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	for k, v := range h.Header {
		req.Header[k] = v
	}
	return h.rt.RoundTrip(req)
}
