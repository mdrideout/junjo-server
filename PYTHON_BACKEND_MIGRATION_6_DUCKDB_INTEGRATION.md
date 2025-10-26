# Phase 6: DuckDB Integration

## Overview

This phase implements DuckDB integration for storing and querying OpenTelemetry span data. DuckDB provides:
- **Columnar Storage**: Efficient analytics queries on span data
- **JSON Support**: Native JSON columns for complex nested data
- **Embedded Database**: No separate server process needed
- **SQL Analytics**: Full SQL support for complex aggregations

The system stores two types of data:
1. **Spans**: Complete OpenTelemetry spans with Junjo-specific fields
2. **State Patches**: Workflow state changes extracted from span events

## Current Go Implementation Analysis

**Database Path** (from `/backend/db_duckdb/duckdb_init.go:26`):
```go
dbPath := "/dbdata/duckdb/otel_data.db"
```

**Initialization** (from `/backend/db_duckdb/duckdb_init.go:24-50`):
1. Open connection to DuckDB file
2. Ping database to verify connection
3. Initialize tables if they don't exist:
   - `spans` table (from embedded SQL schema)
   - `state_patches` table (from embedded SQL schema)

**Schemas** (from `/backend/db_duckdb/otel_spans/*.sql`):

**spans table**:
```sql
CREATE TABLE spans (
  -- OpenTelemetry Standard Attributes
  trace_id VARCHAR(32) NOT NULL,
  span_id VARCHAR(16) NOT NULL,
  parent_span_id VARCHAR(16),
  service_name VARCHAR NOT NULL,
  name VARCHAR,
  kind VARCHAR,
  start_time TIMESTAMPTZ NOT NULL,
  end_time TIMESTAMPTZ NOT NULL,
  status_code VARCHAR,
  status_message VARCHAR,
  attributes_json JSON,
  events_json JSON,
  links_json JSON,
  trace_flags INTEGER,
  trace_state VARCHAR,
  -- Junjo Fields
  junjo_id VARCHAR,
  junjo_parent_id VARCHAR,
  junjo_span_type VARCHAR,
  -- Workflow State (Workflow-level spans only)
  junjo_wf_state_start JSON,
  junjo_wf_state_end JSON,
  junjo_wf_graph_structure JSON,
  junjo_wf_store_id VARCHAR,
  PRIMARY KEY (trace_id, span_id)
);
```

**state_patches table**:
```sql
CREATE TABLE state_patches (
  patch_id VARCHAR PRIMARY KEY,
  service_name VARCHAR NOT NULL,
  trace_id VARCHAR(32) NOT NULL,
  span_id VARCHAR(16) NOT NULL,
  workflow_id VARCHAR NOT NULL,
  node_id VARCHAR NOT NULL,
  event_time TIMESTAMPTZ NOT NULL,
  patch_json JSON NOT NULL,
  patch_store_id VARCHAR NOT NULL,
  FOREIGN KEY (trace_id, span_id) REFERENCES spans (trace_id, span_id)
);
```

**Data Processing** (from `/backend/telemetry/otel_span_processor.go`):
- Converts protobuf spans to DuckDB-compatible format
- Hex-encodes trace_id, span_id, parent_span_id
- Converts nanosecond timestamps to Python datetime
- Marshals attributes/events/links to JSON
- Extracts Junjo-specific attributes to dedicated columns
- Inserts spans and state patches in a transaction
- Uses `INSERT OR IGNORE` for idempotency

## Python Implementation

### Directory Structure

```
python_backend/
├── duckdb_schemas/              # SQL schema files
│   ├── spans_schema.sql         # Copy from Go backend
│   └── state_patches_schema.sql # Copy from Go backend
└── app/
    ├── database/
    │   └── duckdb_config.py     # DuckDB connection management
    ├── features/
    │   └── telemetry/
    │       ├── __init__.py
    │       ├── span_processor.py   # Process and insert spans
    │       └── converters.py       # Protobuf → JSON converters
    └── tests/
        ├── unit/
        │   └── features/
        │       └── telemetry/
        │           ├── test_converters.py
        │           └── test_span_processor.py
        └── integration/
            └── features/
                └── telemetry/
                    └── test_duckdb_integration.py
```

