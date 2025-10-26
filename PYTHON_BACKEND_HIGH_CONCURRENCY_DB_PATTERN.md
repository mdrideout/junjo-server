# High-Concurrency Database Pattern

> **Source**: wt_api_v2 repository (production-validated)
> **Status**: Reference document for Python backend migration
> **Python Version**: 3.14+
> **SQLAlchemy Version**: 2.0+

---

## Overview

This document describes the **asyncio-safe, high-concurrency database pattern** used in the wt_api_v2 repository and adopted for the Junjo Server Python backend migration.

**Key Principle**: Each database operation creates its own session, ensuring complete isolation between concurrent requests with zero risk of session conflicts.

---

## Critical Components

### 1. Session Factory with `expire_on_commit=False`

**Location**: `app/database/db_config.py`

```python
from sqlalchemy.ext.asyncio import create_async_engine, async_sessionmaker
from opentelemetry.instrumentation.sqlalchemy import SQLAlchemyInstrumentor

from app.config.settings import settings

# Create the async engine
engine = create_async_engine(
    settings.database.sqlite_url,
    echo=settings.debug,  # SQL logging in debug mode
)

# Create the async session factory
# CRITICAL: expire_on_commit=False prevents lazy loading errors in async contexts
async_session = async_sessionmaker(
    engine,
    expire_on_commit=False  # ⚠️ REQUIRED for asyncio safety
)

# Instrument SQLAlchemy for OpenTelemetry
SQLAlchemyInstrumentor().instrument(
    engine=engine.sync_engine,  # .sync_engine required for instrumentation
    dbapi_level_span=True,
)


# Dependency for FastAPI (if needed - most repos use static pattern below)
async def get_db():
    """FastAPI dependency to get DB session"""
    async with async_session() as session:
        yield session
```

**Why `expire_on_commit=False` is Critical:**

| Setting | Behavior | Risk without it |
|---------|----------|-----------------|
| `expire_on_commit=True` (default) | Objects expire after commit, require re-loading | `greenlet_spawn has not been called` errors in async contexts |
| `expire_on_commit=False` | Objects remain valid after commit | None (safe for asyncio) |

**When to use `expire_on_commit=False`:**
- ✅ Always in asyncio/async contexts (FastAPI, async workers)
- ✅ When returning ORM objects from functions after commit
- ✅ High-concurrency environments

---

### 2. Repository Pattern with Static Methods

**Location**: `app/database/{feature}/repository.py`

```python
from sqlalchemy import select, update, delete
from sqlalchemy.exc import SQLAlchemyError

from app.database.db_config import async_session
from app.database.users.models import UserTable
from app.database.users.schemas import UserRead, UserCreate
from app.firebase.schemas import AuthenticatedUser


class UserRepository:
    """
    Repository for user database operations.

    Uses static methods to avoid instance state.
    Each method creates its own session for complete isolation.
    """

    @staticmethod
    async def create(user: AuthenticatedUser) -> UserRead:
        """
        Create a new user.

        Pattern:
        1. Create session via context manager
        2. Add object to session
        3. Commit (flush to DB)
        4. Refresh (load generated fields like id, timestamps)
        5. Validate to Pydantic model BEFORE session closes
        6. Return serialized Pydantic model (not SQLAlchemy object)
        """
        try:
            db_obj = UserTable(uid=user.uid, email=user.email)

            # Fresh session for this operation only
            async with async_session() as session:
                session.add(db_obj)
                await session.commit()
                await session.refresh(db_obj)  # Load generated fields

                # CRITICAL: Validate to Pydantic BEFORE session closes
                return UserRead.model_validate(db_obj)

        except SQLAlchemyError as e:
            # Let SQLAlchemy errors propagate for handling at router level
            raise e

    @staticmethod
    async def get_by_id(user_id: str, owner_id: str) -> UserRead | None:
        """
        Get user by ID with authorization check.

        Defense in depth: owner_id baked into WHERE clause.
        """
        try:
            async with async_session() as session:
                stmt = select(UserTable).where(
                    UserTable.id == user_id,
                    UserTable.owner_id == owner_id  # Authorization built-in
                )
                result = await session.execute(stmt)
                db_obj = result.scalar_one_or_none()

                if db_obj:
                    return UserRead.model_validate(db_obj)
                return None

        except SQLAlchemyError as e:
            raise e

    @staticmethod
    async def get_all_by_owner(owner_id: str, limit: int = 100) -> list[UserRead]:
        """
        Get all users for an owner with limit.

        Uses scalar() iterator for memory efficiency.
        """
        try:
            async with async_session() as session:
                stmt = (
                    select(UserTable)
                    .where(UserTable.owner_id == owner_id)
                    .limit(limit)
                    .order_by(UserTable.created_at.desc())
                )
                result = await session.execute(stmt)
                db_objs = result.scalars().all()

                # Validate all to Pydantic before returning
                return [UserRead.model_validate(obj) for obj in db_objs]

        except SQLAlchemyError as e:
            raise e

    @staticmethod
    async def update_email(user_id: str, owner_id: str, new_email: str) -> UserRead:
        """
        Update user email with authorization.

        Uses .returning() to get updated object in single query.
        """
        try:
            async with async_session() as session:
                stmt = (
                    update(UserTable)
                    .where(
                        UserTable.id == user_id,
                        UserTable.owner_id == owner_id  # Authorization
                    )
                    .values(email=new_email)
                    .returning(UserTable)  # Return updated object
                )
                result = await session.execute(stmt)
                await session.commit()
                db_obj = result.scalar_one()

                return UserRead.model_validate(db_obj)

        except SQLAlchemyError as e:
            raise e

    @staticmethod
    async def delete_by_id(user_id: str, owner_id: str) -> bool:
        """
        Delete user by ID with authorization.

        Returns True if deleted, False if not found.
        """
        try:
            async with async_session() as session:
                stmt = delete(UserTable).where(
                    UserTable.id == user_id,
                    UserTable.owner_id == owner_id
                )
                result = await session.execute(stmt)
                await session.commit()

                # Check if any rows were affected
                return result.rowcount > 0

        except SQLAlchemyError as e:
            raise e
```

