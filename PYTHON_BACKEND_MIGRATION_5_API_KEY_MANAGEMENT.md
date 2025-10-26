# Phase 5: API Key Management

## Overview

This phase implements the API key management feature, which allows users to:
- Create new API keys with human-readable names
- List all API keys
- Delete API keys

API keys are used by external clients (via the ingestion-service) to authenticate telemetry data ingestion. The Python backend validates these keys via the gRPC InternalAuthService (Phase 4).

## Current Go Implementation Analysis

**Database Schema** (from `/backend/db/schema.sql:15-20`):
```sql
CREATE TABLE api_keys (
  id TEXT PRIMARY KEY,
  key TEXT NOT NULL UNIQUE,
  name TEXT NOT NULL,
  created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
);
```

**Key Generation** (from `/backend/api_keys/services.go:12-44`):
- **ID**: Generated using `gonanoid.New()` (21 characters, default alphabet)
- **Key**: Generated using `gonanoid.Generate(alphanumeric, 64)` (64 characters, alphanumeric only)
- Both use cryptographically secure random generation

**Endpoints** (from `/backend/api_keys/controller.go:7-13`):
- POST `/api_keys` - Create API key (auth required)
- GET `/api_keys` - List API keys (auth required)
- DELETE `/api_keys/:key` - Delete API key (auth required)

**Repository Operations** (from `/backend/api_keys/repository.go`):
- `CreateAPIKey(ctx, id, key, name)` - Insert new key
- `GetAPIKey(ctx, key)` - Retrieve key by key value
- `ListAPIKeys(ctx)` - List all keys (ordered by created_at DESC)
- `DeleteAPIKey(ctx, key)` - Delete key by key value

## Python Implementation

### Directory Structure

```
python_backend/
└── app/
    ├── database/
    │   └── api_keys/
    │       ├── __init__.py
    │       ├── models.py      # APIKeyTable SQLAlchemy model
    │       ├── repository.py  # API key database operations
    │       └── schemas.py     # Pydantic schemas
    ├── features/
    │   └── api_keys/
    │       ├── __init__.py
    │       ├── router.py      # FastAPI router with endpoints
    │       ├── service.py     # Business logic
    │       └── utils.py       # Key generation utilities
    └── tests/
        ├── unit/
        │   └── features/
        │       └── api_keys/
        │           ├── test_key_generation.py
        │           ├── test_api_key_repository.py
        │           └── test_api_key_service.py
        └── integration/
            └── features/
                └── api_keys/
                    └── test_api_key_endpoints.py
```

### 1. API Key Database Model

**File**: `app/database/api_keys/models.py`

```python
"""
API Key database model.
"""

from datetime import datetime
from typing import Annotated

from sqlalchemy import String, func
from sqlalchemy.orm import Mapped, mapped_column

from app.database.db_config import Base

# Type alias for timestamp
timestamp = Annotated[
    datetime,
    mapped_column(nullable=False, server_default=func.current_timestamp()),
]


class APIKeyTable(Base):
    """
    API Key model for authentication.

    Mirrors the Go schema:
    CREATE TABLE api_keys (
      id TEXT PRIMARY KEY,
      key TEXT NOT NULL UNIQUE,
      name TEXT NOT NULL,
      created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
    );
    """

    __tablename__ = "api_keys"

    id: Mapped[str] = mapped_column(String(255), primary_key=True)
    key: Mapped[str] = mapped_column(String(255), unique=True, nullable=False)
    name: Mapped[str] = mapped_column(String(255), nullable=False)
    created_at: Mapped[timestamp]

    def __repr__(self) -> str:
        return f"<APIKeyTable(id={self.id}, name={self.name})>"
```

### 2. API Key Repository

**File**: `app/database/api_keys/repository.py`