### 1. Copy Schema Files

**Action**: Copy SQL schema files from Go backend

```bash
# From junjo-server root
mkdir -p python_backend/duckdb_schemas
cp backend/db_duckdb/otel_spans/spans_schema.sql python_backend/duckdb_schemas/
cp backend/db_duckdb/otel_spans/state_patches_schema.sql python_backend/duckdb_schemas/
```

### 2. DuckDB Connection Management

**File**: `app/database/duckdb_config.py`

```python
"""
DuckDB connection management for OTEL span analytics.

Mirrors the Go implementation: /backend/db_duckdb/duckdb_init.go
"""

from pathlib import Path
from typing import Optional

import duckdb

from app.core.logger import logger
from app.core.settings import settings

# Global DuckDB connection (managed through lifespan)
_duckdb_conn: Optional[duckdb.DuckDBPyConnection] = None


def get_duckdb_connection() -> duckdb.DuckDBPyConnection:
    """
    Get the global DuckDB connection.

    Returns:
        DuckDB connection

    Raises:
        RuntimeError: If connection not initialized
    """
    if _duckdb_conn is None:
        raise RuntimeError("DuckDB connection not initialized. Call init_duckdb() first.")
    return _duckdb_conn


async def init_duckdb():
    """
    Initialize DuckDB connection and create tables.

    Mirrors Go: Connect() and initializeTables()
    """
    global _duckdb_conn

    db_path = settings.duckdb_path
    logger.info(f"Connecting to DuckDB at {db_path}")

    # Ensure parent directory exists
    db_path_obj = Path(db_path)
    db_path_obj.parent.mkdir(parents=True, exist_ok=True)

    # Connect to DuckDB
    _duckdb_conn = duckdb.connect(str(db_path))
    logger.info("DuckDB connection established")

    # Initialize tables
    await initialize_tables()


async def close_duckdb():
    """
    Close DuckDB connection.

    Mirrors Go: Close()
    """
    global _duckdb_conn

    if _duckdb_conn:
        _duckdb_conn.close()
        _duckdb_conn = None
        logger.info("DuckDB connection closed")


async def initialize_tables():
    """
    Initialize DuckDB tables if they don't exist.

    Mirrors Go: initializeTables()
    """
    conn = get_duckdb_connection()

    # Get schema file paths
    repo_root = Path(__file__).parent.parent.parent
    spans_schema_path = repo_root / "duckdb_schemas" / "spans_schema.sql"
    state_patches_schema_path = repo_root / "duckdb_schemas" / "state_patches_schema.sql"

    # Read schemas
    with open(spans_schema_path) as f:
        spans_schema = f.read()
    with open(state_patches_schema_path) as f:
        state_patches_schema = f.read()

    # Initialize tables
    init_table(conn, "spans", spans_schema)
    init_table(conn, "state_patches", state_patches_schema)


def init_table(conn: duckdb.DuckDBPyConnection, table_name: str, schema: str):
    """
    Initialize a table if it does not exist.

    Mirrors Go: initTable()

    Args:
        conn: DuckDB connection
        table_name: Name of table to check/create
        schema: SQL schema for table creation
    """
    # Check if table exists
    result = conn.execute(
        """
        SELECT EXISTS (
            SELECT 1 FROM information_schema.tables
            WHERE table_name = ?
        )
        """,
        [table_name],
    ).fetchone()

    table_exists = result[0] if result else False

    if not table_exists:
        # Create table
        conn.execute(schema)
        logger.info(f"DuckDB table created: {table_name}")
    else:
        logger.debug(f"DuckDB table already exists: {table_name}")
```

### 3. Protobuf to JSON Converters

**File**: `app/features/telemetry/converters.py`

