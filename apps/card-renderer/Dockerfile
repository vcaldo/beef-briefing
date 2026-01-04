# Card Renderer Service
# Multi-stage build for minimal image size

# Build stage
FROM python:3.12-slim AS builder

WORKDIR /build

# Install build dependencies
RUN apt-get update && apt-get install -y --no-install-recommends \
    gcc \
    libpq-dev \
    && rm -rf /var/lib/apt/lists/*

COPY apps/card-renderer/requirements.txt .
# Install to /usr/local (global) instead of --user for multi-stage compatibility
RUN pip install --no-cache-dir -r requirements.txt

# Runtime stage
FROM python:3.12-slim

# Install runtime dependencies + Playwright browser dependencies
RUN apt-get update && apt-get install -y --no-install-recommends \
    libpq5 \
    # Playwright/Chromium dependencies
    libnss3 \
    libnspr4 \
    libatk1.0-0 \
    libatk-bridge2.0-0 \
    libcups2 \
    libdrm2 \
    libxcomposite1 \
    libxdamage1 \
    libxfixes3 \
    libxrandr2 \
    libgbm1 \
    libpango-1.0-0 \
    libcairo2 \
    libasound2 \
    libxshmfence1 \
    libxkbcommon0 \
    # Fonts
    fonts-noto-color-emoji \
    fonts-liberation \
    fontconfig \
    && rm -rf /var/lib/apt/lists/* \
    && fc-cache -fv

WORKDIR /app

# Copy installed packages from builder (global site-packages)
COPY --from=builder /usr/local/lib/python3.12/site-packages /usr/local/lib/python3.12/site-packages
COPY --from=builder /usr/local/bin /usr/local/bin

# Create non-root user for security
RUN useradd -m -u 1000 appuser

# Install Playwright browsers as appuser
USER appuser
ENV PLAYWRIGHT_BROWSERS_PATH=/home/appuser/.cache/ms-playwright
RUN playwright install chromium

# Switch back to root to copy app files with correct ownership
USER root

# Copy application code
COPY --chown=appuser:appuser apps/card-renderer/config ./config
COPY --chown=appuser:appuser apps/card-renderer/src ./src
COPY --chown=appuser:appuser apps/card-renderer/templates ./templates
COPY --chown=appuser:appuser apps/card-renderer/main.py .

USER appuser

EXPOSE 8051

CMD ["uvicorn", "main:app", "--host", "0.0.0.0", "--port", "8051"]
