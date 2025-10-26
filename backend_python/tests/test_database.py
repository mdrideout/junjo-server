"""Tests for database repositories."""

import pytest
from datetime import datetime, timedelta

from app.database.users.repository import UserRepository
from app.database.sessions.repository import SessionRepository


@pytest.mark.asyncio
async def test_create_user():
    """Test user creation."""
    # Test
    user = await UserRepository.create(
        email="test@example.com",
        password_hash="hashed_password"
    )

    assert user.email == "test@example.com"
    assert user.is_active is True
    assert user.id is not None


@pytest.mark.asyncio
async def test_get_user_by_email():
    """Test getting user by email."""
    # Create user
    await UserRepository.create(
        email="test2@example.com",
        password_hash="hashed_password"
    )

    # Get user
    user = await UserRepository.get_by_email("test2@example.com")

    assert user is not None
    assert user.email == "test2@example.com"
    assert user.password_hash == "hashed_password"


@pytest.mark.asyncio
async def test_create_session():
    """Test session creation."""
    # Create user first
    user = await UserRepository.create(
        email="test3@example.com",
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
async def test_delete_expired_sessions():
    """Test cleaning up expired sessions."""
    # Create user
    user = await UserRepository.create(
        email="test4@example.com",
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
