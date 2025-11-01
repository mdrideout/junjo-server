"""
Integration tests for the internal authentication gRPC service.

These tests connect to a real gRPC server instance and test the complete
request/response flow with actual database access.
"""

import grpc
from loguru import logger
import pytest

from app.config.settings import settings
from app.proto_gen import auth_pb2, auth_pb2_grpc


@pytest.mark.integration
@pytest.mark.requires_grpc_server
@pytest.mark.asyncio
async def test_validate_api_key_integration_with_production_key():
    """
    Integration test: ValidateApiKey with a real API key from production database.

    Requires:
    - The gRPC server to be running on port 50053
    - The production database to have at least one API key

    To create a production key for testing:
        python -c "from app.db_sqlite.api_keys.repository import APIKeyRepository; import asyncio, nanoid; asyncio.run(APIKeyRepository.create(id=nanoid.generate(size=21), key=nanoid.generate(size=64), name='Test Key'))"
    """
    # This test specifically uses the production key created earlier:
    # 9hppr92Y5kZqx4EvQ0oLRFzJ0LGozRO3oIIWrcx6B4qCmI59A8eFJFtbORy8LXBz
    production_test_key = "9hppr92Y5kZqx4EvQ0oLRFzJ0LGozRO3oIIWrcx6B4qCmI59A8eFJFtbORy8LXBz"

    # Connect to the gRPC server (uses production DB)
    async with grpc.aio.insecure_channel(f"localhost:{settings.GRPC_PORT}") as channel:
        stub = auth_pb2_grpc.InternalAuthServiceStub(channel)

        # Make the ValidateApiKey request
        request = auth_pb2.ValidateApiKeyRequest(api_key=production_test_key)
        response = await stub.ValidateApiKey(request)

        # Verify the response - should be valid since it exists in production DB
        assert response.is_valid is True
        logger.info("✓ Integration test passed: Production API key validated successfully")


@pytest.mark.integration
@pytest.mark.requires_grpc_server
@pytest.mark.asyncio
async def test_validate_api_key_integration_invalid():
    """
    Integration test: ValidateApiKey with an invalid API key.

    Requires: gRPC server running on port 50053.
    """
    invalid_key = "invalid_test_key_does_not_exist_12345"

    # Connect to the gRPC server
    async with grpc.aio.insecure_channel(f"localhost:{settings.GRPC_PORT}") as channel:
        stub = auth_pb2_grpc.InternalAuthServiceStub(channel)

        # Make the ValidateApiKey request
        request = auth_pb2.ValidateApiKeyRequest(api_key=invalid_key)
        response = await stub.ValidateApiKey(request)

        # Verify the response
        assert response.is_valid is False
        logger.info("✓ Integration test passed: Invalid API key rejected")


@pytest.mark.integration
@pytest.mark.requires_grpc_server
@pytest.mark.asyncio
async def test_validate_api_key_integration_empty():
    """
    Integration test: ValidateApiKey with an empty API key.

    Requires: gRPC server running on port 50053.
    """
    # Connect to the gRPC server
    async with grpc.aio.insecure_channel(f"localhost:{settings.GRPC_PORT}") as channel:
        stub = auth_pb2_grpc.InternalAuthServiceStub(channel)

        # Make the ValidateApiKey request with empty key
        request = auth_pb2.ValidateApiKeyRequest(api_key="")
        response = await stub.ValidateApiKey(request)

        # Verify the response (should be rejected)
        assert response.is_valid is False
        logger.info("✓ Integration test passed: Empty API key rejected")


@pytest.mark.integration
@pytest.mark.requires_grpc_server
@pytest.mark.asyncio
async def test_grpc_server_connectivity():
    """
    Integration test: Verify gRPC server is accessible and responsive.

    Requires: gRPC server running on port 50053.
    """
    try:
        # Connect to the gRPC server with a short timeout
        async with grpc.aio.insecure_channel(
            f"localhost:{settings.GRPC_PORT}",
            options=[
                ("grpc.keepalive_time_ms", 10000),
                ("grpc.keepalive_timeout_ms", 5000),
            ],
        ) as channel:
            # Try to create a stub (this will fail if server is not running)
            stub = auth_pb2_grpc.InternalAuthServiceStub(channel)

            # Make a simple request to verify server is responding
            request = auth_pb2.ValidateApiKeyRequest(api_key="connectivity_test")
            response = await stub.ValidateApiKey(request, timeout=5.0)

            # We don't care about the response value, just that we got a response
            assert response is not None
            logger.info(
                f"✓ Integration test passed: gRPC server is accessible on port {settings.GRPC_PORT}"
            )

    except grpc.aio.AioRpcError as e:
        pytest.fail(
            f"gRPC server is not accessible on port {settings.GRPC_PORT}: {e.code()} - {e.details()}"
        )
