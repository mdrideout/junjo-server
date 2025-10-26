# Python Backend Testing Strategy

> **Status**: Reference document for all migration phases
> **Python Version**: 3.14
> **Testing Framework**: pytest + pytest-asyncio
> **Database**: SQLite with WAL mode

---

## Overview

This document defines the comprehensive testing strategy for the Python backend migration. **All phases must include tests before completion** to protect against regressions and integration failures from library changes.

### Testing Philosophy

1. **Test First, Not After** - Write tests alongside implementation, not as an afterthought
2. **Protect Against Regressions** - Tests must catch breaking changes from dependency updates
3. **Test Each Layer** - Unit tests for repositories, integration tests for full flows
4. **Migration Safety** - Every migration must be tested (upgrade and downgrade)
5. **Concurrency Safety** - Test high-concurrency scenarios explicitly
6. **Real Database** - Use real SQLite for integration tests (not mocks)
7. **SQLite-Specific** - Configure test databases with proper PRAGMA settings

---

## Table of Contents

1. [Test Structure](#test-structure)
2. [Testing Tools](#testing-tools)
3. [Test Types](#test-types)
4. [SQLite Test Configuration](#sqlite-test-configuration)
5. [Layer-Specific Patterns](#layer-specific-patterns)
6. [Migration Testing](#migration-testing)
7. [Concurrency Testing](#concurrency-testing)
8. [Test Coverage Requirements](#test-coverage-requirements)
9. [Phase Completion Criteria](#phase-completion-criteria)

---

## Test Structure

### Directory Organization

```
backend_python/
├── app/
│   ├── database/
│   │   └── users/
│   │       ├── models.py
│   │       ├── schemas.py
│   │       ├── repository.py
│   │       └── test_repository.py        # Co-located unit tests
│   │
│   └── features/
│       └── auth/
│           ├── router.py
│           ├── service.py
│           ├── test_service.py           # Co-located unit tests
│           └── test_router.py            # Co-located integration tests
│
└── tests/
    ├── conftest.py                       # Global fixtures
    ├── integration/                      # Integration tests
    │   ├── test_auth_flow.py
    │   ├── test_api_keys_flow.py
    │   └── test_llm_playground_flow.py
    ├── migrations/                       # Migration tests
    │   ├── test_migrations.py
    │   └── test_schema_consistency.py
    └── concurrency/                      # Concurrency tests
        └── test_concurrent_requests.py
```

### Naming Conventions

| Test Type | Location | Naming Pattern | Example |
|-----------|----------|----------------|---------|
| **Unit Tests** | Co-located with code | `test_{module}.py` | `test_repository.py` |
| **Integration Tests** | `tests/integration/` | `test_{feature}_flow.py` | `test_auth_flow.py` |
| **Migration Tests** | `tests/migrations/` | `test_migrations.py` | `test_migrations.py` |
| **Concurrency Tests** | `tests/concurrency/` | `test_concurrent_{feature}.py` | `test_concurrent_requests.py` |

---

## Testing Tools

### Core Dependencies

```toml
[project.optional-dependencies]
dev = [
    # Testing Framework
    "pytest>=8.3.0",
    "pytest-asyncio>=0.24.0",
    "pytest-cov>=6.0.0",              # Coverage reporting
    "pytest-xdist>=3.6.0",            # Parallel test execution

    # HTTP Testing
    "httpx>=0.28.0",                  # Async HTTP client for FastAPI testing

    # Database Testing
    "pytest-alembic>=0.11.0",         # Migration testing

    # Mocking
    "pytest-mock>=3.14.0",

    # Other Tools
    "faker>=33.0.0",                  # Generate test data
    "freezegun>=1.5.0",               # Mock datetime
]
```

### pytest Configuration

Create `backend_python/pytest.ini`:

```ini
[pytest]
# Asyncio mode
asyncio_mode = auto
asyncio_default_fixture_loop_scope = function

# Test paths
testpaths = tests
python_files = test_*.py
python_classes = Test*
python_functions = test_*

# Output options
addopts =
    -v
    --strict-markers
    --tb=short
    --cov=app
    --cov-report=term-missing
    --cov-report=html
    --cov-fail-under=80

# Markers
markers =
    unit: Unit tests (fast, isolated)
    integration: Integration tests (slower, uses real DB)
    migration: Migration tests
    concurrency: Concurrency/race condition tests
    slow: Slow tests (run separately in CI)

# Ignore paths
norecursedirs = .git .venv venv migrations
```

---

## Test Types

### 1. Unit Tests

**Purpose**: Test individual components in isolation

**Characteristics**:
- ✅ Fast (< 100ms per test)
- ✅ Isolated (no external dependencies)
- ✅ Mock external services
- ✅ Co-located with code

**Example - Repository Unit Test:**

```python
# app/database/users/test_repository.py
"""
Unit tests for UserRepository.

Tests database operations in isolation.
"""

import pytest
from datetime import datetime

from app.database.users.repository import UserRepository
from app.database.users.schemas import UserRead


@pytest.mark.unit
@pytest.mark.asyncio
async def test_create_user(test_db):
    """Test creating a user"""
    # Arrange
    import app.database.db_config as db_config
    db_config.async_session = test_db

    # Act
    user = await UserRepository.create(
        email="test@example.com",
        password_hash="hashed_password_123"
    )

    # Assert
    assert user.email == "test@example.com"
    assert user.is_active is True
    assert user.id is not None
    assert isinstance(user.created_at, datetime)


@pytest.mark.unit
@pytest.mark.asyncio
async def test_get_user_by_email_not_found(test_db):
    """Test getting user that doesn't exist"""
    # Arrange
    import app.database.db_config as db_config
    db_config.async_session = test_db

    # Act
    user = await UserRepository.get_by_email("nonexistent@example.com")

    # Assert
    assert user is None


@pytest.mark.unit
@pytest.mark.asyncio
async def test_get_user_by_email_found(test_db):
    """Test getting existing user"""
    # Arrange
    import app.database.db_config as db_config
    db_config.async_session = test_db

    await UserRepository.create(
        email="test@example.com",
        password_hash="hashed"
    )

    # Act
    user = await UserRepository.get_by_email("test@example.com")

    # Assert
    assert user is not None
    assert user.email == "test@example.com"
    assert user.password_hash == "hashed"
```

### 2. Integration Tests

**Purpose**: Test multiple components working together

**Characteristics**:
- ✅ Test real flows (e.g., user registration → login → API call)
- ✅ Use real database (not mocks)
- ✅ Test FastAPI endpoints with httpx
- ✅ Slower than unit tests (acceptable)

**Example - Auth Integration Test:**

```python
# tests/integration/test_auth_flow.py
"""
Integration tests for authentication flow.

Tests the complete auth flow: register → login → authenticated request.
"""

import pytest
from httpx import AsyncClient, ASGITransport

from app.main import app


@pytest.mark.integration
@pytest.mark.asyncio
async def test_user_registration_and_login_flow(test_db):
    """Test complete user registration and login flow"""
    # Override database session
    import app.database.db_config as db_config
    db_config.async_session = test_db

    transport = ASGITransport(app=app)
    async with AsyncClient(transport=transport, base_url="http://test") as client:
        # 1. Register new user
        register_response = await client.post(
            "/api/v1/auth/register",
            json={
                "email": "newuser@example.com",
                "password": "SecurePassword123!"
            }
        )
        assert register_response.status_code == 201
        user_data = register_response.json()
        assert user_data["email"] == "newuser@example.com"

        # 2. Login with credentials
        login_response = await client.post(
            "/api/v1/auth/login",
            json={
                "email": "newuser@example.com",
                "password": "SecurePassword123!"
            }
        )
        assert login_response.status_code == 200

        # Session cookie should be set
        cookies = login_response.cookies
        assert "session_id" in cookies

        # 3. Access protected endpoint with session
        me_response = await client.get(
            "/api/v1/auth/me",
            cookies=cookies
        )
        assert me_response.status_code == 200
        me_data = me_response.json()
        assert me_data["email"] == "newuser@example.com"

        # 4. Logout
        logout_response = await client.post(
            "/api/v1/auth/logout",
            cookies=cookies
        )
        assert logout_response.status_code == 200

        # 5. Verify session invalidated
        me_after_logout = await client.get(
            "/api/v1/auth/me",
            cookies=cookies
        )
        assert me_after_logout.status_code == 401
```

### 3. Migration Tests

**Purpose**: Ensure database migrations work correctly

**Example:**

```python
# tests/migrations/test_migrations.py
"""
Migration tests using pytest-alembic.

Ensures migrations can upgrade and downgrade safely.
"""

import pytest
from pytest_alembic import Config as AlembicConfig


@pytest.mark.migration
def test_single_head_revision():
    """Test that there's only one head revision"""
    alembic_config = AlembicConfig()
    alembic_config.assert_single_head_revision()


@pytest.mark.migration
def test_upgrade_and_downgrade():
    """Test that migrations can upgrade and downgrade"""
    alembic_config = AlembicConfig()

    # Test upgrade to head
    alembic_config.upgrade("head")

    # Test downgrade one step
    alembic_config.downgrade("-1")

    # Test upgrade again
    alembic_config.upgrade("head")


@pytest.mark.migration
def test_model_definitions_match_ddl():
    """Test that SQLAlchemy models match database schema"""
    alembic_config = AlembicConfig()

    # Upgrade to head
    alembic_config.upgrade("head")

    # Compare models to database
    alembic_config.assert_model_definitions_match_ddl()


@pytest.mark.migration
def test_up_down_consistency():
    """Test that upgrade followed by downgrade returns to original state"""
    alembic_config = AlembicConfig()

    # This tests that migrations are reversible
    alembic_config.test_up_down_consistency()
```

---

## SQLite Test Configuration

### Critical: Proper PRAGMA Settings for Tests

**IMPORTANT**: Test databases must use the same PRAGMA settings as production to catch real-world issues.

### Global Test Fixtures (`tests/conftest.py`)

```python
"""
Global test fixtures with SQLite-specific configuration.

CRITICAL: Test database must match production PRAGMA settings:
- foreign_keys=ON
- journal_mode=WAL
- synchronous=NORMAL
"""

import pytest
import sqlite3
from sqlalchemy import event
from sqlalchemy.ext.asyncio import create_async_engine, async_sessionmaker
from faker import Faker

from app.database.base import Base


fake = Faker()


def set_sqlite_pragma(dbapi_conn, connection_record):
    """
    Set SQLite PRAGMA settings for test database.

    MUST match production settings to catch real issues.
    """
    cursor = dbapi_conn.cursor()
    cursor.execute("PRAGMA foreign_keys=ON")
    cursor.execute("PRAGMA journal_mode=WAL")
    cursor.execute("PRAGMA synchronous=NORMAL")
    cursor.close()


@pytest.fixture(scope="session")
def faker():
    """Faker instance for generating test data"""
    return Faker()


@pytest.fixture
async def test_db_engine():
    """
    Create test database engine (in-memory SQLite).

    CRITICAL: Uses same PRAGMA settings as production.
    """
    engine = create_async_engine(
        "sqlite+aiosqlite:///:memory:",
        echo=False,  # Set to True to debug SQL queries
        connect_args={"check_same_thread": False},  # Required for async
    )

    # Set PRAGMA settings (same as production)
    event.listen(engine.sync_engine, "connect", set_sqlite_pragma)

    # Create all tables
    async with engine.begin() as conn:
        await conn.run_sync(Base.metadata.create_all)

    yield engine

    # Cleanup
    await engine.dispose()


@pytest.fixture
async def test_db(test_db_engine):
    """
    Create test database session factory.

    expire_on_commit=False matches production setting.
    """
    async_session_test = async_sessionmaker(
        test_db_engine,
        expire_on_commit=False  # MUST match production
    )
    return async_session_test


@pytest.fixture
async def sample_user(test_db):
    """Create a sample user for testing"""
    from app.database.users.repository import UserRepository
    import app.database.db_config as db_config

    db_config.async_session = test_db

    user = await UserRepository.create(
        email=fake.email(),
        password_hash="hashed_password"
    )
    return user


@pytest.fixture
async def authenticated_client(test_db, sample_user):
    """HTTP client with authenticated session"""
    from httpx import AsyncClient, ASGITransport
    from app.main import app
    import app.database.db_config as db_config

    db_config.async_session = test_db

    transport = ASGITransport(app=app)
    async with AsyncClient(transport=transport, base_url="http://test") as client:
        # Login to get session cookie
        response = await client.post(
            "/api/v1/auth/login",
            json={
                "email": sample_user.email,
                "password": "test_password"
            }
        )
        # Client now has session cookie
        yield client
```

### Testing Foreign Key Constraints

```python
# tests/test_sqlite_config.py
"""
Tests to ensure SQLite is properly configured.

These tests verify that PRAGMA settings are active.
"""

import pytest
from sqlalchemy import text


@pytest.mark.unit
@pytest.mark.asyncio
async def test_foreign_keys_enabled(test_db_engine):
    """Verify foreign key constraints are enabled"""
    async with test_db_engine.connect() as conn:
        result = await conn.execute(text("PRAGMA foreign_keys"))
        fk_enabled = result.scalar()
        assert fk_enabled == 1, "Foreign keys must be enabled in tests"


@pytest.mark.unit
@pytest.mark.asyncio
async def test_wal_mode_enabled(test_db_engine):
    """Verify WAL journal mode is enabled"""
    async with test_db_engine.connect() as conn:
        result = await conn.execute(text("PRAGMA journal_mode"))
        journal_mode = result.scalar()
        assert journal_mode.upper() == "WAL", "WAL mode must be enabled in tests"


@pytest.mark.unit
@pytest.mark.asyncio
async def test_foreign_key_constraint_enforced(test_db):
    """Test that foreign key constraints are actually enforced"""
    from app.database.sessions.repository import SessionRepository
    import app.database.db_config as db_config

    db_config.async_session = test_db

    # Try to create session with non-existent user_id
    from sqlalchemy.exc import IntegrityError
    from datetime import datetime, timedelta

    with pytest.raises(IntegrityError):
        await SessionRepository.create(
            user_id="nonexistent-user-id",
            expires_at=datetime.utcnow() + timedelta(days=1)
        )
```

---

## Layer-Specific Patterns

### Repository Layer Tests

**What to Test:**
- ✅ CRUD operations (Create, Read, Update, Delete)
- ✅ Query filters and conditions
- ✅ Foreign key relationships
- ✅ Unique constraints
- ✅ NULL handling
- ✅ Error cases (not found, duplicate, etc.)

**Template:**

```python
@pytest.mark.unit
@pytest.mark.asyncio
async def test_repository_operation(test_db):
    """Test description"""
    # Arrange
    import app.database.db_config as db_config
    db_config.async_session = test_db

    # Act
    result = await Repository.method(...)

    # Assert
    assert result is not None
    # Additional assertions
```

### Service Layer Tests

**What to Test:**
- ✅ Business logic
- ✅ Validation rules
- ✅ Error handling
- ✅ Multiple repository coordination
- ✅ Edge cases

**Template:**

```python
@pytest.mark.unit
@pytest.mark.asyncio
async def test_service_business_logic(mocker):
    """Test business logic in service"""
    # Arrange - Mock repository
    mock_repo = mocker.AsyncMock()
    mock_repo.get_by_email.return_value = None

    # Act
    result = await Service.method(...)

    # Assert
    mock_repo.get_by_email.assert_called_once_with("test@example.com")
    # Additional assertions
```

### Router (API) Layer Tests

**What to Test:**
- ✅ HTTP status codes
- ✅ Request validation (Pydantic)
- ✅ Response serialization
- ✅ Authentication/authorization
- ✅ Error responses
- ✅ CORS headers

**Template:**

```python
@pytest.mark.integration
@pytest.mark.asyncio
async def test_api_endpoint(test_db):
    """Test API endpoint"""
    # Arrange
    import app.database.db_config as db_config
    db_config.async_session = test_db

    transport = ASGITransport(app=app)
    async with AsyncClient(transport=transport, base_url="http://test") as client:
        # Act
        response = await client.post("/api/v1/endpoint", json={...})

        # Assert
        assert response.status_code == 200
        data = response.json()
        assert data["field"] == "value"
```

---

## Concurrency Testing

**Purpose**: Ensure high-concurrency pattern works correctly

### Test Concurrent Database Operations

```python
# tests/concurrency/test_concurrent_requests.py
"""
Concurrency tests for database operations.

Ensures the high-concurrency pattern (fresh session per operation)
prevents race conditions and data corruption.
"""

import pytest
import asyncio
from faker import Faker

from app.database.users.repository import UserRepository


fake = Faker()


@pytest.mark.concurrency
@pytest.mark.asyncio
async def test_concurrent_user_creation(test_db):
    """Test creating multiple users concurrently"""
    # Arrange
    import app.database.db_config as db_config
    db_config.async_session = test_db

    num_users = 50
    emails = [fake.email() for _ in range(num_users)]

    # Act - Create users concurrently
    tasks = [
        UserRepository.create(email=email, password_hash="hashed")
        for email in emails
    ]
    users = await asyncio.gather(*tasks)

    # Assert - All users created successfully
    assert len(users) == num_users
    assert len(set(user.id for user in users)) == num_users  # All unique IDs

    # Verify all users in database
    for email in emails:
        user = await UserRepository.get_by_email(email)
        assert user is not None


@pytest.mark.concurrency
@pytest.mark.asyncio
async def test_concurrent_reads_no_blocking(test_db):
    """Test that concurrent reads don't block each other"""
    # Arrange
    import app.database.db_config as db_config
    db_config.async_session = test_db

    # Create test user
    user = await UserRepository.create(
        email="test@example.com",
        password_hash="hashed"
    )

    # Act - Read same user concurrently 100 times
    tasks = [
        UserRepository.get_by_id(user.id)
        for _ in range(100)
    ]

    import time
    start = time.time()
    results = await asyncio.gather(*tasks)
    elapsed = time.time() - start

    # Assert - All reads successful and fast
    assert all(r is not None for r in results)
    assert elapsed < 1.0  # Should be fast (no blocking)
```

---

## Test Coverage Requirements

### Minimum Coverage Targets

| Layer | Coverage Target | Rationale |
|-------|----------------|-----------|
| **Repository** | 90%+ | Critical for data integrity |
| **Service** | 85%+ | Core business logic |
| **Router** | 80%+ | API contract validation |
| **Models** | N/A | Declarative, no logic |
| **Schemas** | N/A | Pydantic validates itself |
| **Overall** | 80%+ | Baseline for production |

### Coverage Report

```bash
# Generate coverage report
pytest --cov=app --cov-report=html --cov-report=term-missing

# View HTML report
open htmlcov/index.html

# Fail if coverage below threshold
pytest --cov=app --cov-fail-under=80
```

---

## Phase Completion Criteria

**For each migration phase to be considered complete:**

1. ✅ **Unit Tests Written** - All repositories, services tested
2. ✅ **Integration Tests Written** - API flows tested end-to-end
3. ✅ **Migration Tests Pass** - Upgrade/downgrade tested
4. ✅ **Coverage ≥ 80%** - Verified with `pytest --cov`
5. ✅ **Concurrency Tests Pass** - High-concurrency scenarios tested (if applicable)
6. ✅ **SQLite Config Verified** - PRAGMA settings tested
7. ✅ **All Tests Pass** - `pytest` exits with 0
8. ✅ **No Regressions** - Existing tests still pass

---

## Running Tests

### Local Development

```bash
cd backend_python

# Run all tests
pytest

# Run specific test type
pytest -m unit           # Unit tests only
pytest -m integration    # Integration tests only
pytest -m migration      # Migration tests only

# Run with coverage
pytest --cov=app --cov-report=html

# Run in parallel (fast)
pytest -n auto

# Run specific file
pytest tests/integration/test_auth_flow.py

# Run specific test
pytest tests/integration/test_auth_flow.py::test_user_registration_and_login_flow
```

### Before Committing

```bash
# Full test suite with coverage check
pytest --cov=app --cov-fail-under=80 --cov-report=term-missing

# Linting
ruff check app/

# Type checking
mypy app/
```

---

## Best Practices

1. **AAA Pattern** - Arrange, Act, Assert structure
2. **One Assertion Per Test** - Keep tests focused
3. **Descriptive Names** - Test name should describe what's being tested
4. **Fast Tests** - Unit tests should run in milliseconds
5. **Isolated Tests** - Each test should be independent
6. **Clean Test Data** - Use fixtures, clean up after tests
7. **Test Edge Cases** - Not just happy path
8. **Test Errors** - Verify error handling works
9. **Avoid Test Logic** - Tests should be simple, no complex logic
10. **DRY (Don't Repeat Yourself)** - Use fixtures for common setup
11. **Match Production Config** - Test database PRAGMA settings must match production

---

## References

- [pytest Documentation](https://docs.pytest.org/)
- [pytest-asyncio](https://pytest-asyncio.readthedocs.io/)
- [pytest-alembic](https://pytest-alembic.readthedocs.io/)
- [FastAPI Testing](https://fastapi.tiangolo.com/tutorial/testing/)
- [SQLite PRAGMA Documentation](https://www.sqlite.org/pragma.html)
- [wt_api_v2 Test Patterns](file:///Users/matt/repos/wt_api_v2/tests/)
