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
sources:
  - type: weather
    latitude: 52.52
    longitude: 13.405
  - type: reddit
    subreddits:
      - de
      - golang
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

	// Check reddit subreddits
	var redditSource *SourceConfig
	for i := range cfg.Sources {
		if cfg.Sources[i].Type == "reddit" {
			redditSource = &cfg.Sources[i]
			break
		}
	}
	if redditSource == nil {
		t.Fatal("expected a reddit source")
	}
	if len(redditSource.Subreddits) != 2 {
		t.Fatalf("expected 2 subreddits, got %d", len(redditSource.Subreddits))
	}
	if redditSource.Subreddits[0] != "de" {
		t.Errorf("expected first subreddit 'de', got %q", redditSource.Subreddits[0])
	}
	if redditSource.Subreddits[1] != "golang" {
		t.Errorf("expected second subreddit 'golang', got %q", redditSource.Subreddits[1])
	}
}

func TestLoadSingleSubredditBackwardCompat(t *testing.T) {
	content := `
schedule: "0 7 * * *"
email:
  from: "burrow@localhost"
  to: "you@localhost"
  resend_api_key: "re_test"
sources:
  - type: reddit
    subreddit: "de"
`
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	os.WriteFile(path, []byte(content), 0644)

	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var redditSource *SourceConfig
	for i := range cfg.Sources {
		if cfg.Sources[i].Type == "reddit" {
			redditSource = &cfg.Sources[i]
			break
		}
	}
	if redditSource == nil {
		t.Fatal("expected a reddit source")
	}
	if redditSource.Subreddit != "de" {
		t.Errorf("expected subreddit 'de', got %q", redditSource.Subreddit)
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