```python
"""
API Key repository for database operations.

Following the high-concurrency pattern from wt_api_v2:
- Static methods (no instance state)
- Fresh session per operation
- expire_on_commit=False for asyncio safety
- Pydantic validation before session closes
"""

from typing import List, Optional

from sqlalchemy import select
from sqlalchemy.ext.asyncio import AsyncSession

from app.database.api_keys.models import APIKeyTable
from app.database.api_keys.schemas import APIKey, APIKeyCreate
from app.database.db_config import get_async_session


class APIKeyRepository:
    """Repository for API key database operations."""

    @staticmethod
    async def create_api_key(api_key_create: APIKeyCreate) -> APIKey:
        """
        Create a new API key.

        Mirrors Go: CreateAPIKey(ctx, id, key, name)

        Args:
            api_key_create: API key creation data

        Returns:
            Created API key

        Raises:
            IntegrityError: If key already exists (UNIQUE constraint)
        """
        async with get_async_session() as session:
            api_key = APIKeyTable(
                id=api_key_create.id,
                key=api_key_create.key,
                name=api_key_create.name,
            )
            session.add(api_key)
            await session.commit()
            await session.refresh(api_key)

            # Validate with Pydantic before session closes
            return APIKey.model_validate(api_key)

    @staticmethod
    async def get_api_key_by_key(key: str) -> Optional[APIKey]:
        """
        Get API key by key value.

        Mirrors Go: GetAPIKey(ctx, key)

        Args:
            key: API key value

        Returns:
            API key if found, None otherwise
        """
        async with get_async_session() as session:
            result = await session.execute(
                select(APIKeyTable).where(APIKeyTable.key == key)
            )
            api_key = result.scalar_one_or_none()

            if api_key is None:
                return None

            # Validate with Pydantic before session closes
            return APIKey.model_validate(api_key)

    @staticmethod
    async def list_api_keys() -> List[APIKey]:
        """
        List all API keys, ordered by creation date descending.

        Mirrors Go: ListAPIKeys(ctx)

        Returns:
            List of all API keys
        """
        async with get_async_session() as session:
            result = await session.execute(
                select(APIKeyTable).order_by(APIKeyTable.created_at.desc())
            )
            api_keys = result.scalars().all()

            # Validate with Pydantic before session closes
            return [APIKey.model_validate(api_key) for api_key in api_keys]

    @staticmethod
    async def delete_api_key(key: str) -> bool:
        """
        Delete an API key by key value.

        Mirrors Go: DeleteAPIKey(ctx, key)

        Args:
            key: API key value to delete

        Returns:
            True if key was deleted, False if key not found
        """
        async with get_async_session() as session:
            result = await session.execute(
                select(APIKeyTable).where(APIKeyTable.key == key)
            )
            api_key = result.scalar_one_or_none()

            if api_key is None:
                return False

            await session.delete(api_key)
            await session.commit()
            return True
```

### 3. API Key Schemas

**File**: `app/database/api_keys/schemas.py`

```python
"""
Pydantic schemas for API Key model.
"""

from datetime import datetime

from pydantic import BaseModel, ConfigDict, Field


class APIKeyCreate(BaseModel):
    """Schema for creating an API key (internal use)."""

    id: str = Field(..., min_length=1, max_length=255)
    key: str = Field(..., min_length=1, max_length=255)
    name: str = Field(..., min_length=1, max_length=255)

    model_config = ConfigDict(from_attributes=True)


class APIKey(BaseModel):
    """
    API Key schema (public).

    Returned from all endpoints and repository methods.
    """

    id: str
    key: str
    name: str
    created_at: datetime

    model_config = ConfigDict(from_attributes=True)


class CreateAPIKeyRequest(BaseModel):
    """Schema for API key creation request (API endpoint)."""

    name: str = Field(..., min_length=1, max_length=255, description="Human-readable name for the API key")


class DeleteAPIKeyResponse(BaseModel):
    """Response for API key deletion."""

    message: str
```

### 4. Key Generation Utilities

**File**: `app/features/api_keys/utils.py`

```python
"""
Utilities for API key generation.

Mirrors the Go implementation using nanoid for cryptographically secure key generation.
"""

import string

from nanoid import generate

# Alphanumeric alphabet (matches Go implementation)
ALPHANUMERIC = string.digits + string.ascii_letters


def generate_api_key_id() -> str:
    """
    Generate a unique ID for an API key.

    Mirrors Go: gonanoid.New() (21 characters, default alphabet)

    Returns:
        Unique ID string (21 characters)
    """
    return generate(size=21)


def generate_api_key() -> str:
    """
    Generate a cryptographically secure API key.

    Mirrors Go: gonanoid.Generate(alphanumeric, 64)

    Returns:
        Secure API key string (64 alphanumeric characters)
    """
    return generate(ALPHANUMERIC, size=64)
```

### 5. API Key Service

**File**: `app/features/api_keys/service.py`

