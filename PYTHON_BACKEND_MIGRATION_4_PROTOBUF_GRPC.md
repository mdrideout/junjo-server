# Phase 4: Protobuf + gRPC Client/Server

## Overview

This phase implements the gRPC communication layer for:
1. **gRPC Client**: Reading spans from the ingestion-service (InternalIngestionService)
2. **gRPC Server**: Validating API keys for the ingestion-service (InternalAuthService)

The Python backend will maintain compatibility with the existing Go ingestion-service by using the same protobuf definitions.

## Current Go Implementation Analysis

### gRPC Services

**InternalIngestionService** (Client - `/backend/proto/ingestion.proto:9-13`):
```protobuf
service InternalIngestionService {
  rpc ReadSpans(ReadSpansRequest) returns (stream ReadSpansResponse) {}
}
```
- **Purpose**: Read spans from ingestion-service's BadgerDB WAL
- **Type**: Server-streaming RPC (ingestion-service streams spans to backend)
- **Connection**: Backend connects to `junjo-server-ingestion:50052`

**InternalAuthService** (Server - `/backend/proto/auth.proto:15-18`):
```protobuf
service InternalAuthService {
  rpc ValidateApiKey(ValidateApiKeyRequest) returns (ValidateApiKeyResponse) {}
}
```
- **Purpose**: Validate API keys for ingestion-service
- **Type**: Unary RPC (ingestion-service calls backend to validate keys)
- **Listen Address**: Backend listens on `:50053`

### Client Implementation

**File**: `/backend/ingestion_client/client.go`

```go
func NewClient() (*Client, error) {
    addr := "junjo-server-ingestion:50052"
    conn, err := grpc.NewClient(addr, grpc.WithTransportCredentials(insecure.NewCredentials()))
    // ...
}

func (c *Client) ReadSpans(ctx context.Context, startKey []byte, batchSize uint32) ([]*SpanWithResource, error) {
    req := &pb.ReadSpansRequest{
        StartKeyUlid: startKey,
        BatchSize:    batchSize,
    }
    stream, err := c.client.ReadSpans(ctx, req)
    // ... stream.Recv() loop
}
```

### Server Implementation

**File**: `/backend/api/internal_auth/grpc_api_key_auth.go`

```go
func (s *InternalAuthService) ValidateApiKey(ctx context.Context, req *pb.ValidateApiKeyRequest) (*pb.ValidateApiKeyResponse, error) {
    _, err := api_keys.GetAPIKey(ctx, req.ApiKey)
    if err != nil {
        if err == sql.ErrNoRows {
            return &pb.ValidateApiKeyResponse{IsValid: false}, nil
        }
        return nil, status.Errorf(codes.Internal, "failed to get API key: %v", err)
    }
    return &pb.ValidateApiKeyResponse{IsValid: true}, nil
}
```

**Main Setup** (from `/backend/main.go:250-274`):
```go
go func() {
    internalGrpcAddr := ":50053"
    lis, err := net.Listen("tcp", internalGrpcAddr)
    // ...
    grpcServer := grpc.NewServer(/* ... */)
    internalAuthSvc := internal_auth.NewInternalAuthService()
    pb.RegisterInternalAuthServiceServer(grpcServer, internalAuthSvc)
    // ...
}()
```

## Python Implementation

### Directory Structure

```
python_backend/
├── proto/                       # Shared protobuf definitions
│   ├── ingestion.proto          # Copy from backend/proto/
│   ├── auth.proto               # Copy from backend/proto/
│   └── span_data_container.proto # Copy from backend/proto/
├── app/
│   ├── grpc_services/
│   │   ├── __init__.py
│   │   ├── proto_gen/           # Generated Python code (gitignored)
│   │   │   ├── __init__.py
│   │   │   ├── ingestion_pb2.py
│   │   │   ├── ingestion_pb2_grpc.py
│   │   │   ├── auth_pb2.py
│   │   │   └── auth_pb2_grpc.py
│   │   ├── ingestion_client.py  # Client for reading spans
│   │   ├── auth_server.py       # Server for API key validation
│   │   └── utils.py             # gRPC utilities
│   └── tests/
│       ├── unit/
│       │   └── grpc_services/
│       │       ├── test_ingestion_client.py
│       │       └── test_auth_server.py
│       └── integration/
│           └── grpc_services/
│               └── test_grpc_integration.py
└── scripts/
    └── generate_protos.py       # Script to generate Python code from .proto files
```

