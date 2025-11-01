# Junjo Server - Backend Service

FastAPI backend service for the Junjo Server LLM observability platform.

## Overview

The backend service provides:
- **HTTP REST API** for frontend and programmatic access
- **User authentication** with session management
- **LLM playground** for testing prompts across providers (OpenAI, Anthropic, Gemini)
- **Span querying & analytics** using DuckDB
- **Internal gRPC server** for authentication (port 50053)

**Tech Stack**: Python 3.13+, FastAPI, SQLAlchemy, SQLite (metadata), DuckDB (analytics), Loguru

---

## Running the Backend

### Primary Method: Docker Compose (Recommended)

**For running the full Junjo Server stack**, see the [root README.md](../README.md) Quick Start guide. The backend is part of the complete Docker Compose setup with all three services (backend, ingestion, frontend).

```bash
# From repository root
docker compose up -d

# View backend logs
docker compose logs -f junjo-server-backend

# Restart backend only
docker compose restart junjo-server-backend
```

The backend will be available at:
- **API**: http://localhost:1323
- **Health Check**: http://localhost:1323/health
- **gRPC (internal)**: localhost:50053

---

### Secondary Method: Direct Execution with uv (Development)

Run the backend directly for development, testing, or debugging. This is useful when:
- Working on backend-specific features
- Running integration tests locally
- Debugging without Docker overhead

#### Prerequisites

