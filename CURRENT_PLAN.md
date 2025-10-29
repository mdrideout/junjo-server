# Phase 6: DuckDB Integration - Part 1 (Schema & Indexing)

## Overview
Reorganize database structure and implement DuckDB span indexing foundation. **NOT** implementing gRPC polling or query endpoints yet (those are next).

---

## Part 1: Update Project Documentation

### File: `PYTHON_BACKEND_MIGRATION_0_Master.md`

**Update "Status" section** (top of file):
```markdown
> **Status**: **Phase 5 Complete - API Keys CRUD Working**
> **Current**: Phase 6 in progress - DuckDB Integration
```

**Update "What's Been Completed"**:
```markdown
✅ **Phase 5: API Keys Feature** (COMPLETE)
- Full CRUD implementation: POST, GET, DELETE /api_keys
- Router → Service → Repository pattern
- 25 comprehensive tests (all passing)
- E2E integration with frontend confirmed
- Uses ID-based deletion (more secure than Go's key-based)

✅ **gRPC Internal Auth Service** (COMPLETE)
- Concurrent FastAPI + gRPC server on port 50053
- ValidateApiKey service for ingestion-service
- 8 integration & concurrency tests passing
- Critical bug fixed (None check)
- E2E confirmed working

✅ **Go Backend Migration** (COMPLETE)
- Go backend disabled in docker-compose
- All services depend on Python backend
- Frontend routing updated
- E2E flow confirmed with real traffic

**Test Summary: 60 tests passing**
```

**Update "Next Steps"**:
```markdown
### Next Steps (Phase 6: DuckDB Integration)

**In Progress:**
- DuckDB schema setup (spans + state_patches tables)
- Span indexing service/repository
- gRPC poller for ingestion service (TODO)
- Query API endpoints (TODO)
```

---

## Part 2: Reorganize Database Structure

### Step 1: Move SQLite to `app/db_sqlite/`

**Move entire directory**:
```bash
mv app/database/ app/db_sqlite/
```

**Files affected** (14 files):
- `app/db_sqlite/__init__.py`
- `app/db_sqlite/base.py`
- `app/db_sqlite/db_config.py` ← Main config
- `app/db_sqlite/models.py`
- `app/db_sqlite/README.md`
- `app/db_sqlite/users/*` (7 files)
- `app/db_sqlite/api_keys/*` (7 files)

### Step 2: Update all imports from `app.database` → `app.db_sqlite`

**Files to update** (~20 files):
- `app/main.py` (line 78: import db_config, line 80: checkpoint_wal)
- `app/features/auth/router.py` (imports UserRepository)
- `app/features/auth/service.py` (imports UserRepository)
- `app/features/api_keys/router.py` (imports APIKeyRepository, schemas)
- `app/features/api_keys/service.py` (imports APIKeyRepository)
- `app/features/internal_auth/grpc_service.py` (imports APIKeyRepository)
- `app/grpc_server.py` (imports db_config)
- All test files that import from `app.database`

**Find/replace pattern**:
```python
# OLD
from app.database import db_config
from app.database.users.repository import UserRepository
from app.database.api_keys.repository import APIKeyRepository

# NEW
from app.db_sqlite import db_config
from app.db_sqlite.users.repository import UserRepository
from app.db_sqlite.api_keys.repository import APIKeyRepository
```

---

## Part 3: Create DuckDB Directory Structure

### Create: `app/db_duckdb/`

**New files**:
```
app/db_duckdb/
├── __init__.py
├── db_config.py           # DuckDB connection setup
├── models.py              # SQLAlchemy models (spans, state_patches)
├── repository.py          # Repository for span operations
├── schemas/
│   ├── spans_schema.sql
│   └── state_patches_schema.sql
└── README.md              # DuckDB architecture docs
```

---

## Part 4: Implement DuckDB Schema

### File: `app/db_duckdb/schemas/spans_schema.sql`

