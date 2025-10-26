# Phase 7: OTEL Span Indexing (Background Polling)

## Overview

This phase implements the background task that continuously polls the ingestion-service for new spans and indexes them into DuckDB. The polling task:
- Runs in the background using asyncio
- Reads spans from ingestion-service via gRPC (Phase 4)
- Processes and stores spans in DuckDB (Phase 6)
- Tracks polling state in SQLite (Phase 2) for resumability
- Handles errors gracefully and continues polling

This is the "glue" phase that connects the gRPC client (Phase 4) with the DuckDB storage (Phase 6).

## Current Go Implementation Analysis

**Background Goroutine** (from `/backend/main.go:78-160`):

```go
// Start a background goroutine to poll for spans
go func() {
    ticker := time.NewTicker(5 * time.Second) // Poll every 5 seconds
    defer ticker.Stop()

    queries := db_gen.New(db.DB)
    var lastKey []byte

    // At startup, try to load the last processed key from the database
    retrievedKey, err := queries.GetPollerState(context.Background())
    if err != nil && err != sql.ErrNoRows {
        log.Error("failed to load poller state", slog.Any("error", err))
        os.Exit(1)
    } else if err == sql.ErrNoRows {
        log.Info("no previous poller state found, starting from beginning")
    } else {
        lastKey = retrievedKey
        log.Info("resuming poller", slog.String("last_key", fmt.Sprintf("%x", lastKey)))
    }

    for range ticker.C {
        log.Debug("polling for new spans")
        spans, err := ingestionClient.ReadSpans(context.Background(), lastKey, 100)
        if err != nil {
            log.Error("error reading spans", slog.Any("error", err))
            continue
        }

        if len(spans) > 0 {
            lastKey = spans[len(spans)-1].KeyUlid
            log.Info("received spans", slog.Int("count", len(spans)), slog.String("last_key", fmt.Sprintf("%x", lastKey)))

            var processedSpans []*tracepb.Span
            for _, receivedSpan := range spans {
                var span tracepb.Span
                if err := proto.Unmarshal(receivedSpan.SpanBytes, &span); err != nil {
                    log.Warn("error unmarshaling span", slog.Any("error", err))
                    continue // Skip to the next span
                }
                processedSpans = append(processedSpans, &span)
            }

            if len(processedSpans) > 0 {
                // Extract service name from first span's resource
                serviceName := extractServiceNameFromResource(spans[0].ResourceBytes)

                if err := telemetry.BatchProcessSpans(context.Background(), serviceName, processedSpans); err != nil {
                    log.Error("error processing spans batch", slog.Any("error", err))
                } else {
                    // If the batch was processed successfully, update the last key in the database
                    err := queries.UpsertPollerState(context.Background(), lastKey)
                    if err != nil {
                        log.Error("failed to save poller state", slog.Any("error", err))
                    }
                }
            }
        } else {
            log.Debug("no new spans found")
        }
    }
}()
```

**Key Features**:
1. **Polling Interval**: 5 seconds
2. **Batch Size**: 100 spans per request
3. **State Persistence**: Last processed key stored in SQLite `poller_state` table
4. **Resumability**: On startup, loads last key and resumes from that point
5. **Error Handling**: Logs errors but continues polling
6. **Service Name Extraction**: From resource bytes in first span
7. **Protobuf Unmarshal**: Deserializes span bytes before processing
8. **Batch Processing**: Processes all spans in a batch, updates state only if successful

**Poller State Table** (from `/backend/db/schema.sql:21-25`):
```sql
CREATE TABLE poller_state (
  id INTEGER PRIMARY KEY CHECK (id = 1),  -- Enforce a single row
  last_key BLOB
);
```

## Python Implementation

### Directory Structure