### 1. Copy Protobuf Definitions

**Action**: Copy proto files from `/backend/proto/` to `/python_backend/proto/`

```bash
# From junjo-server root
mkdir -p python_backend/proto
cp backend/proto/ingestion.proto python_backend/proto/
cp backend/proto/auth.proto python_backend/proto/
cp backend/proto/span_data_container.proto python_backend/proto/
```

**Important**: These proto files must remain in sync with the Go backend to maintain compatibility with the ingestion-service.

### 2. Proto Generation Script

**File**: `scripts/generate_protos.py`

```python
"""
Generate Python code from protobuf definitions.

This script uses grpcio-tools to compile .proto files into Python code.
Run this script whenever .proto files change.

Usage:
    python scripts/generate_protos.py
"""

import subprocess
import sys
from pathlib import Path

from app.core.logger import logger


def generate_protos():
    """Generate Python code from .proto files."""
    # Paths
    repo_root = Path(__file__).parent.parent
    proto_dir = repo_root / "proto"
    output_dir = repo_root / "app" / "grpc_services" / "proto_gen"

    # Create output directory
    output_dir.mkdir(parents=True, exist_ok=True)
    (output_dir / "__init__.py").touch()

    # Find all .proto files
    proto_files = list(proto_dir.glob("*.proto"))
    if not proto_files:
        logger.error(f"No .proto files found in {proto_dir}")
        sys.exit(1)

    logger.info(f"Found {len(proto_files)} .proto files")

    # Generate Python code for each .proto file
    for proto_file in proto_files:
        logger.info(f"Generating Python code for {proto_file.name}")

        # grpc_tools.protoc command
        # Equivalent to: python -m grpc_tools.protoc -I./proto --python_out=./app/grpc_services/proto_gen --grpc_python_out=./app/grpc_services/proto_gen ./proto/ingestion.proto
        cmd = [
            sys.executable,
            "-m",
            "grpc_tools.protoc",
            f"-I{proto_dir}",
            f"--python_out={output_dir}",
            f"--grpc_python_out={output_dir}",
            f"--pyi_out={output_dir}",  # Generate type stubs (.pyi files)
            str(proto_file),
        ]

        result = subprocess.run(cmd, capture_output=True, text=True)
        if result.returncode != 0:
            logger.error(f"Failed to generate code for {proto_file.name}")
            logger.error(result.stderr)
            sys.exit(1)

        logger.info(f"Successfully generated code for {proto_file.name}")

    logger.info("Protobuf code generation complete!")


if __name__ == "__main__":
    generate_protos()
```

**Run Command**:
```bash
# Generate Python code from .proto files
uv run python scripts/generate_protos.py
```

**Add to `.gitignore`**:
```
# Generated protobuf code
app/grpc_services/proto_gen/*.py
app/grpc_services/proto_gen/*.pyi
!app/grpc_services/proto_gen/__init__.py
```

### 3. Ingestion Client (Read Spans)

**File**: `app/grpc_services/ingestion_client.py`