```python
"""
API Key service layer.

Contains business logic for API key operations.
"""

from typing import List, Optional

from app.database.api_keys.repository import APIKeyRepository
from app.database.api_keys.schemas import APIKey, APIKeyCreate
from app.features.api_keys.utils import generate_api_key, generate_api_key_id


class APIKeyService:
    """Service for API key operations."""

    @staticmethod
    async def create_api_key(name: str) -> APIKey:
        """
        Create a new API key.

        Mirrors Go: HandleCreateAPIKey in services.go:24-53

        Args:
            name: Human-readable name for the key

        Returns:
            Created API key with generated ID and key

        Raises:
            IntegrityError: If key generation produces a duplicate (extremely unlikely)
        """
        # Generate unique ID and key
        api_key_id = generate_api_key_id()
        api_key_value = generate_api_key()

        # Create API key in database
        api_key_create = APIKeyCreate(
            id=api_key_id,
            key=api_key_value,
            name=name,
        )

        return await APIKeyRepository.create_api_key(api_key_create)

    @staticmethod
    async def get_api_key_by_key(key: str) -> Optional[APIKey]:
        """
        Get API key by key value.

        Args:
            key: API key value

        Returns:
            API key if found, None otherwise
        """
        return await APIKeyRepository.get_api_key_by_key(key)

    @staticmethod
    async def list_api_keys() -> List[APIKey]:
        """
        List all API keys.

        Returns:
            List of all API keys, ordered by creation date descending
        """
        return await APIKeyRepository.list_api_keys()

    @staticmethod
    async def delete_api_key(key: str) -> bool:
        """
        Delete an API key.

        Args:
            key: API key value to delete

        Returns:
            True if key was deleted, False if not found
        """
        return await APIKeyRepository.delete_api_key(key)
```

### 6. API Key Router

**File**: `app/features/api_keys/router.py`

```python
"""
API Key router.

Implements all API key endpoints, mirroring the Go implementation.
"""

from typing import List

from fastapi import APIRouter, HTTPException, status

from app.core.logger import logger
from app.database.api_keys.schemas import APIKey, CreateAPIKeyRequest, DeleteAPIKeyResponse
from app.features.api_keys.service import APIKeyService
from app.features.auth.dependencies import CurrentUserEmail

router = APIRouter()


@router.post("", response_model=APIKey, status_code=status.HTTP_201_CREATED)
async def create_api_key(request: CreateAPIKeyRequest, current_user_email: CurrentUserEmail):
    """
    Create a new API key.

    Mirrors Go: HandleCreateAPIKey in services.go:24-53

    Auth required: Uses CurrentUserEmail dependency.

    Args:
        request: API key creation request with name
        current_user_email: Current user email from session (auth dependency)

    Returns:
        Created API key with generated ID and key value

    Raises:
        HTTPException: 500 if key generation or database operation fails
    """
    try:
        api_key = await APIKeyService.create_api_key(request.name)
        logger.info(f"Created API key: {api_key.id} (name: {api_key.name})")
        return api_key
    except Exception as e:
        logger.error(f"Failed to create API key: {e}")
        raise HTTPException(
            status_code=status.HTTP_500_INTERNAL_SERVER_ERROR,
            detail="Failed to create API key",
        )


@router.get("", response_model=List[APIKey])
async def list_api_keys(current_user_email: CurrentUserEmail):
    """
    List all API keys.

    Mirrors Go: HandleListAPIKeys in services.go:55-69

    Auth required: Uses CurrentUserEmail dependency.

    Args:
        current_user_email: Current user email from session (auth dependency)

    Returns:
        List of all API keys (empty list if no keys exist)

    Raises:
        HTTPException: 500 if database operation fails
    """
    try:
        api_keys = await APIKeyService.list_api_keys()
        return api_keys
    except Exception as e:
        logger.error(f"Failed to list API keys: {e}")
        raise HTTPException(
            status_code=status.HTTP_500_INTERNAL_SERVER_ERROR,
            detail="Failed to retrieve API keys",
        )


@router.delete("/{key}", status_code=status.HTTP_204_NO_CONTENT)
async def delete_api_key(key: str, current_user_email: CurrentUserEmail):
    """
    Delete an API key by key value.

    Mirrors Go: HandleDeleteAPIKey in services.go:71-95

    Auth required: Uses CurrentUserEmail dependency.

    Args:
        key: API key value to delete
        current_user_email: Current user email from session (auth dependency)

    Returns:
        204 No Content on success

    Raises:
        HTTPException: 400 if key is empty, 404 if key not found
    """
    if not key:
        raise HTTPException(
            status_code=status.HTTP_400_BAD_REQUEST,
            detail="API key parameter is required",
        )

    deleted = await APIKeyService.delete_api_key(key)

    if not deleted:
        raise HTTPException(
            status_code=status.HTTP_404_NOT_FOUND,
            detail="API key not found",
        )

    logger.info(f"Deleted API key: {key}")
    return None  # 204 No Content
```

