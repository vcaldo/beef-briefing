# Card Image Generator Service
# Multi-stage build for minimal image size

# Build stage
FROM python:3.12-slim AS builder

WORKDIR /build

# Install build dependencies
RUN apt-get update && apt-get install -y --no-install-recommends \
    gcc \
    libpq-dev \
    && rm -rf /var/lib/apt/lists/*

COPY apps/card-image-generator/requirements.txt .
RUN pip install --no-cache-dir --user -r requirements.txt

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

# Copy installed packages from builder
COPY --from=builder /root/.local /root/.local
ENV PATH=/root/.local/bin:$PATH

# Install Playwright browsers (Chromium only)
RUN playwright install chromium

# Copy application code
COPY apps/card-image-generator/config ./config
COPY apps/card-image-generator/src ./src
COPY apps/card-image-generator/templates ./templates
COPY apps/card-image-generator/main.py .

# Create non-root user for security
RUN useradd -m -u 1000 appuser && chown -R appuser:appuser /app
USER appuser

EXPOSE 8051

CMD ["uvicorn", "main:app", "--host", "0.0.0.0", "--port", "8051"]