```python
"""
Converters for OpenTelemetry protobuf data to JSON.

Mirrors the Go implementation: /backend/telemetry/otel_span_processor.go
"""

import json
from typing import Any, Dict, List

from opentelemetry.proto.common.v1 import common_pb2
from opentelemetry.proto.trace.v1 import trace_pb2


def convert_kind(kind: int) -> str:
    """
    Convert OTLP span kind integer to string.

    Mirrors Go: convertKind()

    Args:
        kind: OTLP SpanKind enum value

    Returns:
        String representation of span kind
    """
    kind_map = {
        0: "UNSPECIFIED",
        1: "CLIENT",
        2: "SERVER",
        3: "INTERNAL",
        4: "PRODUCER",
        5: "CONSUMER",
    }
    return kind_map.get(kind, "UNSPECIFIED")


def extract_string_attribute(attributes: List[common_pb2.KeyValue], key: str) -> str:
    """
    Extract a string attribute from protobuf attributes.

    Mirrors Go: extractStringAttribute()

    Args:
        attributes: List of protobuf KeyValue attributes
        key: Attribute key to extract

    Returns:
        String value if found, empty string otherwise
    """
    for attr in attributes:
        if attr.key == key:
            if attr.value.HasField("string_value"):
                return attr.value.string_value
    return ""


def extract_json_attribute(attributes: List[common_pb2.KeyValue], key: str) -> str:
    """
    Extract a JSON-encoded string attribute.

    Mirrors Go: extractJSONAttribute()

    Args:
        attributes: List of protobuf KeyValue attributes
        key: Attribute key to extract

    Returns:
        JSON string if found, "{}" otherwise
    """
    for attr in attributes:
        if attr.key == key:
            if attr.value.HasField("string_value"):
                return attr.value.string_value
    return "{}"


def convert_anyvalue_to_python(value: common_pb2.AnyValue) -> Any:
    """
    Convert a protobuf AnyValue to Python object.

    Args:
        value: Protobuf AnyValue

    Returns:
        Python representation (str, int, float, bool, list, dict)
    """
    if value.HasField("string_value"):
        return value.string_value
    elif value.HasField("int_value"):
        return value.int_value
    elif value.HasField("double_value"):
        return value.double_value
    elif value.HasField("bool_value"):
        return value.bool_value
    elif value.HasField("array_value"):
        return [
            convert_anyvalue_to_python(item) for item in value.array_value.values
        ]
    elif value.HasField("kvlist_value"):
        return {
            kv.key: convert_anyvalue_to_python(kv.value)
            for kv in value.kvlist_value.values
        }
    elif value.HasField("bytes_value"):
        return value.bytes_value.hex()
    else:
        return None


def convert_attributes_to_json(attributes: List[common_pb2.KeyValue]) -> str:
    """
    Convert protobuf KeyValue attributes to JSON string.

    Mirrors Go: convertAttributesToJson()

    Args:
        attributes: List of protobuf KeyValue attributes

    Returns:
        JSON string representation
    """
    attr_dict: Dict[str, Any] = {}

    for attr in attributes:
        attr_dict[attr.key] = convert_anyvalue_to_python(attr.value)

    return json.dumps(attr_dict)


def convert_events_to_json(events: List[trace_pb2.Span.Event]) -> str:
    """
    Convert protobuf events to JSON string.

    Mirrors Go: convertEventsToJson()

    Args:
        events: List of protobuf Span.Event

    Returns:
        JSON string representation
    """
    event_list = []

    for event in events:
        event_dict = {
            "name": event.name,
            "timeUnixNano": event.time_unix_nano,
            "droppedAttributesCount": event.dropped_attributes_count,
            "attributes": json.loads(convert_attributes_to_json(event.attributes)),
        }
        event_list.append(event_dict)

    return json.dumps(event_list)


def convert_links_to_json(links: List[trace_pb2.Span.Link]) -> str:
    """
    Convert protobuf links to JSON string.

    Args:
        links: List of protobuf Span.Link

    Returns:
        JSON string representation
    """
    link_list = []

    for link in links:
        link_dict = {
            "traceId": link.trace_id.hex(),
            "spanId": link.span_id.hex(),
            "traceState": link.trace_state,
            "droppedAttributesCount": link.dropped_attributes_count,
            "attributes": json.loads(convert_attributes_to_json(link.attributes)),
        }
        link_list.append(link_dict)

    return json.dumps(link_list)
```

### 4. Span Processor

**File**: `app/features/telemetry/span_processor.py`