### 7. Update `main.py`

**File**: `app/main.py` (add API keys router)

```python
# Add import
from app.features.api_keys.router import router as api_keys_router

# In create_app() function, add API keys router:
app.include_router(api_keys_router, prefix="/api_keys", tags=["api_keys"])
```

## Testing Requirements

### Unit Tests

**File**: `tests/unit/features/api_keys/test_key_generation.py`

```python
"""Unit tests for API key generation."""

import pytest

from app.features.api_keys.utils import generate_api_key, generate_api_key_id


@pytest.mark.unit
def test_generate_api_key_id():
    """Test API key ID generation."""
    key_id = generate_api_key_id()

    # Should be 21 characters (matching Go implementation)
    assert len(key_id) == 21
    assert isinstance(key_id, str)


@pytest.mark.unit
def test_generate_api_key():
    """Test API key generation."""
    api_key = generate_api_key()

    # Should be 64 alphanumeric characters
    assert len(api_key) == 64
    assert isinstance(api_key, str)
    assert api_key.isalnum()


@pytest.mark.unit
def test_generate_api_key_uniqueness():
    """Test that generated API keys are unique."""
    key1 = generate_api_key()
    key2 = generate_api_key()

    # Should be different (cryptographically random)
    assert key1 != key2


@pytest.mark.unit
def test_generate_api_key_id_uniqueness():
    """Test that generated API key IDs are unique."""
    id1 = generate_api_key_id()
    id2 = generate_api_key_id()

    # Should be different (cryptographically random)
    assert id1 != id2
```

**File**: `tests/unit/features/api_keys/test_api_key_repository.py`

```python
"""Unit tests for API key repository."""

import pytest
from sqlalchemy.exc import IntegrityError

from app.database.api_keys.repository import APIKeyRepository
from app.database.api_keys.schemas import APIKeyCreate


@pytest.mark.unit
@pytest.mark.asyncio
async def test_create_api_key(test_db_engine):
    """Test API key creation."""
    api_key_create = APIKeyCreate(
        id="test-id-123",
        key="test-key-abc",
        name="Test API Key",
    )
    api_key = await APIKeyRepository.create_api_key(api_key_create)

    assert api_key.id == "test-id-123"
    assert api_key.key == "test-key-abc"
    assert api_key.name == "Test API Key"
    assert api_key.created_at is not None


@pytest.mark.unit
@pytest.mark.asyncio
async def test_create_api_key_duplicate_key(test_db_engine):
    """Test API key creation with duplicate key raises IntegrityError."""
    api_key_create = APIKeyCreate(
        id="test-id-123",
        key="test-key-abc",
        name="Test API Key",
    )
    await APIKeyRepository.create_api_key(api_key_create)

    # Try to create another key with same key value
    api_key_create2 = APIKeyCreate(
        id="test-id-456",
        key="test-key-abc",  # Duplicate key
        name="Duplicate Key",
    )
    with pytest.raises(IntegrityError):
        await APIKeyRepository.create_api_key(api_key_create2)


@pytest.mark.unit
@pytest.mark.asyncio
async def test_get_api_key_by_key(test_db_engine):
    """Test get API key by key value."""
    api_key_create = APIKeyCreate(
        id="test-id-123",
        key="test-key-abc",
        name="Test API Key",
    )
    created_key = await APIKeyRepository.create_api_key(api_key_create)

    # Retrieve key
    api_key = await APIKeyRepository.get_api_key_by_key("test-key-abc")
    assert api_key is not None
    assert api_key.id == created_key.id
    assert api_key.key == created_key.key


@pytest.mark.unit
@pytest.mark.asyncio
async def test_get_api_key_by_key_not_found(test_db_engine):
    """Test get API key by key when key doesn't exist."""
    api_key = await APIKeyRepository.get_api_key_by_key("nonexistent-key")
    assert api_key is None


@pytest.mark.unit
@pytest.mark.asyncio
async def test_list_api_keys(test_db_engine):
    """Test list all API keys."""
    # Create multiple keys
    key1 = APIKeyCreate(id="id-1", key="key-1", name="Key 1")
    key2 = APIKeyCreate(id="id-2", key="key-2", name="Key 2")
    await APIKeyRepository.create_api_key(key1)
    await APIKeyRepository.create_api_key(key2)

    # List keys
    api_keys = await APIKeyRepository.list_api_keys()
    assert len(api_keys) == 2
    assert {key.name for key in api_keys} == {"Key 1", "Key 2"}


@pytest.mark.unit
@pytest.mark.asyncio
async def test_list_api_keys_ordered_by_created_at_desc(test_db_engine):
    """Test that list_api_keys returns keys ordered by created_at descending."""
    # Create keys in sequence
    key1 = APIKeyCreate(id="id-1", key="key-1", name="Key 1")
    key2 = APIKeyCreate(id="id-2", key="key-2", name="Key 2")
    created_key1 = await APIKeyRepository.create_api_key(key1)
    created_key2 = await APIKeyRepository.create_api_key(key2)

    # List keys (should be ordered by created_at DESC)
    api_keys = await APIKeyRepository.list_api_keys()

    # Most recent (key2) should be first
    assert api_keys[0].id == created_key2.id
    assert api_keys[1].id == created_key1.id


@pytest.mark.unit
@pytest.mark.asyncio
async def test_delete_api_key(test_db_engine):
    """Test API key deletion."""
    api_key_create = APIKeyCreate(id="id-1", key="key-1", name="Key 1")
    await APIKeyRepository.create_api_key(api_key_create)

    # Delete key
    deleted = await APIKeyRepository.delete_api_key("key-1")
    assert deleted is True

    # Verify key is gone
    api_key = await APIKeyRepository.get_api_key_by_key("key-1")
    assert api_key is None


@pytest.mark.unit
@pytest.mark.asyncio
async def test_delete_api_key_not_found(test_db_engine):
    """Test deleting non-existent API key."""
    deleted = await APIKeyRepository.delete_api_key("nonexistent-key")
    assert deleted is False
```

