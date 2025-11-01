# Junjo Server - Python Backend

Python/FastAPI backend for the Junjo Server LLM observability platform.

## Technology Stack

- **Python 3.14+** (latest stable)
- **FastAPI** (async web framework)
- **Pydantic v2+** (data validation and settings)
- **Loguru** (structured logging)
- **uv** (fast package management)

## Quick Start

### Prerequisites

- Python 3.14+
- [uv](https://github.com/astral-sh/uv) (recommended)

### Installation

```bash
# Navigate to backend directory
cd backend_python

# Install dependencies including dev tools (pytest, ruff, mypy)
uv sync --all-extras

# Or install only production dependencies
uv sync
```

**Note:** Use `--all-extras` to install development dependencies (pytest, pytest-asyncio, ruff, mypy). This is required for running tests and linters.

### Running the Development Server

#### Option 1: Using uv run (Recommended)
```bash
# Start the server with hot reload
uv run uvicorn app.main:app --reload --host 0.0.0.0 --port 1324
```

#### Option 2: Using Python directly
```bash
# Start via main module
uv run python -m app.main
```

#### Option 3: Using the virtual environment directly
```bash
# Activate virtual environment
source .venv/bin/activate  # On Windows: .venv\Scripts\activate

# Run uvicorn
uvicorn app.main:app --reload --host 0.0.0.0 --port 1324
```

The API will be available at:
- **API**: http://localhost:1324
- **Interactive Docs (Swagger)**: http://localhost:1324/docs
- **Alternative Docs (ReDoc)**: http://localhost:1324/redoc
- **Health Check**: http://localhost:1324/health
- **Ping**: http://localhost:1324/ping

### Testing Endpoints

```bash
# Test ping endpoint
curl http://localhost:1324/ping

# Test health endpoint
curl http://localhost:1324/health

# Test root endpoint
curl http://localhost:1324/
```

### Running Tests

```bash
# Run all tests with pytest-asyncio
uv run pytest

# Run tests with verbose output
uv run pytest -v

# Run specific test file
uv run pytest tests/test_main.py -v

# Run with coverage report
uv run pytest --cov=app --cov-report=term-missing

# Generate HTML coverage report
uv run pytest --cov=app --cov-report=html
# Then open: open htmlcov/index.html
```

## Project Structure

```
backend_python/
├── app/
│   ├── config/          # Settings and configuration
│   ├── features/        # Feature-based modules
│   ├── common/          # Shared utilities
│   └── main.py          # FastAPI app
├── tests/               # Tests
└── pyproject.toml       # Dependencies
```

## Configuration

Configuration is managed via environment variables (see `.env.example`).

Settings are loaded using Pydantic Settings with the following precedence:
1. Environment variables
2. `.env` file
3. Default values in `app/config/settings.py`

### Key Configuration Variables

```bash
# Application
DEBUG=true                           # Enable debug logging
PORT=1324                           # Server port (1324 for dev, 1323 for production)
CORS_ORIGINS=["http://localhost:5151"]  # Allowed CORS origins

# Database
DB_SQLITE_PATH=./dbdata/junjo.db    # SQLite database path
DB_DUCKDB_PATH=./dbdata/traces.duckdb  # DuckDB database path

# Ingestion Service
INGESTION_HOST=localhost             # Ingestion service host
INGESTION_PORT=50052                # Ingestion service port
```

## Development Tools

### Linting

```bash
# Run ruff linter
uv run ruff check app/

# Auto-fix issues
uv run ruff check app/ --fix

# Format code
uv run ruff format app/
```

### Type Checking

```bash
# Run mypy type checker
uv run mypy app/
```

## Docker

### Building

```bash
# Build production image
docker build -t junjo-backend:latest .

# Build development image with hot reload
docker build -t junjo-backend:dev --target dev .
```

### Running

```bash
# Run production container
docker run -p 1324:1324 --env-file .env junjo-backend:latest

# Run development container with volume mount
docker run -p 1324:1324 \
  -v $(pwd)/app:/app/app \
  --env-file .env \
  junjo-backend:dev
```

### Using Docker Compose

```bash
# Start Python backend alongside Go backend
docker compose -f ../docker-compose.dev.yml up junjo-server-backend-python

# View logs
docker compose -f ../docker-compose.dev.yml logs -f junjo-server-backend-python
```

## Troubleshooting

### Port Already in Use

```bash
# Check what's using port 1324
lsof -i :1324

# Kill the process
kill -9 <PID>
```

### Module Import Errors

If you see `ModuleNotFoundError: No module named 'app'`:

```bash
# Make sure you're in the backend_python directory
cd backend_python

# Ensure dependencies are installed
uv sync

# Run with PYTHONPATH set
PYTHONPATH=. uv run uvicorn app.main:app --reload
```

### Virtual Environment Issues

```bash
# Remove existing venv and reinstall
rm -rf .venv
uv sync
```

## CI/CD Notes

For automated testing in CI/CD pipelines:

```bash
# Install dependencies (including dev tools)
uv sync --all-extras

# Run linter
uv run ruff check app/

# Run type checker
uv run mypy app/

# Run tests with coverage
uv run pytest --cov=app --cov-fail-under=80
```
