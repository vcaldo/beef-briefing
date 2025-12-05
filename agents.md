## Code Style Guidelines

### Formatting and Naming

#### Project Structure
- Use monorepo structure with separate Go modules per service
- Service module paths follow pattern: `beef-briefing/apps/{service-name}`
- Service names use kebab-case: `api-service`, `telegram-bot`, `llm-analyzer`

#### Directory Organization
```
apps/{service-name}/
├── cmd/
│   └── main.go           # Entry point for the service
├── internal/             # Private packages (optional, for larger services)
├── pkg/                  # Public packages (optional, if exported)
├── go.mod
├── go.sum
└── Dockerfile
```

#### Naming Conventions
- **Packages**: lowercase, short, single word when possible (e.g., `handler`, `store`, `client`)
- **Files**: lowercase with underscores for multi-word names (e.g., `error_handler.go`, `db_config.go`)
- **Functions**: CamelCase, exported functions start with uppercase, unexported start with lowercase
- **Constants**: CamelCase like variables (e.g., `MaxRetries`, `DefaultTimeout`); UPPER_SNAKE_CASE is not idiomatic Go
- **Variables**: camelCase for clarity

#### Code Organization
- Import statements grouped in three sections with blank lines:
  1. Standard library imports
  2. External/third-party packages
  3. Local/internal packages
- One import statement per line (use `gofmt` default style)
- Maximum line length: 100 characters (Go convention), exceeding is acceptable for readability

### Error Handling

#### Error Wrapping
- Always wrap errors with context using `fmt.Errorf("context: %w", err)`
- The `%w` verb (Go 1.13+) enables error chain inspection via `errors.Is()` and `errors.As()`

#### Pattern Examples
```go
// Good: Context + error wrapping
if err := db.Connect(); err != nil {
    return fmt.Errorf("failed to connect to database: %w", err)
}

// Good: Adding context at each layer
user, err := getUserByID(ctx, id)
if err != nil {
    return fmt.Errorf("fetching user profile: %w", err)
}
```

#### Custom Error Types (Optional, when needed)
- Define custom error types for domain-specific errors
- Implement `Error()` string method for custom types
- Use sentinel errors for comparison: `var ErrNotFound = errors.New("record not found")`

#### Error Handling Strategy
- Don't ignore errors: every error return must be handled
- Propagate errors up with context, don't log and return
- Only log errors at the boundary (HTTP handlers, CLI entry points)
- Use `errors.Is()` for sentinel error comparison, `errors.As()` for type assertion

### Logging

#### Structured Logging with slog
- Use Go's standard `log/slog` package for all logging
- Always use structured key-value logging, never string formatting in log messages
- Avoid `fmt.Sprintf()` in log statements; use slog attributes instead

#### Log Level Guidelines
- **DEBUG**: Development-only detailed information, variable states, function calls
- **INFO**: General informational messages, significant application events
- **WARN**: Potentially problematic situations, degraded functionality
- **ERROR**: Error events where the application continues (actual errors)

#### Logging Patterns
```go
// Good: Structured logging with context
slog.Info("user created", "user_id", userID, "email", email)
slog.Error("database connection failed", "error", err, "host", dbHost)

// Bad: String formatting
slog.Info(fmt.Sprintf("user %d created with email %s", userID, email))
```

#### Environment-Based Configuration
- **Development**: Text handler (human-readable) with DEBUG log level
- **Production**: JSON handler (machine-readable) with INFO log level
- Configure via environment variable: `LOG_LEVEL` (debug, info, warn, error)
- Set `ENVIRONMENT` variable to control output format (dev/prod)

#### Logger Initialization
```go
// Pseudo-code for environment-aware logger setup
if os.Getenv("ENVIRONMENT") == "production" {
    handler := slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo})
    slog.SetDefault(slog.New(handler))
} else {
    handler := slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelDebug})
    slog.SetDefault(slog.New(handler))
}
```

### Configuration Management

