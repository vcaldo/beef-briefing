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
- **Constants**: UPPER_SNAKE_CASE for package-level constants
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

Example pattern:
```go
type DBConfig struct {
    Host string `envconfig:"DB_HOST" default:"localhost"`
    Port int    `envconfig:"DB_PORT" default:"5432"`
    // ... other fields with defaults
}

func (c *DBConfig) DSN() string { /* ... */ }

func LoadConfig() (*DBConfig, error) {
    _ = godotenv.Load()
    var cfg DBConfig
    if err := envconfig.Process("", &cfg); err != nil {
        return nil, fmt.Errorf("failed to parse config: %w", err)
    }
    return &cfg, nil
}
```

### Testing

#### Test File Organization
- Place tests in same package with `_test.go` suffix
- Example: `handler.go` paired with `handler_test.go`
- Use `internal/` packages for non-exported functionality

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

### Additional Context

#### Go Version
- **Minimum**: Go 1.25+
- All services use the same version for consistency
- Specified in all `go.mod` files

#### Build Standards
- **Build type**: Static binaries with CGO disabled
- Use multi-stage Docker builds: compile in `golang:1.25` image, run in minimal base
- Build command: `go build -a -installsuffix cgo -o {binary-name} ./cmd`
- Environment: `CGO_ENABLED=0` to eliminate C dependencies

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
