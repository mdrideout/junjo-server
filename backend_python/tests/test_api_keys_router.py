"""Integration tests for API keys router.

Tests the complete API key flow: create → list → delete.

Note: Database isolation is handled automatically by the autouse fixture in conftest.py.
Each test gets its own temporary SQLite database that is cleaned up after the test.
"""

import pytest
from httpx import ASGITransport, AsyncClient

from app.main import app


@pytest.mark.integration
@pytest.mark.asyncio
async def test_create_api_key():
    """Test POST /api_keys creates a new API key."""
    transport = ASGITransport(app=app)
    async with AsyncClient(transport=transport, base_url="http://test") as client:
        response = await client.post(
            "/api_keys",
            json={"name": "Test API Key"}
        )

        assert response.status_code == 201
        data = response.json()
        assert data["name"] == "Test API Key"
        assert "id" in data
        assert "key" in data
        assert "created_at" in data
        assert len(data["key"]) == 64  # 64-char nanoid
        assert len(data["id"]) == 21  # 21-char nanoid


@pytest.mark.integration
@pytest.mark.asyncio
async def test_list_api_keys_empty():
    """Test GET /api_keys returns empty list when no keys exist."""
    transport = ASGITransport(app=app)
    async with AsyncClient(transport=transport, base_url="http://test") as client:
        response = await client.get("/api_keys")

        assert response.status_code == 200
        assert response.json() == []


@pytest.mark.integration
@pytest.mark.asyncio
async def test_list_api_keys():
    """Test GET /api_keys returns all API keys."""
    transport = ASGITransport(app=app)
    async with AsyncClient(transport=transport, base_url="http://test") as client:
        # Create two keys
        response1 = await client.post(
            "/api_keys",
            json={"name": "Key 1"}
        )
        key1_id = response1.json()["id"]

        response2 = await client.post(
            "/api_keys",
            json={"name": "Key 2"}
        )
        key2_id = response2.json()["id"]

        # List keys
        response = await client.get("/api_keys")

        assert response.status_code == 200
        data = response.json()
        assert len(data) == 2

        # Check IDs are present
        ids = [key["id"] for key in data]
        assert key1_id in ids
        assert key2_id in ids

        # Check names are present
        names = [key["name"] for key in data]
        assert "Key 1" in names
        assert "Key 2" in names


@pytest.mark.integration
@pytest.mark.asyncio
async def test_delete_api_key():
    """Test DELETE /api_keys/{id} deletes an API key."""
    transport = ASGITransport(app=app)
    async with AsyncClient(transport=transport, base_url="http://test") as client:
        # Create a key
        create_response = await client.post(
            "/api_keys",
            json={"name": "Delete Me"}
        )
        key_id = create_response.json()["id"]

        # Delete the key
        delete_response = await client.delete(f"/api_keys/{key_id}")

        assert delete_response.status_code == 204

        # Verify it's gone
        list_response = await client.get("/api_keys")
        data = list_response.json()
        ids = [key["id"] for key in data]
        assert key_id not in ids


@pytest.mark.integration
@pytest.mark.asyncio
async def test_delete_nonexistent_api_key():
    """Test DELETE /api_keys/{id} returns 404 for non-existent key."""
    transport = ASGITransport(app=app)
    async with AsyncClient(transport=transport, base_url="http://test") as client:
        response = await client.delete("/api_keys/nonexistent_id_12345")

        assert response.status_code == 404
        assert "not found" in response.json()["detail"].lower()


@pytest.mark.integration
@pytest.mark.asyncio
async def test_api_key_fields_snake_case():
    """Test that API responses use snake_case (Python convention)."""
    transport = ASGITransport(app=app)
    async with AsyncClient(transport=transport, base_url="http://test") as client:
        response = await client.post(
            "/api_keys",
            json={"name": "Test Key"}
        )

        data = response.json()

        # Verify snake_case fields
        assert "created_at" in data
        assert "createdAt" not in data  # Should NOT be camelCase


@pytest.mark.integration
@pytest.mark.asyncio
async def test_create_multiple_keys_unique():
    """Test creating multiple keys generates unique key values."""
    transport = ASGITransport(app=app)
    async with AsyncClient(transport=transport, base_url="http://test") as client:
        # Create 10 keys
        keys = []
        for i in range(10):
            response = await client.post(
                "/api_keys",
                json={"name": f"Key {i}"}
            )
            assert response.status_code == 201
            keys.append(response.json()["key"])

        # All key values should be unique
        assert len(set(keys)) == 10