**Key Pattern Points:**

1. ✅ **Static methods** - No `self`, no instance state, thread-safe
2. ✅ **Fresh session per method** - Complete isolation between operations
3. ✅ **Context manager** - Automatic cleanup on error or success
4. ✅ **Pydantic validation before return** - Serialize before session closes
5. ✅ **Authorization in queries** - Defense in depth (user_id in WHERE)
6. ✅ **Let exceptions propagate** - Handle at router/service level

---

### 3. SQLAlchemy Models (ORM)

**Location**: `app/database/{feature}/models.py`

```python
import enum
from datetime import datetime
from sqlalchemy import String, DateTime, ForeignKey, Enum, Text, Boolean
from sqlalchemy.orm import Mapped, mapped_column
from sqlalchemy.sql import func

from app.database.base import Base
from app.common.utils import generate_id


class UserStatus(enum.StrEnum):
    """User status enum (StrEnum for JSON serialization)"""
    ACTIVE = "ACTIVE"
    SUSPENDED = "SUSPENDED"
    DELETED = "DELETED"


class UserTable(Base):
    """
    User database model.

    Uses modern SQLAlchemy 2.0 syntax:
    - Mapped[] type hints
    - mapped_column() for column definitions
    - server_default for DB-level defaults
    """
    __tablename__ = "users"

    # Primary key with custom ID generator
    id: Mapped[str] = mapped_column(
        String(22),
        primary_key=True,
        default=lambda: generate_id(size=22)  # nanoid or similar
    )

    # Foreign keys
    owner_id: Mapped[str] = mapped_column(
        String,
        ForeignKey("users.id"),
        nullable=False,
        index=True  # Index for performance on FK lookups
    )

    # Required fields
    email: Mapped[str] = mapped_column(
        String,
        nullable=False,
        unique=True,
        index=True
    )

    # Optional fields (use | None)
    display_name: Mapped[str | None] = mapped_column(
        String,
        nullable=True,
        default=None
    )

    # Enums (native_enum=False for cross-DB compatibility)
    status: Mapped[UserStatus] = mapped_column(
        Enum(UserStatus, native_enum=False),  # Stores as VARCHAR
        nullable=False,
        default=UserStatus.ACTIVE
    )

    # Booleans
    is_verified: Mapped[bool] = mapped_column(
        Boolean,
        nullable=False,
        default=False
    )

    # Timestamps (server_default for DB-level defaults)
    created_at: Mapped[datetime] = mapped_column(
        DateTime,
        server_default=func.now(),  # DB generates on insert
        nullable=False
    )

    updated_at: Mapped[datetime] = mapped_column(
        DateTime,
        server_default=func.now(),
        onupdate=func.now(),  # DB updates on UPDATE
        nullable=False
    )

    # Nullable text field
    bio: Mapped[str | None] = mapped_column(
        Text,
        nullable=True
    )
```

