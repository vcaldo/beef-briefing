package config

import (
	"fmt"

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

	// Application Settings
	Environment string `envconfig:"ENVIRONMENT" default:"development"`
	LogLevel    string `envconfig:"LOG_LEVEL" default:"info"`
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

// LoadConfig loads configuration from environment variables
func LoadConfig() (*Config, error) {
	// Load .env file (ignore error if not found)
	_ = godotenv.Load()

	var cfg Config
	if err := envconfig.Process("", &cfg); err != nil {
		return nil, fmt.Errorf("failed to parse config: %w", err)
	}

	return &cfg, nil
}
