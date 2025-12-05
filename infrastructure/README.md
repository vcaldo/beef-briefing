# Infrastructure & Deployment

Configuration and orchestration files for deploying beef-briefing.

## Docker Compose Files

### docker-compose.cloud.yml
Cloud environment for VPS box. Services:
- PostgreSQL (database)
- API Service (Golang)
- Telegram Bot (Golang)
- Dashboard (Python Streamlit)
- MinIO (S3-compatible object storage)

Deploy with:
```bash
docker-compose -f docker-compose.cloud.yml --env-file .env.cloud up
```

### docker-compose.llm.yml
LLM environment for GPU box. Services:
- Ollama (LLM server)
- LLM Analyzer (Golang)

Deploy with:
```bash
docker-compose -f docker-compose.llm.yml --env-file .env.llm up
```

### docker-compose.dev.yml
Development environment for local machine. Includes all services:
- PostgreSQL
- API Service
- Telegram Bot
- Dashboard
- Ollama
- LLM Analyzer
- MinIO (S3-compatible object storage)

Deploy with:
```bash
docker-compose -f docker-compose.dev.yml --env-file .env.dev up
```

## Environment Files

- `.env.example` - Template for environment variables
- `.env.cloud` - Cloud box configuration
- `.env.llm` - LLM box configuration
- `.env.dev` - Development configuration

Copy `.env.example` and fill in values for each environment.

## Service Communication

### Cloud → LLM Communication
- LLM Analyzer needs to reach API Service on Cloud box
- Set `CLOUD_API_HOST` in `.env.llm` to Cloud box IP/hostname
- Set `CLOUD_API_PORT` to API Service port (default: 8080)

### Within Same Box
- Services communicate via Docker network using service names
- Example: `http://api-service:8080` (if on same network)

## Volumes

### Cloud
- `postgres_data` - PostgreSQL data persistence
- `minio_data` - MinIO object storage persistence

### LLM
- `ollama_models` - Ollama model storage (persistent)

### Dev
- `postgres_data_dev` - Local PostgreSQL data
- `ollama_models_dev` - Local Ollama models
- `minio_data_dev` - Local MinIO object storage

## MinIO Configuration

### Default Credentials
- Username: `minioadmin`
- Password: `minioadmin`

Override via environment variables in `.env` files:
```bash
MINIO_ROOT_USER=your_username
MINIO_ROOT_PASSWORD=your_password
```

### Access
- MinIO API: `http://localhost:9000` (dev) or configured port (cloud)
- MinIO Console: `http://localhost:9001` (dev) or configured port (cloud)

## Networks

Each compose file creates an isolated bridge network:
- Cloud: `beef-cloud-network`
- LLM: `beef-llm-network`
- Dev: `beef-dev-network`
