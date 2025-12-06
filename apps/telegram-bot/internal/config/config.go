package config

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/joho/godotenv"
	"github.com/kelseyhightower/envconfig"
)

type Config struct {
	// Telegram Bot Configuration
	TelegramBotToken string `envconfig:"TELEGRAM_BOT_TOKEN" required:"true"`

	// Database Configuration
	DBHost     string `envconfig:"DB_HOST" default:"localhost"`
	DBPort     int    `envconfig:"DB_PORT" default:"5432"`
	DBUser     string `envconfig:"DB_USER" default:"postgres"`
	DBPassword string `envconfig:"DB_PASSWORD" default:""`
	DBName     string `envconfig:"DB_NAME" default:"beef_db"`
	DBSSLMode  string `envconfig:"DB_SSL_MODE" default:"disable"`

	// MinIO Configuration
	MinIOEndpoint  string `envconfig:"MINIO_ENDPOINT" default:"localhost:9000"`
	MinIOAccessKey string `envconfig:"MINIO_ACCESS_KEY" default:"minioadmin"`
	MinIOSecretKey string `envconfig:"MINIO_SECRET_KEY" default:"minioadmin"`
	MinIOBucket    string `envconfig:"MINIO_BUCKET" default:"telegram-media"`
	MinIOUseSSL    bool   `envconfig:"MINIO_USE_SSL" default:"false"`

	// Application Configuration
	Environment string `envconfig:"ENVIRONMENT" default:"development"`
	LogLevel    string `envconfig:"LOG_LEVEL" default:"info"`

	// New Relic Configuration (optional)
	NewRelicLicenseKey string `envconfig:"NEW_RELIC_LICENSE_KEY"`
	NewRelicAppName    string `envconfig:"NEW_RELIC_APP_NAME" default:"beef-briefing-telegram-bot"`
	NewRelicEnabled    bool   `envconfig:"NEW_RELIC_ENABLED" default:"false"`

	// Import Configuration
	AdminUserIDs    string `envconfig:"ADMIN_USER_IDS" default:""`
	MaxImportSizeMB int    `envconfig:"MAX_IMPORT_SIZE_MB" default:"4096"`
	ImportChunkSize int    `envconfig:"IMPORT_CHUNK_SIZE" default:"5000"`
	LocalImportPath string `envconfig:"LOCAL_IMPORT_PATH" default:"/app/local_import"`
}

func (c *Config) DSN() string {
	return fmt.Sprintf("host=%s port=%d user=%s password=%s dbname=%s sslmode=%s",
		c.DBHost, c.DBPort, c.DBUser, c.DBPassword, c.DBName, c.DBSSLMode)
}

func (c *Config) IsProduction() bool {
	return c.Environment == "production"
}

func (c *Config) IsAdmin(userID int64) bool {
	if c.AdminUserIDs == "" {
		return false
	}

	adminIDs := strings.Split(c.AdminUserIDs, ",")
	for _, idStr := range adminIDs {
		idStr = strings.TrimSpace(idStr)
		if id, err := strconv.ParseInt(idStr, 10, 64); err == nil && id == userID {
			return true
		}
	}
	return false
}

func LoadConfig() (*Config, error) {
	// Load .env file (ignore error if not found)
	_ = godotenv.Load()

	var cfg Config
	if err := envconfig.Process("", &cfg); err != nil {
		return nil, fmt.Errorf("failed to parse config: %w", err)
	}
	return &cfg, nil
}
