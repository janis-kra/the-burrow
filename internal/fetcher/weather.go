package fetcher

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
)

type WeatherData struct {
	Temperature   float64
	HighTemp      float64
	LowTemp       float64
	Precipitation float64
	WeatherCode   int
	Description   string
	Location      string
}

type openMeteoResponse struct {
	Hourly struct {
		Time        []string  `json:"time"`
		Temperature []float64 `json:"temperature_2m"`
		WeatherCode []int     `json:"weather_code"`
	} `json:"hourly"`
	Daily struct {
		TemperatureMax []float64 `json:"temperature_2m_max"`
		TemperatureMin []float64 `json:"temperature_2m_min"`
		Precipitation  []float64 `json:"precipitation_probability_max"`
	} `json:"daily"`
}

type Weather struct {
	client    *http.Client
	latitude  float64
	longitude float64
	location  string
	baseURL   string
}

func NewWeather(client *http.Client, lat, lon float64, location string) *Weather {
	return &Weather{client: client, latitude: lat, longitude: lon, location: location, baseURL: "https://api.open-meteo.com/v1/forecast"}
}

func (w *Weather) Name() string { return "Weather" }

func (w *Weather) Fetch(ctx context.Context) (any, error) {
	url := fmt.Sprintf(
		w.baseURL+"?latitude=%.4f&longitude=%.4f"+
			"&hourly=temperature_2m,weather_code"+
			"&daily=temperature_2m_max,temperature_2m_min,precipitation_probability_max"+
			"&timezone=auto&forecast_days=1",
		w.latitude, w.longitude,
	)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("creating request: %w", err)
	}

	resp, err := w.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("fetching weather: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("Open-Meteo API returned status %d", resp.StatusCode)
	}

	var result openMeteoResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("decoding weather response: %w", err)
	}

	// Find the midday (12:00) index in the hourly data
	middayIdx := 12 // hourly data starts at 00:00, index 12 = 12:00
	for i, t := range result.Hourly.Time {
		if len(t) >= 13 && t[11:13] == "12" {
			middayIdx = i
			break
		}
	}

	var middayTemp float64
	var middayCode int
	if middayIdx < len(result.Hourly.Temperature) {
		middayTemp = result.Hourly.Temperature[middayIdx]
	}
	if middayIdx < len(result.Hourly.WeatherCode) {
		middayCode = result.Hourly.WeatherCode[middayIdx]
	}

	data := WeatherData{
		Temperature: middayTemp,
		WeatherCode: middayCode,
		Description: weatherDescription(middayCode),
		Location:    w.location,
	}

	if len(result.Daily.TemperatureMax) > 0 {
		data.HighTemp = result.Daily.TemperatureMax[0]
	}
	if len(result.Daily.TemperatureMin) > 0 {
		data.LowTemp = result.Daily.TemperatureMin[0]
	}
	if len(result.Daily.Precipitation) > 0 {
		data.Precipitation = result.Daily.Precipitation[0]
	}

	return data, nil
}

func weatherDescription(code int) string {
	switch {
	case code == 0:
		return "Clear sky"
	case code <= 3:
		return "Partly cloudy"
	case code <= 48:
		return "Foggy"
	case code <= 57:
		return "Drizzle"
	case code <= 67:
		return "Rain"
	case code <= 77:
		return "Snow"
	case code <= 82:
		return "Rain showers"
	case code <= 86:
		return "Snow showers"
	case code <= 99:
		return "Thunderstorm"
	default:
		return "Unknown"
	}
}