Embed exact schema from Go backend (`backend/db_duckdb/otel_spans/spans_schema.sql`):
```sql
CREATE TABLE IF NOT EXISTS spans (
  -- OpenTelemetry Standard Fields
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

  -- Junjo Custom Fields
  junjo_id VARCHAR,
  junjo_parent_id VARCHAR,
  junjo_span_type VARCHAR,

  -- Workflow State (workflow/subflow spans only)
  junjo_wf_state_start JSON,
  junjo_wf_state_end JSON,
  junjo_wf_graph_structure JSON,
  junjo_wf_store_id VARCHAR,

  PRIMARY KEY (trace_id, span_id)
);

CREATE INDEX IF NOT EXISTS idx_trace_id ON spans (trace_id);
CREATE INDEX IF NOT EXISTS idx_parent_span_id ON spans (parent_span_id);
CREATE INDEX IF NOT EXISTS idx_start_time ON spans (start_time);
CREATE INDEX IF NOT EXISTS idx_end_time ON spans (end_time);
CREATE INDEX IF NOT EXISTS idx_junjo_id ON spans (junjo_id);
CREATE INDEX IF NOT EXISTS idx_junjo_span_type ON spans (junjo_span_type);
```

### File: `app/db_duckdb/schemas/state_patches_schema.sql`

```sql
CREATE TABLE IF NOT EXISTS state_patches (
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

CREATE INDEX IF NOT EXISTS idx_state_patches_trace_id_span_id
  ON state_patches (trace_id, span_id);
CREATE INDEX IF NOT EXISTS idx_state_patches_workflow_id
  ON state_patches (workflow_id);
CREATE INDEX IF NOT EXISTS idx_state_patches_node_id
  ON state_patches (node_id);
CREATE INDEX IF NOT EXISTS idx_state_patches_event_time
  ON state_patches (event_time);
```

---

## Part 5: DuckDB Connection Configuration

### File: `app/db_duckdb/db_config.py`

```python
"""DuckDB configuration for OTEL span storage.

DuckDB is used for analytics-optimized storage of OpenTelemetry spans.
Unlike SQLite, DuckDB excels at analytical queries on large datasets.

Connection Pattern:
- Use duckdb-engine with SQLAlchemy async
- Single connection pool for all operations
- Path: /dbdata/duckdb/traces.duckdb (from env)

See: PYTHON_BACKEND_HIGH_CONCURRENCY_DB_PATTERN.md
"""

from loguru import logger
from sqlalchemy.ext.asyncio import async_sessionmaker, create_async_engine

from app.config.settings import settings

# Create async DuckDB engine
# Note: duckdb-engine supports async via duckdb_engine.aio
engine = create_async_engine(
    settings.database.duckdb_url,  # "duckdb+aiosqlite:///dbdata/duckdb/traces.duckdb"
    echo=settings.debug,
)

# Create async session factory
async_session = async_sessionmaker(
    engine,
    expire_on_commit=False,
)

logger.info(f"DuckDB engine created: {settings.database.duckdb_path}")

# Import models to register with metadata
from app.db_duckdb import models  # noqa: F401, E402


async def initialize_tables():
    """Initialize DuckDB tables and indexes.

    Creates spans and state_patches tables if they don't exist.
    Should be called on application startup.
    """
    from pathlib import Path
    import duckdb

    # For table creation, use synchronous DuckDB connection
    # (async not needed for DDL)
    conn = duckdb.connect(settings.database.duckdb_path)

    # Read and execute schema files
    schema_dir = Path(__file__).parent / "schemas"

    spans_schema = (schema_dir / "spans_schema.sql").read_text()
    state_patches_schema = (schema_dir / "state_patches_schema.sql").read_text()

    conn.execute(spans_schema)
    conn.execute(state_patches_schema)

    conn.close()
    logger.info("DuckDB tables initialized")
```

### File: `app/config/settings.py` (add DuckDB settings)

