package config

import (
	"fmt"
	"os"
	"regexp"
	"strings"

	"gopkg.in/yaml.v3"
)

type Config struct {
	Schedule string         `yaml:"schedule"`
	Edition  int            `yaml:"edition"`
	Email    EmailConfig    `yaml:"email"`
	Sources  []SourceConfig `yaml:"sources"`
}

type EmailConfig struct {
	From         string `yaml:"from"`
	To           string `yaml:"to"`
	TestTo       string `yaml:"test_to"`
	ResendAPIKey string `yaml:"resend_api_key"`
}

type SourceConfig struct {
	Type      string  `yaml:"type"`
	// Weather fields
	Latitude  float64 `yaml:"latitude,omitempty"`
	Longitude float64 `yaml:"longitude,omitempty"`
	Name      string  `yaml:"name,omitempty"`
	// Readwise fields
	APIToken  string  `yaml:"api_token,omitempty"`
	// Reddit fields
	Subreddit string  `yaml:"subreddit,omitempty"`
	// Nitter fields
	NitterInstance string   `yaml:"nitter_instance,omitempty"`
	Usernames      []string `yaml:"usernames,omitempty"`
	Limit          int      `yaml:"limit,omitempty"`
	// Unsplash fields
	Query string `yaml:"query,omitempty"`
}

var envVarPattern = regexp.MustCompile(`\$\{([^}]+)\}`)

func expandEnvVars(data []byte) []byte {
	return envVarPattern.ReplaceAllFunc(data, func(match []byte) []byte {
		expr := strings.TrimSuffix(strings.TrimPrefix(string(match), "${"), "}")

		// Support ${VAR:-default} syntax
		varName, defaultVal, hasDefault := strings.Cut(expr, ":-")
		if val, ok := os.LookupEnv(varName); ok {
			return []byte(val)
		}
		if hasDefault {
			return []byte(defaultVal)
		}
		return match
	})
}

// IncrementEdition bumps the edition counter and writes it back to the config file.
func IncrementEdition(path string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("reading config file: %w", err)
	}

	// Simple find-and-replace on the raw YAML to avoid rewriting env vars like ${RESEND_API_KEY}
	lines := strings.Split(string(data), "\n")
	found := false
	for i, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "edition:") {
			// Parse current value, increment, replace line
			var current int
			fmt.Sscanf(trimmed, "edition: %d", &current)
			lines[i] = fmt.Sprintf("edition: %d", current+1)
			found = true
			break
		}
	}
	if !found {
		// Insert edition field after the schedule line (or at top)
		inserted := false
		for i, line := range lines {
			if strings.HasPrefix(strings.TrimSpace(line), "schedule:") {
				newLines := make([]string, 0, len(lines)+1)
				newLines = append(newLines, lines[:i+1]...)
				newLines = append(newLines, "edition: 1")
				newLines = append(newLines, lines[i+1:]...)
				lines = newLines
				inserted = true
				break
			}
		}
		if !inserted {
			lines = append([]string{"edition: 1"}, lines...)
		}
	}

	return os.WriteFile(path, []byte(strings.Join(lines, "\n")), 0644)
}

func Load(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("reading config file: %w", err)
	}

	data = expandEnvVars(data)

	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("parsing config file: %w", err)
	}

	return &cfg, nil
}
