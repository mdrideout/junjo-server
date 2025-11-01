# Stage 1: Base - Install dependencies using uv
FROM python:3.14-alpine AS base

# Install build dependencies
RUN apk add --no-cache \
    git \
    gcc \
    g++ \
    musl-dev \
    libffi-dev

# Install uv
COPY --from=ghcr.io/astral-sh/uv:latest /uv /uvx /bin/

# Set working directory
WORKDIR /app

# Copy shared proto files (needed for code generation)
COPY proto/ ../proto/

# Copy dependency files
COPY backend/pyproject.toml backend/uv.lock* ./

# Install dependencies into virtual environment
RUN --mount=type=cache,target=/root/.cache/uv \
    uv sync --locked --no-install-project --no-dev

# Stage 2: Production - Final image
FROM python:3.14-alpine AS production

# Install tini for proper signal handling and libstdc++ for greenlet
RUN apk add --no-cache tini libstdc++

# Set working directory
WORKDIR /app

# Copy virtual environment from base stage
COPY --from=base /app/.venv ./.venv

# Set PATH to use virtual environment
ENV PATH="/app/.venv/bin:$PATH"
ENV PYTHONPATH=/app

# Copy application code first
COPY backend/app ./app

# Copy proto generation script and shared proto files
COPY backend/scripts/generate_proto.sh /tmp/generate_proto.sh
COPY proto/ ../proto/

# Generate proto files using unified script
RUN chmod +x /tmp/generate_proto.sh && \
    /tmp/generate_proto.sh /app/app/proto_gen /app/../proto && \
    rm /tmp/generate_proto.sh
COPY backend/.env.example ./.env.example

# Copy Alembic configuration and migrations
COPY backend/alembic.ini ./alembic.ini
COPY backend/migrations ./migrations

# Copy entrypoint script
COPY backend/entrypoint.sh /entrypoint.sh
RUN chmod +x /entrypoint.sh

# Create data directory
RUN mkdir -p ./dbdata

# Expose ports
EXPOSE 1323
EXPOSE 50053

# Health check
HEALTHCHECK --interval=30s --timeout=3s --start-period=5s --retries=3 \
    CMD python -c "import urllib.request; urllib.request.urlopen('http://localhost:1323/ping')"

# Use tini with entrypoint script for proper signal handling and migrations
ENTRYPOINT ["/sbin/tini", "--", "/entrypoint.sh"]

# Run uvicorn
CMD ["uvicorn", "app.main:app", "--host", "0.0.0.0", "--port", "1323"]

# Stage 3: Development - Add dev dependencies and hot reload
FROM production AS dev

# Install development dependencies using pip (uv not available in production stage)
RUN pip install watchfiles

# Override CMD for hot reload
CMD ["uvicorn", "app.main:app", "--host", "0.0.0.0", "--port", "1323", "--reload"]