```python
"""
OpenTelemetry span processor for DuckDB storage.

Mirrors the Go implementation: /backend/telemetry/otel_span_processor.go
"""

import uuid
from datetime import datetime, timezone
from typing import List, Optional

from opentelemetry.proto.trace.v1 import trace_pb2

from app.core.logger import logger
from app.database.duckdb_config import get_duckdb_connection
from app.features.telemetry.converters import (
    convert_attributes_to_json,
    convert_events_to_json,
    convert_kind,
    convert_links_to_json,
    extract_json_attribute,
    extract_string_attribute,
)


def process_span(
    cursor,
    service_name: str,
    span: trace_pb2.Span,
) -> None:
    """
    Process a single OpenTelemetry span and insert into DuckDB.

    Mirrors Go: processSpan()

    Args:
        cursor: DuckDB cursor (within transaction)
        service_name: Service name for the span
        span: Protobuf Span object
    """
    # 1. Encode IDs (hex encoding)
    trace_id = span.trace_id.hex()
    span_id = span.span_id.hex()
    parent_span_id = span.parent_span_id.hex() if span.parent_span_id else None

    # 2. Timestamps (convert from nanoseconds)
    start_time = datetime.fromtimestamp(span.start_time_unix_nano / 1e9, tz=timezone.utc)
    end_time = datetime.fromtimestamp(span.end_time_unix_nano / 1e9, tz=timezone.utc)

    # 3. Standard Attributes
    kind_str = convert_kind(span.kind)
    status_code = span.status.code.name if span.HasField("status") else ""
    status_message = span.status.message if span.HasField("status") else ""

    # 4. Junjo Attributes (dedicated columns)
    junjo_span_type = extract_string_attribute(span.attributes, "junjo.span_type")
    junjo_parent_id = extract_string_attribute(span.attributes, "junjo.parent_id")
    junjo_id = extract_string_attribute(span.attributes, "junjo.id")

    workflow_id = ""
    if junjo_span_type == "workflow":
        workflow_id = extract_string_attribute(span.attributes, "junjo.id")

    node_id = ""
    if junjo_span_type == "node":
        node_id = extract_string_attribute(span.attributes, "junjo.id")

    # JSON attributes (dedicated columns for workflow/subflow spans)
    junjo_initial_state = "{}"
    junjo_final_state = "{}"
    junjo_graph_structure = "{}"
    junjo_wf_store_id = ""

    if junjo_span_type in ("workflow", "subflow"):
        junjo_initial_state = extract_json_attribute(
            span.attributes, "junjo.workflow.state.start"
        )
        junjo_final_state = extract_json_attribute(
            span.attributes, "junjo.workflow.state.end"
        )
        junjo_graph_structure = extract_json_attribute(
            span.attributes, "junjo.workflow.graph_structure"
        )
        junjo_wf_store_id = extract_string_attribute(
            span.attributes, "junjo.workflow.store.id"
        )

    # Filter out attributes that go in dedicated columns
    dedicated_keys = {
        "junjo.workflow_id",
        "node.id",
        "junjo.id",
        "junjo.parent_id",
        "junjo.span_type",
        "junjo.workflow.state.start",
        "junjo.workflow.state.end",
        "junjo.workflow.graph_structure",
        "junjo.workflow.store.id",
    }

    filtered_attributes = [
        attr for attr in span.attributes if attr.key not in dedicated_keys
    ]

    # Convert to JSON
    attributes_json = convert_attributes_to_json(filtered_attributes)
    events_json = convert_events_to_json(span.events)
    links_json = convert_links_to_json(span.links)

    trace_state = span.trace_state if span.trace_state else None

    # Insert into `spans` table (INSERT OR IGNORE for idempotency)
    span_insert_query = """
        INSERT OR IGNORE INTO spans (
            trace_id, span_id, parent_span_id, service_name, name, kind,
            start_time, end_time, status_code, status_message,
            attributes_json, events_json, links_json, trace_flags, trace_state,
            junjo_id, junjo_parent_id, junjo_span_type,
            junjo_wf_state_start, junjo_wf_state_end, junjo_wf_graph_structure, junjo_wf_store_id
        ) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?);
    """

    cursor.execute(
        span_insert_query,
        [
            trace_id,
            span_id,
            parent_span_id,
            service_name,
            span.name,
            kind_str,
            start_time,
            end_time,
            status_code,
            status_message,
            attributes_json,
            events_json,
            links_json,
            span.flags,
            trace_state,
            junjo_id,
            junjo_parent_id,
            junjo_span_type,
            junjo_initial_state,
            junjo_final_state,
            junjo_graph_structure,
            junjo_wf_store_id,
        ],
    )

    # Insert State Patches (from "set_state" events)
    patch_insert_query = """
        INSERT OR IGNORE INTO state_patches (
            patch_id, service_name, trace_id, span_id, workflow_id, node_id,
            event_time, patch_json, patch_store_id
        ) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?);
    """

    for event in span.events:
        if event.name == "set_state":
            event_time = datetime.fromtimestamp(
                event.time_unix_nano / 1e9, tz=timezone.utc
            )
            patch_json = extract_json_attribute(
                event.attributes, "junjo.state_json_patch"
            )
            patch_store_id = extract_string_attribute(
                event.attributes, "junjo.store.id"
            )
            patch_id = str(uuid.uuid4())

            try:
                cursor.execute(
                    patch_insert_query,
                    [
                        patch_id,
                        service_name,
                        trace_id,
                        span_id,
                        workflow_id,
                        node_id,
                        event_time,
                        patch_json,
                        patch_store_id,
                    ],
                )
            except Exception as e:
                logger.error(f"Error inserting patch: {e}")


async def batch_process_spans(service_name: str, spans: List[trace_pb2.Span]) -> None:
    """
    Process a batch of OpenTelemetry spans in a single transaction.

    Mirrors Go: BatchProcessSpans()

    Args:
        service_name: Service name for the spans
        spans: List of protobuf Span objects

    Raises:
        Exception: If processing fails
    """
    conn = get_duckdb_connection()

    # Begin transaction
    cursor = conn.cursor()
    cursor.execute("BEGIN TRANSACTION")

    try:
        for span in spans:
            process_span(cursor, service_name, span)

        # Commit transaction
        cursor.execute("COMMIT")
        logger.debug(f"Processed {len(spans)} spans for service: {service_name}")

    except Exception as e:
        # Rollback on error
        cursor.execute("ROLLBACK")
        logger.error(f"Error processing spans batch: {e}")
        raise
    finally:
        cursor.close()
```

