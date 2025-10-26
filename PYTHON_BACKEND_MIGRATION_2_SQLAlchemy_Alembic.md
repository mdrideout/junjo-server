# Phase 2: SQLAlchemy + Alembic Setup

> **Status**: Ready to implement
> **Estimated Time**: 3-4 hours
> **Dependencies**: Phase 1 (Base FastAPI Setup)
> **Python Version**: 3.14
> **SQLAlchemy Version**: 2.0+

---

## Overview

This phase establishes database infrastructure with:
- **SQLAlchemy 2.0** (async ORM with modern syntax)
- **Alembic** (database migrations)
- **High-concurrency pattern** (validated from wt_api_v2)
- **SQLite** for application data (users, sessions, API keys)
- Initial schema for authentication (users, sessions tables)
- Database session management with `expire_on_commit=False`
- Testing database setup

**Pattern Reference**: See `PYTHON_BACKEND_HIGH_CONCURRENCY_DB_PATTERN.md`

---

## Table of Contents

1. [Directory Structure](#directory-structure)
2. [Dependencies](#dependencies)
3. [File-by-File Implementation](#file-by-file-implementation)
4. [Initial Schema Design](#initial-schema-design)
5. [Alembic Setup](#alembic-setup)
6. [Testing](#testing)
7. [Success Criteria](#success-criteria)
8. [Troubleshooting](#troubleshooting)

---

## Directory Structure

```
backend_python/
├── app/
│   ├── database/
│   │   ├── __init__.py              # Imports all models (for Alembic)
│   │   ├── base.py                  # SQLAlchemy Base
│   │   ├── db_config.py             # Engine & session factory
│   │   │
│   │   ├── users/                   # Users feature
│   │   │   ├── __init__.py
│   │   │   ├── models.py            # UserTable
│   │   │   ├── schemas.py           # UserRead, UserCreate
│   │   │   └── repository.py        # UserRepository
│   │   │
│   │   └── sessions/                # Sessions feature (for cookie auth)
│   │       ├── __init__.py
│   │       ├── models.py            # SessionTable
│   │       ├── schemas.py           # SessionRead, SessionCreate
│   │       └── repository.py        # SessionRepository
│   │
│   └── common/
│       └── utils.py                 # generate_id() utility
│
├── migrations/                      # Alembic migrations
│   ├── versions/
│   │   └── 2025_XX_XX_XXXX-xxxxxxx_initial_schema.py
│   ├── env.py
│   ├── script.py.mako
│   └── README
│
├── alembic.ini
└── tests/
    └── test_database.py
```

---

## Dependencies

Update `backend_python/pyproject.toml`:

```toml
[project]
dependencies = [
    # ... existing dependencies

    # Database (SQLAlchemy 2.0 + async)
    "sqlalchemy>=2.0.36",
    "alembic>=1.14.0",
    "aiosqlite>=0.20.0",          # Async SQLite driver

    # OpenTelemetry for SQLAlchemy
    "opentelemetry-instrumentation-sqlalchemy>=0.49b0",

    # Utilities
    "nanoid>=2.0.0",              # Short IDs (like wt_api_v2)
]
```

Install:

```bash
cd backend_python
uv sync
```

---

## File-by-File Implementation

### 1. `app/common/utils.py` - ID Generation

```python
"""
Common utilities.

Pattern from wt_api_v2 (using nanoid for short, unique IDs).
"""

from nanoid import generate as nanoid_generate


def generate_id(size: int = 22) -> str:
    """
    Generate a unique ID using nanoid.

    Args:
        size: Length of the ID (default 22 characters)

    Returns:
        A URL-safe, unique identifier

    Example:
        >>> generate_id(22)
        'V1StGXR8_Z5jdHi6B-myT'
    """
    return nanoid_generate(size=size)
```

### 2. `app/database/base.py` - SQLAlchemy Base

```python
"""
SQLAlchemy declarative base.

All models inherit from this base.
Pattern from wt_api_v2.
"""

from sqlalchemy.orm import DeclarativeBase


class Base(DeclarativeBase):
    """Base class for all SQLAlchemy models"""
    pass
```

### 3. `app/database/db_config.py` - Engine & Session Factory

**CRITICAL**: This file implements the high-concurrency pattern with SQLite-specific configurations.

```python
"""
Database configuration for SQLAlchemy with SQLite.

High-concurrency async pattern validated from wt_api_v2:
- async_sessionmaker with expire_on_commit=False
- Each repository method creates its own session
- SQLite-specific PRAGMA settings for performance and safety

SQLite Idiosyncrasies:
- check_same_thread: False (required for async)
- PRAGMA foreign_keys=ON (enable FK constraints)
- PRAGMA journal_mode=WAL (Write-Ahead Logging for concurrency)
- PRAGMA synchronous=NORMAL (balance safety/performance)
- WAL checkpoint on shutdown

See: PYTHON_BACKEND_HIGH_CONCURRENCY_DB_PATTERN.md
"""

from sqlalchemy import event, text
from sqlalchemy.ext.asyncio import create_async_engine, async_sessionmaker
from loguru import logger

from app.config.settings import settings


# Create async engine with SQLite-specific settings
engine = create_async_engine(
    settings.database.sqlite_url,
    echo=settings.debug,  # Log SQL queries in debug mode
    connect_args={"check_same_thread": False},  # Required for async SQLite
)


# Set PRAGMA settings for every new SQLite connection
# These are critical for SQLite performance and safety
@event.listens_for(engine.sync_engine, "connect")
def set_sqlite_pragmas(dbapi_connection, connection_record):
    """
    Configure SQLite PRAGMA settings on connection.

    - foreign_keys=ON: Enable foreign key constraints (off by default in SQLite)
    - journal_mode=WAL: Write-Ahead Logging for better concurrency
    - synchronous=NORMAL: Balance between safety and performance
    """
    cursor = dbapi_connection.cursor()
    cursor.execute("PRAGMA foreign_keys=ON")
    cursor.execute("PRAGMA journal_mode=WAL")
    cursor.execute("PRAGMA synchronous=NORMAL")
    cursor.close()


# Create async session factory
# CRITICAL: expire_on_commit=False prevents lazy loading errors in async contexts
async_session = async_sessionmaker(
    engine,
    expire_on_commit=False  # ⚠️ REQUIRED for asyncio safety
)

logger.info(f"Database engine created: {settings.database.sqlite_path}")


# FastAPI dependency (optional - most code uses static repository pattern)
async def get_db():
    """
    FastAPI dependency to get database session.

    Usage:
        @router.get("/")
        async def endpoint(db: AsyncSession = Depends(get_db)):
            ...

    Note: In Junjo, we primarily use the static repository pattern
    (repositories create their own sessions), but this dependency
    is available if needed.
    """
    async with async_session() as session:
        yield session


async def checkpoint_wal():
    """
    Forces a WAL checkpoint to ensure all transactions are written to the main database file.

    Should be called on application shutdown to ensure data persistence.
    WAL (Write-Ahead Logging) mode keeps changes in separate files until checkpoint.
    """
    async with engine.connect() as conn:
        logger.info("Checkpointing WAL")
        await conn.execute(text("PRAGMA wal_checkpoint(TRUNCATE);"))
        logger.info("Checkpoint complete")
```

### 4. `app/config/settings.py` - Add Database URL

Update the `DatabaseSettings` class:

```python
from typing import Annotated
from pydantic import Field, computed_field
from pydantic_settings import BaseSettings, SettingsConfigDict


class DatabaseSettings(BaseSettings):
    """Database configuration"""

    sqlite_path: Annotated[
        str,
        Field(
            default="./dbdata/junjo.db",
            description="Path to SQLite database file"
        )
    ]
    duckdb_path: Annotated[
        str,
        Field(
            default="./dbdata/traces.duckdb",
            description="Path to DuckDB database file"
        )
    ]

    @computed_field  # type: ignore[prop-decorator]
    @property
    def sqlite_url(self) -> str:
        """Computed SQLite async URL"""
        return f"sqlite+aiosqlite:///{self.sqlite_path}"

    model_config = SettingsConfigDict(
        env_prefix="DB_",
        env_file=".env",
        env_file_encoding="utf-8",
        extra="ignore",
    )
```

### 5. `app/database/users/models.py` - User Model

```python
"""
User database model.

Uses modern SQLAlchemy 2.0 syntax with Mapped[] type hints.
"""

from datetime import datetime
from sqlalchemy import String, DateTime, Boolean
from sqlalchemy.orm import Mapped, mapped_column
from sqlalchemy.sql import func

from app.database.base import Base
from app.common.utils import generate_id


class UserTable(Base):
    """
    User model for authentication.

    Stores user account information.
    """
    __tablename__ = "users"

    # Primary key
    id: Mapped[str] = mapped_column(
        String(22),
        primary_key=True,
        default=lambda: generate_id(size=22)
    )

    # Required fields
    email: Mapped[str] = mapped_column(
        String,
        nullable=False,
        unique=True,
        index=True
    )

    # Password hash (bcrypt)
    password_hash: Mapped[str] = mapped_column(
        String,
        nullable=False
    )

    # Account status
    is_active: Mapped[bool] = mapped_column(
        Boolean,
        nullable=False,
        default=True
    )

    # Timestamps
    created_at: Mapped[datetime] = mapped_column(
        DateTime,
        server_default=func.now(),
        nullable=False
    )

    updated_at: Mapped[datetime] = mapped_column(
        DateTime,
        server_default=func.now(),
        onupdate=func.now(),
        nullable=False
    )
```

### 6. `app/database/users/schemas.py` - User Schemas

```python
"""
User Pydantic schemas for validation and serialization.
"""

from datetime import datetime
from pydantic import BaseModel, EmailStr, Field, ConfigDict


class UserCreate(BaseModel):
    """Schema for creating a user"""
    email: EmailStr
    password: str = Field(..., min_length=8)


class UserRead(BaseModel):
    """Schema for reading a user (no password hash)"""
    id: str
    email: EmailStr
    is_active: bool
    created_at: datetime
    updated_at: datetime

    model_config = ConfigDict(from_attributes=True)


class UserInDB(UserRead):
    """Schema for user with password hash (internal use only)"""
    password_hash: str
```

### 7. `app/database/users/repository.py` - User Repository

```python
"""
User repository using high-concurrency pattern.

Each method creates its own session for complete isolation.
See: PYTHON_BACKEND_HIGH_CONCURRENCY_DB_PATTERN.md
"""

from sqlalchemy import select
from sqlalchemy.exc import SQLAlchemyError

from app.database.db_config import async_session
from app.database.users.models import UserTable
from app.database.users.schemas import UserRead, UserInDB


class UserRepository:
    """Repository for user database operations"""

    @staticmethod
    async def create(email: str, password_hash: str) -> UserRead:
        """
        Create a new user.

        Pattern:
        1. Create session
        2. Add object
        3. Commit
        4. Refresh (load generated fields)
        5. Validate to Pydantic BEFORE session closes
        """
        try:
            db_obj = UserTable(
                email=email,
                password_hash=password_hash
            )

            async with async_session() as session:
                session.add(db_obj)
                await session.commit()
                await session.refresh(db_obj)

                # Validate to Pydantic before session closes
                return UserRead.model_validate(db_obj)

        except SQLAlchemyError as e:
            raise e

    @staticmethod
    async def get_by_email(email: str) -> UserInDB | None:
        """
        Get user by email (including password hash for authentication).

        Returns UserInDB (with password_hash) for internal use.
        """
        try:
            async with async_session() as session:
                stmt = select(UserTable).where(UserTable.email == email)
                result = await session.execute(stmt)
                db_obj = result.scalar_one_or_none()

                if db_obj:
                    return UserInDB.model_validate(db_obj)
                return None

        except SQLAlchemyError as e:
            raise e

    @staticmethod
    async def get_by_id(user_id: str) -> UserRead | None:
        """Get user by ID (no password hash)"""
        try:
            async with async_session() as session:
                stmt = select(UserTable).where(UserTable.id == user_id)
                result = await session.execute(stmt)
                db_obj = result.scalar_one_or_none()

                if db_obj:
                    return UserRead.model_validate(db_obj)
                return None

        except SQLAlchemyError as e:
            raise e
```

### 8. `app/database/sessions/models.py` - Session Model

```python
"""
Session database model for cookie-based authentication.
"""

from datetime import datetime
from sqlalchemy import String, DateTime, ForeignKey, Text
from sqlalchemy.orm import Mapped, mapped_column
from sqlalchemy.sql import func

from app.database.base import Base
from app.common.utils import generate_id


class SessionTable(Base):
    """
    Session model for cookie-based authentication.

    Stores active user sessions.
    """
    __tablename__ = "sessions"

    # Session ID (stored in cookie)
    id: Mapped[str] = mapped_column(
        String(22),
        primary_key=True,
        default=lambda: generate_id(size=22)
    )

    # User reference
    user_id: Mapped[str] = mapped_column(
        String(22),
        ForeignKey("users.id", ondelete="CASCADE"),
        nullable=False,
        index=True
    )

    # Session data (JSON string)
    data: Mapped[str | None] = mapped_column(
        Text,
        nullable=True
    )

    # Timestamps
    created_at: Mapped[datetime] = mapped_column(
        DateTime,
        server_default=func.now(),
        nullable=False
    )

    expires_at: Mapped[datetime] = mapped_column(
        DateTime,
        nullable=False,
        index=True  # Index for cleanup queries
    )
```

### 9. `app/database/sessions/schemas.py` - Session Schemas

```python
"""
Session Pydantic schemas.
"""

from datetime import datetime
from pydantic import BaseModel, Field, ConfigDict


class SessionCreate(BaseModel):
    """Schema for creating a session"""
    user_id: str
    expires_at: datetime
    data: str | None = None


class SessionRead(BaseModel):
    """Schema for reading a session"""
    id: str
    user_id: str
    data: str | None
    created_at: datetime
    expires_at: datetime

    model_config = ConfigDict(from_attributes=True)
```

### 10. `app/database/sessions/repository.py` - Session Repository

```python
"""
Session repository using high-concurrency pattern.
"""

from datetime import datetime
from sqlalchemy import select, delete
from sqlalchemy.exc import SQLAlchemyError

from app.database.db_config import async_session
from app.database.sessions.models import SessionTable
from app.database.sessions.schemas import SessionRead


class SessionRepository:
    """Repository for session database operations"""

    @staticmethod
    async def create(user_id: str, expires_at: datetime, data: str | None = None) -> SessionRead:
        """Create a new session"""
        try:
            db_obj = SessionTable(
                user_id=user_id,
                expires_at=expires_at,
                data=data
            )

            async with async_session() as session:
                session.add(db_obj)
                await session.commit()
                await session.refresh(db_obj)
                return SessionRead.model_validate(db_obj)

        except SQLAlchemyError as e:
            raise e

    @staticmethod
    async def get_by_id(session_id: str) -> SessionRead | None:
        """Get session by ID"""
        try:
            async with async_session() as session:
                stmt = select(SessionTable).where(SessionTable.id == session_id)
                result = await session.execute(stmt)
                db_obj = result.scalar_one_or_none()

                if db_obj:
                    return SessionRead.model_validate(db_obj)
                return None

        except SQLAlchemyError as e:
            raise e

    @staticmethod
    async def delete_by_id(session_id: str) -> bool:
        """Delete session by ID"""
        try:
            async with async_session() as session:
                stmt = delete(SessionTable).where(SessionTable.id == session_id)
                result = await session.execute(stmt)
                await session.commit()
                return result.rowcount > 0

        except SQLAlchemyError as e:
            raise e

    @staticmethod
    async def delete_expired() -> int:
        """Delete all expired sessions (cleanup task)"""
        try:
            async with async_session() as session:
                stmt = delete(SessionTable).where(
                    SessionTable.expires_at < datetime.utcnow()
                )
                result = await session.execute(stmt)
                await session.commit()
                return result.rowcount

        except SQLAlchemyError as e:
            raise e
```

### 11. `app/database/__init__.py` - Import All Models

**CRITICAL**: Alembic needs all models imported to detect schema changes.

```python
"""
Database package.

Imports all models so Alembic can detect them for autogenerate.

IMPORTANT: Every new model must be imported here for Alembic to work.
"""

# Import base first
from app.database.base import Base  # noqa: F401

# Import all models (order matters for foreign keys)
from app.database.users.models import UserTable  # noqa: F401
from app.database.sessions.models import SessionTable  # noqa: F401

# Add future models here:
# from app.database.api_keys.models import APIKeyTable  # noqa: F401
```

---

## Alembic Setup

### 1. Initialize Alembic

```bash
cd backend_python

# Initialize Alembic
alembic init migrations

# This creates:
# - migrations/ directory
# - migrations/env.py
# - migrations/script.py.mako
# - alembic.ini
```

### 2. Configure `alembic.ini`

Edit `backend_python/alembic.ini`:

```ini
# A generic, single database configuration.

[alembic]
# path to migration scripts
script_location = migrations

# Template with timestamp prefix (like wt_api_v2)
file_template = %%(year)d_%%(month).2d_%%(day).2d_%%(hour).2d%%(minute).2d-%%(rev)s_%%(slug)s

# Add app to sys.path
prepend_sys_path = .

# Version path separator
version_path_separator = os

[loggers]
keys = root,sqlalchemy,alembic

[handlers]
keys = console

[formatters]
keys = generic

[logger_root]
level = WARN
handlers = console
qualname =

[logger_sqlalchemy]
level = WARN
handlers =
qualname = sqlalchemy.engine

[logger_alembic]
level = INFO
handlers =
qualname = alembic

[handler_console]
class = StreamHandler
args = (sys.stderr,)
level = NOTSET
formatter = generic

[formatter_generic]
format = %(levelname)-5.5s [%(name)s] %(message)s
datefmt = %H:%M:%S
```

### 3. Configure `migrations/env.py`

Replace `migrations/env.py` with this async-compatible version (from wt_api_v2):

```python
"""
Alembic environment configuration.

Async-compatible migration runner with SQLite-specific settings.
Pattern from wt_api_v2 (validated for production).

SQLite-specific:
- render_as_batch=True (required for ALTER TABLE support)
- Direct model imports (not package import)
"""

import asyncio
from logging.config import fileConfig

from sqlalchemy import pool
from sqlalchemy.engine import Connection
from sqlalchemy.ext.asyncio import async_engine_from_config

from alembic import context

# Import settings to get database URL
from app.config.settings import settings

# Import Base to get metadata
from app.database.base import Base

# Import all models DIRECTLY (CRITICAL - ensures Alembic sees all tables)
# Add new models here as they are created
from app.database.users.models import UserTable  # noqa: F401
from app.database.sessions.models import SessionTable  # noqa: F401
# Future models:
# from app.database.api_keys.models import APIKeyTable  # noqa: F401

# Alembic Config object
config = context.config

# Interpret config file for Python logging
if config.config_file_name is not None:
    fileConfig(config.config_file_name)

# Set target metadata from Base
target_metadata = Base.metadata

# Set naming convention for constraints
target_metadata.naming_convention = {
    "ix": "ix_%(column_0_label)s",
    "uq": "uq_%(table_name)s_%(column_0_name)s",
    "ck": "ck_%(table_name)s_%(constraint_name)s",
    "fk": "fk_%(table_name)s_%(column_0_name)s_%(referred_table_name)s",
    "pk": "pk_%(table_name)s",
}


def run_migrations_offline() -> None:
    """
    Run migrations in 'offline' mode.

    Generates SQL script without connecting to database.
    """
    url = settings.database.sqlite_url
    context.configure(
        url=url,
        target_metadata=target_metadata,
        literal_binds=True,
        dialect_opts={"paramstyle": "named"},
    )

    with context.begin_transaction():
        context.run_migrations()


def do_run_migrations(connection: Connection) -> None:
    """
    Execute migrations with provided connection.

    IMPORTANT: render_as_batch=True is REQUIRED for SQLite.
    SQLite has limited ALTER TABLE support. Batch mode creates
    a new table, copies data, and swaps tables to work around this.
    """
    context.configure(
        connection=connection,
        target_metadata=target_metadata,
        render_as_batch=True  # ⚠️ REQUIRED for SQLite ALTER TABLE support
    )

    with context.begin_transaction():
        context.run_migrations()


async def run_async_migrations() -> None:
    """
    Run migrations in 'online' mode with async engine.

    Creates engine and runs migrations asynchronously.
    """
    configuration = config.get_section(config.config_ini_section, {})
    configuration["sqlalchemy.url"] = settings.database.sqlite_url

    connectable = async_engine_from_config(
        configuration,
        prefix="sqlalchemy.",
        poolclass=pool.NullPool,
    )

    async with connectable.connect() as connection:
        await connection.run_sync(do_run_migrations)

    await connectable.dispose()


def run_migrations_online() -> None:
    """Run migrations in 'online' mode"""
    asyncio.run(run_async_migrations())


if context.is_offline_mode():
    run_migrations_offline()
else:
    run_migrations_online()
```

### 4. Generate Initial Migration

```bash
cd backend_python

# Create initial migration (autogenerate from models)
alembic revision --autogenerate -m "Initial schema: users and sessions"

# This creates: migrations/versions/2025_XX_XX_XXXX-xxxxxxx_initial_schema_users_and_sessions.py
```

### 5. Review and Apply Migration

```bash
# Review the generated migration file
cat migrations/versions/2025_*_initial_schema*.py

# Apply migration
alembic upgrade head

# Verify database created
ls -la dbdata/junjo.db

# Check migration status
alembic current
alembic history
```

---

## Initial Schema Design

The initial migration creates these tables:

```sql
-- users table
CREATE TABLE users (
    id VARCHAR(22) PRIMARY KEY,
    email VARCHAR NOT NULL UNIQUE,
    password_hash VARCHAR NOT NULL,
    is_active BOOLEAN NOT NULL DEFAULT 1,
    created_at DATETIME DEFAULT (CURRENT_TIMESTAMP) NOT NULL,
    updated_at DATETIME DEFAULT (CURRENT_TIMESTAMP) NOT NULL
);

CREATE INDEX ix_users_email ON users(email);

-- sessions table
CREATE TABLE sessions (
    id VARCHAR(22) PRIMARY KEY,
    user_id VARCHAR(22) NOT NULL,
    data TEXT,
    created_at DATETIME DEFAULT (CURRENT_TIMESTAMP) NOT NULL,
    expires_at DATETIME NOT NULL,
    FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE
);

CREATE INDEX ix_sessions_user_id ON sessions(user_id);
CREATE INDEX ix_sessions_expires_at ON sessions(expires_at);
```

---

## Testing

### 1. Test Database Setup

Create `tests/conftest.py`:

```python
"""
Pytest configuration and fixtures.

Provides test database setup.
"""

import pytest
from sqlalchemy.ext.asyncio import create_async_engine, async_sessionmaker

from app.database.base import Base


@pytest.fixture
async def test_db_engine():
    """Create test database engine (in-memory SQLite)"""
    engine = create_async_engine("sqlite+aiosqlite:///:memory:")

    # Create all tables
    async with engine.begin() as conn:
        await conn.run_sync(Base.metadata.create_all)

    yield engine

    # Cleanup
    await engine.dispose()


@pytest.fixture
async def test_db(test_db_engine):
    """Create test database session factory"""
    async_session_test = async_sessionmaker(
        test_db_engine,
        expire_on_commit=False
    )
    return async_session_test
```

### 2. Test User Repository

Create `tests/test_database.py`:

```python
"""
Tests for database repositories.
"""

import pytest
from datetime import datetime, timedelta

from app.database.users.repository import UserRepository
from app.database.sessions.repository import SessionRepository


@pytest.mark.asyncio
async def test_create_user(test_db):
    """Test user creation"""
    # Override global session with test session
    import app.database.db_config as db_config
    db_config.async_session = test_db

    # Test
    user = await UserRepository.create(
        email="test@example.com",
        password_hash="hashed_password"
    )

    assert user.email == "test@example.com"
    assert user.is_active is True
    assert user.id is not None


@pytest.mark.asyncio
async def test_get_user_by_email(test_db):
    """Test getting user by email"""
    import app.database.db_config as db_config
    db_config.async_session = test_db

    # Create user
    await UserRepository.create(
        email="test@example.com",
        password_hash="hashed_password"
    )

    # Get user
    user = await UserRepository.get_by_email("test@example.com")

    assert user is not None
    assert user.email == "test@example.com"
    assert user.password_hash == "hashed_password"


@pytest.mark.asyncio
async def test_create_session(test_db):
    """Test session creation"""
    import app.database.db_config as db_config
    db_config.async_session = test_db

    # Create user first
    user = await UserRepository.create(
        email="test@example.com",
        password_hash="hashed_password"
    )

    # Create session
    expires_at = datetime.utcnow() + timedelta(days=7)
    session = await SessionRepository.create(
        user_id=user.id,
        expires_at=expires_at
    )

    assert session.user_id == user.id
    assert session.id is not None


@pytest.mark.asyncio
async def test_delete_expired_sessions(test_db):
    """Test cleaning up expired sessions"""
    import app.database.db_config as db_config
    db_config.async_session = test_db

    # Create user
    user = await UserRepository.create(
        email="test@example.com",
        password_hash="hashed_password"
    )

    # Create expired session
    expired_time = datetime.utcnow() - timedelta(days=1)
    await SessionRepository.create(
        user_id=user.id,
        expires_at=expired_time
    )

    # Create valid session
    valid_time = datetime.utcnow() + timedelta(days=7)
    await SessionRepository.create(
        user_id=user.id,
        expires_at=valid_time
    )

    # Delete expired
    deleted_count = await SessionRepository.delete_expired()

    assert deleted_count == 1
```

Run tests:

```bash
cd backend_python
pytest tests/test_database.py -v
```

---

## Success Criteria

Phase 2 is complete when:

- ✅ `app/database/` directory structure created
- ✅ SQLAlchemy dependencies installed
- ✅ `Base` declarative base created
- ✅ `db_config.py` with async engine and session factory
- ✅ `async_session` configured with `expire_on_commit=False`
- ✅ OpenTelemetry SQLAlchemy instrumentation enabled
- ✅ User model (`UserTable`) created with modern Mapped[] syntax
- ✅ Session model (`SessionTable`) created
- ✅ Pydantic schemas for users and sessions
- ✅ Repositories with static methods and high-concurrency pattern
- ✅ All models imported in `app/database/__init__.py`
- ✅ Alembic initialized with async-compatible `env.py`
- ✅ Initial migration generated
- ✅ Migration applied successfully (`alembic upgrade head`)
- ✅ `dbdata/junjo.db` file created
- ✅ Database tables created (users, sessions)
- ✅ Tests pass for user and session repositories
- ✅ Can create users via repository
- ✅ Can create sessions via repository

---

## Troubleshooting

### Issue: Alembic doesn't detect models

**Solution:**
1. Ensure all models are imported in `app/database/__init__.py`:
   ```python
   from app.database.users.models import UserTable  # noqa: F401
   from app.database.sessions.models import SessionTable  # noqa: F401
   ```
2. Verify `migrations/env.py` imports `app.database`:
   ```python
   import app.database  # noqa: F401
   ```
3. Check models inherit from correct Base:
   ```python
   from app.database.base import Base
   class UserTable(Base):
   ```

### Issue: `greenlet_spawn has not been called` error

**Solution:**
Ensure `expire_on_commit=False` in session factory:
```python
async_session = async_sessionmaker(
    engine,
    expire_on_commit=False  # Required!
)
```

### Issue: Alembic migration fails with async engine

**Solution:**
Use `run_sync()` for sync operations:
```python
async with engine.begin() as conn:
    await conn.run_sync(Base.metadata.create_all)
```

### Issue: Foreign key constraint fails

**Solution:**
1. Ensure parent table created first (import order in `__init__.py`)
2. Use `ondelete="CASCADE"` in ForeignKey:
   ```python
   ForeignKey("users.id", ondelete="CASCADE")
   ```

### Issue: Migration file not using timestamp format

**Solution:**
Check `alembic.ini` has `file_template` setting:
```ini
file_template = %%(year)d_%%(month).2d_%%(day).2d_%%(hour).2d%%(minute).2d-%%(rev)s_%%(slug)s
```

---

## Next Steps

Once Phase 2 is complete:

1. ✅ Verify all success criteria met
2. ✅ Commit changes to git
3. ✅ Test database operations manually
4. ✅ Update `PYTHON_BACKEND_MIGRATION_0_Master.md` with Phase 2 completion
5. ➡️ **Proceed to Phase 3**: Session/Cookie Authentication

---

## References

- [SQLAlchemy 2.0 Documentation](https://docs.sqlalchemy.org/en/20/)
- [Alembic Documentation](https://alembic.sqlalchemy.org/)
- [wt_api_v2 Database Pattern](file:///Users/matt/repos/wt_api_v2/src/app/database/)
- [High-Concurrency DB Pattern](./PYTHON_BACKEND_HIGH_CONCURRENCY_DB_PATTERN.md)
- [FastAPI with SQLAlchemy](https://fastapi.tiangolo.com/tutorial/sql-databases/)