#### Environment Variables (Primary Method)
- All configuration should be injected via environment variables
- Follow 12-factor app principles for environment-based configuration
- Document required and optional env vars in service README
- Common patterns:
  - `DATABASE_URL` or `DB_HOST`, `DB_PORT`, `DB_USER`, `DB_PASSWORD`, `DB_NAME`
  - `API_PORT` or `PORT` for HTTP server port
  - `ENVIRONMENT` set to `development` or `production`
  - `LOG_LEVEL` for controlling log verbosity

#### Configuration Loading

Use the `godotenv` + `envconfig` pattern for config with sensible defaults:

- Define a config struct with `envconfig` tags and `default` values
- Create a `LoadConfig()` function that calls `godotenv.Load()` then `envconfig.Process()`
- Add helper methods to config struct (e.g., `DSN()`, `ConnectionString()`)
- Load config in `main()` and pass to services

#### Config Package Location
- Shared config goes in `pkg/config/config.go`
- Service-specific config in `apps/{service}/internal/config/config.go`

Example pattern:
```go
// pkg/config/config.go
package config

import (
    "fmt"

    "github.com/joho/godotenv"
    "github.com/kelseyhightower/envconfig"
)

type Config struct {
    // Required fields (no default = must be set)
    DatabaseURL string `envconfig:"DATABASE_URL" required:"true"`

    // Optional fields with defaults
    Host        string `envconfig:"DB_HOST" default:"localhost"`
    Port        int    `envconfig:"DB_PORT" default:"5432"`
    User        string `envconfig:"DB_USER" default:"postgres"`
    Password    string `envconfig:"DB_PASSWORD" default:""`
    DBName      string `envconfig:"DB_NAME" default:"beef_briefing"`
    SSLMode     string `envconfig:"DB_SSL_MODE" default:"disable"`

    // Application settings
    Environment string `envconfig:"ENVIRONMENT" default:"development"`
    LogLevel    string `envconfig:"LOG_LEVEL" default:"info"`
    APIPort     int    `envconfig:"API_PORT" default:"8080"`
}

func (c *Config) DSN() string {
    return fmt.Sprintf("host=%s port=%d user=%s password=%s dbname=%s sslmode=%s",
        c.Host, c.Port, c.User, c.Password, c.DBName, c.SSLMode)
}

func (c *Config) IsProduction() bool {
    return c.Environment == "production"
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
```

### Context Usage

#### context.Context Guidelines
- Always pass `ctx context.Context` as the **first parameter** to functions
- Never store context in structs; pass it explicitly
- Use context for cancellation, timeouts, and request-scoped values

#### Pattern Examples
```go
// Good: ctx as first parameter
func GetUser(ctx context.Context, id string) (*User, error) {
    // Use ctx for database queries, HTTP calls, etc.
    return db.QueryRowContext(ctx, "SELECT * FROM users WHERE id = $1", id)
}

// Good: HTTP handler with context
func (h *Handler) GetUser(w http.ResponseWriter, r *http.Request) {
    ctx := r.Context()
    user, err := h.userService.GetUser(ctx, userID)
    // ...
}

// Good: Adding timeout to context
func FetchExternalData(ctx context.Context, url string) ([]byte, error) {
    ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
    defer cancel()

    req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
    // ...
}
```

#### Context Best Practices
- Always call `cancel()` when using `context.WithTimeout` or `context.WithCancel`
- Use `defer cancel()` immediately after creating cancellable context
- Check `ctx.Err()` in long-running loops
- Pass context through the entire call chain

### Graceful Shutdown

#### Signal Handling Pattern
```go
func main() {
    cfg, err := config.LoadConfig()
    if err != nil {
        slog.Error("failed to load config", "error", err)
        os.Exit(1)
    }

    // Initialize server
    server := &http.Server{
        Addr:         fmt.Sprintf(":%d", cfg.APIPort),
        Handler:      setupRouter(),
        ReadTimeout:  15 * time.Second,
        WriteTimeout: 15 * time.Second,
        IdleTimeout:  60 * time.Second,
    }

    // Start server in goroutine
    go func() {
        slog.Info("server starting", "port", cfg.APIPort)
        if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
            slog.Error("server error", "error", err)
            os.Exit(1)
        }
    }()

    // Wait for interrupt signal
    quit := make(chan os.Signal, 1)
    signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
    <-quit

    slog.Info("shutting down server...")

    // Give outstanding requests 30 seconds to complete
    ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
    defer cancel()

    if err := server.Shutdown(ctx); err != nil {
        slog.Error("server forced to shutdown", "error", err)
        os.Exit(1)
    }

    slog.Info("server stopped gracefully")
}
```