### 5. Update Settings

**File**: `app/core/settings.py` (add DuckDB settings)

```python
class Settings(BaseSettings):
    # ... existing settings ...

    # DuckDB settings
    duckdb_path: str = "/dbdata/duckdb/otel_data.db"  # Path to DuckDB file

    model_config = SettingsConfigDict(
        env_file=".env",
        env_file_encoding="utf-8",
        case_sensitive=False,
    )
```

### 6. Update `main.py` Lifespan

**File**: `app/main.py` (add DuckDB initialization)

```python
from app.database.duckdb_config import close_duckdb, init_duckdb


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
    await init_duckdb()

    # Initialize gRPC client/server
    # (already done in Phase 4)

    # TODO: Start background span polling task (Phase 7)

    yield

    # --- Shutdown ---
    logger.info("Shutting down application")

    # Stop background tasks
    # TODO: Cancel span polling task (Phase 7)

    # Close gRPC connections
    await close_ingestion_client()
    await stop_auth_grpc_server()

    # Close DuckDB
    await close_duckdb()

    # Database cleanup
    await checkpoint_wal()
    await engine.dispose()

    logger.info("Application shutdown complete")
```

## Testing Requirements

### Unit Tests

**File**: `tests/unit/features/telemetry/test_converters.py`

```python
"""Unit tests for protobuf converters."""

import json

import pytest
from opentelemetry.proto.common.v1 import common_pb2
from opentelemetry.proto.trace.v1 import trace_pb2

from app.features.telemetry.converters import (
    convert_attributes_to_json,
    convert_events_to_json,
    convert_kind,
    extract_string_attribute,
)


@pytest.mark.unit
def test_convert_kind():
    """Test span kind conversion."""
    assert convert_kind(0) == "UNSPECIFIED"
    assert convert_kind(1) == "CLIENT"
    assert convert_kind(2) == "SERVER"
    assert convert_kind(3) == "INTERNAL"
    assert convert_kind(4) == "PRODUCER"
    assert convert_kind(5) == "CONSUMER"
    assert convert_kind(999) == "UNSPECIFIED"  # Unknown


@pytest.mark.unit
def test_extract_string_attribute():
    """Test string attribute extraction."""
    attributes = [
        common_pb2.KeyValue(
            key="test.key",
            value=common_pb2.AnyValue(string_value="test_value"),
        ),
    ]

    result = extract_string_attribute(attributes, "test.key")
    assert result == "test_value"

    # Non-existent key
    result = extract_string_attribute(attributes, "nonexistent")
    assert result == ""


@pytest.mark.unit
def test_convert_attributes_to_json():
    """Test attributes to JSON conversion."""
    attributes = [
        common_pb2.KeyValue(
            key="string_attr",
            value=common_pb2.AnyValue(string_value="test"),
        ),
        common_pb2.KeyValue(
            key="int_attr",
            value=common_pb2.AnyValue(int_value=42),
        ),
        common_pb2.KeyValue(
            key="bool_attr",
            value=common_pb2.AnyValue(bool_value=True),
        ),
    ]

    result = convert_attributes_to_json(attributes)
    result_dict = json.loads(result)

    assert result_dict["string_attr"] == "test"
    assert result_dict["int_attr"] == 42
    assert result_dict["bool_attr"] is True


@pytest.mark.unit
def test_convert_events_to_json():
    """Test events to JSON conversion."""
    events = [
        trace_pb2.Span.Event(
            name="test_event",
            time_unix_nano=1234567890,
            attributes=[
                common_pb2.KeyValue(
                    key="event_attr",
                    value=common_pb2.AnyValue(string_value="value"),
                ),
            ],
        ),
    ]

    result = convert_events_to_json(events)
    result_list = json.loads(result)

    assert len(result_list) == 1
    assert result_list[0]["name"] == "test_event"
    assert result_list[0]["timeUnixNano"] == 1234567890
    assert result_list[0]["attributes"]["event_attr"] == "value"
```

