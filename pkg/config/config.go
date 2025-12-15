package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/joho/godotenv"
	"github.com/kelseyhightower/envconfig"
)

type Config struct {
	// Database Configuration
	DBHost     string `envconfig:"DB_HOST" default:"localhost"`
	DBPort     int    `envconfig:"DB_PORT" default:"5432"`
	DBUser     string `envconfig:"DB_USER" default:"postgres"`
	DBPassword string `envconfig:"DB_PASSWORD" default:""`
	DBName     string `envconfig:"DB_NAME" default:"beef_briefing"`
	DBSSLMode  string `envconfig:"DB_SSL_MODE" default:"disable"`

	// API Service Configuration
	APIPort         int    `envconfig:"API_PORT" default:"8080"`
	MaxUploadSizeMB int    `envconfig:"MAX_UPLOAD_SIZE_MB" default:"100"`
	APIServiceURL   string `envconfig:"API_SERVICE_URL" default:"http://api-service:8080"`

	// Telegram Bot Configuration
	TelegramBotToken string `envconfig:"TELEGRAM_BOT_TOKEN" required:"true"`

	// MinIO Configuration
	MinIOEndpoint  string `envconfig:"MINIO_ENDPOINT" default:"localhost:9000"`
	MinIOAccessKey string `envconfig:"MINIO_ACCESS_KEY" default:"minioadmin"`
	MinIOSecretKey string `envconfig:"MINIO_SECRET_KEY" default:"minioadmin"`
	MinIOBucket    string `envconfig:"MINIO_BUCKET" default:"telegram-media"`
	MinIOUseSSL    bool   `envconfig:"MINIO_USE_SSL" default:"false"`
	MinIORegion    string `envconfig:"MINIO_REGION" default:""`

	// Application Settings
	Environment string `envconfig:"ENVIRONMENT" default:"development"`
	LogLevel    string `envconfig:"LOG_LEVEL" default:"info"`

	// API Authentication (for clients like telegram-bot)
	APIKey     string `envconfig:"API_KEY"`
	APIKeyFile string `envconfig:"API_KEY_FILE"`

	// API Service App Keys Directory (for api-service to validate incoming requests)
	AppKeysDir     string `envconfig:"APP_KEYS_DIR"`
	AppKeysDirFile string `envconfig:"APP_KEYS_DIR_FILE"`

	// New Relic APM Configuration
	NewRelicAppName    string `envconfig:"NEW_RELIC_APP_NAME"`
	NewRelicLicenseKey string `envconfig:"NEW_RELIC_LICENSE_KEY"`

	// Admin Configuration
	AdminUserIDs string `envconfig:"ADMIN_USER_IDS"` // Comma-separated list of admin user IDs
}

// DSN returns PostgreSQL connection string
func (c *Config) DSN() string {
	return fmt.Sprintf("host=%s port=%d user=%s password=%s dbname=%s sslmode=%s",
		c.DBHost, c.DBPort, c.DBUser, c.DBPassword, c.DBName, c.DBSSLMode)
}

// IsProduction returns true if running in production environment
func (c *Config) IsProduction() bool {
	return c.Environment == "production"
}

// MaxUploadSizeBytes returns the max upload size in bytes
func (c *Config) MaxUploadSizeBytes() int64 {
	return int64(c.MaxUploadSizeMB) * 1024 * 1024
}

// NewRelicEnabled returns true if New Relic APM is configured
func (c *Config) NewRelicEnabled() bool {
	return c.NewRelicAppName != "" && c.NewRelicLicenseKey != ""
}

// IsAdmin checks if the given user ID is in the admin list
func (c *Config) IsAdmin(userID int64) bool {
	if c.AdminUserIDs == "" {
		return false
	}
	for _, idStr := range strings.Split(c.AdminUserIDs, ",") {
		idStr = strings.TrimSpace(idStr)
		if id, err := strconv.ParseInt(idStr, 10, 64); err == nil && id == userID {
			return true
		}
	}
	return false
}

// LoadConfig loads configuration from environment variables
func LoadConfig() (*Config, error) {
	// Load .env file (ignore error if not found)
	_ = godotenv.Load()

	var cfg Config
	if err := envconfig.Process("", &cfg); err != nil {
		return nil, fmt.Errorf("failed to parse config: %w", err)
	}

	// Load API key from file if specified (for clients like telegram-bot)
	if cfg.APIKeyFile != "" {
		apiKey, err := readSecretFromFile(cfg.APIKeyFile)
		if err != nil {
			return nil, fmt.Errorf("failed to read API key from file: %w", err)
		}
		cfg.APIKey = apiKey
	}

	// Load app keys directory path from file if specified (for api-service)
	if cfg.AppKeysDirFile != "" {
		appKeysDir, err := readSecretFromFile(cfg.AppKeysDirFile)
		if err != nil {
			return nil, fmt.Errorf("failed to read app keys dir from file: %w", err)
		}
		cfg.AppKeysDir = appKeysDir
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

// LoadAppKeysFromDir reads all key files from a directory.
// Each file name is the app name, and the file content is the API key.
// Returns a map of appName -> apiKey.
func LoadAppKeysFromDir(dirPath string) (map[string]string, error) {
	if dirPath == "" {
		return nil, nil
	}

	entries, err := os.ReadDir(dirPath)
	if err != nil {
		return nil, fmt.Errorf("reading app keys directory %s: %w", dirPath, err)
	}

	appKeys := make(map[string]string)
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}

		appName := entry.Name()
		filePath := dirPath + "/" + appName

		key, err := readSecretFromFile(filePath)
		if err != nil {
			return nil, fmt.Errorf("reading app key for %s: %w", appName, err)
		}

		if key != "" {
			appKeys[appName] = key
		}
	}

	return appKeys, nil
}