#### Cleanup Order
1. Stop accepting new requests
2. Wait for in-flight requests to complete
3. Close database connections
4. Close message queue connections
5. Flush logs

### Observability (New Relic)

#### Configuration
- Set via environment variables (never hardcode license keys):
  - `NEW_RELIC_LICENSE_KEY` (required)
  - `NEW_RELIC_APP_NAME` (required, use service name: `beef-briefing-api-service`)
  - `NEW_RELIC_ENABLED` (optional, default: `true`)
  - `NEW_RELIC_LOG_LEVEL` (optional: `debug`, `info`, `warn`, `error`)

#### Go Agent Setup
```go
package main

import (
    "context"
    "fmt"
    "log/slog"
    "net/http"
    "os"

    "github.com/newrelic/go-agent/v3/newrelic"
)

func initNewRelic(appName string) (*newrelic.Application, error) {
    licenseKey := os.Getenv("NEW_RELIC_LICENSE_KEY")
    if licenseKey == "" {
        return nil, fmt.Errorf("NEW_RELIC_LICENSE_KEY is required")
    }

    app, err := newrelic.NewApplication(
        newrelic.ConfigAppName(appName),
        newrelic.ConfigLicense(licenseKey),
        newrelic.ConfigDistributedTracerEnabled(true),
        newrelic.ConfigAppLogForwardingEnabled(true),
        // Enable code-level metrics for better function visibility
        newrelic.ConfigCodeLevelMetricsEnabled(true),
    )
    if err != nil {
        return nil, fmt.Errorf("failed to create New Relic app: %w", err)
    }

    return app, nil
}
```

#### Context Propagation (Critical)
Always propagate the New Relic transaction via context:
```go
// HTTP Handler - extract transaction and add to context
func (h *Handler) GetUser(w http.ResponseWriter, r *http.Request) {
    // Transaction is automatically added by nrgorilla/nrhttp middleware
    txn := newrelic.FromContext(r.Context())

    // Add custom attributes for debugging
    txn.AddAttribute("user_id", userID)

    // Pass context with transaction to downstream calls
    user, err := h.userService.GetUser(r.Context(), userID)
    if err != nil {
        txn.NoticeError(err)
        // handle error...
    }
}

// Service layer - use context to continue transaction
func (s *UserService) GetUser(ctx context.Context, id string) (*User, error) {
    // Create a segment for this operation
    defer newrelic.FromContext(ctx).StartSegment("UserService.GetUser").End()

    return s.repo.GetByID(ctx, id)
}

// Database layer - instrument queries
func (r *PostgresUserRepo) GetByID(ctx context.Context, id string) (*User, error) {
    // Use DatastoreSegment for database operations
    segment := newrelic.DatastoreSegment{
        StartTime:  newrelic.FromContext(ctx).StartSegmentNow(),
        Product:    newrelic.DatastorePostgres,
        Collection: "users",
        Operation:  "SELECT",
    }
    defer segment.End()

    row := r.db.QueryRowContext(ctx, "SELECT id, email FROM users WHERE id = $1", id)
    // ...
}
```

#### HTTP Middleware Integration
```go
// Using gorilla/mux with New Relic
import (
    "github.com/gorilla/mux"
    "github.com/newrelic/go-agent/v3/integrations/nrgorilla"
)

func setupRouter(nrApp *newrelic.Application) *mux.Router {
    r := mux.NewRouter()

    // Apply New Relic middleware to all routes
    r.Use(nrgorilla.Middleware(nrApp))

    // Routes...
    r.HandleFunc("/users/{id}", handler.GetUser).Methods("GET")

    return r
}

// Using standard library net/http
import "github.com/newrelic/go-agent/v3/newrelic"

func setupRouter(nrApp *newrelic.Application) http.Handler {
    mux := http.NewServeMux()

    // Wrap each handler with New Relic
    mux.HandleFunc(newrelic.WrapHandleFunc(nrApp, "/users", handler.GetUsers))

    return mux
}
```