```python
"""
gRPC client for reading spans from the ingestion-service.

Mirrors the Go implementation: /backend/ingestion_client/client.go
"""

from typing import AsyncIterator, List, Optional

import grpc
from grpc import aio

from app.core.logger import logger
from app.core.settings import settings
from app.grpc_services.proto_gen import ingestion_pb2, ingestion_pb2_grpc


class SpanWithResource:
    """
    Container for span and resource data.

    Mirrors Go struct: SpanWithResource
    """

    def __init__(self, key_ulid: bytes, span_bytes: bytes, resource_bytes: bytes):
        self.key_ulid = key_ulid
        self.span_bytes = span_bytes
        self.resource_bytes = resource_bytes


class IngestionClient:
    """
    gRPC client for the InternalIngestionService.

    Connects to the ingestion-service to read spans from the BadgerDB WAL.
    """

    def __init__(self, address: Optional[str] = None):
        """
        Initialize the ingestion client.

        Args:
            address: gRPC server address (default: from settings)
        """
        self.address = address or settings.ingestion_grpc_address
        self.channel: Optional[aio.Channel] = None
        self.stub: Optional[ingestion_pb2_grpc.InternalIngestionServiceStub] = None

    async def connect(self):
        """
        Establish connection to the ingestion-service.

        Mirrors Go: grpc.NewClient(addr, grpc.WithTransportCredentials(insecure.NewCredentials()))
        """
        logger.info(f"Connecting to ingestion-service at {self.address}")

        # Create insecure channel (internal network)
        self.channel = aio.insecure_channel(self.address)
        self.stub = ingestion_pb2_grpc.InternalIngestionServiceStub(self.channel)

        logger.info("Connected to ingestion-service")

    async def close(self):
        """Close the gRPC connection."""
        if self.channel:
            await self.channel.close()
            logger.info("Closed ingestion-service connection")

    async def read_spans(
        self, start_key: Optional[bytes], batch_size: int = 100
    ) -> List[SpanWithResource]:
        """
        Read a batch of spans from the ingestion-service.

        Mirrors Go: ReadSpans(ctx, startKey, batchSize)

        Args:
            start_key: ULID of last processed span (None to start from beginning)
            batch_size: Maximum number of spans to read

        Returns:
            List of SpanWithResource objects

        Raises:
            grpc.RpcError: If the RPC fails
        """
        if self.stub is None:
            raise RuntimeError("Client not connected. Call connect() first.")

        request = ingestion_pb2.ReadSpansRequest(
            start_key_ulid=start_key or b"",
            batch_size=batch_size,
        )

        logger.debug(
            f"Reading spans (start_key={start_key.hex() if start_key else 'None'}, batch_size={batch_size})"
        )

        spans: List[SpanWithResource] = []

        try:
            # Stream responses from server
            stream: AsyncIterator[ingestion_pb2.ReadSpansResponse] = self.stub.ReadSpans(
                request
            )

            async for response in stream:
                spans.append(
                    SpanWithResource(
                        key_ulid=response.key_ulid,
                        span_bytes=response.span_bytes,
                        resource_bytes=response.resource_bytes,
                    )
                )

        except grpc.RpcError as e:
            logger.error(f"Error reading spans: {e.code()} - {e.details()}")
            raise

        logger.debug(f"Received {len(spans)} spans")
        return spans


# Global client instance (initialized at startup)
_ingestion_client: Optional[IngestionClient] = None


async def get_ingestion_client() -> IngestionClient:
    """
    Get the global ingestion client instance.

    Returns:
        Configured IngestionClient

    Raises:
        RuntimeError: If client not initialized
    """
    if _ingestion_client is None:
        raise RuntimeError("Ingestion client not initialized. Call init_ingestion_client() first.")
    return _ingestion_client


async def init_ingestion_client():
    """Initialize the global ingestion client (called at startup)."""
    global _ingestion_client
    _ingestion_client = IngestionClient()
    await _ingestion_client.connect()
    logger.info("Ingestion client initialized")


async def close_ingestion_client():
    """Close the global ingestion client (called at shutdown)."""
    global _ingestion_client
    if _ingestion_client:
        await _ingestion_client.close()
        _ingestion_client = None
```

### 4. Auth Server (Validate API Keys)

**File**: `app/grpc_services/auth_server.py`

