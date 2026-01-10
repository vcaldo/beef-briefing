FROM python:3.12-slim

WORKDIR /app

# Install dependencies
COPY apps/ml-dashboard/backend/requirements.txt .
RUN pip install --no-cache-dir -r requirements.txt

# Copy source
COPY apps/ml-dashboard/backend/ .

# Run with uvicorn
CMD ["uvicorn", "main:app", "--host", "0.0.0.0", "--port", "8052", "--reload"]
