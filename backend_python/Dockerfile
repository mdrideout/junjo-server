# Stage 1: Base - Install dependencies using uv
FROM python:3.14-alpine AS base

# Install build dependencies
RUN apk add --no-cache \
    git \
    gcc \
    musl-dev \
    libffi-dev

# Install uv
COPY --from=ghcr.io/astral-sh/uv:latest /uv /uvx /bin/

# Set working directory
WORKDIR /app

# Copy dependency files
COPY pyproject.toml uv.lock* ./

# Install dependencies into virtual environment
RUN --mount=type=cache,target=/root/.cache/uv \
    uv sync --locked --no-install-project --no-dev

# Stage 2: Production - Final image
FROM python:3.14-alpine AS production

# Install tini for proper signal handling
RUN apk add --no-cache tini

# Set working directory
WORKDIR /app

# Copy virtual environment from base stage
COPY --from=base /app/.venv ./.venv

# Set PATH to use virtual environment
ENV PATH="/app/.venv/bin:$PATH"
ENV PYTHONPATH=/app

# Copy application code
COPY app ./app
COPY .env.example ./.env.example

# Create data directory
RUN mkdir -p ./dbdata

# Expose port
EXPOSE 1324

# Health check
HEALTHCHECK --interval=30s --timeout=3s --start-period=5s --retries=3 \
    CMD python -c "import urllib.request; urllib.request.urlopen('http://localhost:1324/ping')"

# Use tini as entrypoint for proper signal handling
ENTRYPOINT ["/sbin/tini", "--"]

# Run uvicorn
CMD ["uvicorn", "app.main:app", "--host", "0.0.0.0", "--port", "1324"]

# Stage 3: Development - Add dev dependencies and hot reload
FROM production AS dev

# Install development dependencies
RUN --mount=type=cache,target=/root/.cache/uv \
    /bin/uv pip install watchfiles

# Override CMD for hot reload
CMD ["uvicorn", "app.main:app", "--host", "0.0.0.0", "--port", "1324", "--reload"]