```python
"""
gRPC server for validating API keys.

Mirrors the Go implementation: /backend/api/internal_auth/grpc_api_key_auth.go
"""

import grpc
from grpc import aio

from app.core.logger import logger
from app.database.api_keys.repository import APIKeyRepository
from app.grpc_services.proto_gen import auth_pb2, auth_pb2_grpc


class InternalAuthServicer(auth_pb2_grpc.InternalAuthServiceServicer):
    """
    gRPC servicer for InternalAuthService.

    Provides API key validation for the ingestion-service.
    """

    async def ValidateApiKey(
        self,
        request: auth_pb2.ValidateApiKeyRequest,
        context: grpc.aio.ServicerContext,
    ) -> auth_pb2.ValidateApiKeyResponse:
        """
        Validate an API key.

        Mirrors Go: ValidateApiKey(ctx, req) in grpc_api_key_auth.go:25-39

        Args:
            request: Contains the API key to validate
            context: gRPC context

        Returns:
            ValidateApiKeyResponse with is_valid flag
        """
        api_key = request.api_key
        logger.info(f"Validating API key: {api_key}")

        try:
            # Look up API key in database
            key_record = await APIKeyRepository.get_api_key_by_key(api_key)

            if key_record is None:
                # Key not found - invalid
                logger.warning(f"API key not found: {api_key}")
                return auth_pb2.ValidateApiKeyResponse(is_valid=False)

            # Key found - valid
            logger.info(f"API key validated: {api_key}")
            return auth_pb2.ValidateApiKeyResponse(is_valid=True)

        except Exception as e:
            # Database error
            logger.error(f"Failed to validate API key: {e}")
            await context.abort(
                grpc.StatusCode.INTERNAL,
                f"Failed to validate API key: {e}",
            )


async def serve_auth_grpc(address: str = "0.0.0.0:50053"):
    """
    Start the gRPC server for InternalAuthService.

    Mirrors Go: lines 250-274 in main.go

    Args:
        address: Address to listen on (default: 0.0.0.0:50053)
    """
    server = aio.server()
    auth_pb2_grpc.add_InternalAuthServiceServicer_to_server(
        InternalAuthServicer(), server
    )
    server.add_insecure_port(address)

    logger.info(f"Starting internal gRPC server on {address}")
    await server.start()

    return server


# Global server instance
_auth_grpc_server: Optional[aio.Server] = None


async def start_auth_grpc_server():
    """Start the auth gRPC server (called at startup)."""
    global _auth_grpc_server
    _auth_grpc_server = await serve_auth_grpc()
    logger.info("Auth gRPC server started")


async def stop_auth_grpc_server():
    """Stop the auth gRPC server (called at shutdown)."""
    global _auth_grpc_server
    if _auth_grpc_server:
        await _auth_grpc_server.stop(grace=5)
        _auth_grpc_server = None
        logger.info("Auth gRPC server stopped")
```

### 5. Update Settings

**File**: `app/core/settings.py` (add gRPC settings)

```python
class Settings(BaseSettings):
    # ... existing settings ...

    # gRPC settings
    ingestion_grpc_address: str = "junjo-server-ingestion:50052"  # Ingestion-service address
    auth_grpc_listen_address: str = "0.0.0.0:50053"  # Internal auth server listen address

    model_config = SettingsConfigDict(
        env_file=".env",
        env_file_encoding="utf-8",
        case_sensitive=False,
    )
```

### 6. Update `main.py` Lifespan

**File**: `app/main.py` (add gRPC startup/shutdown)

```python
from contextlib import asynccontextmanager

from fastapi import FastAPI

from app.core.logger import logger
from app.database.db_config import checkpoint_wal, engine
from app.grpc_services.auth_server import start_auth_grpc_server, stop_auth_grpc_server
from app.grpc_services.ingestion_client import close_ingestion_client, init_ingestion_client


@asynccontextmanager
async def lifespan(app: FastAPI):
    """
    Lifespan context manager for startup and shutdown.

    Handles:
    - Database setup
    - gRPC client/server initialization
    - Background tasks (span polling)
    """
    # --- Startup ---
    logger.info("Starting application")

    # Initialize database
    # (already done in Phase 2)

    # Initialize gRPC client (for reading spans)
    await init_ingestion_client()

    # Start gRPC server (for API key validation)
    await start_auth_grpc_server()

    # TODO: Start background span polling task (Phase 7)

    yield

    # --- Shutdown ---
    logger.info("Shutting down application")

    # Stop background tasks
    # TODO: Cancel span polling task (Phase 7)

    # Close gRPC connections
    await close_ingestion_client()
    await stop_auth_grpc_server()

    # Database cleanup
    await checkpoint_wal()
    await engine.dispose()

    logger.info("Application shutdown complete")


def create_app() -> FastAPI:
    """Create and configure the FastAPI application."""
    app = FastAPI(
        title="Junjo Backend",
        lifespan=lifespan,
    )

    # ... existing router setup ...

    return app
```