### Integration Tests

**File**: `tests/integration/features/api_keys/test_api_key_endpoints.py`

```python
"""Integration tests for API key endpoints."""

import pytest
from httpx import AsyncClient

from app.database.api_keys.repository import APIKeyRepository


@pytest.mark.integration
@pytest.mark.asyncio
async def test_create_api_key(authenticated_client: AsyncClient, test_db_engine):
    """Test POST /api_keys endpoint."""
    response = await authenticated_client.post(
        "/api_keys",
        json={"name": "Test API Key"},
    )
    assert response.status_code == 201

    data = response.json()
    assert "id" in data
    assert "key" in data
    assert data["name"] == "Test API Key"
    assert len(data["id"]) == 21  # nanoid default length
    assert len(data["key"]) == 64  # 64 alphanumeric characters


@pytest.mark.integration
@pytest.mark.asyncio
async def test_create_api_key_unauthenticated(test_client: AsyncClient, test_db_engine):
    """Test POST /api_keys requires authentication."""
    response = await test_client.post(
        "/api_keys",
        json={"name": "Test API Key"},
    )
    assert response.status_code == 401


@pytest.mark.integration
@pytest.mark.asyncio
async def test_list_api_keys(authenticated_client: AsyncClient, test_db_engine):
    """Test GET /api_keys endpoint."""
    # Create some API keys
    await authenticated_client.post("/api_keys", json={"name": "Key 1"})
    await authenticated_client.post("/api_keys", json={"name": "Key 2"})

    # List keys
    response = await authenticated_client.get("/api_keys")
    assert response.status_code == 200

    data = response.json()
    assert len(data) == 2
    assert {key["name"] for key in data} == {"Key 1", "Key 2"}


@pytest.mark.integration
@pytest.mark.asyncio
async def test_list_api_keys_empty(authenticated_client: AsyncClient, test_db_engine):
    """Test GET /api_keys returns empty list when no keys exist."""
    response = await authenticated_client.get("/api_keys")
    assert response.status_code == 200
    assert response.json() == []


@pytest.mark.integration
@pytest.mark.asyncio
async def test_delete_api_key(authenticated_client: AsyncClient, test_db_engine):
    """Test DELETE /api_keys/{key} endpoint."""
    # Create API key
    create_response = await authenticated_client.post(
        "/api_keys",
        json={"name": "Key to Delete"},
    )
    api_key_value = create_response.json()["key"]

    # Delete key
    response = await authenticated_client.delete(f"/api_keys/{api_key_value}")
    assert response.status_code == 204

    # Verify key is deleted
    api_key = await APIKeyRepository.get_api_key_by_key(api_key_value)
    assert api_key is None


@pytest.mark.integration
@pytest.mark.asyncio
async def test_delete_api_key_not_found(authenticated_client: AsyncClient, test_db_engine):
    """Test DELETE /api_keys/{key} with non-existent key."""
    response = await authenticated_client.delete("/api_keys/nonexistent-key")
    assert response.status_code == 404
```

