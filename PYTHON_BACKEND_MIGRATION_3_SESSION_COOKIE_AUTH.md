# Phase 3: Session/Cookie Authentication

## Overview

This phase implements the authentication system using session cookies, mirroring the current Go implementation. The auth system uses:
- **Session Storage**: Cookie-based sessions signed with a secret key
- **Password Hashing**: bcrypt for secure password storage
- **Session Middleware**: FastAPI dependency injection for route protection
- **User Model**: SQLAlchemy model with email/password authentication

## Current Go Implementation Analysis

**Session Configuration** (from `/backend/main.go:218`):
```go
sessionSecret := os.Getenv("JUNJO_SESSION_SECRET")
e.Use(session.Middleware(sessions.NewCookieStore([]byte(sessionSecret))))
```

**Session Options** (from `/backend/auth/services.go:183-199`):
- Path: "/"
- MaxAge: 86400 * 30 (30 days)
- HttpOnly: true
- Secure: true (HTTPS in production)
- SameSite: SameSiteStrictMode
- Domain: Production domain with subdomain support ("." + domain)

**Session Data**:
- Stores only `userEmail` string

**Auth Middleware** (from `/backend/middleware/auth.go:15-37`):
- Skip routes: `/ping`, `/sign-in`, `/csrf`, `/users/create-first-user`, `/users/db-has-users`, `/.well-known/jwks.json`
- Validates session presence
- Extracts userEmail from session
- Sets userEmail in request context

**User Schema** (from `/backend/db/schema.sql:8-14`):
```sql
CREATE TABLE users (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  email TEXT NOT NULL UNIQUE,
  password_hash TEXT NOT NULL,
  created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
);
CREATE INDEX idx_users_email ON users (email);
```