**Modern SQLAlchemy 2.0 Patterns:**

| Pattern | Example | Benefit |
|---------|---------|---------|
| `Mapped[]` type hints | `id: Mapped[str]` | Type safety, IDE autocomplete |
| `mapped_column()` | `mapped_column(String)` | Unified column definition |
| `server_default=func.now()` | Timestamps | DB generates values |
| `Enum(..., native_enum=False)` | Status enums | Cross-DB compatibility |
| `| None` for nullable | `name: Mapped[str \| None]` | Python 3.10+ union syntax |

---

### 4. Pydantic Schemas (Validation/Serialization)

**Location**: `app/database/{feature}/schemas.py`

```python
from datetime import datetime
from pydantic import BaseModel, EmailStr, Field, ConfigDict

from app.database.users.models import UserStatus


class UserCreate(BaseModel):
    """Schema for creating a user (input validation)"""
    email: EmailStr
    display_name: str | None = None
    owner_id: str = Field(..., min_length=1)


class UserUpdate(BaseModel):
    """Schema for updating a user (partial updates)"""
    email: EmailStr | None = None
    display_name: str | None = None
    bio: str | None = None
    status: UserStatus | None = None


class UserRead(BaseModel):
    """
    Schema for reading a user (output serialization).

    CRITICAL: ConfigDict(from_attributes=True) allows validation
    from SQLAlchemy ORM objects.
    """
    id: str
    email: EmailStr
    display_name: str | None
    status: UserStatus
    is_verified: bool
    created_at: datetime
    updated_at: datetime
    bio: str | None

    model_config = ConfigDict(
        from_attributes=True,  # ⚠️ REQUIRED for SQLAlchemy models
        frozen=True,  # Immutable after creation (optional)
    )


class UserWithDetails(BaseModel):
    """Schema for complex nested data"""
    user: UserRead
    post_count: int
    last_login: datetime | None

    model_config = ConfigDict(from_attributes=True)
```

**Pydantic v2+ Conventions:**

1. ✅ **`ConfigDict(from_attributes=True)`** - Validates from SQLAlchemy models
2. ✅ **Separate schemas** - Create, Update, Read (no mixing concerns)
3. ✅ **Type hints with `|`** - Python 3.10+ union syntax
4. ✅ **`EmailStr` for emails** - Automatic validation
5. ✅ **`Field(...)` for constraints** - min_length, max_length, ge, le
6. ✅ **`frozen=True` for responses** - Immutable output (optional)

---

## Complete Example: User Feature

### Directory Structure

```
app/database/users/
├── __init__.py
├── models.py      # SQLAlchemy ORM model (UserTable)
├── schemas.py     # Pydantic models (UserCreate, UserRead, etc.)
└── repository.py  # Database operations (UserRepository)
```

### Usage in Service/Router

```python
# app/features/users/service.py
from app.database.users.repository import UserRepository
from app.database.users.schemas import UserRead, UserCreate
from app.firebase.schemas import AuthenticatedUser


class UserService:
    """Business logic for users"""

    @staticmethod
    async def create_user(data: UserCreate, current_user: AuthenticatedUser) -> UserRead:
        """
        Create a new user.

        Repository handles database operations.
        Service handles business logic.
        """
        # Business logic: check if email already exists
        existing = await UserRepository.get_by_email(data.email)
        if existing:
            raise ValueError("Email already registered")

        # Repository handles database transaction
        return await UserRepository.create(
            email=data.email,
            owner_id=current_user.uid
        )

    @staticmethod
    async def get_user(user_id: str, current_user: AuthenticatedUser) -> UserRead | None:
        """Get user by ID (with authorization)"""
        return await UserRepository.get_by_id(
            user_id=user_id,
            owner_id=current_user.uid  # Authorization
        )


# app/features/users/router.py
from fastapi import APIRouter, Depends, HTTPException, status
from app.features.users.service import UserService
from app.features.users.schemas import UserRead, UserCreate
from app.features.auth.dependencies import get_current_user
from app.firebase.schemas import AuthenticatedUser

router = APIRouter(prefix="/api/v1/users", tags=["Users"])


@router.post("/", response_model=UserRead, status_code=status.HTTP_201_CREATED)
async def create_user(
    data: UserCreate,
    current_user: AuthenticatedUser = Depends(get_current_user)
) -> UserRead:
    """Create a new user"""
    try:
        return await UserService.create_user(data, current_user)
    except ValueError as e:
        raise HTTPException(status_code=400, detail=str(e))


@router.get("/{user_id}", response_model=UserRead)
async def get_user(
    user_id: str,
    current_user: AuthenticatedUser = Depends(get_current_user)
) -> UserRead:
    """Get user by ID"""
    user = await UserService.get_user(user_id, current_user)
    if not user:
        raise HTTPException(status_code=404, detail="User not found")
    return user
```