- **Python 3.13+**
- **[uv](https://github.com/astral-sh/uv)** (fast package manager)
- **`.env` file** configured (see root README)

#### Setup

```bash
# Navigate to backend directory
cd backend

# Install dependencies (includes dev tools: pytest, ruff)
uv sync --all-extras

# Or install only production dependencies
uv sync
```

**Note**: `--all-extras` installs development dependencies (pytest, pytest-asyncio, httpx, ruff). Required for running tests and linters.

#### Start the Backend

```bash
# Option 1: Using uv run (recommended)
uv run uvicorn app.main:app --reload --host 0.0.0.0 --port 1323

# Option 2: Via main module
uv run python -m app.main

# Option 3: With activated virtual environment
source .venv/bin/activate  # On Windows: .venv\Scripts\activate
uvicorn app.main:app --reload --host 0.0.0.0 --port 1323
```

The backend will be available at:
- **API**: http://localhost:1323
- **Health Check**: http://localhost:1323/health

**Important**: The backend automatically starts its internal gRPC server on port 50053 via the FastAPI lifespan manager. No additional steps needed.

#### Quick Test

```bash
# Test health endpoint
curl http://localhost:1323/health

# Test ping endpoint
curl http://localhost:1323/ping
```

---

## Testing

### Test Categories

Tests use pytest markers for organization:

- **`unit`**: Fast, isolated unit tests (no external dependencies)
- **`integration`**: Integration tests (require running backend service)
- **`requires_grpc_server`**: Tests requiring gRPC server on port 50053
- **`requires_gemini_api`**: Tests requiring `GEMINI_API_KEY` environment variable
- **`requires_openai_api`**: Tests requiring `OPENAI_API_KEY` environment variable
- **`requires_anthropic_api`**: Tests requiring `ANTHROPIC_API_KEY` environment variable
- **`security`**: Security tests (auth bypass, SQL injection)
- **`concurrency`**: Concurrency and race condition tests
- **`error_recovery`**: Error recovery and resilience tests

### Running Tests

#### Unit Tests (Fast, No Dependencies)

```bash
# Run all unit tests (excludes integration tests)
uv run pytest -m "not integration" -v

# Run specific test file
uv run pytest tests/test_main.py -v

# Run with coverage
uv run pytest -m "not integration" --cov=app --cov-report=term-missing
```

#### Integration Tests (Requires Backend Running)

Integration tests require the backend service to be running (gRPC server on port 50053).

**Option 1: Run Backend Service Directly** (Recommended for local development)

```bash
# Terminal 1: Start backend
cd backend
uv run uvicorn app.main:app --host 0.0.0.0 --port 1323

# Terminal 2: Run integration tests
cd backend
uv run pytest -m "integration" -v
```

**Option 2: Use Docker Compose** (Matches CI environment)

```bash
# Terminal 1: Start all services
docker compose up --build

# Terminal 2: Run integration tests
cd backend
uv run pytest -m "integration" -v
```

#### LLM Playground Tests (Requires API Keys)

```bash
# Run Gemini tests (requires GEMINI_API_KEY in .env)
uv run pytest -m "requires_gemini_api" -v

# Run OpenAI tests (requires OPENAI_API_KEY in .env)
uv run pytest -m "requires_openai_api" -v

# Run Anthropic tests (requires ANTHROPIC_API_KEY in .env)
uv run pytest -m "requires_anthropic_api" -v
```

#### All Tests

```bash
# Run everything (requires backend running + API keys)
uv run pytest -v
```

### GitHub Actions CI

Tests run automatically on pull requests:
- **Unit tests**: Fast execution with no external dependencies
- **Integration tests**: Run with Docker Compose and GitHub Secrets for API keys

See [`.github/workflows/test.yml`](../.github/workflows/test.yml) for CI configuration.

---

## Development Tools

### Linting and Formatting

```bash
# Run ruff linter
uv run ruff check app/

# Auto-fix issues
uv run ruff check app/ --fix

# Format code
uv run ruff format app/
```

### Code Quality Checks

```bash
# Run all checks before committing
uv run ruff check app/
uv run pytest -m "not integration" -v
```

---

## Project Structure

```
backend/
├── app/
│   ├── config/                 # Settings and configuration
│   │   ├── settings.py         # Pydantic settings (env vars)
│   │   └── logger.py           # Loguru setup
│   ├── features/               # Feature modules
│   │   ├── auth/               # Authentication & sessions
│   │   ├── api_keys/           # API key management
│   │   ├── llm_playground/     # LLM playground
│   │   ├── otel_spans/         # Span querying
│   │   └── span_ingestion/     # Span ingestion from gRPC
│   ├── common/                 # Shared utilities
│   │   ├── audit.py            # Audit logging
│   │   └── responses.py        # Common response models
│   ├── db_sqlite/              # SQLite (users, API keys)
│   ├── db_duckdb/              # DuckDB (span analytics)
│   ├── grpc_server.py          # Internal gRPC server
│   └── main.py                 # FastAPI app entry point
├── tests/                      # Test suite
│   ├── test_main.py            # Basic tests
│   ├── integration/            # Integration tests
│   ├── security/               # Security tests
│   └── error_recovery/         # Error recovery tests
├── pyproject.toml              # Dependencies & tool config
└── README.md                   # This file
```

---

## Configuration

The backend reads configuration from environment variables (`.env` file at repository root).

**See the [root README.md](../README.md#configuration) for complete configuration details.**

### Key Backend-Specific Variables

```bash
# Ports
PORT=1323                       # Backend HTTP port
GRPC_PORT=50053                 # Internal gRPC port

# Database paths
DB_SQLITE_PATH=/dbdata/sqlite/junjo.db
DB_DUCKDB_PATH=/dbdata/duckdb/traces.duckdb

# Logging
LOG_LEVEL=info                  # debug | info | warn | error
LOG_FORMAT=text                 # json | text

# LLM API keys (optional, for playground)
OPENAI_API_KEY=sk-...
ANTHROPIC_API_KEY=sk-ant-...
GEMINI_API_KEY=...
```

Configuration is loaded using **Pydantic Settings** with precedence:
1. Environment variables
2. `.env` file
3. Default values in `app/config/settings.py`

---

## Troubleshooting

### Port Already in Use

```bash
# Check what's using port 1323
lsof -i :1323

# Kill the process
kill -9 <PID>

# Or change the port in .env
PORT=1324
```

### Module Import Errors

If you see `ModuleNotFoundError: No module named 'app'`:

```bash
# Ensure you're in the backend directory
cd backend

# Reinstall dependencies
uv sync --all-extras

# Run with PYTHONPATH set
PYTHONPATH=. uv run uvicorn app.main:app --reload
```

### Virtual Environment Issues

```bash
# Remove and recreate virtual environment
rm -rf .venv
uv venv --python 3.13
uv sync --all-extras
```

### Integration Test Failures

**Symptom**: `pytest -m "integration"` fails with connection errors

**Solution**: Ensure backend is running on port 1323 with gRPC server on port 50053

```bash
# Terminal 1: Start backend
uv run uvicorn app.main:app --host 0.0.0.0 --port 1323

# Terminal 2: Verify gRPC server is running
lsof -i :50053

# Terminal 2: Run tests
uv run pytest -m "integration" -v
```

---

## Additional Resources

- **[Root README](../README.md)** - Full Junjo Server documentation
- **[Deployment Guide](../docs/DEPLOYMENT.md)** - Production deployment instructions
- **[Junjo Python Library](https://github.com/mdrideout/junjo)** - AI graph workflow framework
