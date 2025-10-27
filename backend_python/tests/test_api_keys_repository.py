"""Tests for API key repository."""

import pytest

from app.database.api_keys.repository import APIKeyRepository


@pytest.mark.unit
@pytest.mark.asyncio
async def test_create_api_key():
    """Test API key creation."""
    # Test
    api_key = await APIKeyRepository.create(
        id="test_id_123",
        key="test_key_64_chars_alphanumeric_xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx",
        name="Test Key"
    )

    assert api_key.id == "test_id_123"
    assert api_key.key == "test_key_64_chars_alphanumeric_xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx"
    assert api_key.name == "Test Key"
    assert api_key.created_at is not None


@pytest.mark.unit
@pytest.mark.asyncio
async def test_list_all_api_keys_empty():
    """Test listing API keys when database is empty."""
    # Test
    api_keys = await APIKeyRepository.list_all()

    assert api_keys == []


@pytest.mark.unit
@pytest.mark.asyncio
async def test_list_all_api_keys():
    """Test listing all API keys."""
    # Create keys
    await APIKeyRepository.create(
        id="id1",
        key="key1_xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx",
        name="Key 1"
    )
    await APIKeyRepository.create(
        id="id2",
        key="key2_xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx",
        name="Key 2"
    )

    # Test
    api_keys = await APIKeyRepository.list_all()

    assert len(api_keys) == 2
    # Should be ordered by created_at desc, so most recent first
    assert api_keys[0].name in ["Key 1", "Key 2"]
    assert api_keys[1].name in ["Key 1", "Key 2"]


@pytest.mark.unit
@pytest.mark.asyncio
async def test_get_by_id_found():
    """Test getting API key by ID when it exists."""
    # Create key
    created = await APIKeyRepository.create(
        id="test_id",
        key="test_key_xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx",
        name="Test Key"
    )

    # Test
    api_key = await APIKeyRepository.get_by_id("test_id")

    assert api_key is not None
    assert api_key.id == created.id
    assert api_key.key == created.key
    assert api_key.name == created.name


@pytest.mark.unit
@pytest.mark.asyncio
async def test_get_by_id_not_found():
    """Test getting API key by ID when it doesn't exist."""
    # Test
    api_key = await APIKeyRepository.get_by_id("nonexistent_id")

    assert api_key is None


@pytest.mark.unit
@pytest.mark.asyncio
async def test_get_by_key_found():
    """Test getting API key by key value when it exists."""
    # Create key
    created = await APIKeyRepository.create(
        id="test_id",
        key="unique_key_xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx",
        name="Test Key"
    )

    # Test
    api_key = await APIKeyRepository.get_by_key("unique_key_xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx")

    assert api_key is not None
    assert api_key.id == created.id
    assert api_key.key == created.key
    assert api_key.name == created.name


@pytest.mark.unit
@pytest.mark.asyncio
async def test_get_by_key_not_found():
    """Test getting API key by key value when it doesn't exist."""
    # Test
    api_key = await APIKeyRepository.get_by_key("nonexistent_key")

    assert api_key is None


@pytest.mark.unit
@pytest.mark.asyncio
async def test_delete_by_id_success():
    """Test deleting API key by ID when it exists."""
    # Create key
    await APIKeyRepository.create(
        id="delete_me",
        key="key_to_delete_xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx",
        name="Delete Me"
    )

    # Test delete
    result = await APIKeyRepository.delete_by_id("delete_me")

    assert result is True

    # Verify it's gone
    api_key = await APIKeyRepository.get_by_id("delete_me")
    assert api_key is None


@pytest.mark.unit
@pytest.mark.asyncio
async def test_delete_by_id_not_found():
    """Test deleting API key by ID when it doesn't exist."""
    # Test
    result = await APIKeyRepository.delete_by_id("nonexistent_id")

    assert result is False


@pytest.mark.unit
@pytest.mark.asyncio
async def test_key_uniqueness():
    """Test that duplicate keys are rejected (unique constraint)."""
    # Create first key
    await APIKeyRepository.create(
        id="id1",
        key="duplicate_key_xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx",
        name="Key 1"
    )

    # Try to create second key with same key value
    with pytest.raises(Exception):  # SQLAlchemy will raise IntegrityError
        await APIKeyRepository.create(
            id="id2",
            key="duplicate_key_xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx",
            name="Key 2"
        )
