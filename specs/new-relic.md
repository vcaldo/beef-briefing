# New Relic APM Implementation Specification

This document describes the New Relic Application Performance Monitoring (APM) integration for the Beef Briefing system.

## Overview

New Relic APM is integrated across all long-running services to provide:
- Transaction tracing for HTTP requests and background jobs
- Distributed tracing across service boundaries
- Custom attributes for business context
- Error tracking and alerting
- External service call monitoring (Telegram API, internal APIs)
- Database and storage operation visibility

## Environment Variables

### Required Variables

| Variable | Description | Example |
|----------|-------------|---------|
| `NEW_RELIC_APP_NAME` | Base application name (service suffix added automatically) | `beef-briefing` |
| `NEW_RELIC_LICENSE_KEY` | New Relic license key from your account | `eu01xx...NRAL` |

### Optional Variables (set by services)

| Variable | Description | Set By |
|----------|-------------|--------|
| `NEW_RELIC_LOG_LEVEL` | Agent log verbosity | Dashboard (Python) |

### Configuration Pattern

New Relic is **optional** - if either variable is empty, services run without instrumentation.

```go
// Go services check:
func (c *Config) NewRelicEnabled() bool {
    return c.NewRelicAppName != "" && c.NewRelicLicenseKey != ""
}
```

```python
# Python dashboard check:
def new_relic_enabled(self) -> bool:
    return bool(self.new_relic_app_name and self.new_relic_license_key)
```

## Application Naming Convention

Each service appends its identifier and environment to the base name:

| Service | App Name Format | Example (Production) |
|---------|-----------------|----------------------|
| API Service | `{base}-api-service-{env}` | `beef-briefing-api-service-production` |
| Telegram Bot | `{base}-telegram-bot-{env}` | `beef-briefing-telegram-bot-production` |
| Dashboard | `{base}-dashboard-{env}` | `beef-briefing-dashboard-production` |

This creates separate applications in New Relic for independent monitoring while maintaining correlation.

## Service Instrumentation Details

### API Service (Go)

**Agent Version:** `github.com/newrelic/go-agent/v3 v3.42.0`

**Integrations:**
- `nrgorilla` - Automatic HTTP transaction instrumentation

**Initialization:**
```go
app, err := newrelic.NewApplication(
    newrelic.ConfigAppName(appName),
    newrelic.ConfigLicense(cfg.NewRelicLicenseKey),
    newrelic.ConfigDistributedTracerEnabled(true),
    newrelic.ConfigApplicationLogging(newrelic.ConfigApplicationLoggingForwardingEnabled(true), ...),
)
// 250ms wait for connection establishment
app.WaitForConnection(250 * time.Millisecond)
```

**Instrumented Layers:**

| Layer | Method | Segment Naming |
|-------|--------|----------------|
| HTTP Routes | Automatic (nrgorilla middleware) | `WebTransaction/Go/{route}` |
| Ingest Service | Manual segments | `service:process-update`, `service:process-media` |
| Analytics Service | Manual segments | `service:*` |
| Repositories | Manual segments | `db:insert-message`, `db:get-user`, etc. |
| MinIO Storage | Manual segments | `storage:upload-media`, `storage:check-exists` |

**Graceful Shutdown:**
```go
// 5-second timeout for data flushing
app.Shutdown(5 * time.Second)
```

### Telegram Bot (Go)

**Agent Version:** `github.com/newrelic/go-agent/v3 v3.35.1`

**Transaction Creation:**
```go
// Manual transaction for each Telegram update
txn := nrApp.StartTransaction("telegram:handle-update")
defer txn.End()

// Context propagation
ctx = newrelic.NewContext(ctx, txn)
```

**Custom Attributes:**
```go
txn.AddAttribute("update_id", update.UpdateID)
txn.AddAttribute("update_type", "message|callback_query|etc")
txn.AddAttribute("chat_id", chatID)
txn.AddAttribute("message_id", messageID)
txn.AddAttribute("user_id", userID)
txn.AddAttribute("file_count", len(files))
txn.AddAttribute("retry_count", retries)
```

**External Segments (Telegram API):**
```go
segment := newrelic.ExternalSegment{
    StartTime: txn.StartSegmentNow(),
    URL:       fileURL,
    Host:      "api.telegram.org",
    Procedure: "getFile",
    Library:   "telegram-bot-api",
}
// ... perform request ...
segment.Response = resp
segment.End()
```

**External Segments (API Service):**
```go
segment := newrelic.ExternalSegment{
    StartTime: txn.StartSegmentNow(),
    URL:       apiURL,
    Host:      "api-service",
    Procedure: "POST /api/v1/ingest",
    Library:   "internal-api",
}
```

**Error Tracking:**
```go
if err != nil {
    txn.NoticeError(err)
}
```

### Dashboard (Python/Flask)

**Agent Version:** `newrelic>=11.1.0`

**Initialization:**
```python
def init_new_relic(config: Config) -> None:
    if not config.new_relic_enabled():
        return

    import os
    import newrelic.agent

    os.environ["NEW_RELIC_LOG_LEVEL"] = "info" if config.is_production() else "debug"
    newrelic.agent.initialize()
```

**Note:** Dashboard app name is set with full suffix in docker-compose since Python agent reads `NEW_RELIC_APP_NAME` directly.

## Docker Compose Configuration

### Development (`docker-compose.dev.yml`)