**File**: `tests/conftest.py` (add authenticated_client fixture)

```python
@pytest.fixture
async def authenticated_client(test_client: AsyncClient, test_db_engine) -> AsyncClient:
    """
    Fixture providing an authenticated test client.

    Creates a user, signs in, and returns a client with session cookie.
    """
    # Create first user
    await test_client.post(
        "/users/create-first-user",
        json={"email": "test@example.com", "password": "password123"},
    )

    # Sign in
    sign_in_response = await test_client.post(
        "/sign-in",
        json={"email": "test@example.com", "password": "password123"},
    )

    # Extract session cookie
    session_cookie = sign_in_response.cookies["session"]

    # Set cookie on client
    test_client.cookies.set("session", session_cookie)

    return test_client
```

## Dependencies

Add to `pyproject.toml`:

```toml
[project]
dependencies = [
    # ... existing dependencies ...
    "nanoid>=2.0.0",  # For generating cryptographically secure IDs/keys (matches Go implementation)
]
```

## Migration Script (Alembic)

**File**: `migrations/versions/002_create_api_keys_table.py`

```python
"""Create api_keys table

Revision ID: 002
Revises: 001
Create Date: 2025-01-XX
"""

from typing import Sequence, Union

from alembic import op
import sqlalchemy as sa

# revision identifiers, used by Alembic.
revision: str = "002"
down_revision: Union[str, None] = "001"
branch_labels: Union[str, Sequence[str], None] = None
depends_on: Union[str, Sequence[str], None] = None


def upgrade() -> None:
    # Create api_keys table
    op.create_table(
        "api_keys",
        sa.Column("id", sa.String(length=255), nullable=False),
        sa.Column("key", sa.String(length=255), nullable=False),
        sa.Column("name", sa.String(length=255), nullable=False),
        sa.Column(
            "created_at",
            sa.DateTime(),
            server_default=sa.text("CURRENT_TIMESTAMP"),
            nullable=False,
        ),
        sa.PrimaryKeyConstraint("id"),
        sa.UniqueConstraint("key"),
    )


def downgrade() -> None:
    op.drop_table("api_keys")
```

## Phase Completion Criteria

- [ ] All files implemented and reviewed
- [ ] API key table created in database (via Alembic migration)
- [ ] Key generation works (ID: 21 chars, Key: 64 alphanumeric)
- [ ] API key endpoints work (create, list, delete)
- [ ] Auth protection works (all endpoints require authentication)
- [ ] Unit tests pass (80%+ coverage)
- [ ] Integration tests pass (all CRUD operations verified)
- [ ] Generated keys are cryptographically secure
- [ ] Duplicate key handling works correctly
- [ ] Manual testing completed with frontend

## Notes

1. **Key Generation**: Using Python `nanoid` package to match Go's `go-nanoid` implementation. Both use cryptographically secure random generation.

2. **Key Format**:
   - **ID**: 21 characters, default nanoid alphabet
   - **Key**: 64 characters, alphanumeric only (0-9, a-z, A-Z)

3. **Auth Protection**: All endpoints require authentication via `CurrentUserEmail` dependency (from Phase 3).

4. **Delete by Key**: Unlike users (delete by ID), API keys are deleted by their key value. This matches the Go implementation and makes sense for API key management UX.

5. **Empty List Handling**: Returns `[]` instead of `null` when no keys exist, matching Go implementation.

6. **Defense in Depth**: Following AGENTS.md principle - auth checked at router level (dependency), could also add checks in service layer for sensitive operations.

7. **gRPC Integration**: API keys created here will be validated by the gRPC InternalAuthService (Phase 4) when the ingestion-service calls ValidateApiKey.

8. **No Update Endpoint**: Matching Go implementation - API keys cannot be updated, only created and deleted. This is a security best practice.

## Next Phase

Phase 6 will implement DuckDB integration for analytics queries on telemetry data.
