package weather

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"time"

	_ "time/tzdata"

	"github.com/hashicorp/go-retryablehttp"
)

const (
	defaultBaseURL = "https://api.open-meteo.com/v1/forecast"
	requestTimeout = 3 * time.Second
	timeLayout     = "2006-01-02T15:04"
	timezoneName   = "America/Chicago"
	latitude       = 41.8781
	longitude      = -87.6298
)

var (
	ErrBadStatus     = errors.New("weather: unexpected status code")
	ErrInvalidJSON   = errors.New("weather: invalid json response")
	ErrMissingArrays = errors.New("weather: missing or mismatched hourly arrays")
	ErrHourNotFound  = errors.New("weather: target hour not found in response")
	ErrNullValue     = errors.New("weather: null value for target hour")
)

// forecastResponse mirrors the subset of the Open-Meteo forecast response we
// use. Hourly values are pointers because Open-Meteo can return null for a
// given hour; a bare float64 would silently turn that into 0.
type forecastResponse struct {
	Hourly struct {
		Time          []string   `json:"time"`
		Temperature2m []*float64 `json:"temperature_2m"`
		Precipitation []*float64 `json:"precipitation"`
		Snowfall      []*float64 `json:"snowfall"`
	} `json:"hourly"`
}

type Client struct {
	BaseURL string
	http    *retryablehttp.Client
}

var _ Source = (*Client)(nil)

func NewClient() *Client {
	rc := retryablehttp.NewClient()
	rc.RetryMax = 2
	rc.RetryWaitMin = 50 * time.Millisecond
	rc.RetryWaitMax = 250 * time.Millisecond
	rc.Backoff = retryablehttp.LinearJitterBackoff
	rc.Logger = nil
	rc.HTTPClient.Timeout = requestTimeout

	return &Client{
		BaseURL: defaultBaseURL,
		http:    rc,
	}
}

func (c *Client) NextHour(ctx context.Context, now time.Time) (NextHour, error) {
	loc, err := time.LoadLocation(timezoneName)
	if err != nil {
		return NextHour{}, fmt.Errorf("weather: load location: %w", err)
	}

	target := now.In(loc).Truncate(time.Hour).Add(time.Hour)
	targetStr := target.Format(timeLayout)

	reqURL, err := c.requestURL()
	if err != nil {
		return NextHour{}, fmt.Errorf("weather: build request url: %w", err)
	}

	req, err := retryablehttp.NewRequestWithContext(ctx, http.MethodGet, reqURL, nil)
	if err != nil {
		return NextHour{}, fmt.Errorf("weather: build request: %w", err)
	}

	resp, err := c.http.Do(req)
	if err != nil {
		return NextHour{}, fmt.Errorf("weather: request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return NextHour{}, fmt.Errorf("%w: got %d", ErrBadStatus, resp.StatusCode)
	}

	var parsed forecastResponse
	if err := json.NewDecoder(resp.Body).Decode(&parsed); err != nil {
		return NextHour{}, fmt.Errorf("%w: %v", ErrInvalidJSON, err)
	}

	n := len(parsed.Hourly.Time)
	if n == 0 ||
		len(parsed.Hourly.Temperature2m) != n ||
		len(parsed.Hourly.Precipitation) != n ||
		len(parsed.Hourly.Snowfall) != n {
		return NextHour{}, fmt.Errorf("%w", ErrMissingArrays)
	}

	idx := -1
	for i, t := range parsed.Hourly.Time {
		if t == targetStr {
			idx = i
			break
		}
	}
	if idx == -1 {
		return NextHour{}, fmt.Errorf("%w: %s", ErrHourNotFound, targetStr)
	}

	tempPtr := parsed.Hourly.Temperature2m[idx]
	precipPtr := parsed.Hourly.Precipitation[idx]
	snowPtr := parsed.Hourly.Snowfall[idx]
	if tempPtr == nil || precipPtr == nil || snowPtr == nil {
		return NextHour{}, fmt.Errorf("%w: hour %s", ErrNullValue, targetStr)
	}

	hourTime, err := time.ParseInLocation(timeLayout, targetStr, loc)
	if err != nil {
		return NextHour{}, fmt.Errorf("weather: parse target hour: %w", err)
	}

	condition, multiplier := classify(*tempPtr, *precipPtr, *snowPtr)

	return NextHour{
		Hour:       hourTime,
		TempC:      *tempPtr,
		PrecipMM:   *precipPtr,
		SnowfallCM: *snowPtr,
		Condition:  condition,
		Multiplier: multiplier,
	}, nil
}

func (c *Client) requestURL() (string, error) {
	u, err := url.Parse(c.BaseURL)
	if err != nil {
		return "", err
	}

	q := u.Query()
	q.Set("latitude", strconv.FormatFloat(latitude, 'f', -1, 64))
	q.Set("longitude", strconv.FormatFloat(longitude, 'f', -1, 64))
	q.Set("timezone", timezoneName)
	q.Set("hourly", "temperature_2m,precipitation,snowfall")
	q.Set("forecast_days", "2")
	u.RawQuery = q.Encode()

	return u.String(), nil
}
