package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLoad(t *testing.T) {
	content := `
schedule: "0 7 * * *"
email:
  from: "burrow@localhost"
  to: "you@localhost"
  resend_api_key: "re_test123"
sources:
  - type: weather
    latitude: 52.52
    longitude: 13.405
  - type: hackernews
`
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	os.WriteFile(path, []byte(content), 0644)

	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if cfg.Schedule != "0 7 * * *" {
		t.Errorf("expected schedule '0 7 * * *', got %q", cfg.Schedule)
	}
	if cfg.Email.ResendAPIKey != "re_test123" {
		t.Errorf("expected resend api key 're_test123', got %q", cfg.Email.ResendAPIKey)
	}

	if len(cfg.Sources) != 2 {
		t.Fatalf("expected 2 sources, got %d", len(cfg.Sources))
	}
	if cfg.Sources[0].Type != "weather" {
		t.Errorf("expected first source 'weather', got %q", cfg.Sources[0].Type)
	}
	if cfg.Sources[0].Latitude != 52.52 {
		t.Errorf("expected latitude 52.52, got %v", cfg.Sources[0].Latitude)
	}
	if cfg.Sources[1].Type != "hackernews" {
		t.Errorf("expected second source 'hackernews', got %q", cfg.Sources[1].Type)
	}
}

func TestLoadEnvExpansion(t *testing.T) {
	content := `
schedule: "0 7 * * *"
email:
  from: "burrow@localhost"
  to: "you@localhost"
  resend_api_key: "re_test"
sources:
  - type: readwise
    api_token: "${TEST_BURROW_TOKEN}"
`
	t.Setenv("TEST_BURROW_TOKEN", "secret-123")

	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	os.WriteFile(path, []byte(content), 0644)

	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var readwiseSource *SourceConfig
	for i := range cfg.Sources {
		if cfg.Sources[i].Type == "readwise" {
			readwiseSource = &cfg.Sources[i]
			break
		}
	}
	if readwiseSource == nil {
		t.Fatal("expected a readwise source")
	}
	if readwiseSource.APIToken != "secret-123" {
		t.Errorf("expected token 'secret-123', got %q", readwiseSource.APIToken)
	}
}

func TestLoadImmoConfig(t *testing.T) {
	content := `
schedule: "0 7 * * *"
email:
  from: "burrow@localhost"
  to: "you@localhost"
  resend_api_key: "re_test"
sources: []
immo:
  schedule: "0 8 * * *"
  state_path: "/var/lib/burrow/immo-state.json"
  criteria:
    min_price: 100000
    max_price: 450000
    locations: ["Hameln", "Aerzen"]
  sites:
    - type: weserland
      url: "https://weserland-immobilien.de/immobilien/"
    - type: hapke
      url: "https://www.hapke-immobilien.de/immobilien-hameln-kaufen"
`
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	os.WriteFile(path, []byte(content), 0644)

	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if cfg.Immo == nil {
		t.Fatal("expected immo config to be set")
	}
	if cfg.Immo.Schedule != "0 8 * * *" {
		t.Errorf("unexpected immo schedule: %q", cfg.Immo.Schedule)
	}
	if cfg.Immo.StatePath != "/var/lib/burrow/immo-state.json" {
		t.Errorf("unexpected state path: %q", cfg.Immo.StatePath)
	}
	if cfg.Immo.Criteria.MinPrice != 100000 || cfg.Immo.Criteria.MaxPrice != 450000 {
		t.Errorf("unexpected criteria: %+v", cfg.Immo.Criteria)
	}
	if len(cfg.Immo.Criteria.Locations) != 2 {
		t.Errorf("expected 2 locations, got %v", cfg.Immo.Criteria.Locations)
	}
	if len(cfg.Immo.Sites) != 2 || cfg.Immo.Sites[0].Type != "weserland" || cfg.Immo.Sites[1].Type != "hapke" {
		t.Errorf("unexpected sites: %+v", cfg.Immo.Sites)
	}
}

func TestLoadWithoutImmoConfig(t *testing.T) {
	content := `
schedule: "0 7 * * *"
email:
  from: "burrow@localhost"
  to: "you@localhost"
  resend_api_key: "re_test"
sources: []
`
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	os.WriteFile(path, []byte(content), 0644)

	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.Immo != nil {
		t.Errorf("expected nil immo config, got %+v", cfg.Immo)
	}
}

func TestIncrementEditionPreservesImmoSection(t *testing.T) {
	content := `schedule: "0 7 * * *"
edition: 5
email:
  from: "burrow@localhost"
  to: "you@localhost"
  resend_api_key: "${RESEND_API_KEY}"
immo:
  schedule: "0 8 * * *"
  state_path: "/var/lib/burrow/immo-state.json"
`
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	os.WriteFile(path, []byte(content), 0644)

	if err := IncrementEdition(path); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	data, _ := os.ReadFile(path)
	got := string(data)
	if !strings.Contains(got, "edition: 6") {
		t.Errorf("edition not incremented:\n%s", got)
	}
	if !strings.Contains(got, "state_path: \"/var/lib/burrow/immo-state.json\"") {
		t.Errorf("immo section damaged:\n%s", got)
	}
	if !strings.Contains(got, "${RESEND_API_KEY}") {
		t.Errorf("env var placeholder damaged:\n%s", got)
	}
}
