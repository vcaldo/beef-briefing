"""Card Renderer Service - FastAPI Application."""

import logging
import sys
from contextlib import asynccontextmanager

from fastapi import FastAPI
from fastapi.middleware.cors import CORSMiddleware
from sqlalchemy import create_engine

from config import load_config
from src.api import router
from src.generator import CardGenerator
from src.storage import CardStorageClient

# Load configuration
config = load_config()

# Configure logging
log_level = getattr(logging, config.log_level.upper(), logging.INFO)
log_format = "%(asctime)s - %(name)s - %(levelname)s - %(message)s"

if config.is_production():
    import json

    class JsonFormatter(logging.Formatter):
        def format(self, record):
            log_record = {
                "timestamp": self.formatTime(record),
                "level": record.levelname,
                "name": record.name,
                "message": record.getMessage(),
            }
            if record.exc_info:
                log_record["exception"] = self.formatException(record.exc_info)
            return json.dumps(log_record)

    handler = logging.StreamHandler(sys.stdout)
    handler.setFormatter(JsonFormatter())
else:
    handler = logging.StreamHandler(sys.stdout)
    handler.setFormatter(logging.Formatter(log_format))

logging.basicConfig(level=log_level, handlers=[handler])
logger = logging.getLogger(__name__)

# Initialize components
engine = create_engine(config.dsn(), pool_pre_ping=True)

storage = CardStorageClient(
    endpoint=config.minio_endpoint,
    access_key=config.minio_access_key,
    secret_key=config.minio_secret_key,
    bucket=config.minio_bucket,
    use_ssl=config.minio_use_ssl,
    region=config.minio_region,
)

generator = CardGenerator(
    engine=engine,
    storage=storage,
    templates_dir=config.templates_dir,
    card_width=config.card_width,
    card_height=config.card_height,
    card_scale=config.card_scale,
    tier_class_fn=config.get_tier_class,
)


@asynccontextmanager
async def lifespan(app: FastAPI):
    """Application lifespan handler."""
    logger.info("Starting Card Image Generator service...")

    # Initialize generator (starts Playwright)
    await generator.start()

    # Store generator, API keys, and config in app state
    app.state.generator = generator
    app.state.api_keys = config.get_app_keys()
    app.state.default_theme = config.default_card_theme

    logger.info(
        f"Service ready on {config.card_renderer_host}:{config.card_renderer_port}"
    )

    yield

    # Cleanup
    logger.info("Shutting down Card Image Generator service...")
    await generator.stop()


# Create FastAPI app
app = FastAPI(
    title="Card Renderer",
    description="Generates static card images from user stats",
    version="1.0.0",
    lifespan=lifespan,
)

# Add CORS middleware if origins are configured
cors_origins = config.cors_origins
if cors_origins:
    app.add_middleware(
        CORSMiddleware,
        allow_origins=cors_origins,
        allow_credentials=True,
        allow_methods=["GET", "POST", "OPTIONS"],
        allow_headers=["*"],
    )
    logger.info(f"CORS enabled for origins: {cors_origins}")

# Include router
app.include_router(router)


if __name__ == "__main__":
    import uvicorn

    uvicorn.run(
        "main:app",
        host=config.card_renderer_host,
        port=config.card_renderer_port,
        reload=not config.is_production(),
    )