**File**: `tests/unit/features/telemetry/test_span_processor.py`

```python
"""Unit tests for span processor."""

import pytest
from opentelemetry.proto.common.v1 import common_pb2
from opentelemetry.proto.trace.v1 import trace_pb2

from app.features.telemetry.span_processor import batch_process_spans


@pytest.mark.unit
@pytest.mark.asyncio
async def test_batch_process_spans(duckdb_connection):
    """Test batch span processing."""
    # Create test span
    span = trace_pb2.Span(
        trace_id=bytes.fromhex("0123456789abcdef0123456789abcdef"),
        span_id=bytes.fromhex("0123456789abcdef"),
        name="test_span",
        kind=trace_pb2.Span.SPAN_KIND_INTERNAL,
        start_time_unix_nano=1234567890000000000,
        end_time_unix_nano=1234567891000000000,
        attributes=[
            common_pb2.KeyValue(
                key="test.attribute",
                value=common_pb2.AnyValue(string_value="test_value"),
            ),
        ],
    )

    # Process spans
    await batch_process_spans("test_service", [span])

    # Verify span was inserted
    conn = duckdb_connection
    result = conn.execute(
        "SELECT * FROM spans WHERE trace_id = ?",
        ["0123456789abcdef0123456789abcdef"],
    ).fetchone()

    assert result is not None
    assert result[3] == "test_service"  # service_name column
```

### Integration Tests

**File**: `tests/integration/features/telemetry/test_duckdb_integration.py`