**Endpoints**:
- POST `/sign-in` - Create session after credential validation
- POST `/sign-out` - Destroy session
- GET `/auth-test` - Verify session validity
- GET `/csrf` - Get CSRF token (note: FastAPI doesn't need this by default)
- POST `/users/create-first-user` - Create initial user (no auth required)
- GET `/users/db-has-users` - Check if any users exist (no auth required)
- POST `/users` - Create user (auth required)
- GET `/users` - List users (auth required)
- DELETE `/users/:id` - Delete user (auth required)

## Python Implementation

### Directory Structure

```
python_backend/
└── app/
    ├── database/
    │   └── users/
    │       ├── __init__.py
    │       ├── models.py      # UserTable SQLAlchemy model
    │       ├── repository.py  # User database operations
    │       └── schemas.py     # Pydantic schemas
    ├── features/
    │   └── auth/
    │       ├── __init__.py
    │       ├── router.py      # FastAPI router with endpoints
    │       ├── service.py     # Business logic
    │       ├── dependencies.py # Auth dependencies
    │       └── utils.py       # Password hashing, session utils
    └── tests/
        ├── unit/
        │   └── features/
        │       └── auth/
        │           ├── test_password_hashing.py
        │           ├── test_user_repository.py
        │           └── test_auth_service.py
        └── integration/
            └── features/
                └── auth/
                    ├── test_auth_endpoints.py
                    └── test_auth_middleware.py
```

### 1. User Database Model

**File**: `app/database/users/models.py`

```python
"""
User database model.
"""

from datetime import datetime, timezone
from typing import Annotated

from sqlalchemy import Index, String, func
from sqlalchemy.orm import Mapped, mapped_column

from app.database.db_config import Base

# Type aliases for common column types
intpk = Annotated[int, mapped_column(primary_key=True)]
timestamp = Annotated[
    datetime,
    mapped_column(nullable=False, server_default=func.current_timestamp()),
]


class UserTable(Base):
    """
    User model for authentication.

    Mirrors the Go schema:
    CREATE TABLE users (
      id INTEGER PRIMARY KEY AUTOINCREMENT,
      email TEXT NOT NULL UNIQUE,
      password_hash TEXT NOT NULL,
      created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
    );
    """

    __tablename__ = "users"

    id: Mapped[intpk]
    email: Mapped[str] = mapped_column(String(255), unique=True, nullable=False)
    password_hash: Mapped[str] = mapped_column(String(255), nullable=False)
    created_at: Mapped[timestamp]

    __table_args__ = (Index("idx_users_email", "email"),)

    def __repr__(self) -> str:
        return f"<UserTable(id={self.id}, email={self.email})>"
```

### 2. User Repository

**File**: `app/database/users/repository.py`

```python
"""
User repository for database operations.

Following the high-concurrency pattern from wt_api_v2:
- Static methods (no instance state)
- Fresh session per operation
- expire_on_commit=False for asyncio safety
- Pydantic validation before session closes
"""

from typing import List, Optional

from sqlalchemy import func, select
from sqlalchemy.ext.asyncio import AsyncSession

from app.database.db_config import get_async_session
from app.database.users.models import UserTable
from app.database.users.schemas import User, UserCreate


class UserRepository:
    """Repository for user database operations."""

    @staticmethod
    async def count_users() -> int:
        """
        Count total number of users.

        Returns:
            Total number of users in database
        """
        async with get_async_session() as session:
            result = await session.execute(select(func.count()).select_from(UserTable))
            count = result.scalar_one()
            return count

    @staticmethod
    async def db_has_users() -> bool:
        """
        Check if any users exist.

        Returns:
            True if users exist, False otherwise
        """
        count = await UserRepository.count_users()
        return count > 0

    @staticmethod
    async def create_user(user_create: UserCreate) -> User:
        """
        Create a new user.

        Args:
            user_create: User creation data with email and password_hash

        Returns:
            Created user (without password_hash)

        Raises:
            IntegrityError: If email already exists (UNIQUE constraint)
        """
        async with get_async_session() as session:
            user = UserTable(
                email=user_create.email,
                password_hash=user_create.password_hash,
            )
            session.add(user)
            await session.commit()
            await session.refresh(user)

            # Validate with Pydantic before session closes
            return User.model_validate(user)

    @staticmethod
    async def get_user_by_email(email: str) -> Optional[User]:
        """
        Get user by email.

        Args:
            email: User email address

        Returns:
            User if found, None otherwise
        """
        async with get_async_session() as session:
            result = await session.execute(
                select(UserTable).where(UserTable.email == email)
            )
            user = result.scalar_one_or_none()

            if user is None:
                return None

            # Validate with Pydantic before session closes
            return User.model_validate(user)

    @staticmethod
    async def get_user_by_email_with_password(email: str) -> Optional[UserTable]:
        """
        Get user by email, including password_hash for authentication.

        This is the only method that returns the full UserTable object
        with password_hash. Used only for credential validation.

        Args:
            email: User email address

        Returns:
            UserTable if found (with password_hash), None otherwise
        """
        async with get_async_session() as session:
            result = await session.execute(
                select(UserTable).where(UserTable.email == email)
            )
            user = result.scalar_one_or_none()

            # Return the raw model (not Pydantic validated)
            # This allows access to password_hash
            return user

    @staticmethod
    async def list_users() -> List[User]:
        """
        List all users.

        Returns:
            List of all users (without password_hash)
        """
        async with get_async_session() as session:
            result = await session.execute(select(UserTable))
            users = result.scalars().all()

            # Validate with Pydantic before session closes
            return [User.model_validate(user) for user in users]

    @staticmethod
    async def delete_user(user_id: int) -> bool:
        """
        Delete a user by ID.

        Args:
            user_id: User ID to delete

        Returns:
            True if user was deleted, False if user not found
        """
        async with get_async_session() as session:
            result = await session.execute(
                select(UserTable).where(UserTable.id == user_id)
            )
            user = result.scalar_one_or_none()

            if user is None:
                return False

            await session.delete(user)
            await session.commit()
            return True
```

### 3. User Schemas

**File**: `app/database/users/schemas.py`

```python
"""
Pydantic schemas for User model.
"""

from datetime import datetime

from pydantic import BaseModel, ConfigDict, EmailStr


class UserCreate(BaseModel):
    """Schema for creating a user (internal use with hashed password)."""

    email: EmailStr
    password_hash: str

    model_config = ConfigDict(from_attributes=True)


class User(BaseModel):
    """
    User schema (public - no password_hash).

    This is returned from all endpoints and repository methods
    (except get_user_by_email_with_password).
    """

    id: int
    email: EmailStr
    created_at: datetime

    model_config = ConfigDict(from_attributes=True)


class SignInRequest(BaseModel):
    """Schema for sign-in request."""

    email: EmailStr
    password: str


class CreateUserRequest(BaseModel):
    """Schema for user creation request (API endpoint)."""

    email: EmailStr
    password: str


class UserResponse(BaseModel):
    """Response after successful user operation."""

    message: str


class DbHasUsersResponse(BaseModel):
    """Response for db-has-users check."""

    users_exist: bool


class AuthTestResponse(BaseModel):
    """Response for auth-test endpoint."""

    user_email: EmailStr
```

### 4. Password Hashing & Session Utilities

**File**: `app/features/auth/utils.py`

```python
"""
Authentication utilities for password hashing and session management.
"""

import secrets
from typing import Optional

from itsdangerous import BadSignature, URLSafeTimedSerializer
from passlib.context import CryptContext

from app.core.settings import settings

# Password hashing context (bcrypt with default cost, matching Go implementation)
pwd_context = CryptContext(schemes=["bcrypt"], deprecated="auto")

# Session serializer for cookie signing
# Uses itsdangerous to sign/verify session cookies
session_serializer = URLSafeTimedSerializer(
    settings.session_secret,
    salt="session-cookie",
)


def hash_password(password: str) -> str:
    """
    Hash a password using bcrypt.

    Matches the Go implementation: bcrypt.GenerateFromPassword with DefaultCost.

    Args:
        password: Plain text password

    Returns:
        Hashed password string
    """
    return pwd_context.hash(password)


def verify_password(plain_password: str, hashed_password: str) -> bool:
    """
    Verify a plain password against a hashed password.

    Matches the Go implementation: bcrypt.CompareHashAndPassword.

    Args:
        plain_password: Plain text password from user
        hashed_password: Hashed password from database

    Returns:
        True if password matches, False otherwise
    """
    return pwd_context.verify(plain_password, hashed_password)


def create_session_cookie(user_email: str) -> str:
    """
    Create a signed session cookie value.

    Args:
        user_email: User email to store in session

    Returns:
        Signed cookie value (URL-safe string)
    """
    # Add some entropy to prevent timing attacks
    session_data = {"userEmail": user_email, "nonce": secrets.token_hex(16)}
    return session_serializer.dumps(session_data)


def verify_session_cookie(cookie_value: str, max_age: int = 86400 * 30) -> Optional[str]:
    """
    Verify and decode a session cookie.

    Args:
        cookie_value: Signed cookie value from request
        max_age: Maximum age in seconds (default: 30 days)

    Returns:
        User email if valid, None if invalid or expired
    """
    try:
        session_data = session_serializer.loads(cookie_value, max_age=max_age)
        return session_data.get("userEmail")
    except (BadSignature, KeyError):
        return None
```

### 5. Authentication Dependencies

**File**: `app/features/auth/dependencies.py`

```python
"""
FastAPI dependencies for authentication.
"""

from typing import Annotated, Optional

from fastapi import Cookie, Depends, HTTPException, status

from app.features.auth.utils import verify_session_cookie


async def get_current_user_email(
    session: Annotated[Optional[str], Cookie()] = None,
) -> str:
    """
    Dependency to get current user email from session cookie.

    Mirrors the Go middleware auth check:
    - Reads session cookie
    - Verifies signature and expiration
    - Returns user email

    Args:
        session: Session cookie value (automatically extracted by FastAPI)

    Returns:
        User email from session

    Raises:
        HTTPException: 401 if session is invalid or missing
    """
    if session is None:
        raise HTTPException(
            status_code=status.HTTP_401_UNAUTHORIZED,
            detail="Unauthorized: No valid session",
        )

    user_email = verify_session_cookie(session)
    if user_email is None:
        raise HTTPException(
            status_code=status.HTTP_401_UNAUTHORIZED,
            detail="Unauthorized: Invalid or expired session",
        )

    return user_email


# Type alias for dependency injection
CurrentUserEmail = Annotated[str, Depends(get_current_user_email)]
```

### 6. Authentication Service

**File**: `app/features/auth/service.py`

```python
"""
Authentication service layer.

Contains business logic for authentication operations.
"""

from typing import Optional

from sqlalchemy.exc import IntegrityError

from app.database.users.repository import UserRepository
from app.database.users.schemas import User, UserCreate
from app.features.auth.utils import hash_password, verify_password


class AuthService:
    """Service for authentication operations."""

    @staticmethod
    async def db_has_users() -> bool:
        """Check if any users exist."""
        return await UserRepository.db_has_users()

    @staticmethod
    async def create_first_user(email: str, password: str) -> User:
        """
        Create the first user (only allowed if no users exist).

        Args:
            email: User email
            password: Plain text password

        Returns:
            Created user

        Raises:
            ValueError: If users already exist
            IntegrityError: If email already exists
        """
        # Check if any users exist
        if await AuthService.db_has_users():
            raise ValueError("Users already exist, cannot create first user")

        # Hash password and create user
        hashed_password = hash_password(password)
        user_create = UserCreate(email=email, password_hash=hashed_password)
        return await UserRepository.create_user(user_create)

    @staticmethod
    async def create_user(email: str, password: str) -> User:
        """
        Create a new user.

        Args:
            email: User email
            password: Plain text password

        Returns:
            Created user

        Raises:
            IntegrityError: If email already exists
        """
        hashed_password = hash_password(password)
        user_create = UserCreate(email=email, password_hash=hashed_password)
        return await UserRepository.create_user(user_create)

    @staticmethod
    async def validate_credentials(email: str, password: str) -> Optional[User]:
        """
        Validate user credentials.

        Args:
            email: User email
            password: Plain text password

        Returns:
            User if credentials are valid, None otherwise
        """
        # Get user with password hash
        user_with_password = await UserRepository.get_user_by_email_with_password(email)
        if user_with_password is None:
            return None

        # Verify password
        if not verify_password(password, user_with_password.password_hash):
            return None

        # Return user without password hash
        return await UserRepository.get_user_by_email(email)

    @staticmethod
    async def list_users() -> list[User]:
        """List all users."""
        return await UserRepository.list_users()

    @staticmethod
    async def delete_user(user_id: int) -> bool:
        """
        Delete a user.

        Args:
            user_id: User ID to delete

        Returns:
            True if user was deleted, False if not found
        """
        return await UserRepository.delete_user(user_id)
```

### 7. Authentication Router

**File**: `app/features/auth/router.py`

```python
"""
Authentication router.

Implements all authentication endpoints, mirroring the Go implementation.
"""

import os
from typing import List

from fastapi import APIRouter, HTTPException, Response, status
from sqlalchemy.exc import IntegrityError

from app.core.logger import logger
from app.database.users.schemas import (
    AuthTestResponse,
    CreateUserRequest,
    DbHasUsersResponse,
    SignInRequest,
    User,
    UserResponse,
)
from app.features.auth.dependencies import CurrentUserEmail
from app.features.auth.service import AuthService
from app.features.auth.utils import create_session_cookie

router = APIRouter()


# --- Session Configuration (mirrors Go implementation) ---
SESSION_COOKIE_NAME = "session"
SESSION_MAX_AGE = 86400 * 30  # 30 days
SESSION_PATH = "/"
SESSION_HTTPONLY = True
SESSION_SECURE = True  # HTTPS in production
SESSION_SAMESITE = "strict"

# Production domain support (mirrors Go implementation)
JUNJO_ENV = os.getenv("JUNJO_ENV", "development")
JUNJO_PROD_AUTH_DOMAIN = os.getenv("JUNJO_PROD_AUTH_DOMAIN", "")


def get_session_domain() -> str | None:
    """
    Get session cookie domain based on environment.

    Mirrors Go logic:
    if os.Getenv("JUNJO_ENV") == "production" {
        options.Domain = "." + authDomain  // covers subdomains
    }
    """
    if JUNJO_ENV == "production" and JUNJO_PROD_AUTH_DOMAIN:
        domain = f".{JUNJO_PROD_AUTH_DOMAIN}"
        logger.info(f"Setting production auth domain: {domain}")
        return domain
    return None


# --- Public Endpoints (no auth required) ---


@router.get("/users/db-has-users", response_model=DbHasUsersResponse)
async def db_has_users():
    """Check if any users exist."""
    exists = await AuthService.db_has_users()
    return DbHasUsersResponse(users_exist=exists)


@router.post("/users/create-first-user", response_model=UserResponse)
async def create_first_user(request: CreateUserRequest):
    """
    Create the first user (only allowed if no users exist).

    Mirrors Go implementation: only works if database has no users.
    """
    try:
        await AuthService.create_first_user(request.email, request.password)
        return UserResponse(message="First user created successfully")
    except ValueError as e:
        raise HTTPException(
            status_code=status.HTTP_400_BAD_REQUEST,
            detail=str(e),
        )
    except IntegrityError:
        raise HTTPException(
            status_code=status.HTTP_409_CONFLICT,
            detail="A user with this email already exists",
        )


@router.post("/sign-in", response_model=UserResponse)
async def sign_in(request: SignInRequest, response: Response):
    """
    Sign in with email and password.

    Creates a session cookie on successful authentication.
    Mirrors Go implementation in /backend/auth/services.go:155-214.
    """
    logger.info(f"Sign-in request for email: {request.email}")

    # Validate credentials
    user = await AuthService.validate_credentials(request.email, request.password)
    if user is None:
        logger.warning(f"Failed to validate credentials for: {request.email}")
        raise HTTPException(
            status_code=status.HTTP_401_UNAUTHORIZED,
            detail="Invalid credentials",
        )

    # Create signed session cookie
    session_cookie_value = create_session_cookie(user.email)

    # Set session cookie with configuration matching Go implementation
    response.set_cookie(
        key=SESSION_COOKIE_NAME,
        value=session_cookie_value,
        max_age=SESSION_MAX_AGE,
        path=SESSION_PATH,
        domain=get_session_domain(),
        httponly=SESSION_HTTPONLY,
        secure=SESSION_SECURE,
        samesite=SESSION_SAMESITE,
    )

    return UserResponse(message="signed in")


@router.post("/sign-out", response_model=UserResponse)
async def sign_out(response: Response):
    """
    Sign out by destroying the session cookie.

    Mirrors Go implementation: sets MaxAge to -1.
    """
    # Delete the session cookie (MaxAge = -1)
    response.delete_cookie(
        key=SESSION_COOKIE_NAME,
        path=SESSION_PATH,
        domain=get_session_domain(),
    )

    return UserResponse(message="signed out")


@router.get("/auth-test", response_model=AuthTestResponse)
async def auth_test(current_user_email: CurrentUserEmail):
    """
    Test authentication by returning the current user email.

    Protected by auth dependency - will return 401 if not authenticated.
    """
    return AuthTestResponse(user_email=current_user_email)


# --- Protected Endpoints (auth required via dependency) ---


@router.post("/users", response_model=UserResponse)
async def create_user(request: CreateUserRequest, current_user_email: CurrentUserEmail):
    """
    Create a new user (auth required).

    Uses CurrentUserEmail dependency for auth check.
    """
    try:
        await AuthService.create_user(request.email, request.password)
        return UserResponse(message="User created successfully")
    except IntegrityError:
        raise HTTPException(
            status_code=status.HTTP_409_CONFLICT,
            detail="A user with this email already exists",
        )


@router.get("/users", response_model=List[User])
async def list_users(current_user_email: CurrentUserEmail):
    """
    List all users (auth required).

    Uses CurrentUserEmail dependency for auth check.
    """
    users = await AuthService.list_users()
    return users


@router.delete("/users/{user_id}", response_model=UserResponse)
async def delete_user(user_id: int, current_user_email: CurrentUserEmail):
    """
    Delete a user (auth required).

    Uses CurrentUserEmail dependency for auth check.
    """
    deleted = await AuthService.delete_user(user_id)
    if not deleted:
        raise HTTPException(
            status_code=status.HTTP_404_NOT_FOUND,
            detail="User not found",
        )
    return UserResponse(message="User deleted successfully")
```

### 8. Update `main.py`

**File**: `app/main.py` (add auth router)

```python
# Add import
from app.features.auth.router import router as auth_router

# In create_app() function, add auth router:
app.include_router(auth_router, tags=["auth"])
```

### 9. Update Settings

**File**: `app/core/settings.py` (add session secret)

```python
from pydantic_settings import BaseSettings, SettingsConfigDict

class Settings(BaseSettings):
    # ... existing settings ...

    # Session secret for cookie signing
    session_secret: str  # REQUIRED: set JUNJO_SESSION_SECRET in .env

    # Environment for production domain support
    junjo_env: str = "development"
    junjo_prod_auth_domain: str = ""

    model_config = SettingsConfigDict(
        env_file=".env",
        env_file_encoding="utf-8",
        case_sensitive=False,
    )
```

## Testing Requirements

### Unit Tests

**File**: `tests/unit/features/auth/test_password_hashing.py`

```python
"""Unit tests for password hashing."""

import pytest

from app.features.auth.utils import hash_password, verify_password


@pytest.mark.unit
def test_hash_password():
    """Test password hashing."""
    password = "test_password_123"
    hashed = hash_password(password)

    # Should return a non-empty string
    assert isinstance(hashed, str)
    assert len(hashed) > 0
    assert hashed != password


@pytest.mark.unit
def test_verify_password_correct():
    """Test password verification with correct password."""
    password = "test_password_123"
    hashed = hash_password(password)

    # Should verify correctly
    assert verify_password(password, hashed) is True


@pytest.mark.unit
def test_verify_password_incorrect():
    """Test password verification with incorrect password."""
    password = "test_password_123"
    wrong_password = "wrong_password"
    hashed = hash_password(password)

    # Should fail verification
    assert verify_password(wrong_password, hashed) is False


@pytest.mark.unit
def test_hash_password_different_each_time():
    """Test that hashing the same password twice produces different hashes (due to salt)."""
    password = "test_password_123"
    hash1 = hash_password(password)
    hash2 = hash_password(password)

    # Should be different due to random salt
    assert hash1 != hash2

    # But both should verify correctly
    assert verify_password(password, hash1) is True
    assert verify_password(password, hash2) is True
```

**File**: `tests/unit/features/auth/test_session_utils.py`

```python
"""Unit tests for session utilities."""

import pytest

from app.features.auth.utils import create_session_cookie, verify_session_cookie


@pytest.mark.unit
def test_create_session_cookie():
    """Test session cookie creation."""
    user_email = "test@example.com"
    cookie = create_session_cookie(user_email)

    # Should return a non-empty string
    assert isinstance(cookie, str)
    assert len(cookie) > 0


@pytest.mark.unit
def test_verify_session_cookie_valid():
    """Test session cookie verification with valid cookie."""
    user_email = "test@example.com"
    cookie = create_session_cookie(user_email)

    # Should verify correctly
    verified_email = verify_session_cookie(cookie)
    assert verified_email == user_email


@pytest.mark.unit
def test_verify_session_cookie_invalid():
    """Test session cookie verification with invalid cookie."""
    invalid_cookie = "invalid_cookie_value"

    # Should return None for invalid cookie
    verified_email = verify_session_cookie(invalid_cookie)
    assert verified_email is None


@pytest.mark.unit
def test_verify_session_cookie_expired():
    """Test session cookie verification with expired cookie."""
    user_email = "test@example.com"
    cookie = create_session_cookie(user_email)

    # Verify with max_age=0 (immediately expired)
    verified_email = verify_session_cookie(cookie, max_age=0)
    assert verified_email is None
```

**File**: `tests/unit/features/auth/test_user_repository.py`

```python
"""Unit tests for user repository."""

import pytest
from sqlalchemy.exc import IntegrityError

from app.database.users.repository import UserRepository
from app.database.users.schemas import UserCreate


@pytest.mark.unit
@pytest.mark.asyncio
async def test_create_user(test_db_engine):
    """Test user creation."""
    user_create = UserCreate(email="test@example.com", password_hash="hashed_password")
    user = await UserRepository.create_user(user_create)

    assert user.id > 0
    assert user.email == "test@example.com"
    assert user.created_at is not None


@pytest.mark.unit
@pytest.mark.asyncio
async def test_create_user_duplicate_email(test_db_engine):
    """Test user creation with duplicate email raises IntegrityError."""
    user_create = UserCreate(email="test@example.com", password_hash="hashed_password")
    await UserRepository.create_user(user_create)

    # Try to create another user with same email
    with pytest.raises(IntegrityError):
        await UserRepository.create_user(user_create)


@pytest.mark.unit
@pytest.mark.asyncio
async def test_get_user_by_email(test_db_engine):
    """Test get user by email."""
    user_create = UserCreate(email="test@example.com", password_hash="hashed_password")
    created_user = await UserRepository.create_user(user_create)

    # Retrieve user
    user = await UserRepository.get_user_by_email("test@example.com")
    assert user is not None
    assert user.id == created_user.id
    assert user.email == created_user.email


@pytest.mark.unit
@pytest.mark.asyncio
async def test_get_user_by_email_not_found(test_db_engine):
    """Test get user by email when user doesn't exist."""
    user = await UserRepository.get_user_by_email("nonexistent@example.com")
    assert user is None


@pytest.mark.unit
@pytest.mark.asyncio
async def test_db_has_users(test_db_engine):
    """Test db_has_users check."""
    # Initially no users
    assert await UserRepository.db_has_users() is False

    # Create a user
    user_create = UserCreate(email="test@example.com", password_hash="hashed_password")
    await UserRepository.create_user(user_create)

    # Now should have users
    assert await UserRepository.db_has_users() is True


@pytest.mark.unit
@pytest.mark.asyncio
async def test_list_users(test_db_engine):
    """Test list all users."""
    # Create multiple users
    user1 = UserCreate(email="user1@example.com", password_hash="hash1")
    user2 = UserCreate(email="user2@example.com", password_hash="hash2")
    await UserRepository.create_user(user1)
    await UserRepository.create_user(user2)

    # List users
    users = await UserRepository.list_users()
    assert len(users) == 2
    assert {user.email for user in users} == {"user1@example.com", "user2@example.com"}


@pytest.mark.unit
@pytest.mark.asyncio
async def test_delete_user(test_db_engine):
    """Test user deletion."""
    user_create = UserCreate(email="test@example.com", password_hash="hashed_password")
    user = await UserRepository.create_user(user_create)

    # Delete user
    deleted = await UserRepository.delete_user(user.id)
    assert deleted is True

    # Verify user is gone
    user = await UserRepository.get_user_by_email("test@example.com")
    assert user is None


@pytest.mark.unit
@pytest.mark.asyncio
async def test_delete_user_not_found(test_db_engine):
    """Test deleting non-existent user."""
    deleted = await UserRepository.delete_user(999999)
    assert deleted is False
```

### Integration Tests

**File**: `tests/integration/features/auth/test_auth_endpoints.py`

```python
"""Integration tests for authentication endpoints."""

import pytest
from httpx import AsyncClient

from app.database.users.repository import UserRepository
from app.database.users.schemas import UserCreate


@pytest.mark.integration
@pytest.mark.asyncio
async def test_db_has_users_empty(test_client: AsyncClient, test_db_engine):
    """Test /users/db-has-users when no users exist."""
    response = await test_client.get("/users/db-has-users")
    assert response.status_code == 200
    assert response.json() == {"users_exist": False}


@pytest.mark.integration
@pytest.mark.asyncio
async def test_create_first_user(test_client: AsyncClient, test_db_engine):
    """Test /users/create-first-user endpoint."""
    response = await test_client.post(
        "/users/create-first-user",
        json={"email": "first@example.com", "password": "password123"},
    )
    assert response.status_code == 200
    assert response.json() == {"message": "First user created successfully"}

    # Verify user exists
    user = await UserRepository.get_user_by_email("first@example.com")
    assert user is not None
    assert user.email == "first@example.com"


@pytest.mark.integration
@pytest.mark.asyncio
async def test_create_first_user_when_users_exist(test_client: AsyncClient, test_db_engine):
    """Test /users/create-first-user fails when users already exist."""
    # Create first user
    await test_client.post(
        "/users/create-first-user",
        json={"email": "first@example.com", "password": "password123"},
    )

    # Try to create another "first" user
    response = await test_client.post(
        "/users/create-first-user",
        json={"email": "second@example.com", "password": "password123"},
    )
    assert response.status_code == 400
    assert "already exist" in response.json()["detail"]


@pytest.mark.integration
@pytest.mark.asyncio
async def test_sign_in_success(test_client: AsyncClient, test_db_engine):
    """Test successful sign-in."""
    # Create a user first
    await test_client.post(
        "/users/create-first-user",
        json={"email": "test@example.com", "password": "password123"},
    )

    # Sign in
    response = await test_client.post(
        "/sign-in",
        json={"email": "test@example.com", "password": "password123"},
    )
    assert response.status_code == 200
    assert response.json() == {"message": "signed in"}

    # Verify session cookie was set
    assert "session" in response.cookies
    session_cookie = response.cookies["session"]
    assert len(session_cookie) > 0


@pytest.mark.integration
@pytest.mark.asyncio
async def test_sign_in_invalid_credentials(test_client: AsyncClient, test_db_engine):
    """Test sign-in with invalid credentials."""
    # Create a user first
    await test_client.post(
        "/users/create-first-user",
        json={"email": "test@example.com", "password": "password123"},
    )

    # Try to sign in with wrong password
    response = await test_client.post(
        "/sign-in",
        json={"email": "test@example.com", "password": "wrong_password"},
    )
    assert response.status_code == 401
    assert "Invalid credentials" in response.json()["detail"]


@pytest.mark.integration
@pytest.mark.asyncio
async def test_auth_test_authenticated(test_client: AsyncClient, test_db_engine):
    """Test /auth-test with valid session."""
    # Create user and sign in
    await test_client.post(
        "/users/create-first-user",
        json={"email": "test@example.com", "password": "password123"},
    )
    sign_in_response = await test_client.post(
        "/sign-in",
        json={"email": "test@example.com", "password": "password123"},
    )

    # Use session cookie to access protected endpoint
    session_cookie = sign_in_response.cookies["session"]
    response = await test_client.get(
        "/auth-test",
        cookies={"session": session_cookie},
    )
    assert response.status_code == 200
    assert response.json() == {"user_email": "test@example.com"}


@pytest.mark.integration
@pytest.mark.asyncio
async def test_auth_test_unauthenticated(test_client: AsyncClient, test_db_engine):
    """Test /auth-test without session."""
    response = await test_client.get("/auth-test")
    assert response.status_code == 401
    assert "No valid session" in response.json()["detail"]


@pytest.mark.integration
@pytest.mark.asyncio
async def test_sign_out(test_client: AsyncClient, test_db_engine):
    """Test sign-out."""
    # Create user and sign in
    await test_client.post(
        "/users/create-first-user",
        json={"email": "test@example.com", "password": "password123"},
    )
    sign_in_response = await test_client.post(
        "/sign-in",
        json={"email": "test@example.com", "password": "password123"},
    )
    session_cookie = sign_in_response.cookies["session"]

    # Sign out
    response = await test_client.post("/sign-out")
    assert response.status_code == 200
    assert response.json() == {"message": "signed out"}

    # Verify session cookie was deleted (max_age should be negative)
    # Note: FastAPI/Starlette sets max_age=0 for deleted cookies
    assert "session" in response.cookies


@pytest.mark.integration
@pytest.mark.asyncio
async def test_create_user_authenticated(test_client: AsyncClient, test_db_engine):
    """Test /users POST endpoint with authentication."""
    # Create first user and sign in
    await test_client.post(
        "/users/create-first-user",
        json={"email": "admin@example.com", "password": "password123"},
    )
    sign_in_response = await test_client.post(
        "/sign-in",
        json={"email": "admin@example.com", "password": "password123"},
    )
    session_cookie = sign_in_response.cookies["session"]

    # Create another user (authenticated)
    response = await test_client.post(
        "/users",
        json={"email": "newuser@example.com", "password": "password123"},
        cookies={"session": session_cookie},
    )
    assert response.status_code == 200
    assert response.json() == {"message": "User created successfully"}


@pytest.mark.integration
@pytest.mark.asyncio
async def test_list_users_authenticated(test_client: AsyncClient, test_db_engine):
    """Test /users GET endpoint with authentication."""
    # Create first user and sign in
    await test_client.post(
        "/users/create-first-user",
        json={"email": "test@example.com", "password": "password123"},
    )
    sign_in_response = await test_client.post(
        "/sign-in",
        json={"email": "test@example.com", "password": "password123"},
    )
    session_cookie = sign_in_response.cookies["session"]

    # List users (authenticated)
    response = await test_client.get(
        "/users",
        cookies={"session": session_cookie},
    )
    assert response.status_code == 200
    users = response.json()
    assert len(users) == 1
    assert users[0]["email"] == "test@example.com"


@pytest.mark.integration
@pytest.mark.asyncio
async def test_delete_user_authenticated(test_client: AsyncClient, test_db_engine):
    """Test /users/{id} DELETE endpoint with authentication."""
    # Create first user and sign in
    await test_client.post(
        "/users/create-first-user",
        json={"email": "admin@example.com", "password": "password123"},
    )
    sign_in_response = await test_client.post(
        "/sign-in",
        json={"email": "admin@example.com", "password": "password123"},
    )
    session_cookie = sign_in_response.cookies["session"]

    # Create another user to delete
    await test_client.post(
        "/users",
        json={"email": "todelete@example.com", "password": "password123"},
        cookies={"session": session_cookie},
    )

    # Get user ID
    user = await UserRepository.get_user_by_email("todelete@example.com")
    assert user is not None

    # Delete user (authenticated)
    response = await test_client.delete(
        f"/users/{user.id}",
        cookies={"session": session_cookie},
    )
    assert response.status_code == 200
    assert response.json() == {"message": "User deleted successfully"}

    # Verify user is deleted
    deleted_user = await UserRepository.get_user_by_email("todelete@example.com")
    assert deleted_user is None
```

## Dependencies

Add to `pyproject.toml`:

```toml
[project]
dependencies = [
    # ... existing dependencies ...
    "passlib[bcrypt]>=1.7.4",  # Password hashing with bcrypt
    "itsdangerous>=2.1.2",      # Cookie signing for sessions
]
```

## Migration Script (Alembic)

The user table migration should already exist from Phase 2. If not:

**File**: `migrations/versions/001_create_users_table.py`

```python
"""Create users table

Revision ID: 001
Revises:
Create Date: 2025-01-XX
"""

from typing import Sequence, Union

from alembic import op
import sqlalchemy as sa

# revision identifiers, used by Alembic.
revision: str = "001"
down_revision: Union[str, None] = None
branch_labels: Union[str, Sequence[str], None] = None
depends_on: Union[str, Sequence[str], None] = None


def upgrade() -> None:
    # Create users table
    op.create_table(
        "users",
        sa.Column("id", sa.Integer(), nullable=False),
        sa.Column("email", sa.String(length=255), nullable=False),
        sa.Column("password_hash", sa.String(length=255), nullable=False),
        sa.Column(
            "created_at",
            sa.DateTime(),
            server_default=sa.text("CURRENT_TIMESTAMP"),
            nullable=False,
        ),
        sa.PrimaryKeyConstraint("id"),
        sa.UniqueConstraint("email"),
    )
    op.create_index("idx_users_email", "users", ["email"], unique=False)


def downgrade() -> None:
    op.drop_index("idx_users_email", table_name="users")
    op.drop_table("users")
```

## Environment Variables

Add to `.env`:

```bash
# Session secret for cookie signing (REQUIRED)
# Generate with: python -c "import secrets; print(secrets.token_urlsafe(32))"
JUNJO_SESSION_SECRET=your-secret-key-here

# Environment (optional, defaults to "development")
JUNJO_ENV=development

# Production auth domain (optional, only used if JUNJO_ENV=production)
# Example: "junjo.io" (will be set as ".junjo.io" to cover subdomains)
JUNJO_PROD_AUTH_DOMAIN=
```

## Phase Completion Criteria

- [ ] All files implemented and reviewed
- [ ] User model created in database (via Alembic migration)
- [ ] Password hashing works with bcrypt
- [ ] Session cookies are signed and verified correctly
- [ ] All authentication endpoints work (sign-in, sign-out, auth-test)
- [ ] User CRUD endpoints work with auth protection
- [ ] First user creation flow works
- [ ] Unit tests pass (80%+ coverage)
- [ ] Integration tests pass (all auth flows verified)
- [ ] Session cookie configuration matches Go implementation
- [ ] Production domain support works correctly
- [ ] Manual testing completed with frontend

## Notes

1. **Session Storage**: Using signed cookies (itsdangerous) instead of server-side session storage. This matches the Go implementation using gorilla/sessions with cookie store.

2. **CSRF Protection**: FastAPI doesn't have built-in CSRF like Echo. Consider adding `fastapi-csrf-protect` if needed, or rely on SameSite=Strict + CORS for protection.

3. **Password Hashing**: Using passlib with bcrypt, which is compatible with Go's bcrypt implementation.

4. **Dependency Injection**: FastAPI's dependency system is more elegant than Echo's middleware. Auth is enforced at the route level via `CurrentUserEmail` dependency.

5. **Defense in Depth**: Following AGENTS.md principle - auth checked in both router (dependency) and potentially in service layer for sensitive operations.

6. **Session Expiration**: 30 days default, matching Go implementation. Automatically enforced by itsdangerous `max_age` parameter.

7. **Production Domain**: Subdomain support via "." prefix, matching Go implementation.