```yaml
services:
  api-service:
    environment:
      NEW_RELIC_APP_NAME: ${NEW_RELIC_APP_NAME}
      NEW_RELIC_LICENSE_KEY: ${NEW_RELIC_LICENSE_KEY}
      ENVIRONMENT: development

  telegram-bot:
    environment:
      NEW_RELIC_APP_NAME: ${NEW_RELIC_APP_NAME}
      NEW_RELIC_LICENSE_KEY: ${NEW_RELIC_LICENSE_KEY}
      ENVIRONMENT: development

  dashboard:
    environment:
      # Full name with suffix (Python agent reads directly)
      NEW_RELIC_APP_NAME: ${NEW_RELIC_APP_NAME}-dashboard-development
      NEW_RELIC_LICENSE_KEY: ${NEW_RELIC_LICENSE_KEY}
      ENVIRONMENT: development
```

### Production (`docker-compose.prod.yml`)

Same pattern with `ENVIRONMENT: production` and `-dashboard-production` suffix.

## Environment File Examples

### Development (`.env.dev`)

```bash
# NEW RELIC APM CONFIGURATION (OPTIONAL)
# Leave blank to disable monitoring
# NEW_RELIC_APP_NAME=beef-briefing
# NEW_RELIC_LICENSE_KEY=your_license_key_here
```

### Production (`.env.prod`)

```bash
# NEW RELIC APM CONFIGURATION
NEW_RELIC_APP_NAME=beef-briefing
NEW_RELIC_LICENSE_KEY=eu01xxxxxxxxxxxxxxxxxxxxxxxxxxNRAL
```

## Instrumentation Patterns

### Basic Segment (Database/Storage)

```go
func (r *MessageRepository) Insert(ctx context.Context, msg *models.Message) error {
    txn := newrelic.FromContext(ctx)
    segment := txn.StartSegment("db:insert-message")
    defer segment.End()

    // ... database operation ...
}
```

### External Segment (HTTP Call)

```go
func callExternalAPI(ctx context.Context, url string) (*http.Response, error) {
    txn := newrelic.FromContext(ctx)
    segment := newrelic.ExternalSegment{
        StartTime: txn.StartSegmentNow(),
        URL:       url,
        Host:      "api.example.com",
        Procedure: "GET /endpoint",
        Library:   "http-client",
    }
    defer segment.End()

    resp, err := http.Get(url)
    segment.Response = resp
    return resp, err
}
```

### Context Propagation

```go
// Pass transaction through context
ctx = newrelic.NewContext(ctx, txn)

// Retrieve in downstream functions
func downstream(ctx context.Context) {
    txn := newrelic.FromContext(ctx)
    segment := txn.StartSegment("downstream-work")
    defer segment.End()
}
```

## Distributed Tracing

Distributed tracing is enabled across all services:

```
Telegram --> Telegram Bot --> API Service --> PostgreSQL
                 |                 |
                 v                 v
           Telegram CDN         MinIO
```

Cross-service trace context is propagated via:
- HTTP headers for API calls (automatic with external segments)
- Context objects within services

## What Gets Tracked

### API Service

| Category | Metrics |
|----------|---------|
| HTTP | Request rate, response time, error rate per endpoint |
| Database | Query time per operation type (insert, select, update) |
| Storage | Upload/download time, file sizes |
| Business | Updates processed, media files handled |

### Telegram Bot

| Category | Metrics |
|----------|---------|
| Updates | Processing time, update types distribution |
| Downloads | Telegram API latency, file download times |
| API Calls | Internal API call latency, retry rates |
| Errors | Failed downloads, API errors, timeout rates |

### Dashboard

| Category | Metrics |
|----------|---------|
| HTTP | Page load times, API endpoint latency |
| Database | Query performance for analytics |
| Errors | Application errors, timeout rates |

## Import CLI

The `import-cli` tool does **not** have New Relic instrumentation. As a CLI tool for one-off data imports, it doesn't benefit from APM monitoring.

## Alerts Configuration (Recommended)

Configure these alerts in New Relic:

| Alert | Condition | Threshold |
|-------|-----------|-----------|
| High Error Rate | Error % > threshold | > 5% for 5 min |
| Slow Transactions | Avg response time | > 2s for 5 min |
| Service Down | No transactions | 0 for 3 min |
| Telegram API Latency | External segment time | > 5s avg |
| Database Slow | DB segment time | > 500ms avg |

## Troubleshooting

### No Data Appearing

1. Verify environment variables are set:
   ```bash
   docker exec api-service printenv | grep NEW_RELIC
   ```

2. Check agent logs:
   ```bash
   docker logs api-service 2>&1 | grep -i newrelic
   ```

3. Verify license key is valid and not expired

### High Overhead

If New Relic adds noticeable latency:
- Reduce log forwarding verbosity
- Adjust sampling rates in New Relic UI
- Disable in development environments

### Missing Segments

Ensure context propagation is correct:
```go
// Wrong - transaction not in context
txn := newrelic.FromContext(ctx) // returns nil

// Correct - ensure context has transaction
ctx = newrelic.NewContext(ctx, txn)
```

## Dependencies

### Go Services

```go
// go.mod
require (
    github.com/newrelic/go-agent/v3 v3.42.0
    github.com/newrelic/go-agent/v3/integrations/nrgorilla v1.2.5
)
```

### Dashboard

```txt
# requirements.txt
newrelic>=11.1.0
```

## Future Enhancements

1. **Database Query Visibility**: Add `nrpq` integration for PostgreSQL to capture actual SQL queries instead of generic segment names

2. **Custom Dashboards**: Build New Relic dashboards using custom attributes:
   - Messages per chat over time
   - Media upload success rates
   - User activity patterns
   - Telegram API health

3. **Browser Monitoring**: Add browser agent to dashboard for frontend performance

4. **Infrastructure Monitoring**: Deploy New Relic infrastructure agent on Linode server