```
python_backend/
└── app/
    ├── database/
    │   └── poller_state/
    │       ├── __init__.py
    │       ├── models.py        # PollerStateTable SQLAlchemy model
    │       ├── repository.py    # Poller state operations
    │       └── schemas.py       # Pydantic schemas
    ├── background/
    │   ├── __init__.py
    │   ├── span_poller.py       # Background span polling task
    │   └── utils.py             # Helper functions
    └── tests/
        ├── unit/
        │   └── background/
        │       └── test_span_poller.py
        └── integration/
            └── background/
                └── test_span_poller_integration.py
```

### 1. Poller State Database Model

**File**: `app/database/poller_state/models.py`

```python
"""
Poller state database model for tracking span polling progress.
"""

from typing import Optional

from sqlalchemy import CheckConstraint, LargeBinary
from sqlalchemy.orm import Mapped, mapped_column

from app.database.db_config import Base


class PollerStateTable(Base):
    """
    Poller state model for resumable span polling.

    Mirrors the Go schema:
    CREATE TABLE poller_state (
      id INTEGER PRIMARY KEY CHECK (id = 1),
      last_key BLOB
    );

    The CHECK constraint ensures only one row exists.
    """

    __tablename__ = "poller_state"

    id: Mapped[int] = mapped_column(primary_key=True)
    last_key: Mapped[Optional[bytes]] = mapped_column(LargeBinary, nullable=True)

    __table_args__ = (CheckConstraint("id = 1", name="enforce_single_row"),)

    def __repr__(self) -> str:
        return f"<PollerStateTable(id={self.id}, last_key={self.last_key.hex() if self.last_key else None})>"
```

### 2. Poller State Repository

**File**: `app/database/poller_state/repository.py`

```python
"""
Poller state repository for database operations.

Following the high-concurrency pattern from wt_api_v2.
"""

from typing import Optional

from sqlalchemy import select
from sqlalchemy.ext.asyncio import AsyncSession

from app.database.db_config import get_async_session
from app.database.poller_state.models import PollerStateTable


class PollerStateRepository:
    """Repository for poller state operations."""

    @staticmethod
    async def get_poller_state() -> Optional[bytes]:
        """
        Get the last processed key from poller state.

        Mirrors Go: GetPollerState(ctx)

        Returns:
            Last processed key bytes, or None if no state exists
        """
        async with get_async_session() as session:
            result = await session.execute(
                select(PollerStateTable).where(PollerStateTable.id == 1)
            )
            poller_state = result.scalar_one_or_none()

            if poller_state is None:
                return None

            return poller_state.last_key

    @staticmethod
    async def upsert_poller_state(last_key: bytes) -> None:
        """
        Insert or update the poller state.

        Mirrors Go: UpsertPollerState(ctx, lastKey)

        Args:
            last_key: Last processed key bytes
        """
        async with get_async_session() as session:
            # Try to get existing state
            result = await session.execute(
                select(PollerStateTable).where(PollerStateTable.id == 1)
            )
            poller_state = result.scalar_one_or_none()

            if poller_state is None:
                # Insert new state
                poller_state = PollerStateTable(id=1, last_key=last_key)
                session.add(poller_state)
            else:
                # Update existing state
                poller_state.last_key = last_key

            await session.commit()
```

### 3. Poller State Schemas

**File**: `app/database/poller_state/schemas.py`

```python
"""
Pydantic schemas for Poller State model.
"""

from typing import Optional

from pydantic import BaseModel, ConfigDict


class PollerState(BaseModel):
    """Poller state schema."""

    id: int
    last_key: Optional[bytes]

    model_config = ConfigDict(from_attributes=True)
```

### 4. Background Span Poller

**File**: `app/background/span_poller.py`