**Add to DatabaseSettings class**:
```python
class DatabaseSettings(BaseModel):
    sqlite_path: str = "/dbdata/sqlite/junjo-python.db"
    duckdb_path: str = "/dbdata/duckdb/traces.duckdb"

    @property
    def sqlite_url(self) -> str:
        return f"sqlite+aiosqlite:///{self.sqlite_path}"

    @property
    def duckdb_url(self) -> str:
        # Note: duckdb-engine uses aiosqlite compatibility layer
        return f"duckdb+aiosqlite:///{self.duckdb_path}"
```

---

## Part 6: Span Data Models

### File: `app/db_duckdb/models.py`

**SQLAlchemy models** (for type safety, even though we'll use raw SQL for inserts):
```python
"""DuckDB SQLAlchemy models for OTEL spans.

Note: These models are primarily for documentation and type hints.
Actual inserts use raw SQL for performance (batch inserts).
"""

from datetime import datetime
from sqlalchemy import Column, String, Integer, JSON, ForeignKeyConstraint, Index
from sqlalchemy.orm import declarative_base

Base = declarative_base()


class SpanTable(Base):
    __tablename__ = "spans"

    # Primary Key
    trace_id = Column(String(32), primary_key=True)
    span_id = Column(String(16), primary_key=True)

    # OTEL Standard Fields
    parent_span_id = Column(String(16), nullable=True)
    service_name = Column(String, nullable=False)
    name = Column(String, nullable=True)
    kind = Column(String, nullable=True)
    start_time = Column(String, nullable=False)  # TIMESTAMPTZ as string
    end_time = Column(String, nullable=False)
    status_code = Column(String, nullable=True)
    status_message = Column(String, nullable=True)
    attributes_json = Column(JSON, nullable=True)
    events_json = Column(JSON, nullable=True)
    links_json = Column(JSON, nullable=True)
    trace_flags = Column(Integer, nullable=True)
    trace_state = Column(String, nullable=True)

    # Junjo Custom Fields
    junjo_id = Column(String, nullable=True)
    junjo_parent_id = Column(String, nullable=True)
    junjo_span_type = Column(String, nullable=True)
    junjo_wf_state_start = Column(JSON, nullable=True)
    junjo_wf_state_end = Column(JSON, nullable=True)
    junjo_wf_graph_structure = Column(JSON, nullable=True)
    junjo_wf_store_id = Column(String, nullable=True)


class StatePatchTable(Base):
    __tablename__ = "state_patches"

    patch_id = Column(String, primary_key=True)
    service_name = Column(String, nullable=False)
    trace_id = Column(String(32), nullable=False)
    span_id = Column(String(16), nullable=False)
    workflow_id = Column(String, nullable=False)
    node_id = Column(String, nullable=False)
    event_time = Column(String, nullable=False)  # TIMESTAMPTZ as string
    patch_json = Column(JSON, nullable=False)
    patch_store_id = Column(String, nullable=False)

    __table_args__ = (
        ForeignKeyConstraint(
            ['trace_id', 'span_id'],
            ['spans.trace_id', 'spans.span_id']
        ),
    )
```

---

## Part 7: Span Repository (Insert Operations)

### File: `app/db_duckdb/repository.py`

**Implement batch span insert**:
```python
"""Repository for DuckDB span operations.

Handles batch insertion of OTEL spans and state patches.
Uses raw SQL for performance (DuckDB optimized for bulk inserts).
"""

from datetime import datetime
from typing import Any
from loguru import logger
import duckdb

from app.config.settings import settings


class SpanRepository:
    """Repository for span database operations."""

    @staticmethod
    def batch_insert_spans(spans: list[dict[str, Any]]) -> int:
        """Batch insert spans into DuckDB.

        Args:
            spans: List of span dictionaries with all fields

        Returns:
            Number of spans inserted (may be less than len(spans) due to duplicates)

        Note:
            Uses INSERT OR IGNORE for idempotency.
            Duplicate (trace_id, span_id) are silently skipped.
        """
        if not spans:
            return 0

        conn = duckdb.connect(settings.database.duckdb_path)

        try:
            # DuckDB supports INSERT OR IGNORE for duplicate PK
            sql = """
                INSERT OR IGNORE INTO spans (
                    trace_id, span_id, parent_span_id, service_name, name, kind,
                    start_time, end_time, status_code, status_message,
                    attributes_json, events_json, links_json, trace_flags, trace_state,
                    junjo_id, junjo_parent_id, junjo_span_type,
                    junjo_wf_state_start, junjo_wf_state_end,
                    junjo_wf_graph_structure, junjo_wf_store_id
                ) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
            """

            # Convert spans to tuples for bulk insert
            values = [
                (
                    span["trace_id"],
                    span["span_id"],
                    span.get("parent_span_id"),
                    span["service_name"],
                    span.get("name"),
                    span.get("kind"),
                    span["start_time"],
                    span["end_time"],
                    span.get("status_code"),
                    span.get("status_message"),
                    span.get("attributes_json"),
                    span.get("events_json"),
                    span.get("links_json"),
                    span.get("trace_flags"),
                    span.get("trace_state"),
                    span.get("junjo_id"),
                    span.get("junjo_parent_id"),
                    span.get("junjo_span_type"),
                    span.get("junjo_wf_state_start"),
                    span.get("junjo_wf_state_end"),
                    span.get("junjo_wf_graph_structure"),
                    span.get("junjo_wf_store_id"),
                )
                for span in spans
            ]

            conn.executemany(sql, values)
            inserted = conn.total_changes  # DuckDB-specific

            logger.info(f"Inserted {inserted} spans (attempted {len(spans)})")
            return inserted

        except Exception as e:
            logger.error(f"Failed to batch insert spans: {e}")
            raise
        finally:
            conn.close()

    @staticmethod
    def batch_insert_state_patches(patches: list[dict[str, Any]]) -> int:
        """Batch insert state patches into DuckDB.

        Args:
            patches: List of state patch dictionaries

        Returns:
            Number of patches inserted
        """
        if not patches:
            return 0

        conn = duckdb.connect(settings.database.duckdb_path)

        try:
            sql = """
                INSERT OR IGNORE INTO state_patches (
                    patch_id, service_name, trace_id, span_id,
                    workflow_id, node_id, event_time,
                    patch_json, patch_store_id
                ) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
            """

            values = [
                (
                    patch["patch_id"],
                    patch["service_name"],
                    patch["trace_id"],
                    patch["span_id"],
                    patch["workflow_id"],
                    patch["node_id"],
                    patch["event_time"],
                    patch["patch_json"],
                    patch["patch_store_id"],
                )
                for patch in patches
            ]

            conn.executemany(sql, values)
            inserted = conn.total_changes

            logger.info(f"Inserted {inserted} state patches (attempted {len(patches)})")
            return inserted

        except Exception as e:
            logger.error(f"Failed to batch insert state patches: {e}")
            raise
        finally:
            conn.close()
```

---

## Part 8: Span Processing Service (Placeholder)

### File: `app/db_duckdb/span_processor.py`

**Create placeholder** (will be implemented when we add gRPC polling):
```python
"""Span processor for converting OTLP protobuf to DuckDB-ready format.

TODO: Implement in next phase (gRPC polling integration)

Functions needed:
- process_span(otlp_span, service_name) -> dict
- extract_junjo_attributes(attributes) -> tuple[dict, dict]  # (junjo_fields, filtered_attrs)
- extract_state_patches(events, trace_id, span_id, workflow_id, node_id) -> list[dict]
- convert_otlp_attributes_to_json(attributes) -> dict
"""

# Placeholder - will implement with gRPC poller
pass
```

---

## Part 9: Dependencies

### File: `pyproject.toml`

**Add DuckDB dependencies**:
```toml
[tool.uv]
dependencies = [
    # ... existing dependencies ...
    "duckdb>=1.0.0",           # DuckDB Python driver
    "duckdb-engine>=0.13.0",   # SQLAlchemy dialect for DuckDB
]
```

---

## Part 10: Application Startup

### File: `app/main.py`

**Add DuckDB initialization to lifespan**:
```python
@asynccontextmanager
async def lifespan(app: FastAPI):
    # Startup
    logger.info("=" * 60)
    # ... existing startup logs ...

    # Initialize DuckDB tables
    from app.db_duckdb.db_config import initialize_tables
    await initialize_tables()
    logger.info("DuckDB tables initialized")

    # TODO: Start gRPC poller for ingestion service (Phase 6b)

    yield

    # Shutdown
    # ... existing shutdown code ...
```

---

## Part 11: Add TODO Comments

**Create**: `app/db_duckdb/README.md`

```markdown
# DuckDB Integration

## Status: Phase 6a Complete (Schema & Repository)

✅ **Implemented**:
- DuckDB connection configuration
- Table schemas (spans + state_patches)
- Repository for batch inserts
- Application startup integration

📋 **TODO (Phase 6b - Next)**:
1. **gRPC Client for Ingestion Service**
   - Create `app/ingestion_client/client.py`
   - Implement `read_spans(start_key, batch_size)` gRPC call
   - Connect to `junjo-server-ingestion:50052`

2. **OTLP Span Processor**
   - Implement `app/db_duckdb/span_processor.py`
   - Deserialize OTLP protobuf spans
   - Extract OTEL standard fields (trace_id, span_id, timestamps, etc.)
   - Extract Junjo custom fields (junjo_span_type, junjo_id, etc.)
   - Filter attributes_json (remove extracted Junjo fields)
   - Extract state patches from "set_state" events

3. **Background Poller**
   - Create `app/telemetry/ingestion_poller.py`
   - Async infinite loop (5 second interval)
   - Poll ingestion service for new spans
   - Process and insert into DuckDB
   - Track last key in SQLite `poller_state` table
   - Add to application lifespan

4. **Testing**
   - Unit tests for span processor
   - Integration tests for poller
   - Mock ingestion service responses

📋 **TODO (Phase 6c - Later)**:
5. **Query Endpoints** (6 REST APIs)
   - Create `app/features/otel/router.py`
   - Implement `/otel/span_service_names` endpoint
   - Implement `/otel/service/{service_name}/root_spans` endpoint
   - Implement `/otel/service/{service_name}/root_spans_filtered` endpoint
   - Implement `/otel/trace/{trace_id}/nested_spans` endpoint
   - Implement `/otel/trace/{trace_id}/span/{span_id}` endpoint
   - Implement `/otel/spans/type/workflow/{service_name}` endpoint

## Architecture

See Go backend reference: `/Users/matt/repos/junjo-server/backend/db_duckdb/`
```

---

## Summary

### Files Created (9):
1. `PYTHON_BACKEND_MIGRATION_0_Master.md` (updated)
2. `app/db_duckdb/__init__.py`
3. `app/db_duckdb/db_config.py`
4. `app/db_duckdb/models.py`
5. `app/db_duckdb/repository.py`
6. `app/db_duckdb/span_processor.py` (placeholder)
7. `app/db_duckdb/schemas/spans_schema.sql`
8. `app/db_duckdb/schemas/state_patches_schema.sql`
9. `app/db_duckdb/README.md`

### Files Moved:
- `app/database/` → `app/db_sqlite/` (entire directory)

### Files Modified (~20):
- `app/main.py` - Add DuckDB initialization
- `app/config/settings.py` - Add DuckDB settings
- `pyproject.toml` - Add DuckDB dependencies
- All files with `from app.database` imports → `from app.db_sqlite`

### Next Steps (NOT in this plan):
- Phase 6b: gRPC poller + span processor
- Phase 6c: Query endpoints

### Testing:
- Run existing tests to verify refactor: `uv run pytest -v`
- Expected: All 60 tests still passing after import changes
