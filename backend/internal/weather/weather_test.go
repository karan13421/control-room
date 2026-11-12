package weather

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func chicago(t *testing.T) *time.Location {
	t.Helper()
	loc, err := time.LoadLocation(timezoneName)
	if err != nil {
		t.Fatalf("load location: %v", err)
	}
	return loc
}

func f(v float64) *float64 { return &v }

// hourlyPayload builds a minimal Open-Meteo-shaped response body with a
// single hourly entry at timeStr.
type hourlyEntry struct {
	time   string
	temp   *float64
	precip *float64
	snow   *float64
}

func hourlyBody(entries ...hourlyEntry) []byte {
	var resp forecastResponse
	for _, e := range entries {
		resp.Hourly.Time = append(resp.Hourly.Time, e.time)
		resp.Hourly.Temperature2m = append(resp.Hourly.Temperature2m, e.temp)
		resp.Hourly.Precipitation = append(resp.Hourly.Precipitation, e.precip)
		resp.Hourly.Snowfall = append(resp.Hourly.Snowfall, e.snow)
	}
	body, _ := json.Marshal(resp)
	return body
}

func newTestClient(t *testing.T, handler http.HandlerFunc) *Client {
	t.Helper()
	srv := httptest.NewServer(handler)
	t.Cleanup(srv.Close)

	c := NewClient()
	c.BaseURL = srv.URL
	// keep tests fast regardless of the production backoff window
	c.http.RetryWaitMin = 1 * time.Millisecond
	c.http.RetryWaitMax = 5 * time.Millisecond
	return c
}

func TestNextHour_PicksTheRightHour(t *testing.T) {
	loc := chicago(t)
	now := time.Date(2024, 1, 15, 17, 20, 0, 0, loc)

	c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		w.Write(hourlyBody(
			hourlyEntry{"2024-01-15T17:00", f(5), f(0), f(0)},
			hourlyEntry{"2024-01-15T18:00", f(10), f(0), f(0)},
			hourlyEntry{"2024-01-15T19:00", f(15), f(0), f(0)},
		))
	})

	got, err := c.NextHour(context.Background(), now)
	if err != nil {
		t.Fatalf("NextHour: %v", err)
	}

	want := time.Date(2024, 1, 15, 18, 0, 0, 0, loc)
	if !got.Hour.Equal(want) {
		t.Errorf("Hour = %v, want %v", got.Hour, want)
	}
	if got.TempC != 10 {
		t.Errorf("TempC = %v, want 10", got.TempC)
	}
	if got.Condition != ConditionNormal || got.Multiplier != 1.0 {
		t.Errorf("Condition/Multiplier = %v/%v, want normal/1.0", got.Condition, got.Multiplier)
	}
}

func TestNextHour_MidnightRollover(t *testing.T) {
	loc := chicago(t)
	now := time.Date(2024, 1, 15, 23, 30, 0, 0, loc)

	c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		w.Write(hourlyBody(hourlyEntry{"2024-01-16T00:00", f(1), f(0), f(0)}))
	})

	got, err := c.NextHour(context.Background(), now)
	if err != nil {
		t.Fatalf("NextHour: %v", err)
	}

	want := time.Date(2024, 1, 16, 0, 0, 0, 0, loc)
	if !got.Hour.Equal(want) {
		t.Errorf("Hour = %v, want %v", got.Hour, want)
	}
}

func TestNextHour_Classification(t *testing.T) {
	loc := chicago(t)
	now := time.Date(2024, 1, 15, 17, 20, 0, 0, loc)
	const targetStr = "2024-01-15T18:00"

	tests := []struct {
		name     string
		temp     float64
		precip   float64
		snow     float64
		wantCond Condition
		wantMult float64
	}{
		{"snow at threshold", 0, 0, 0.1, ConditionSnow, 0.96},
		{"just under snow threshold falls through", 0, 0, 0.09, ConditionNormal, 1.0},
		{"snow beats rain", 0, 5.0, 0.1, ConditionSnow, 0.96},
		{"rain at threshold", 0, 0.2, 0, ConditionRain, 1.04},
		{"just under rain threshold falls through", 0, 0.19, 0, ConditionNormal, 1.0},
		{"rain beats cold", -20, 0.2, 0, ConditionRain, 1.04},
		{"cold at threshold", -15, 0, 0, ConditionCold, 0.96},
		{"just above cold threshold is normal", -14.9, 0, 0, ConditionNormal, 1.0},
		{"hot at threshold", 30, 0, 0, ConditionHot, 0.95},
		{"just below hot threshold is normal", 29.9, 0, 0, ConditionNormal, 1.0},
		{"normal", 20, 0, 0, ConditionNormal, 1.0},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
				w.Write(hourlyBody(hourlyEntry{targetStr, f(tc.temp), f(tc.precip), f(tc.snow)}))
			})

			got, err := c.NextHour(context.Background(), now)
			if err != nil {
				t.Fatalf("NextHour: %v", err)
			}
			if got.Condition != tc.wantCond {
				t.Errorf("Condition = %v, want %v", got.Condition, tc.wantCond)
			}
			if got.Multiplier != tc.wantMult {
				t.Errorf("Multiplier = %v, want %v", got.Multiplier, tc.wantMult)
			}
		})
	}
}

