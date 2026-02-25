package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoad(t *testing.T) {
	content := `
schedule: "0 7 * * *"
email:
  from: "burrow@localhost"
  to: "you@localhost"
  resend_api_key: "re_test123"
weather:
  latitude: 52.52
  longitude: 13.405
readwise:
  api_token: "test-token"
reddit:
  subreddit: "de"
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
	if cfg.Weather.Latitude != 52.52 {
		t.Errorf("expected latitude 52.52, got %f", cfg.Weather.Latitude)
	}
	if cfg.Reddit.Subreddit != "de" {
		t.Errorf("expected subreddit 'de', got %q", cfg.Reddit.Subreddit)
	}
}

func TestLoadEnvExpansion(t *testing.T) {
	content := `
schedule: "0 7 * * *"
email:
  from: "burrow@localhost"
  to: "you@localhost"
  resend_api_key: "re_test"
weather:
  latitude: 52.52
  longitude: 13.405
readwise:
  api_token: "${TEST_BURROW_TOKEN}"
reddit:
  subreddit: "de"
`
	t.Setenv("TEST_BURROW_TOKEN", "secret-123")

	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	os.WriteFile(path, []byte(content), 0644)

	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if cfg.Readwise.APIToken != "secret-123" {
		t.Errorf("expected token 'secret-123', got %q", cfg.Readwise.APIToken)
	}
}
