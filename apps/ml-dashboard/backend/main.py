"""
ML Dashboard Backend - FastAPI application for exploring ML-processed data.

Dev-only tool for browsing ML results, semantic search, topics, and user analytics.
"""

import logging
import sys
from contextlib import asynccontextmanager
from typing import Optional

from fastapi import FastAPI
from fastapi.middleware.cors import CORSMiddleware
from sqlalchemy import create_engine, text

from config import settings
from db import DashboardQueries
from vector import EmbeddingService, QdrantSearcher

# Configure logging
log_level = getattr(logging, settings.log_level.upper(), logging.INFO)
logging.basicConfig(
    level=log_level,
    format="%(asctime)s - %(name)s - %(levelname)s - %(message)s",
    stream=sys.stdout,
)
logger = logging.getLogger(__name__)

# Suppress verbose logging from libraries
logging.getLogger("httpx").setLevel(logging.WARNING)
logging.getLogger("sentence_transformers").setLevel(logging.WARNING)
logging.getLogger("qdrant_client").setLevel(logging.WARNING)

# Global instances
queries: Optional[DashboardQueries] = None
qdrant: Optional[QdrantSearcher] = None
embeddings: Optional[EmbeddingService] = None


@asynccontextmanager
async def lifespan(app: FastAPI):
    """Application lifespan - setup and teardown."""
    global queries, qdrant, embeddings

    # Setup database
    logger.info(f"Connecting to database at {settings.db_host}:{settings.db_port}")
    engine = create_engine(settings.dsn(), pool_pre_ping=True)

    # Test database connection
    try:
        with engine.connect() as conn:
            conn.execute(text("SELECT 1"))
        logger.info("Database connection successful")
    except Exception as e:
        logger.error(f"Database connection failed: {e}")
        raise

    queries = DashboardQueries(engine)

    # Setup Qdrant (optional - may not be available)
    if settings.qdrant_enabled:
        logger.info(f"Connecting to Qdrant at {settings.qdrant_host}:{settings.qdrant_port}")
        qdrant = QdrantSearcher(host=settings.qdrant_host, port=settings.qdrant_port)
        if qdrant.is_available():
            info = qdrant.get_collection_info()
            logger.info(f"Qdrant connected: {info['points_count']} embeddings")
        else:
            logger.warning("Qdrant not available - semantic search disabled")
            qdrant = None
    else:
        logger.info("Qdrant disabled by configuration")

    # Setup embedding service (lazy loaded)
    embeddings = EmbeddingService(model_name=settings.embedding_model)
    logger.info(f"Embedding service initialized (model: {settings.embedding_model})")

    logger.info("ML Dashboard backend started")
    yield

    # Cleanup
    logger.info("ML Dashboard backend shutting down")


# Create FastAPI app
app = FastAPI(
    title="ML Dashboard",
    description="Dev-only dashboard for exploring ML-processed Telegram data",
    version="0.1.0",
    lifespan=lifespan,
)

# CORS middleware for frontend
app.add_middleware(
    CORSMiddleware,
    allow_origins=["*"],  # Dev-only, allow all origins
    allow_credentials=True,
    allow_methods=["*"],
    allow_headers=["*"],
)

# Import and include routers
from api import messages_router, search_router, topics_router, users_router

app.include_router(messages_router)
app.include_router(search_router)
app.include_router(topics_router)
app.include_router(users_router)


@app.get("/health")
def health():
    """Health check endpoint."""
    return {"status": "healthy"}


@app.get("/api/stats")
def stats(chat_id: int):
    """Get ML processing statistics for a chat."""
    if not queries:
        return {"error": "Database not connected"}

    processing_stats = queries.get_processing_stats(chat_id)

    # Get Qdrant status
    qdrant_status = {"available": False, "points_count": 0}
    if qdrant:
        qdrant_status = qdrant.get_collection_info()

    return {
        "processing": processing_stats,
        "qdrant": qdrant_status,
    }


@app.get("/api/chats")
def list_chats():
    """Get list of all chats with message counts."""
    if not queries:
        return {"error": "Database not connected"}

    chats = queries.get_chats()
    return {"chats": chats}


if __name__ == "__main__":
    import uvicorn

    uvicorn.run(
        "main:app",
        host="0.0.0.0",
        port=8052,
        reload=True,
    )
