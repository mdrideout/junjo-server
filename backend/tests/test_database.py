"""Tests for database repositories."""

import pytest

from app.db_sqlite.users.repository import UserRepository


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