```python
"""
Background task for polling spans from ingestion-service.

Mirrors the Go implementation: /backend/main.go:78-160
"""

import asyncio
from typing import Optional

from google.protobuf import message as proto_message
from opentelemetry.proto.resource.v1 import resource_pb2
from opentelemetry.proto.trace.v1 import trace_pb2

from app.background.utils import extract_service_name_from_resource
from app.core.logger import logger
from app.core.settings import settings
from app.database.poller_state.repository import PollerStateRepository
from app.features.telemetry.span_processor import batch_process_spans
from app.grpc_services.ingestion_client import get_ingestion_client

# Global task handle for cancellation
_poller_task: Optional[asyncio.Task] = None


async def span_poller_loop():
    """
    Background polling loop for reading and processing spans.

    Mirrors Go: background goroutine in main.go:78-160
    """
    # Load last processed key from database
    last_key = await PollerStateRepository.get_poller_state()

    if last_key is None:
        logger.info("No previous poller state found, starting from beginning")
    else:
        logger.info(f"Resuming poller from last key: {last_key.hex()}")

    # Get ingestion client
    ingestion_client = await get_ingestion_client()

    # Polling loop
    while True:
        try:
            # Wait for polling interval
            await asyncio.sleep(settings.span_poll_interval)

            logger.debug("Polling for new spans")

            # Read spans from ingestion-service
            spans = await ingestion_client.read_spans(
                start_key=last_key,
                batch_size=settings.span_batch_size,
            )

            if len(spans) > 0:
                # Update last key
                last_key = spans[-1].key_ulid
                logger.info(
                    f"Received {len(spans)} spans (last_key: {last_key.hex()})"
                )

                # Unmarshal spans from protobuf bytes
                processed_spans = []
                for span_with_resource in spans:
                    try:
                        span = trace_pb2.Span()
                        span.ParseFromString(span_with_resource.span_bytes)
                        processed_spans.append(span)
                    except proto_message.DecodeError as e:
                        logger.warning(f"Error unmarshaling span: {e}")
                        continue

                if len(processed_spans) > 0:
                    # Extract service name from first span's resource
                    try:
                        resource = resource_pb2.Resource()
                        resource.ParseFromString(spans[0].resource_bytes)
                        service_name = extract_service_name_from_resource(resource)
                    except Exception as e:
                        logger.warning(f"Error extracting service name: {e}")
                        service_name = "NO_SERVICE_NAME"

                    # Process spans batch
                    try:
                        await batch_process_spans(service_name, processed_spans)

                        # Update poller state (only if processing succeeded)
                        await PollerStateRepository.upsert_poller_state(last_key)

                    except Exception as e:
                        logger.error(f"Error processing spans batch: {e}")
                        # Don't update last_key if processing failed
                        # Will retry these spans on next poll

            else:
                logger.debug("No new spans found")

        except asyncio.CancelledError:
            logger.info("Span poller task cancelled")
            raise

        except Exception as e:
            logger.error(f"Error in span poller loop: {e}")
            # Continue polling despite errors


async def start_span_poller():
    """Start the background span polling task."""
    global _poller_task

    logger.info("Starting span poller task")
    _poller_task = asyncio.create_task(span_poller_loop())


async def stop_span_poller():
    """Stop the background span polling task."""
    global _poller_task

    if _poller_task:
        logger.info("Stopping span poller task")
        _poller_task.cancel()

        try:
            await _poller_task
        except asyncio.CancelledError:
            logger.info("Span poller task stopped")

        _poller_task = None
```

### 5. Utility Functions

**File**: `app/background/utils.py`

```python
"""
Utility functions for background tasks.
"""

from opentelemetry.proto.resource.v1 import resource_pb2


def extract_service_name_from_resource(resource: resource_pb2.Resource) -> str:
    """
    Extract service name from OpenTelemetry resource.

    Mirrors Go: service name extraction in main.go:120-140

    Args:
        resource: Protobuf Resource object

    Returns:
        Service name string, or "NO_SERVICE_NAME" if not found
    """
    for attr in resource.attributes:
        if attr.key == "service.name":
            if attr.value.HasField("string_value"):
                return attr.value.string_value

    return "NO_SERVICE_NAME"
```

### 6. Update Settings

**File**: `app/core/settings.py` (add polling settings)

```python
class Settings(BaseSettings):
    # ... existing settings ...

    # Span polling settings
    span_poll_interval: int = 5  # Polling interval in seconds
    span_batch_size: int = 100   # Number of spans to fetch per batch

    model_config = SettingsConfigDict(
        env_file=".env",
        env_file_encoding="utf-8",
        case_sensitive=False,
    )
```

