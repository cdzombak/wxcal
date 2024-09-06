package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net"
	"net/http"
	"time"
)

// TODO(cdzombak): refactor this into an actual struct

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
	Number           int         `json:"number"`
	Name             string      `json:"name"`
	StartTime        time.Time   `json:"startTime"`
	EndTime          time.Time   `json:"endTime"`
	Daytime          bool        `json:"isDaytime"`
	Temperature      json.Number `json:"temperature"`
	TemperatureUnit  string      `json:"temperatureUnit"`
	WindSpeed        string      `json:"windSpeed"`
	WindDirection    string      `json:"windDirection"`
	ShortForecast    string      `json:"shortForecast"`
	DetailedForecast string      `json:"detailedForecast"`
}

// ForecastResponse represents a subset of a response from the /gridpoints/{x},{y}/forecast API
// https://www.weather.gov/documentation/services-web-api#/default/get_gridpoints__wfo___x___y__forecast
type ForecastResponse struct {
	Properties struct {
		Updated         time.Time        `json:"updated"`
		ForecastPeriods []ForecastPeriod `json:"periods"`
	} `json:"properties"`
}

// WxGovAPIOpts contains optional configuration for the weather.gov API client
type WxGovAPIOpts struct {
	ForceIpv4 bool
	UaEmail   string
}

const typeGeoJSON = "application/geo+json"
const apiTimeout = 5 * time.Second

// makeHTTPClient returns an http.Client configured for use with the weather.gov forecast API
// (including the necessary headers)
func makeHTTPClient(opts *WxGovAPIOpts) *http.Client {
	httpClient := &http.Client{Timeout: apiTimeout}
	if opts != nil && opts.ForceIpv4 {
		// ugly hack adapted from https://stackoverflow.com/questions/77718022/go-http-get-force-to-use-ipv4
		// to work around https://github.com/weather-gov/api/discussions/763
		httpClient.Transport = &http.Transport{
			Proxy: http.ProxyFromEnvironment,
			DialContext: func(ctx context.Context, _ string, addr string) (net.Conn, error) {
				return (&net.Dialer{
					Timeout:   30 * time.Second,
					KeepAlive: 30 * time.Second,
				}).DialContext(ctx, "tcp4", addr)
			},
			ForceAttemptHTTP2:     true,
			MaxIdleConns:          100,
			IdleConnTimeout:       90 * time.Second,
			TLSHandshakeTimeout:   10 * time.Second,
			ExpectContinueTimeout: 1 * time.Second,
			ResponseHeaderTimeout: 10 * time.Second,
		}
	}
	rt := withHeader(httpClient.Transport)
	rt.Set("Accept", typeGeoJSON)
	rt.Set("User-Agent", userAgent(opts))
	httpClient.Transport = rt
	return httpClient
}

// doJSONRequest performs the given API request, decoding the body as JSON into the given respBody.
func doJSONRequest(httpClient *http.Client, req *http.Request, respBody interface{}) error {
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
		return fmt.Errorf("error decoding JSON response from body: %w\n\n%s", err, string(body))
	}
	return nil
}

// GetForecast returns a ForecastResponse representing the weather.gov forecast for the given latitude/longitude.
func GetForecast(opts *WxGovAPIOpts, lat float64, lon float64) (*ForecastResponse, error) {
	httpClient := makeHTTPClient(opts)

	reqURL := fmt.Sprintf("https://api.weather.gov/points/%.2f,%.2f", lat, lon)
	req, err := http.NewRequest(http.MethodGet, reqURL, nil)
	if err != nil {
		return nil, err
	}

	pointsResp := PointsResponse{}
	if err = doJSONRequest(httpClient, req, &pointsResp); err != nil {
		return nil, err
	}

	reqURL = pointsResp.Properties.ForecastURL
	req, err = http.NewRequest(http.MethodGet, reqURL, nil)
	if err != nil {
		return nil, err
	}

	forecastResp := ForecastResponse{}
	if err = doJSONRequest(httpClient, req, &forecastResp); err != nil {
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

func userAgent(opts *WxGovAPIOpts) string {
	if opts != nil && opts.UaEmail != "" {
		return fmt.Sprintf("%s %s (contact: %s)", ProductID, ProductVersion, opts.UaEmail)
	}
	return fmt.Sprintf("%s %s", ProductID, ProductVersion)
}
