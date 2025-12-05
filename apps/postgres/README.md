# PostgreSQL Database Service

PostgreSQL database for storing:
- Messages from Telegram
- Analysis results from LLM Analyzer
- Application metadata

## Structure

- `migrations/` - Database migration scripts (numbered SQL files)
- `seeds/` - Initial data seed scripts
- `Dockerfile` - Custom PostgreSQL image with initialization scripts

## Configuration

Environment variables (from `.env.cloud` or `.env.dev`):
- `POSTGRES_USER` - Database user
- `POSTGRES_PASSWORD` - Database password
- `POSTGRES_DB` - Default database name
- `POSTGRES_PORT` - Port (default: 5432)

## Usage

All `.sql` files in `migrations/` and `seeds/` directories are automatically run on container startup in alphabetical order.

## Development

```bash
# Connect to running database
psql -h localhost -U postgres -d beef_db
```

## Schema

Database tables and schemas are defined in `migrations/*.sql` files.