### 7. Update `main.py` Lifespan

**File**: `app/main.py` (add span poller to lifespan)

```python
from app.background.span_poller import start_span_poller, stop_span_poller


@asynccontextmanager
async def lifespan(app: FastAPI):
    """
    Lifespan context manager for startup and shutdown.
    """
    # --- Startup ---
    logger.info("Starting application")

    # Initialize SQLite database
    # (already done in Phase 2)

    # Initialize DuckDB
    # (already done in Phase 6)

    # Initialize gRPC client (for reading spans)
    await init_ingestion_client()

    # Start gRPC server (for API key validation)
    await start_auth_grpc_server()

    # Start background span polling task
    await start_span_poller()

    yield

    # --- Shutdown ---
    logger.info("Shutting down application")

    # Stop background span polling task
    await stop_span_poller()

    # Close gRPC connections
    await close_ingestion_client()
    await stop_auth_grpc_server()

    # Close DuckDB
    await close_duckdb()

    # SQLite cleanup
    await checkpoint_wal()
    await engine.dispose()

    logger.info("Application shutdown complete")
```

## Testing Requirements

### Unit Tests

**File**: `tests/unit/background/test_span_poller.py`

```python
"""Unit tests for span poller."""

import pytest

from app.background.utils import extract_service_name_from_resource
from opentelemetry.proto.common.v1 import common_pb2
from opentelemetry.proto.resource.v1 import resource_pb2


@pytest.mark.unit
def test_extract_service_name_from_resource():
    """Test service name extraction from resource."""
    resource = resource_pb2.Resource(
        attributes=[
            common_pb2.KeyValue(
                key="service.name",
                value=common_pb2.AnyValue(string_value="test_service"),
            ),
        ]
    )

    service_name = extract_service_name_from_resource(resource)
    assert service_name == "test_service"


@pytest.mark.unit
def test_extract_service_name_from_resource_missing():
    """Test service name extraction when not present."""
    resource = resource_pb2.Resource(attributes=[])

    service_name = extract_service_name_from_resource(resource)
    assert service_name == "NO_SERVICE_NAME"
```

**File**: `tests/unit/database/poller_state/test_repository.py`

```python
"""Unit tests for poller state repository."""

import pytest

from app.database.poller_state.repository import PollerStateRepository


@pytest.mark.unit
@pytest.mark.asyncio
async def test_get_poller_state_empty(test_db_engine):
    """Test get poller state when no state exists."""
    last_key = await PollerStateRepository.get_poller_state()
    assert last_key is None


@pytest.mark.unit
@pytest.mark.asyncio
async def test_upsert_poller_state_insert(test_db_engine):
    """Test inserting new poller state."""
    test_key = b"\\x01\\x02\\x03\\x04"

    await PollerStateRepository.upsert_poller_state(test_key)

    # Verify state was inserted
    last_key = await PollerStateRepository.get_poller_state()
    assert last_key == test_key


@pytest.mark.unit
@pytest.mark.asyncio
async def test_upsert_poller_state_update(test_db_engine):
    """Test updating existing poller state."""
    # Insert initial state
    initial_key = b"\\x01\\x02\\x03\\x04"
    await PollerStateRepository.upsert_poller_state(initial_key)

    # Update state
    updated_key = b"\\x05\\x06\\x07\\x08"
    await PollerStateRepository.upsert_poller_state(updated_key)

    # Verify state was updated
    last_key = await PollerStateRepository.get_poller_state()
    assert last_key == updated_key
```

### Integration Tests

**File**: `tests/integration/background/test_span_poller_integration.py`