### 7. gRPC Utilities

**File**: `app/grpc_services/utils.py`

```python
"""
Utilities for gRPC operations.
"""

from typing import Optional

import grpc


def is_grpc_error(exception: Exception, code: grpc.StatusCode) -> bool:
    """
    Check if an exception is a gRPC error with a specific status code.

    Args:
        exception: Exception to check
        code: Expected gRPC status code

    Returns:
        True if exception is a gRPC error with the specified code
    """
    if isinstance(exception, grpc.RpcError):
        return exception.code() == code
    return False


def get_grpc_error_details(exception: Exception) -> Optional[str]:
    """
    Extract error details from a gRPC exception.

    Args:
        exception: Exception to extract details from

    Returns:
        Error details string, or None if not a gRPC error
    """
    if isinstance(exception, grpc.RpcError):
        return f"{exception.code()}: {exception.details()}"
    return None
```

## Testing Requirements

### Unit Tests

**File**: `tests/unit/grpc_services/test_ingestion_client.py`

```python
"""Unit tests for ingestion client."""

import pytest

from app.grpc_services.ingestion_client import SpanWithResource


@pytest.mark.unit
def test_span_with_resource_creation():
    """Test SpanWithResource creation."""
    key_ulid = b"01234567890123456789"
    span_bytes = b"span_data"
    resource_bytes = b"resource_data"

    span = SpanWithResource(key_ulid, span_bytes, resource_bytes)

    assert span.key_ulid == key_ulid
    assert span.span_bytes == span_bytes
    assert span.resource_bytes == resource_bytes
```

**File**: `tests/unit/grpc_services/test_auth_server.py`

```python
"""Unit tests for auth server."""

import pytest

from app.database.api_keys.repository import APIKeyRepository
from app.database.api_keys.schemas import APIKeyCreate
from app.grpc_services.auth_server import InternalAuthServicer
from app.grpc_services.proto_gen import auth_pb2


@pytest.mark.unit
@pytest.mark.asyncio
async def test_validate_api_key_valid(test_db_engine):
    """Test ValidateApiKey with valid API key."""
    # Create API key in database
    api_key_create = APIKeyCreate(
        id="test-id-123",
        key="test-key-abc",
        name="Test API Key",
    )
    await APIKeyRepository.create_api_key(api_key_create)

    # Create servicer
    servicer = InternalAuthServicer()

    # Create request
    request = auth_pb2.ValidateApiKeyRequest(api_key="test-key-abc")

    # Mock context (not used in this test)
    class MockContext:
        pass

    context = MockContext()

    # Validate API key
    response = await servicer.ValidateApiKey(request, context)

    assert response.is_valid is True


@pytest.mark.unit
@pytest.mark.asyncio
async def test_validate_api_key_invalid(test_db_engine):
    """Test ValidateApiKey with invalid API key."""
    # Create servicer
    servicer = InternalAuthServicer()

    # Create request with non-existent key
    request = auth_pb2.ValidateApiKeyRequest(api_key="nonexistent-key")

    # Mock context
    class MockContext:
        pass

    context = MockContext()

    # Validate API key
    response = await servicer.ValidateApiKey(request, context)

    assert response.is_valid is False
```

### Integration Tests

**File**: `tests/integration/grpc_services/test_grpc_integration.py`

```python
"""Integration tests for gRPC client/server."""

import pytest

from app.database.api_keys.repository import APIKeyRepository
from app.database.api_keys.schemas import APIKeyCreate
from app.grpc_services.auth_server import InternalAuthServicer
from app.grpc_services.proto_gen import auth_pb2


@pytest.mark.integration
@pytest.mark.asyncio
async def test_auth_server_integration(test_db_engine):
    """Test complete auth server flow with database."""
    # Create test API key
    api_key_create = APIKeyCreate(
        id="integration-test-id",
        key="integration-test-key",
        name="Integration Test Key",
    )
    await APIKeyRepository.create_api_key(api_key_create)

    # Create servicer
    servicer = InternalAuthServicer()

    # Mock context
    class MockContext:
        pass

    context = MockContext()

    # Test valid key
    request = auth_pb2.ValidateApiKeyRequest(api_key="integration-test-key")
    response = await servicer.ValidateApiKey(request, context)
    assert response.is_valid is True

    # Test invalid key
    request = auth_pb2.ValidateApiKeyRequest(api_key="invalid-key")
    response = await servicer.ValidateApiKey(request, context)
    assert response.is_valid is False
```

