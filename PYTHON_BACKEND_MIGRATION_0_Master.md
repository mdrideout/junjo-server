# Python Backend Migration - Master Plan

> **Status**: Planning Phase
> **Target**: Migrate Go backend to Python/FastAPI while maintaining feature parity
> **Strategy**: Build new Python backend adjacent to old Go backend, test incrementally, then cut over

---

## Table of Contents

1. [Migration Philosophy](#migration-philosophy)
2. [Why Python/FastAPI](#why-pythonfastapi)
3. [Architecture Overview](#architecture-overview)
4. [Code Organization Principles](#code-organization-principles)
5. [Technology Stack](#technology-stack)
6. [Migration Phases](#migration-phases)
7. [Directory Structure](#directory-structure)
8. [Testing Strategy](#testing-strategy)
9. [Deployment Strategy](#deployment-strategy)
10. [Rollback Plan](#rollback-plan)

---

## Migration Philosophy

### Core Principles (from AGENTS.md)

The Python backend will maintain the same organizational philosophy as the Go backend:

1. **Feature-Based Organization** - "Delete a feature by deleting its folder"
2. **Layered Separation of Concerns** - Router → Service → Repository
3. **Single Responsibility Principle** - Each module has one clear purpose
4. **Defense in Depth** - Validate early (router), authorize deep (repository)
5. **Co-located Tests** - Tests live next to implementation

These principles are **language-agnostic** and will be preserved in the migration.

---

## Why Python/FastAPI

### Strategic Reasons

1. **LLM Ecosystem Access**
   - LiteLLM (100+ providers, unified API, automatic OpenTelemetry)
   - Official SDKs (OpenAI, Anthropic, Google)
   - Instructor (structured outputs)
   - Eliminates manual schema transformation bugs

2. **Data/Analytics Libraries**
   - DuckDB Python client (mature, Pandas interop)
   - Pandas (data analysis on telemetry)
   - NumPy (numerical operations)
   - Future ML features (anomaly detection, optimization)

3. **Target Audience Fit**
   - AI/ML developers expect Python
   - Better contributor engagement
   - Familiar stack for self-hosting users

4. **FastAPI Advantages**
   - Automatic OpenAPI/Swagger documentation
   - Type safety with Pydantic (runtime validation)
   - Async/await (optimal for I/O-bound LLM proxying)
   - WebSocket support (streaming LLM responses)
   - Best-in-class DX (developer experience)

### What We're NOT Migrating

- **Ingestion Service (Go)** - Stays as-is, it's working perfectly
- **Frontend (React/TypeScript)** - No changes needed
- **BadgerDB WAL** - Ingestion service continues to own this

---

## Architecture Overview

```
┌──────────────────────────────────────────────┐
│ Python/FastAPI Backend (NEW)                 │
│                                              │
│  ┌────────────────────────────────────┐    │
│  │ REST API Layer                     │    │
│  │  - FastAPI routers                 │    │
│  │  - Pydantic validation             │    │
│  │  - Automatic OpenAPI docs          │    │
│  └────────────────┬───────────────────┘    │
│                   ↓                          │
│  ┌────────────────────────────────────┐    │
│  │ Business Logic Layer               │    │
│  │  - Service classes                 │    │
│  │  - Domain logic                    │    │
│  │  - LiteLLM integration             │    │
│  └────────────────┬───────────────────┘    │
│                   ↓                          │
│  ┌────────────────────────────────────┐    │
│  │ Data Layer                         │    │
│  │  - SQLAlchemy models (SQLite)      │    │
│  │  - DuckDB queries (analytics)      │    │
│  │  - Repository pattern              │    │
│  └────────────────────────────────────┘    │
│                                              │
│  ┌────────────────────────────────────┐    │
│  │ Infrastructure                     │    │
│  │  - Session/cookie auth             │    │
│  │  - gRPC client (ingestion service) │    │
│  │  - OTEL span processor             │    │
│  └────────────────────────────────────┘    │
└──────────────────┬───────────────────────────┘
                   │
                   ↓
┌──────────────────────────────────────────────┐
│ Go Ingestion Service (UNCHANGED)             │
│  - gRPC server (receives OTEL spans)         │
│  - BadgerDB WAL                              │
│  - Internal gRPC API (span reader)           │
└──────────────────────────────────────────────┘
```

---

## Code Organization Principles

### Python/FastAPI Translation of AGENTS.md Patterns

#### Go → Python Terminology Mapping

| Go Concept | Python/FastAPI Equivalent |
|------------|---------------------------|
| `handler.go` | `router.py` (FastAPI route functions) |
| `service.go` | `service.py` (business logic classes) |
| `repository.go` | `repository.py` (data access classes) |
| `dto.go` | `schemas.py` (Pydantic models) |
| `models.go` | `models.py` (SQLAlchemy ORM models) |
| Echo middleware | FastAPI dependencies/middleware |
| sqlc queries | SQLAlchemy ORM queries |

#### Feature Structure Pattern

```
backend_python/
  app/
    features/
      api_keys/                    # Feature folder
        __init__.py
        router.py                  # FastAPI routes (was handler.go)
        service.py                 # Business logic (was service.go)
        repository.py              # Database access (was repository.go)
        schemas.py                 # Pydantic models (was dto.go)
        models.py                  # SQLAlchemy models (was models.go)
        dependencies.py            # FastAPI dependencies
        test_router.py             # Router tests (co-located)
        test_service.py            # Service tests (co-located)
        test_repository.py         # Repository tests (co-located)
```

#### Layer Responsibilities (Same as Go)

**Router Layer** (`router.py` - was `handler.go`)
```python
from fastapi import APIRouter, Depends, HTTPException
from .schemas import CreateAPIKeyRequest, APIKeyResponse
from .service import APIKeyService
from .dependencies import get_api_key_service, get_current_user

router = APIRouter(prefix="/api-keys", tags=["api-keys"])

@router.post("/", response_model=APIKeyResponse)
async def create_api_key(
    request: CreateAPIKeyRequest,  # Pydantic auto-validates
    current_user: User = Depends(get_current_user),
    service: APIKeyService = Depends(get_api_key_service)
):
    """
    Responsibilities:
    - Receive HTTP request
    - Validate via Pydantic (automatic)
    - Extract dependencies (user, services)
    - Call service layer
    - Return HTTP response
    - Handle HTTP-specific concerns (status codes)
    """
    try:
        api_key = await service.create_api_key(current_user.id, request)
        return api_key
    except ValueError as e:
        raise HTTPException(status_code=400, detail=str(e))
```

**Service Layer** (`service.py` - same as Go)
```python
from sqlalchemy.ext.asyncio import AsyncSession
from .repository import APIKeyRepository
from .schemas import CreateAPIKeyRequest
import secrets

class APIKeyService:
    """
    Responsibilities:
    - Business logic and workflows
    - Coordinate multiple repositories
    - Transform data between layers
    - Handle business-level errors
    """

    def __init__(self, repository: APIKeyRepository):
        self.repository = repository

    async def create_api_key(
        self,
        user_id: str,
        request: CreateAPIKeyRequest
    ) -> APIKey:
        # Business logic: check limits
        existing_count = await self.repository.count_by_user(user_id)
        if existing_count >= 10:
            raise ValueError("Maximum API keys limit reached")

        # Business logic: generate secure key
        key_value = f"sk_{secrets.token_urlsafe(32)}"

        # Delegate to repository
        return await self.repository.create(user_id, request.name, key_value)
```

**Repository Layer** (`repository.py` - same as Go)
```python
from sqlalchemy import select, func
from sqlalchemy.ext.asyncio import AsyncSession
from .models import APIKey

class APIKeyRepository:
    """
    Responsibilities:
    - Database operations
    - Authorization checks (defense in depth)
    - Transform SQLAlchemy models to domain objects
    - Handle database-specific errors
    """

    def __init__(self, db: AsyncSession):
        self.db = db

    async def create(
        self,
        user_id: str,
        name: str,
        key_value: str
    ) -> APIKey:
        """Defense in depth - user_id baked into query"""
        api_key = APIKey(
            user_id=user_id,
            name=name,
            key_value=key_value
        )
        self.db.add(api_key)
        await self.db.commit()
        await self.db.refresh(api_key)
        return api_key

    async def get_by_id(self, user_id: str, key_id: str) -> APIKey | None:
        """Defense in depth - user_id in WHERE clause"""
        result = await self.db.execute(
            select(APIKey).where(
                APIKey.id == key_id,
                APIKey.user_id == user_id  # Authorization built into query
            )
        )
        return result.scalar_one_or_none()
```

**Schemas** (`schemas.py` - was `dto.go`)
```python
from pydantic import BaseModel, Field, ConfigDict
from datetime import datetime

class CreateAPIKeyRequest(BaseModel):
    """Request validation (like Go struct tags)"""
    name: str = Field(..., min_length=1, max_length=100)

class APIKeyResponse(BaseModel):
    """Response serialization (like Go DTO)"""
    id: str
    name: str
    created_at: datetime
    # key_value intentionally omitted (security)

    model_config = ConfigDict(from_attributes=True)  # Allow from SQLAlchemy
```

**Models** (`models.py` - was Go struct + sqlc)
```python
from sqlalchemy import Column, String, DateTime, ForeignKey
from sqlalchemy.sql import func
from ..database import Base

class APIKey(Base):
    """SQLAlchemy ORM model (replaces sqlc generated code)"""
    __tablename__ = "api_keys"

    id = Column(String, primary_key=True, default=lambda: str(uuid4()))
    user_id = Column(String, ForeignKey("users.id"), nullable=False)
    name = Column(String, nullable=False)
    key_value = Column(String, nullable=False, unique=True)
    created_at = Column(DateTime, server_default=func.now())
```

### Validation and Authorization Strategy (Same as Go)

**Validation:**
- ✅ Validate **as early as possible** - Pydantic in router (automatic)
- ✅ Use Pydantic models for consistency and auto-docs

**Authorization:**
- ✅ **Defense in depth** - same principle
- ✅ Early checks via FastAPI dependencies (user auth)
- ✅ Deep checks in repository (user_id in WHERE clauses)

```python
# Early validation (automatic via Pydantic)
@router.post("/")
async def create_api_key(
    request: CreateAPIKeyRequest,  # ✅ Validated here
    current_user: User = Depends(get_current_user)  # ✅ Authorized here
):
    ...

# Deep authorization (in repository)
async def get_by_id(self, user_id: str, key_id: str):
    # ✅ User ownership built into query
    result = await self.db.execute(
        select(APIKey).where(
            APIKey.id == key_id,
            APIKey.user_id == user_id
        )
    )
```

### Dependency Injection Pattern

FastAPI uses **dependency injection** (vs Go's manual DI):

```python
# dependencies.py
from fastapi import Depends
from sqlalchemy.ext.asyncio import AsyncSession
from ..database import get_db

def get_api_key_repository(
    db: AsyncSession = Depends(get_db)
) -> APIKeyRepository:
    return APIKeyRepository(db)

def get_api_key_service(
    repository: APIKeyRepository = Depends(get_api_key_repository)
) -> APIKeyService:
    return APIKeyService(repository)

# router.py
@router.post("/")
async def create_api_key(
    request: CreateAPIKeyRequest,
    service: APIKeyService = Depends(get_api_key_service)  # Auto-injected
):
    return await service.create_api_key(...)
```

**Benefits:**
- Automatic dependency resolution
- Easy to mock for testing
- Clear dependency graph
- Type-safe

### Testing Strategy (Same Philosophy as Go)

```python
# test_service.py (co-located with service.py)
import pytest
from unittest.mock import AsyncMock
from .service import APIKeyService

@pytest.mark.asyncio
async def test_create_api_key_success():
    # Arrange
    mock_repo = AsyncMock()
    mock_repo.count_by_user.return_value = 5
    service = APIKeyService(mock_repo)

    # Act
    result = await service.create_api_key("user-123", request)

    # Assert
    assert result.user_id == "user-123"
    mock_repo.create.assert_called_once()

@pytest.mark.asyncio
async def test_create_api_key_limit_exceeded():
    # Arrange
    mock_repo = AsyncMock()
    mock_repo.count_by_user.return_value = 10
    service = APIKeyService(mock_repo)

    # Act & Assert
    with pytest.raises(ValueError, match="Maximum API keys limit reached"):
        await service.create_api_key("user-123", request)
```

---

## Technology Stack

### Core Framework
- **FastAPI** (0.115+) - Modern async web framework
- **Uvicorn** - ASGI server
- **Python** (3.12+) - Latest stable

### Database
- **SQLAlchemy** (2.0+) - ORM with async support
- **Alembic** - Database migrations
- **asyncpg** - Async PostgreSQL driver (for future)
- **aiosqlite** - Async SQLite driver (current)
- **DuckDB** (Python client) - Analytics queries

### LLM Integration
- **LiteLLM** (1.50+) - Unified LLM provider interface
- **OpenTelemetry SDK** - OTEL instrumentation

### Authentication
- **python-jose** - JWT handling
- **passlib[bcrypt]** - Password hashing
- **python-multipart** - Form data parsing

### Communication
- **grpcio** - gRPC client (to ingestion service)
- **protobuf** - Protocol buffers

### Development
- **pytest** (8.0+) - Testing framework
- **pytest-asyncio** - Async test support
- **httpx** - Async HTTP client (for testing)
- **black** - Code formatting
- **ruff** - Fast linting
- **mypy** - Static type checking

### Deployment
- **Docker** - Containerization
- **Docker Compose** - Local orchestration

---

## Migration Phases

Each phase is documented in a separate file and can be developed/tested independently.

### Phase 1: Base FastAPI Setup
**File**: `PYTHON_BACKEND_MIGRATION_1_BASE_FASTAPI.md`

**Objectives:**
- Create `backend_python/` directory structure
- Set up FastAPI application with basic routing
- Configure CORS for frontend communication
- Add health check endpoints (`/ping`, `/health`)
- Set up development environment (venv, dependencies)
- Configure Docker build (multi-stage)
- Verify basic HTTP server functionality

**Success Criteria:**
- ✅ FastAPI app starts on port 1324 (adjacent to Go on 1323)
- ✅ CORS allows localhost:5151 (frontend)
- ✅ `/ping` returns 200 OK
- ✅ Docker build succeeds
- ✅ Hot reload works in development

**Dependencies:** None (greenfield)

---

### Phase 2: SQLAlchemy + Alembic Setup
**File**: `PYTHON_BACKEND_MIGRATION_2_SQLAlchemy_Alembic.md`

**Objectives:**
- Set up SQLAlchemy with async support
- Configure Alembic for migrations
- Create initial database schema (users, sessions)
- Implement database session management
- Add database dependency injection
- Test database connections and queries

**Success Criteria:**
- ✅ SQLAlchemy models defined for users
- ✅ Alembic migrations directory initialized
- ✅ `alembic upgrade head` creates tables
- ✅ Database session dependency works
- ✅ Can query users table

**Dependencies:** Phase 1

---

### Phase 3: Session/Cookie Authentication
**File**: `PYTHON_BACKEND_MIGRATION_3_SESSION_COOKIE_AUTH.md`

**Objectives:**
- Implement session-based authentication (matching Go)
- Create user registration endpoint
- Create login endpoint (session cookie)
- Create logout endpoint
- Add `get_current_user` dependency
- Implement password hashing (bcrypt)
- Test auth flow end-to-end

**Success Criteria:**
- ✅ POST `/auth/register` creates user
- ✅ POST `/auth/login` returns session cookie
- ✅ Protected routes require valid session
- ✅ GET `/auth/me` returns current user
- ✅ POST `/auth/logout` clears session
- ✅ Sessions stored in SQLite

**Dependencies:** Phase 2

---

### Phase 4: Protobuf + gRPC Client
**File**: `PYTHON_BACKEND_MIGRATION_4_PROTOBUF.md`

**Objectives:**
- Set up protobuf definitions (shared with Go ingestion)
- Generate Python gRPC client code
- Implement gRPC client to ingestion service
- Add internal auth gRPC client (API key validation)
- Test communication with ingestion service

**Success Criteria:**
- ✅ Protobuf files compiled to Python
- ✅ Can call ingestion service internal API
- ✅ Can read spans from WAL via gRPC
- ✅ Error handling for gRPC failures

**Dependencies:** Phase 1

---

### Phase 5: API Key Management Feature
**File**: `PYTHON_BACKEND_MIGRATION_5_API_KEY_FEATURE.md`

**Objectives:**
- Create API keys feature (first complete feature)
- Implement router/service/repository pattern
- Add CRUD endpoints for API keys
- Add API key authentication dependency
- Test feature end-to-end
- Validate against Go implementation

**Success Criteria:**
- ✅ GET `/api-keys` lists user's keys
- ✅ POST `/api-keys` creates new key
- ✅ DELETE `/api-keys/{id}` deletes key
- ✅ API key auth works for ingestion service
- ✅ Matches Go API behavior exactly

**Dependencies:** Phase 3

---

### Phase 6: DuckDB Integration
**File**: `PYTHON_BACKEND_MIGRATION_6_DUCKDB.md`

**Objectives:**
- Set up DuckDB connection management
- Implement span query repository
- Create trace/span query endpoints
- Add pagination and filtering
- Test query performance
- Validate against Go implementation

**Success Criteria:**
- ✅ DuckDB queries return span data
- ✅ GET `/otel/traces` works
- ✅ GET `/otel/trace/{id}` works
- ✅ Query performance acceptable
- ✅ Results match Go implementation

**Dependencies:** Phase 2

---

### Phase 7: OTEL Span Indexing
**File**: `PYTHON_BACKEND_MIGRATION_7_OTEL_INDEXING.md`

**Objectives:**
- Implement span processor (reads from ingestion service)
- Index spans into DuckDB
- Handle state persistence (last processed key)
- Add background task for continuous polling
- Test indexing flow end-to-end
- Validate data matches Go indexing

**Success Criteria:**
- ✅ Background task polls ingestion service
- ✅ Spans indexed into DuckDB correctly
- ✅ State persisted (resumes after restart)
- ✅ Handles errors gracefully
- ✅ Indexed data matches Go implementation

**Dependencies:** Phase 4, Phase 6

---

### Phase 8: LLM Playground with LiteLLM
**File**: `PYTHON_BACKEND_MIGRATION_8_LITELLM_PLAYGROUND.md`

**Objectives:**
- Integrate LiteLLM for unified provider support
- Create playground generate endpoints
- Configure OpenTelemetry in LiteLLM
- Test OpenAI, Anthropic, Gemini providers
- Verify structured output handling
- Validate telemetry capture

**Success Criteria:**
- ✅ POST `/llm/generate` works for all providers
- ✅ Structured outputs work (JSON schema)
- ✅ Schema transformations automatic
- ✅ OTEL spans captured and sent to ingestion
- ✅ Frontend playground works with Python backend

**Dependencies:** Phase 7

---

### Phase 9: Remaining Features Migration
**File**: `PYTHON_BACKEND_MIGRATION_9_REMAINING_FEATURES.md`

**Objectives:**
- Migrate remaining Go endpoints
- Port all CRUD operations
- Ensure API parity with Go
- Update OpenAPI docs
- Final integration testing

**Success Criteria:**
- ✅ All Go endpoints migrated
- ✅ API contracts match exactly
- ✅ Frontend works without changes
- ✅ OpenAPI docs complete

**Dependencies:** Phases 1-8

---

### Phase 10: Deployment & Cutover
**File**: `PYTHON_BACKEND_MIGRATION_10_DEPLOYMENT.md`

**Objectives:**
- Update docker-compose.yml
- Build production Docker images
- Test deployment in staging
- Create cutover plan
- Execute cutover
- Monitor production
- Deprecate Go backend

**Success Criteria:**
- ✅ Python backend deployed to production
- ✅ Zero downtime cutover
- ✅ All features working
- ✅ Performance acceptable
- ✅ Go backend can be safely removed

**Dependencies:** Phase 9

---

## Directory Structure

```
junjo-server/
├── backend/                          # Old Go backend (keep during migration)
│   └── ... (unchanged)
│
├── backend_python/                   # New Python backend
│   ├── app/
│   │   ├── __init__.py
│   │   ├── main.py                   # FastAPI app entry point
│   │   ├── config.py                 # Configuration (env vars)
│   │   ├── database.py               # SQLAlchemy setup
│   │   ├── duckdb_client.py          # DuckDB connection manager
│   │   │
│   │   ├── features/                 # Feature-based organization
│   │   │   ├── __init__.py
│   │   │   │
│   │   │   ├── auth/                 # Authentication feature
│   │   │   │   ├── __init__.py
│   │   │   │   ├── router.py
│   │   │   │   ├── service.py
│   │   │   │   ├── repository.py
│   │   │   │   ├── schemas.py
│   │   │   │   ├── models.py
│   │   │   │   ├── dependencies.py
│   │   │   │   ├── test_router.py
│   │   │   │   ├── test_service.py
│   │   │   │   └── test_repository.py
│   │   │   │
│   │   │   ├── api_keys/             # API keys feature
│   │   │   │   └── ... (same structure)
│   │   │   │
│   │   │   ├── otel/                 # Telemetry/traces feature
│   │   │   │   └── ... (same structure)
│   │   │   │
│   │   │   └── llm/                  # LLM playground feature
│   │   │       ├── providers/
│   │   │       │   ├── __init__.py
│   │   │       │   └── litellm_service.py
│   │   │       └── ... (same structure)
│   │   │
│   │   ├── infrastructure/           # Cross-cutting concerns
│   │   │   ├── __init__.py
│   │   │   ├── grpc_client.py        # gRPC client to ingestion
│   │   │   ├── otel_processor.py     # Span indexing background task
│   │   │   └── middleware.py         # Custom middleware
│   │   │
│   │   └── common/                   # Shared utilities
│   │       ├── __init__.py
│   │       ├── exceptions.py
│   │       ├── pagination.py
│   │       └── responses.py
│   │
│   ├── alembic/                      # Database migrations
│   │   ├── versions/
│   │   ├── env.py
│   │   └── alembic.ini
│   │
│   ├── proto/                        # Protobuf definitions (shared)
│   │   └── ... (generated Python code)
│   │
│   ├── tests/                        # Integration tests
│   │   ├── __init__.py
│   │   ├── conftest.py               # pytest fixtures
│   │   └── integration/
│   │       └── test_api.py
│   │
│   ├── .env.example
│   ├── pyproject.toml                # Python dependencies (Poetry/pip)
│   ├── requirements.txt              # Or use Poetry
│   ├── Dockerfile
│   └── README.md
│
├── ingestion-service/                # Go ingestion (UNCHANGED)
│   └── ... (keep as-is)
│
├── frontend/                         # React frontend (UNCHANGED)
│   └── ... (keep as-is)
│
├── docker-compose.yml                # Update for Python backend
├── docker-compose.dev.yml            # Development with both backends
└── PYTHON_BACKEND_MIGRATION_*.md     # Migration documentation
```

---

## Testing Strategy

### Test Types

**Unit Tests** (co-located with code)
```python
# app/features/api_keys/test_service.py
# Tests service logic in isolation with mocked repositories
```

**Integration Tests** (in `tests/integration/`)
```python
# tests/integration/test_api_keys.py
# Tests full API flow with real database (test DB)
```

**End-to-End Tests** (manual during cutover)
- Frontend → Python backend → Ingestion service
- Complete user flows
- Performance validation

### Test Database

```python
# tests/conftest.py
import pytest
from sqlalchemy.ext.asyncio import create_async_engine, AsyncSession

@pytest.fixture
async def test_db():
    """Create test database for each test"""
    engine = create_async_engine("sqlite+aiosqlite:///:memory:")
    async with engine.begin() as conn:
        await conn.run_sync(Base.metadata.create_all)

    async with AsyncSession(engine) as session:
        yield session

    await engine.dispose()
```

### Running Tests

```bash
# Run all tests
pytest

# Run with coverage
pytest --cov=app --cov-report=html

# Run specific test file
pytest app/features/api_keys/test_service.py

# Run integration tests only
pytest tests/integration/
```

---

## Deployment Strategy

### Development Environment

```yaml
# docker-compose.dev.yml
services:
  # Old Go backend (keep running during migration)
  junjo-server-backend:
    build: ./backend
    ports:
      - "1323:1323"  # Original port

  # New Python backend (adjacent)
  junjo-server-backend-python:
    build: ./backend_python
    ports:
      - "1324:1324"  # New port for testing
    environment:
      - DATABASE_URL=sqlite:///./dbdata/junjo.db
      - INGESTION_SERVICE_URL=junjo-server-ingestion:50052

  # Ingestion service (unchanged)
  junjo-server-ingestion:
    build: ./ingestion-service
    ports:
      - "50051:50051"
      - "50052:50052"

  # Frontend (unchanged - can test against either backend)
  junjo-server-frontend:
    build: ./frontend
    ports:
      - "5151:80"
    environment:
      - VITE_API_URL=http://localhost:1324  # Point to Python for testing
```

### Production Cutover Strategy

**Phase 1: Parallel Deployment**
- Deploy Python backend on new port
- Keep Go backend running
- Use feature flags to test Python endpoints

**Phase 2: Gradual Traffic Shift**
- Route 10% traffic to Python
- Monitor errors, performance
- Increase to 50%, then 100%

**Phase 3: Deprecation**
- Once stable (1-2 weeks), deprecate Go
- Remove Go backend from deployment
- Update documentation

### Docker Production Image

```dockerfile
# backend_python/Dockerfile
FROM python:3.12-slim as builder

WORKDIR /app
COPY requirements.txt .
RUN pip install --no-cache-dir --user -r requirements.txt

FROM python:3.12-slim

WORKDIR /app
COPY --from=builder /root/.local /root/.local
COPY ./app ./app

ENV PATH=/root/.local/bin:$PATH
EXPOSE 1323

CMD ["uvicorn", "app.main:app", "--host", "0.0.0.0", "--port", "1323"]
```

---

## Rollback Plan

### Immediate Rollback (< 1 hour)

If critical issues arise during cutover:

1. **Update load balancer/reverse proxy** to route back to Go backend
2. **Stop Python backend container**
3. **Verify Go backend is healthy**
4. **Investigate Python issues in staging**

### Data Consistency

- Both backends use **same SQLite database** (read-write safe)
- Both backends connect to **same ingestion service**
- No data migration needed for rollback
- Session cookies work with both backends (same secret)

### Rollback Decision Criteria

Rollback if:
- ❌ Error rate > 1%
- ❌ P95 latency > 2x baseline
- ❌ Critical feature broken
- ❌ Data corruption detected

---

## Success Metrics

### Performance Benchmarks

| Metric | Go Baseline | Python Target |
|--------|-------------|---------------|
| /ping latency | 1-2ms | < 5ms |
| Trace query (P95) | 50ms | < 100ms |
| LLM generate (proxy) | ~2s | < 2.5s |
| Memory usage (idle) | 20MB | < 150MB |
| Container image size | 50MB | < 300MB |

### Migration Completion Criteria

- ✅ All Go endpoints migrated
- ✅ API contracts 100% compatible
- ✅ Frontend requires zero changes
- ✅ Performance within targets
- ✅ Test coverage > 80%
- ✅ Zero critical bugs in production (2 weeks)
- ✅ Documentation complete
- ✅ Team trained on Python codebase

---

## Next Steps

1. **Review this master document** with the team
2. **Adjust conventions** based on your reference repo (`wt_api_v2`)
3. **Begin Phase 1**: Create base FastAPI application
4. **Iterate through phases** one at a time
5. **Test each phase** before moving to next
6. **Deploy to staging** after Phase 9
7. **Cutover to production** in Phase 10

---

## Questions to Resolve

Before starting Phase 1, clarify:

1. **Poetry vs pip** for dependency management?
2. **python-jose vs authlib** for JWT/sessions?
3. **pytest-asyncio** test patterns from `wt_api_v2`?
4. **Logging configuration** - structlog? loguru? standard logging?
5. **Environment variable management** - python-decouple? pydantic-settings?
6. **API versioning strategy** - /v1/ prefix?

---

## References

- [FastAPI Documentation](https://fastapi.tiangolo.com/)
- [SQLAlchemy 2.0 Documentation](https://docs.sqlalchemy.org/en/20/)
- [LiteLLM Documentation](https://docs.litellm.ai/)
- [Alembic Documentation](https://alembic.sqlalchemy.org/)
- AGENTS.md (this repository)
- `wt_api_v2` reference repository (for patterns)

---

**Ready to proceed with Phase 1!**