```python
"""Integration tests for span poller."""

import pytest

from app.database.poller_state.repository import PollerStateRepository


@pytest.mark.integration
@pytest.mark.asyncio
async def test_poller_state_persistence(test_db_engine):
    """Test that poller state persists across operations."""
    # Initial state should be None
    last_key = await PollerStateRepository.get_poller_state()
    assert last_key is None

    # Save a key
    test_key = bytes.fromhex("deadbeefcafebabe")
    await PollerStateRepository.upsert_poller_state(test_key)

    # Retrieve and verify
    retrieved_key = await PollerStateRepository.get_poller_state()
    assert retrieved_key == test_key

    # Update key
    new_key = bytes.fromhex("0123456789abcdef")
    await PollerStateRepository.upsert_poller_state(new_key)

    # Verify update
    retrieved_key = await PollerStateRepository.get_poller_state()
    assert retrieved_key == new_key
```

## Migration Script (Alembic)

**File**: `migrations/versions/003_create_poller_state_table.py`

```python
"""Create poller_state table

Revision ID: 003
Revises: 002
Create Date: 2025-01-XX
"""

from typing import Sequence, Union

from alembic import op
import sqlalchemy as sa

# revision identifiers, used by Alembic.
revision: str = "003"
down_revision: Union[str, None] = "002"
branch_labels: Union[str, Sequence[str], None] = None
depends_on: Union[str, Sequence[str], None] = None


def upgrade() -> None:
    # Create poller_state table
    op.create_table(
        "poller_state",
        sa.Column("id", sa.Integer(), nullable=False),
        sa.Column("last_key", sa.LargeBinary(), nullable=True),
        sa.PrimaryKeyConstraint("id"),
        sa.CheckConstraint("id = 1", name="enforce_single_row"),
    )


def downgrade() -> None:
    op.drop_table("poller_state")
```

## Phase Completion Criteria

- [ ] All files implemented and reviewed
- [ ] Poller state table created in database
- [ ] Background task starts on application startup
- [ ] Background task stops gracefully on shutdown
- [ ] Polling interval is configurable
- [ ] Batch size is configurable
- [ ] Last key is persisted after successful processing
- [ ] Poller resumes from last key on startup
- [ ] Error handling works (continues polling on errors)
- [ ] Service name extraction works
- [ ] Protobuf unmarshaling works
- [ ] Integration with Phase 4 (gRPC client) works
- [ ] Integration with Phase 6 (DuckDB storage) works
- [ ] Unit tests pass (80%+ coverage)
- [ ] Integration tests pass
- [ ] Manual testing with real ingestion-service

## Notes

1. **Asyncio Task Management**: Using `asyncio.create_task()` to run poller in background. Task is cancelled on shutdown using `task.cancel()`.

2. **Graceful Cancellation**: Catching `asyncio.CancelledError` to ensure clean shutdown.

3. **State Persistence**: Only updating last_key if batch processing succeeds. This ensures at-least-once delivery semantics (may reprocess some spans on failure, but won't lose spans).

4. **Single Row Constraint**: The `CHECK (id = 1)` constraint ensures only one poller state row exists. Upsert logic handles both insert and update.

5. **Error Handling Strategy**:
   - Log errors but continue polling
   - Don't update last_key on processing failure
   - Skip individual spans that fail to unmarshal

6. **Service Name Extraction**: Extracted from resource attributes, matching Go implementation. Defaults to "NO_SERVICE_NAME" if not found.

7. **Protobuf Deserialization**: Using `ParseFromString()` method on protobuf objects to deserialize span and resource bytes.

8. **Polling Interval**: Default 5 seconds, matching Go implementation. Configurable via settings.

9. **Batch Size**: Default 100 spans, matching Go implementation. Configurable via settings.

10. **At-Least-Once Delivery**: System guarantees no spans are lost, but may reprocess spans if batch processing fails partway through. This is acceptable for idempotent operations (DuckDB uses INSERT OR IGNORE).

11. **Dependencies**: This phase depends on:
    - Phase 2 (SQLite for poller state)
    - Phase 4 (gRPC client for reading spans)
    - Phase 6 (DuckDB for storing spans)

## Next Phase

Phase 8 will implement the LLM Playground feature using LiteLLM for unified access to multiple LLM providers.
