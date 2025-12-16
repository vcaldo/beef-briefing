# Build stage - install dependencies
# Note: CUDA version must match host driver capabilities (nvidia-smi shows supported version)
FROM nvidia/cuda:12.6.3-runtime-ubuntu24.04 AS builder

WORKDIR /build

# Install Python 3.14 from deadsnakes PPA
RUN apt-get update && apt-get install -y --no-install-recommends \
    software-properties-common gnupg \
    && add-apt-repository -y ppa:deadsnakes/ppa \
    && apt-get update \
    && apt-get install -y --no-install-recommends \
    python3.14 python3.14-venv python3.14-dev \
    && rm -rf /var/lib/apt/lists/*

# Create virtual environment
RUN python3.14 -m venv /opt/venv
ENV PATH="/opt/venv/bin:$PATH"

# Copy requirements first for layer caching
COPY apps/ml-processor/requirements.txt .

# Install PyTorch with CUDA 12.6 support from PyTorch index
RUN pip install --no-cache-dir torch --index-url https://download.pytorch.org/whl/cu126

# Install remaining dependencies
RUN pip install --no-cache-dir -r requirements.txt

# Runtime stage
FROM nvidia/cuda:12.6.3-runtime-ubuntu24.04

# Install Python 3.14 runtime from deadsnakes PPA
RUN apt-get update && apt-get install -y --no-install-recommends \
    software-properties-common gnupg \
    && add-apt-repository -y ppa:deadsnakes/ppa \
    && apt-get update \
    && apt-get install -y --no-install-recommends \
    python3.14 python3.14-venv \
    && rm -rf /var/lib/apt/lists/*

WORKDIR /app

# Copy virtual environment from builder
COPY --from=builder /opt/venv /opt/venv
ENV PATH="/opt/venv/bin:$PATH"

# Set Python to run unbuffered for proper logging
ENV PYTHONUNBUFFERED=1

# Copy application code
COPY apps/ml-processor/config ./config
COPY apps/ml-processor/src ./src
COPY apps/ml-processor/main.py .

CMD ["python", "main.py"]
