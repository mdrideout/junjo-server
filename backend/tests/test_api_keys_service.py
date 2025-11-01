"""Tests for API key service.

Only tests business logic - library behavior is tested by the libraries themselves.
"""

import pytest

from app.features.api_keys.service import APIKeyService


@pytest.mark.unit
def test_generate_key_uniqueness():
    """Test that generated keys are unique (tests our usage pattern)."""
    keys = [APIKeyService.generate_key() for _ in range(100)]

    # All should be unique
    assert len(set(keys)) == 100


@pytest.mark.unit
def test_generate_id_uniqueness():
    """Test that generated IDs are unique (tests our usage pattern)."""
    ids = [APIKeyService.generate_id() for _ in range(100)]

    # All should be unique
    assert len(set(ids)) == 100


@pytest.mark.unit
@pytest.mark.asyncio
async def test_create_api_key():
    """Test creating API key via service."""
    # Test
    api_key = await APIKeyService.create_api_key(name="Test Key")

    # Should have generated ID and key
    assert api_key.id is not None
    assert len(api_key.id) == 21
    assert api_key.key is not None
    assert len(api_key.key) == 64
    assert api_key.name == "Test Key"
    assert api_key.created_at is not None


@pytest.mark.unit
@pytest.mark.asyncio
async def test_list_api_keys():
    """Test listing API keys via service."""
    # Create some keys
    await APIKeyService.create_api_key(name="Key 1")
    await APIKeyService.create_api_key(name="Key 2")

    # List
    api_keys = await APIKeyService.list_api_keys()

    assert len(api_keys) >= 2
    # Check names appear in list
    names = [key.name for key in api_keys]
    assert "Key 1" in names
    assert "Key 2" in names


@pytest.mark.unit
@pytest.mark.asyncio
async def test_delete_api_key():
    """Test deleting API key via service."""
    # Create key
    created = await APIKeyService.create_api_key(name="Delete Me")

    # Delete
    result = await APIKeyService.delete_api_key(created.id)

    assert result is True

    # Verify it's gone
    api_keys = await APIKeyService.list_api_keys()
    ids = [key.id for key in api_keys]
    assert created.id not in ids


@pytest.mark.unit
@pytest.mark.asyncio
async def test_delete_nonexistent_api_key():
    """Test deleting non-existent API key."""
    result = await APIKeyService.delete_api_key("nonexistent_id")

    assert result is False