#### External HTTP Calls
```go
import "github.com/newrelic/go-agent/v3/newrelic"

func (c *ExternalClient) FetchData(ctx context.Context, url string) ([]byte, error) {
    // Create request with context
    req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
    if err != nil {
        return nil, fmt.Errorf("creating request: %w", err)
    }

    // Wrap the transport to instrument external calls
    txn := newrelic.FromContext(ctx)
    client := &http.Client{
        Transport: newrelic.NewRoundTripper(http.DefaultTransport),
    }

    // Add distributed tracing headers
    req = newrelic.RequestWithTransactionContext(req, txn)

    resp, err := client.Do(req)
    // ...
}
```

#### Custom Transactions (Background Jobs)
```go
func (w *Worker) ProcessMessage(ctx context.Context, msg Message) error {
    // Start a background transaction for non-HTTP work
    txn := w.nrApp.StartTransaction("ProcessMessage")
    defer txn.End()

    // Add context with transaction for downstream calls
    ctx = newrelic.NewContext(ctx, txn)

    txn.AddAttribute("message_id", msg.ID)
    txn.AddAttribute("message_type", msg.Type)

    if err := w.handler.Handle(ctx, msg); err != nil {
        txn.NoticeError(err)
        return err
    }

    return nil
}
```

#### Error Tracking
```go
// Notice errors with context
func (h *Handler) CreateUser(w http.ResponseWriter, r *http.Request) {
    txn := newrelic.FromContext(r.Context())

    user, err := h.userService.Create(r.Context(), input)
    if err != nil {
        // Record error with attributes
        txn.NoticeError(newrelic.Error{
            Message: err.Error(),
            Class:   "UserCreationError",
            Attributes: map[string]interface{}{
                "input_email": input.Email,
            },
        })
        http.Error(w, "failed to create user", http.StatusInternalServerError)
        return
    }
}
```

#### Custom Metrics
```go
// Record custom metrics for business KPIs
func (s *BeefService) AnalyzeConflict(ctx context.Context, chatID string) (*Report, error) {
    txn := newrelic.FromContext(ctx)
    start := time.Now()

    report, err := s.analyzer.Analyze(ctx, chatID)

    // Record duration metric
    duration := time.Since(start)
    txn.Application().RecordCustomMetric("Custom/BeefAnalysis/Duration", duration.Seconds())

    // Record count metric
    if report != nil {
        txn.Application().RecordCustomMetric("Custom/BeefAnalysis/ConflictScore", float64(report.Score))
    }

    return report, err
}
```

#### Graceful Shutdown with New Relic
```go
func main() {
    nrApp, err := initNewRelic("beef-briefing-api-service")
    if err != nil {
        slog.Error("failed to init New Relic", "error", err)
        os.Exit(1)
    }

    // ... server setup ...

    // Shutdown sequence
    <-quit
    slog.Info("shutting down...")

    // Shutdown HTTP server first
    ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
    defer cancel()
    server.Shutdown(ctx)

    // Shutdown New Relic last to flush remaining data
    nrApp.Shutdown(10 * time.Second)

    slog.Info("shutdown complete")
}
```

#### Best Practices
- **Always propagate context**: Every function that does I/O should accept `context.Context`
- **Name transactions meaningfully**: Use `Controller/Action` pattern (e.g., `Users/Create`)
- **Add relevant attributes**: Include IDs, types, and other debugging info
- **Use segments for visibility**: Wrap significant operations in segments
- **Don't over-instrument**: Focus on critical paths, not every function
- **Handle nil transactions**: Check `if txn := newrelic.FromContext(ctx); txn != nil`

### Testing

#### Test File Organization
- Place tests in same package with `_test.go` suffix
- Example: `handler.go` paired with `handler_test.go`
- Use `internal/` packages for non-exported functionality

