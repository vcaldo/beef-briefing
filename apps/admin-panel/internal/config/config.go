package config

import (
	"encoding/base64"
	"fmt"
	"os"
	"strings"

	"github.com/joho/godotenv"
	"github.com/kelseyhightower/envconfig"
)

// Config holds all configuration for the admin panel
type Config struct {
	// API Service
	APIServiceURL       string `envconfig:"API_SERVICE_URL" default:"http://api-service:8080"`
	AnalyticsAPIKey     string `envconfig:"ANALYTICS_API_KEY"`
	AnalyticsAPIKeyFile string `envconfig:"ANALYTICS_API_KEY_FILE"`

	// Admin Panel
	AdminPanelPort        int    `envconfig:"ADMIN_PANEL_PORT" default:"8081"`
	AdminUsername         string `envconfig:"ADMIN_USERNAME" default:"admin"`
	AdminPasswordHash     string `envconfig:"ADMIN_PASSWORD_HASH"`
	AdminPasswordHashFile string `envconfig:"ADMIN_PASSWORD_HASH_FILE"`
	SessionSecret         string `envconfig:"SESSION_SECRET"`
	SessionSecretFile     string `envconfig:"SESSION_SECRET_FILE"`
	SecureCookies         bool   `envconfig:"SECURE_COOKIES" default:"true"`

	// Application
	Environment string `envconfig:"ENVIRONMENT" default:"development"`
	LogLevel    string `envconfig:"LOG_LEVEL" default:"info"`

	// New Relic APM (optional - leave empty to disable)
	NewRelicAppName    string `envconfig:"NEW_RELIC_APP_NAME"`
	NewRelicLicenseKey string `envconfig:"NEW_RELIC_LICENSE_KEY"`
}

// IsProduction returns true if running in production environment
func (c *Config) IsProduction() bool {
	return c.Environment == "production"
}

// NewRelicEnabled returns true if New Relic is configured
func (c *Config) NewRelicEnabled() bool {
	return c.NewRelicAppName != "" && c.NewRelicLicenseKey != ""
}

// SessionSecretBytes returns the session secret decoded from base64
// gorilla/sessions requires the secret key as raw bytes
func (c *Config) SessionSecretBytes() ([]byte, error) {
	// Try to decode as base64 first (new format)
	decoded, err := base64.StdEncoding.DecodeString(c.SessionSecret)
	if err == nil && len(decoded) >= 32 {
		return decoded, nil
	}

	// Fall back to using the string directly (legacy format)
	// This handles cases where the secret is stored as plain text
	if len(c.SessionSecret) >= 32 {
		return []byte(c.SessionSecret), nil
	}

	return nil, fmt.Errorf("session secret must be at least 32 bytes")
}

// LoadConfig loads configuration from environment variables
func LoadConfig() (*Config, error) {
	// Load .env file (ignore error if not found)
	_ = godotenv.Load()

	var cfg Config
	if err := envconfig.Process("", &cfg); err != nil {
		return nil, fmt.Errorf("failed to parse config: %w", err)
	}

	// Load admin password hash from file if specified
	if cfg.AdminPasswordHashFile != "" {
		hash, err := readSecretFromFile(cfg.AdminPasswordHashFile)
		if err != nil {
			return nil, fmt.Errorf("failed to read admin password hash from file: %w", err)
		}
		cfg.AdminPasswordHash = hash
	}

	// Load session secret from file if specified
	if cfg.SessionSecretFile != "" {
		secret, err := readSecretFromFile(cfg.SessionSecretFile)
		if err != nil {
			return nil, fmt.Errorf("failed to read session secret from file: %w", err)
		}
		cfg.SessionSecret = secret
	}

	// Load analytics API key from file if specified
	if cfg.AnalyticsAPIKeyFile != "" {
		apiKey, err := readSecretFromFile(cfg.AnalyticsAPIKeyFile)
		if err != nil {
			return nil, fmt.Errorf("failed to read analytics API key from file: %w", err)
		}
		cfg.AnalyticsAPIKey = apiKey
	}

	// Validate that we have the required secrets
	if cfg.AdminPasswordHash == "" {
		return nil, fmt.Errorf("ADMIN_PASSWORD_HASH or ADMIN_PASSWORD_HASH_FILE must be set")
	}

	if cfg.SessionSecret == "" {
		return nil, fmt.Errorf("SESSION_SECRET or SESSION_SECRET_FILE must be set")
	}

	if cfg.AnalyticsAPIKey == "" {
		return nil, fmt.Errorf("ANALYTICS_API_KEY or ANALYTICS_API_KEY_FILE must be set")
	}

	// Validate session secret can be decoded and is the right length
	secretBytes, err := cfg.SessionSecretBytes()
	if err != nil {
		return nil, fmt.Errorf("invalid session secret: %w", err)
	}
	if len(secretBytes) < 32 {
		return nil, fmt.Errorf("session secret must be at least 32 bytes, got %d bytes", len(secretBytes))
	}

	return &cfg, nil
}

// readSecretFromFile reads a secret from a file and trims whitespace
func readSecretFromFile(filepath string) (string, error) {
	data, err := os.ReadFile(filepath)
	if err != nil {
		return "", fmt.Errorf("reading file %s: %w", filepath, err)
	}
	return strings.TrimSpace(string(data)), nil
}