## Dependencies

Add to `pyproject.toml`:

```toml
[project]
dependencies = [
    # ... existing dependencies ...
    "grpcio>=1.60.0",           # gRPC runtime
    "grpcio-tools>=1.60.0",     # Protobuf compiler (for code generation)
    "protobuf>=4.25.0",         # Protocol Buffers
]

[project.optional-dependencies]
dev = [
    # ... existing dev dependencies ...
    "grpcio-reflection>=1.60.0",  # gRPC reflection for debugging
]
```

## Development Workflow

### 1. Generate Protobuf Code

After cloning or modifying .proto files:

```bash
# Generate Python code from .proto files
uv run python scripts/generate_protos.py
```

This will create:
- `app/grpc_services/proto_gen/ingestion_pb2.py`
- `app/grpc_services/proto_gen/ingestion_pb2_grpc.py`
- `app/grpc_services/proto_gen/auth_pb2.py`
- `app/grpc_services/proto_gen/auth_pb2_grpc.py`
- Type stub files (`.pyi`)

### 2. Testing gRPC Server

Use `grpcurl` (command-line gRPC client) to test the auth server:

```bash
# Install grpcurl (if not already installed)
brew install grpcurl  # macOS

# Enable reflection in dev (add to auth_server.py for development)
from grpc_reflection.v1alpha import reflection
reflection.enable_server_reflection(server_descriptor_pool, server)

# List services
grpcurl -plaintext localhost:50053 list

# Call ValidateApiKey
grpcurl -plaintext -d '{"api_key": "test-key"}' localhost:50053 ingestion.InternalAuthService/ValidateApiKey
```

### 3. Syncing Proto Files

**IMPORTANT**: Proto files must stay in sync with the Go backend to maintain compatibility with the ingestion-service.

If proto files are updated:
1. Update in `/backend/proto/`
2. Copy to `/python_backend/proto/`
3. Regenerate Python code: `uv run python scripts/generate_protos.py`
4. Test both Go and Python implementations

## Phase Completion Criteria

- [ ] Proto files copied from Go backend
- [ ] Proto generation script works correctly
- [ ] Generated Python code compiles without errors
- [ ] IngestionClient implemented and tested
- [ ] InternalAuthServicer implemented and tested
- [ ] gRPC client connects to ingestion-service successfully
- [ ] gRPC server starts and listens on :50053
- [ ] API key validation works end-to-end
- [ ] Lifespan manager handles gRPC startup/shutdown
- [ ] Unit tests pass (80%+ coverage)
- [ ] Integration tests pass
- [ ] gRPC error handling works correctly
- [ ] Connection cleanup works on shutdown

## Notes

1. **Proto Compatibility**: The Python backend MUST use the same proto files as the Go backend to maintain wire-level compatibility with the ingestion-service.

2. **Insecure Credentials**: Using insecure gRPC connections since this is internal network communication. If security is required later, add TLS certificates.

3. **Connection Management**: Both client and server are managed through the FastAPI lifespan, ensuring proper cleanup on shutdown.

4. **Streaming vs Unary**:
   - ReadSpans is a server-streaming RPC (ingestion-service streams spans to backend)
   - ValidateApiKey is a unary RPC (single request/response)

5. **Error Handling**: gRPC errors are propagated and logged. The auth server returns `is_valid=false` for not-found keys (not an error), but returns gRPC INTERNAL status for database errors.

6. **Background Polling**: The span polling task (which uses the ingestion client) will be implemented in Phase 7 (OTEL Span Indexing).

7. **Type Safety**: Generated code includes `.pyi` type stubs for better IDE support and type checking.

8. **Production Considerations**:
   - Consider adding connection retry logic
   - Consider adding gRPC interceptors for logging/metrics
   - Consider enabling gRPC reflection only in development

## Next Phase

Phase 5 will implement API Key Management, which is required for the auth server to function. The database repository and models for API keys will be created in that phase.