#### Mocking with Interfaces
```go
// Define interface for testability
type UserRepository interface {
    GetByID(ctx context.Context, id string) (*User, error)
    Create(ctx context.Context, user *User) error
}

// Production implementation
type PostgresUserRepo struct {
    db *sql.DB
}

// Test mock
type MockUserRepo struct {
    users map[string]*User
    err   error
}

func (m *MockUserRepo) GetByID(ctx context.Context, id string) (*User, error) {
    if m.err != nil {
        return nil, m.err
    }
    return m.users[id], nil
}
```

#### Test Patterns
- **Table-driven tests** for multiple input scenarios:
  ```go
  func TestValidateEmail(t *testing.T) {
      tests := []struct {
          name    string
          email   string
          wantErr bool
      }{
          {"valid", "user@example.com", false},
          {"invalid", "not-an-email", true},
      }
      for _, tt := range tests {
          t.Run(tt.name, func(t *testing.T) {
              err := ValidateEmail(tt.email)
              if (err != nil) != tt.wantErr {
                  t.Errorf("ValidateEmail(%q) error = %v, wantErr %v", tt.email, err, tt.wantErr)
              }
          })
      }
  }
  ```
- Use `testing.T` methods: `t.Run()`, `t.Errorf()`, `t.Fatal()`, `t.Skip()`
- Tests should be independent and runnable in any order
- Use test fixtures and helper functions to reduce duplication

#### Running Tests
```bash
go test ./...
go test -v ./...          # Verbose output
go test -cover ./...       # Coverage report
go test -race ./...        # Detect race conditions
```

#### Integration Tests
- Use build tags to separate unit and integration tests
- Name integration test files: `*_integration_test.go`
- Add build tag at top of file: `//go:build integration`
```bash
# Run only unit tests (default)
go test ./...

# Run integration tests
go test -tags=integration ./...

# Run all tests
go test -tags=integration ./...
```

#### Test Database Setup
```go
//go:build integration

func TestMain(m *testing.M) {
    // Setup test database
    db := setupTestDB()
    defer db.Close()

    // Run tests
    code := m.Run()

    // Cleanup
    teardownTestDB(db)
    os.Exit(code)
}
```

### Database/SQL Conventions

#### Naming Conventions
- **Tables**: snake_case, plural (e.g., `users`, `chat_messages`, `beef_reports`)
- **Columns**: snake_case (e.g., `created_at`, `user_id`, `message_content`)
- **Primary keys**: `id` (prefer `BIGSERIAL` or `UUID`)
- **Foreign keys**: `{referenced_table_singular}_id` (e.g., `user_id`, `chat_id`)
- **Indexes**: `idx_{table}_{column(s)}` (e.g., `idx_messages_created_at`)
- **Unique constraints**: `uq_{table}_{column(s)}`

#### Timestamp Handling
- Always store timestamps in **UTC**
- Use `TIMESTAMPTZ` (timestamp with time zone) in PostgreSQL
- Standard columns: `created_at`, `updated_at`, `deleted_at` (for soft deletes)
```sql
CREATE TABLE users (
    id BIGSERIAL PRIMARY KEY,
    email VARCHAR(255) NOT NULL UNIQUE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
```

#### Migration File Naming
- Format: `{sequence}_{description}.sql` (e.g., `001_initial.sql`, `002_add_users_table.sql`)
- Keep migrations small and focused
- Never modify existing migrations; create new ones for changes
- Include both `UP` and `DOWN` migrations when using migration tools

#### Query Patterns in Go
```go
// Use parameterized queries to prevent SQL injection
row := db.QueryRowContext(ctx, "SELECT id, email FROM users WHERE id = $1", userID)

// Scan into struct fields
var user User
err := row.Scan(&user.ID, &user.Email)
```

### Python Guidelines (Dashboard Service)

#### Code Style
- Follow **PEP 8** style guide
- Use **type hints** for function signatures (Python 3.9+)
- Maximum line length: 88 characters (Black formatter default)
- Use `black` for formatting, `ruff` or `flake8` for linting

