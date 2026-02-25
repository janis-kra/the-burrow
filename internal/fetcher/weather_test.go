package fetcher

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestWeatherFetch(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{
			"current": {"temperature_2m": 18.5, "weather_code": 0},
			"daily": {
				"temperature_2m_max": [22.0],
				"temperature_2m_min": [14.0],
				"precipitation_probability_max": [10.0]
			}
		}`))
	}))
	defer server.Close()

	weather := NewWeather(server.Client(), 52.52, 13.405, "Berlin")
	weather.baseURL = server.URL

	result, err := weather.Fetch(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	data, ok := result.(WeatherData)
	if !ok {
		t.Fatal("result is not WeatherData")
	}

	if data.Temperature != 18.5 {
		t.Errorf("expected temperature 18.5, got %f", data.Temperature)
	}
	if data.HighTemp != 22.0 {
		t.Errorf("expected high 22.0, got %f", data.HighTemp)
	}
	if data.LowTemp != 14.0 {
		t.Errorf("expected low 14.0, got %f", data.LowTemp)
	}
	if data.Description != "Clear sky" {
		t.Errorf("expected 'Clear sky', got %q", data.Description)
	}
}

func TestWeatherDescription(t *testing.T) {
	tests := []struct {
		code int
		want string
	}{
		{0, "Clear sky"},
		{2, "Partly cloudy"},
		{45, "Foggy"},
		{55, "Drizzle"},
		{63, "Rain"},
		{73, "Snow"},
		{95, "Thunderstorm"},
	}

	for _, tt := range tests {
		if got := weatherDescription(tt.code); got != tt.want {
			t.Errorf("weatherDescription(%d) = %q, want %q", tt.code, got, tt.want)
		}
	}
}