---

## Concurrency Safety Summary

| Component | Pattern | Why It's Safe |
|-----------|---------|---------------|
| **Session Factory** | `expire_on_commit=False` | No lazy loading errors |
| **Repository** | Static methods | No instance state |
| **Session Creation** | Fresh per operation | Complete isolation |
| **Context Manager** | `async with async_session()` | Auto cleanup |
| **Validation** | Pydantic before session closes | No detached instances |
| **Authorization** | In WHERE clauses | Defense in depth |

### What NOT to Do

❌ **Don't share sessions across operations:**
```python
# BAD - session shared across operations
async with async_session() as session:
    user = await create_user(session)  # ❌
    post = await create_post(session)  # ❌
    # Risk: transaction couples unrelated operations
```

✅ **Do create fresh session per operation:**
```python
# GOOD - each operation gets fresh session
user = await UserRepository.create(...)  # ✅ Own session
post = await PostRepository.create(...)  # ✅ Own session
# Each operation isolated, can run concurrently
```

❌ **Don't return SQLAlchemy objects:**
```python
# BAD - returning ORM object
async with async_session() as session:
    result = await session.execute(select(UserTable))
    return result.scalar_one()  # ❌ Object detached after session closes
```

✅ **Do validate to Pydantic before returning:**
```python
# GOOD - validate to Pydantic before session closes
async with async_session() as session:
    result = await session.execute(select(UserTable))
    db_obj = result.scalar_one()
    return UserRead.model_validate(db_obj)  # ✅ Serialized before return
```

---

## Performance Considerations

### Connection Pooling

SQLAlchemy handles connection pooling automatically:

```python
# db_config.py
engine = create_async_engine(
    settings.database.sqlite_url,
    echo=False,
    pool_size=20,          # Max connections in pool
    max_overflow=10,       # Additional connections when pool full
    pool_pre_ping=True,    # Verify connections before use
    pool_recycle=3600,     # Recycle connections after 1 hour
)
```

### N+1 Query Prevention

Use `selectinload()` or `joinedload()` for relationships:

```python
from sqlalchemy.orm import selectinload

# BAD - N+1 queries
users = await session.execute(select(UserTable))
for user in users.scalars():
    posts = await session.execute(select(PostTable).where(PostTable.user_id == user.id))
    # ❌ N queries for N users

# GOOD - eager loading
stmt = select(UserTable).options(selectinload(UserTable.posts))
result = await session.execute(stmt)
users = result.scalars().all()
# ✅ 2 queries total (users + all posts)
```

---

## Testing Pattern

```python
# tests/conftest.py
import pytest
from sqlalchemy.ext.asyncio import create_async_engine, async_sessionmaker

from app.database.base import Base


@pytest.fixture
async def test_db():
    """Create test database for each test"""
    # In-memory SQLite for fast tests
    engine = create_async_engine("sqlite+aiosqlite:///:memory:")

    # Create all tables
    async with engine.begin() as conn:
        await conn.run_sync(Base.metadata.create_all)

    # Create session factory
    async_session_test = async_sessionmaker(engine, expire_on_commit=False)

    yield async_session_test

    # Cleanup
    await engine.dispose()


@pytest.mark.asyncio
async def test_create_user(test_db):
    """Test user creation"""
    from app.database.users.repository import UserRepository

    # Override global session with test session
    import app.database.db_config as db_config
    db_config.async_session = test_db

    # Test
    user = await UserRepository.create(email="test@example.com", owner_id="test-owner")
    assert user.email == "test@example.com"
```

---

## References

- [SQLAlchemy 2.0 Documentation](https://docs.sqlalchemy.org/en/20/)
- [FastAPI with SQLAlchemy](https://fastapi.tiangolo.com/tutorial/sql-databases/)
- [Pydantic v2 Documentation](https://docs.pydantic.dev/latest/)
- [wt_api_v2 Source](file:///Users/matt/repos/wt_api_v2/src/app/database/)

---

**Use this pattern for all database operations in the Junjo Server Python backend migration.**