#### Project Structure
```
apps/dashboard/
├── src/
│   ├── main.py           # Streamlit entry point
│   ├── pages/            # Streamlit multi-page apps
│   ├── components/       # Reusable UI components
│   └── utils/            # Helper functions
├── tests/
│   └── test_*.py
├── requirements.txt
├── pyproject.toml        # (optional) for modern tooling
└── Dockerfile
```

#### Naming Conventions
- **Files/Modules**: snake_case (e.g., `data_loader.py`)
- **Classes**: PascalCase (e.g., `DataProcessor`)
- **Functions/Variables**: snake_case (e.g., `load_data`, `user_count`)
- **Constants**: UPPER_SNAKE_CASE (e.g., `MAX_RETRIES`)

#### Type Hints Example
```python
from typing import Optional

def get_user_by_id(user_id: int) -> Optional[dict]:
    """Fetch user from database by ID."""
    # Implementation
    pass

def process_messages(messages: list[str]) -> dict[str, int]:
    """Process messages and return word counts."""
    # Implementation
    pass
```

#### Logging
```python
import logging
import os

log_level = os.getenv("LOG_LEVEL", "INFO").upper()
logging.basicConfig(
    level=getattr(logging, log_level),
    format="%(asctime)s - %(name)s - %(levelname)s - %(message)s"
)
logger = logging.getLogger(__name__)

logger.info("Processing started", extra={"user_id": user_id})
```

#### Configuration
```python
import os
from dataclasses import dataclass

@dataclass
class Config:
    database_url: str = os.getenv("DATABASE_URL", "")
    environment: str = os.getenv("ENVIRONMENT", "development")
    log_level: str = os.getenv("LOG_LEVEL", "INFO")

    @property
    def is_production(self) -> bool:
        return self.environment == "production"

config = Config()
```

#### Testing with pytest
```bash
pytest tests/
pytest tests/ -v              # Verbose
pytest tests/ --cov=src       # Coverage
```

### Additional Context

#### Go Version
- **Minimum**: Go 1.25+
- All services use the same version for consistency
- Specified in all `go.mod` files

#### Shared Packages
- Shared code lives in `pkg/` at the repository root
- Module path: `beef-briefing/pkg`
- Structure:
  ```
  pkg/
  ├── go.mod              # module beef-briefing/pkg
  ├── config/             # Shared configuration loading
  │   └── config.go
  ├── logging/            # Shared logger setup
  │   └── logging.go
  └── models/             # Shared domain models
      └── models.go
  ```
- Services import shared packages: `import "beef-briefing/pkg/config"`
- Use Go workspaces (`go.work`) for local development across modules

#### Build Standards
- **Build type**: Static binaries with CGO disabled
- Use multi-stage Docker builds: compile in `golang:1.23` image, run in minimal base
- Build command: `go build -a -installsuffix cgo -o {binary-name} ./cmd`
- Environment: `CGO_ENABLED=0` to eliminate C dependencies

#### Dockerfile Template (Go Services)
```dockerfile
# Build stage
FROM golang:1.25-alpine AS builder
WORKDIR /app

# Copy go mod files first for caching
COPY go.mod go.sum ./
RUN go mod download

# Copy source and build
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o main ./cmd

# Runtime stage
FROM alpine:3.23
RUN apk --no-cache add ca-certificates tzdata
WORKDIR /app
COPY --from=builder /app/main .
EXPOSE 8080
CMD ["./main"]
```

#### .dockerignore
```
.git
.gitignore
*.md
Dockerfile
.env*
tmp/
```

#### Dependencies
- Managed via Go modules (`go.mod`, `go.sum`)
- Run `go mod download` in Docker builds before compiling
- Prefer standard library packages where possible
- Minimize external dependencies for production stability

#### Deployment & Networking
- All services containerized with Docker
- Services communicate via Docker network (DNS-based service discovery)
- Use environment variables for service URLs and credentials
- Restart policy: `unless-stopped` for production deployments

#### Code Quality
- Use `gofmt` (automatic code formatting, no configuration needed)
- Optional: `golangci-lint` for linting (when tooling is set up)
- No trailing whitespace
- Use meaningful commit messages following conventional commits when possible