```python
"""Integration tests for DuckDB span storage."""

import json

import pytest
from opentelemetry.proto.common.v1 import common_pb2
from opentelemetry.proto.trace.v1 import trace_pb2

from app.features.telemetry.span_processor import batch_process_spans


@pytest.mark.integration
@pytest.mark.asyncio
async def test_span_storage_end_to_end(duckdb_connection):
    """Test complete span storage flow."""
    # Create test span with various attribute types
    span = trace_pb2.Span(
        trace_id=bytes.fromhex("deadbeefdeadbeefdeadbeefdeadbeef"),
        span_id=bytes.fromhex("cafebabecafebabe"),
        parent_span_id=bytes.fromhex("1234567890abcdef"),
        name="integration_test_span",
        kind=trace_pb2.Span.SPAN_KIND_SERVER,
        start_time_unix_nano=1700000000000000000,
        end_time_unix_nano=1700000001000000000,
        attributes=[
            common_pb2.KeyValue(
                key="http.method",
                value=common_pb2.AnyValue(string_value="POST"),
            ),
            common_pb2.KeyValue(
                key="http.status_code",
                value=common_pb2.AnyValue(int_value=200),
            ),
            common_pb2.KeyValue(
                key="junjo.span_type",
                value=common_pb2.AnyValue(string_value="node"),
            ),
            common_pb2.KeyValue(
                key="junjo.id",
                value=common_pb2.AnyValue(string_value="test_node_123"),
            ),
        ],
        events=[
            trace_pb2.Span.Event(
                name="test_event",
                time_unix_nano=1700000000500000000,
                attributes=[
                    common_pb2.KeyValue(
                        key="event.key",
                        value=common_pb2.AnyValue(string_value="event_value"),
                    ),
                ],
            ),
        ],
    )

    # Process span
    await batch_process_spans("integration_test_service", [span])

    # Verify span in database
    conn = duckdb_connection
    result = conn.execute(
        "SELECT * FROM spans WHERE trace_id = ?",
        ["deadbeefdeadbeefdeadbeefdeadbeef"],
    ).fetchone()

    assert result is not None
    assert result[3] == "integration_test_service"  # service_name
    assert result[6] == "SERVER"  # kind
    assert result[17] == "node"  # junjo_span_type
    assert result[15] == "test_node_123"  # junjo_id

    # Verify attributes JSON
    attributes_json = json.loads(result[10])
    assert attributes_json["http.method"] == "POST"
    assert attributes_json["http.status_code"] == 200

    # Verify events JSON
    events_json = json.loads(result[11])
    assert len(events_json) == 1
    assert events_json[0]["name"] == "test_event"
```

## Dependencies

Add to `pyproject.toml`:

```toml
[project]
dependencies = [
    # ... existing dependencies ...
    "duckdb>=0.10.0",                          # DuckDB Python library
    "opentelemetry-proto>=1.24.0",             # OpenTelemetry protobuf definitions
]
```

## Phase Completion Criteria

- [ ] All files implemented and reviewed
- [ ] DuckDB connection management works
- [ ] Schema files copied and tables created
- [ ] Span processing works (protobuf → DuckDB)
- [ ] State patch extraction works
- [ ] JSON conversion works for all OTLP types
- [ ] Transaction handling works correctly
- [ ] Idempotency works (INSERT OR IGNORE)
- [ ] Unit tests pass (80%+ coverage)
- [ ] Integration tests pass
- [ ] DuckDB file created at correct path
- [ ] Manual testing with real spans

## Notes

1. **Python DuckDB Library**: Using native Python DuckDB library (`duckdb` package) instead of SQLAlchemy. DuckDB Python API is simpler and more direct.

2. **Transaction Management**: Using explicit BEGIN TRANSACTION / COMMIT / ROLLBACK like Go implementation.

3. **Schema Management**: Copying exact SQL schemas from Go backend to ensure compatibility. Schemas use DuckDB's native syntax (not PostgreSQL or SQLite specific).

4. **Protobuf Handling**: Using `opentelemetry-proto` package which provides pre-generated Python protobuf classes for OTLP.

5. **Hex Encoding**: IDs (trace_id, span_id) are hex-encoded strings, matching Go implementation.

6. **Timestamp Conversion**: Converting from nanoseconds (OTLP) to Python datetime with UTC timezone.

7. **JSON Columns**: DuckDB has native JSON type support, making queries on nested data efficient.

8. **Idempotency**: Using `INSERT OR IGNORE` to prevent duplicate span insertion if reprocessing occurs.

9. **State Patches**: Extracted from span events with name "set_state", using UUID for patch_id.

10. **File Path**: Default `/dbdata/duckdb/otel_data.db` matches Go implementation. Parent directory is created if it doesn't exist.

11. **Performance**: DuckDB is designed for analytics queries. Future phases can add API endpoints for querying span data.

## Next Phase

Phase 7 will implement the background span polling task that:
- Reads spans from ingestion-service via gRPC (Phase 4)
- Processes and stores them in DuckDB (Phase 6)
- Tracks polling state in SQLite (Phase 2)