func TestNextHour_Errors(t *testing.T) {
	loc := chicago(t)
	now := time.Date(2024, 1, 15, 17, 20, 0, 0, loc)
	const targetStr = "2024-01-15T18:00"

	tests := []struct {
		name    string
		handler http.HandlerFunc
		wantErr error // checked with errors.Is when non-nil
	}{
		{
			name: "non-200 status",
			handler: func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusNotFound)
			},
			wantErr: ErrBadStatus,
		},
		{
			name: "invalid json",
			handler: func(w http.ResponseWriter, r *http.Request) {
				w.Write([]byte("{not json"))
			},
			wantErr: ErrInvalidJSON,
		},
		{
			name: "missing arrays",
			handler: func(w http.ResponseWriter, r *http.Request) {
				w.Write([]byte(`{"hourly":{"time":[]}}`))
			},
			wantErr: ErrMissingArrays,
		},
		{
			name: "mismatched array lengths",
			handler: func(w http.ResponseWriter, r *http.Request) {
				w.Write([]byte(`{"hourly":{"time":["` + targetStr + `"],"temperature_2m":[1,2],"precipitation":[0],"snowfall":[0]}}`))
			},
			wantErr: ErrMissingArrays,
		},
		{
			name: "target hour not found",
			handler: func(w http.ResponseWriter, r *http.Request) {
				w.Write(hourlyBody(hourlyEntry{"2024-01-15T20:00", f(1), f(0), f(0)}))
			},
			wantErr: ErrHourNotFound,
		},
		{
			name: "null value in target hour",
			handler: func(w http.ResponseWriter, r *http.Request) {
				w.Write(hourlyBody(hourlyEntry{targetStr, nil, f(0), f(0)}))
			},
			wantErr: ErrNullValue,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			c := newTestClient(t, tc.handler)

			_, err := c.NextHour(context.Background(), now)
			if err == nil {
				t.Fatal("NextHour: want error, got nil")
			}
			if tc.wantErr != nil && !errors.Is(err, tc.wantErr) {
				t.Errorf("NextHour error = %v, want wrapping %v", err, tc.wantErr)
			}
		})
	}
}

func TestNextHour_ContextTimeout(t *testing.T) {
	now := time.Date(2024, 1, 15, 17, 20, 0, 0, chicago(t))

	c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		select {
		case <-time.After(200 * time.Millisecond):
		case <-r.Context().Done():
		}
	})

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer cancel()

	_, err := c.NextHour(ctx, now)
	if err == nil {
		t.Fatal("NextHour: want error, got nil")
	}
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Errorf("NextHour error = %v, want wrapping context.DeadlineExceeded", err)
	}
}

func TestNextHour_RetriesTransientFailures(t *testing.T) {
	loc := chicago(t)
	now := time.Date(2024, 1, 15, 17, 20, 0, 0, loc)
	const targetStr = "2024-01-15T18:00"

	var attempts int
	c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		attempts++
		if attempts < 3 {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		w.Write(hourlyBody(hourlyEntry{targetStr, f(10), f(0), f(0)}))
	})

	got, err := c.NextHour(context.Background(), now)
	if err != nil {
		t.Fatalf("NextHour: %v", err)
	}
	if attempts != 3 {
		t.Errorf("attempts = %d, want 3 (2 failures + 1 success)", attempts)
	}
	if got.TempC != 10 {
		t.Errorf("TempC = %v, want 10", got.TempC)
	}
}

func TestNextHour_DoesNotRetryClientErrors(t *testing.T) {
	now := time.Date(2024, 1, 15, 17, 20, 0, 0, chicago(t))

	var attempts int
	c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		attempts++
		w.WriteHeader(http.StatusBadRequest)
	})

	_, err := c.NextHour(context.Background(), now)
	if err == nil {
		t.Fatal("NextHour: want error, got nil")
	}
	if !errors.Is(err, ErrBadStatus) {
		t.Errorf("NextHour error = %v, want wrapping ErrBadStatus", err)
	}
	if attempts != 1 {
		t.Errorf("attempts = %d, want 1 (4xx must not be retried)", attempts)
	}
}
