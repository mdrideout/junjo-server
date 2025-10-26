# Phase 9: Remaining Features Migration

## Overview

This phase covers the remaining API endpoints and features that need to be migrated to Python. Primarily, this includes:
1. **OTEL Spans Query API**: Endpoints for querying span data from DuckDB
2. **Health/Status Endpoints**: System health checks
3. **Any other utility endpoints**

## OTEL Spans Query API

### Current Go Implementation

**Routes** (from `/backend/api/routes.go:11-16`):
```go
e.GET("/otel/span-service-names", otel.GetDistinctServiceNames)
e.GET("/otel/service/:serviceName/root-spans", otel.GetRootSpans)
e.GET("/otel/service/:serviceName/root-spans-filtered", otel.GetRootSpansFiltered)
e.GET("/otel/trace/:traceId/nested-spans", otel.GetNestedSpans)
e.GET("/otel/trace/:traceId/span/:spanId", otel.GetSpan)
e.GET("/otel/spans/type/workflow/:serviceName", otel.GetSpansTypeWorkflow)
```

**Implementation**:
- Queries DuckDB directly using embedded SQL files
- Returns JSON responses with span data
- Used by frontend for trace visualization

### Python Implementation

#### Directory Structure

```
python_backend/
├── sql_queries/                 # SQL query files (copy from Go backend)
│   ├── query_distinct_service_names.sql
│   ├── query_root_spans.sql
│   ├── query_root_spans_filtered.sql
│   ├── query_nested_spans.sql
│   ├── query_span.sql
│   └── query_spans_type_workflow.sql
└── app/
    ├── features/
    │   └── otel_api/
    │       ├── __init__.py
    │       ├── router.py      # FastAPI endpoints
    │       ├── service.py     # Query execution logic
    │       └── schemas.py     # Response schemas
    └── tests/
        ├── unit/
        │   └── features/
        │       └── otel_api/
        │           └── test_service.py
        └── integration/
            └── features/
                └── otel_api/
                    └── test_otel_endpoints.py
```

#### Implementation

**File**: `app/features/otel_api/schemas.py`

```python
"""
Schemas for OTEL API responses.
"""

from datetime import datetime
from typing import Any, Dict, List, Optional

from pydantic import BaseModel


class ServiceName(BaseModel):
    """Service name response."""

    service_name: str


class SpanSummary(BaseModel):
    """Span summary for lists."""

    trace_id: str
    span_id: str
    parent_span_id: Optional[str]
    service_name: str
    name: str
    kind: str
    start_time: datetime
    end_time: datetime
    duration_ms: float
    status_code: Optional[str]
    junjo_id: Optional[str]
    junjo_span_type: Optional[str]


class SpanDetail(BaseModel):
    """Complete span details."""

    trace_id: str
    span_id: str
    parent_span_id: Optional[str]
    service_name: str
    name: str
    kind: str
    start_time: datetime
    end_time: datetime
    status_code: Optional[str]
    status_message: Optional[str]
    attributes_json: Dict[str, Any]
    events_json: List[Dict[str, Any]]
    links_json: List[Dict[str, Any]]
    trace_flags: int
    trace_state: Optional[str]
    junjo_id: Optional[str]
    junjo_parent_id: Optional[str]
    junjo_span_type: Optional[str]
    junjo_wf_state_start: Optional[Dict[str, Any]]
    junjo_wf_state_end: Optional[Dict[str, Any]]
    junjo_wf_graph_structure: Optional[Dict[str, Any]]
    junjo_wf_store_id: Optional[str]
```

**File**: `app/features/otel_api/service.py`

```python
"""
Service for querying OTEL span data from DuckDB.
"""

import json
from pathlib import Path
from typing import Any, Dict, List

from app.core.logger import logger
from app.database.duckdb_config import get_duckdb_connection


class OTELQueryService:
    """Service for OTEL span queries."""

    @staticmethod
    def _load_query(query_name: str) -> str:
        """
        Load SQL query from file.

        Args:
            query_name: Query file name (without .sql extension)

        Returns:
            SQL query string
        """
        repo_root = Path(__file__).parent.parent.parent.parent
        query_path = repo_root / "sql_queries" / f"{query_name}.sql"

        with open(query_path) as f:
            return f.read()

    @staticmethod
    def get_service_names() -> List[str]:
        """
        Get distinct service names from spans.

        Returns:
            List of service names
        """
        conn = get_duckdb_connection()
        query = OTELQueryService._load_query("query_distinct_service_names")

        result = conn.execute(query).fetchall()
        return [row[0] for row in result]

    @staticmethod
    def get_root_spans(service_name: str, limit: int = 100) -> List[Dict[str, Any]]:
        """
        Get root spans for a service.

        Args:
            service_name: Service name
            limit: Maximum number of spans to return

        Returns:
            List of span dictionaries
        """
        conn = get_duckdb_connection()
        query = OTELQueryService._load_query("query_root_spans")

        result = conn.execute(query, [service_name, limit]).fetchall()

        # Convert to list of dicts
        columns = [desc[0] for desc in result.description] if result.description else []
        return [dict(zip(columns, row)) for row in result]

    @staticmethod
    def get_root_spans_filtered(
        service_name: str,
        start_time: str,
        end_time: str,
        limit: int = 100,
    ) -> List[Dict[str, Any]]:
        """
        Get filtered root spans for a service.

        Args:
            service_name: Service name
            start_time: Start time filter (ISO format)
            end_time: End time filter (ISO format)
            limit: Maximum number of spans to return

        Returns:
            List of span dictionaries
        """
        conn = get_duckdb_connection()
        query = OTELQueryService._load_query("query_root_spans_filtered")

        result = conn.execute(query, [service_name, start_time, end_time, limit]).fetchall()

        columns = [desc[0] for desc in result.description] if result.description else []
        return [dict(zip(columns, row)) for row in result]

    @staticmethod
    def get_nested_spans(trace_id: str) -> List[Dict[str, Any]]:
        """
        Get all spans for a trace (nested hierarchy).

        Args:
            trace_id: Trace ID

        Returns:
            List of span dictionaries
        """
        conn = get_duckdb_connection()
        query = OTELQueryService._load_query("query_nested_spans")

        result = conn.execute(query, [trace_id]).fetchall()

        columns = [desc[0] for desc in result.description] if result.description else []
        spans = [dict(zip(columns, row)) for row in result]

        # Parse JSON columns
        for span in spans:
            if span.get("attributes_json"):
                span["attributes_json"] = json.loads(span["attributes_json"])
            if span.get("events_json"):
                span["events_json"] = json.loads(span["events_json"])
            if span.get("links_json"):
                span["links_json"] = json.loads(span["links_json"])

        return spans

    @staticmethod
    def get_span(trace_id: str, span_id: str) -> Dict[str, Any]:
        """
        Get a single span by trace_id and span_id.

        Args:
            trace_id: Trace ID
            span_id: Span ID

        Returns:
            Span dictionary

        Raises:
            ValueError: If span not found
        """
        conn = get_duckdb_connection()
        query = OTELQueryService._load_query("query_span")

        result = conn.execute(query, [trace_id, span_id]).fetchone()

        if result is None:
            raise ValueError(f"Span not found: {trace_id}/{span_id}")

        columns = [desc[0] for desc in result.description] if result.description else []
        span = dict(zip(columns, result))

        # Parse JSON columns
        if span.get("attributes_json"):
            span["attributes_json"] = json.loads(span["attributes_json"])
        if span.get("events_json"):
            span["events_json"] = json.loads(span["events_json"])
        if span.get("links_json"):
            span["links_json"] = json.loads(span["links_json"])
        if span.get("junjo_wf_state_start"):
            span["junjo_wf_state_start"] = json.loads(span["junjo_wf_state_start"])
        if span.get("junjo_wf_state_end"):
            span["junjo_wf_state_end"] = json.loads(span["junjo_wf_state_end"])
        if span.get("junjo_wf_graph_structure"):
            span["junjo_wf_graph_structure"] = json.loads(span["junjo_wf_graph_structure"])

        return span

    @staticmethod
    def get_workflow_spans(service_name: str, limit: int = 100) -> List[Dict[str, Any]]:
        """
        Get workflow-type spans for a service.

        Args:
            service_name: Service name
            limit: Maximum number of spans to return

        Returns:
            List of span dictionaries
        """
        conn = get_duckdb_connection()
        query = OTELQueryService._load_query("query_spans_type_workflow")

        result = conn.execute(query, [service_name, limit]).fetchall()

        columns = [desc[0] for desc in result.description] if result.description else []
        return [dict(zip(columns, row)) for row in result]
```

**File**: `app/features/otel_api/router.py`

```python
"""
OTEL API router for querying span data.
"""

from typing import List

from fastapi import APIRouter, HTTPException, Query, status

from app.core.logger import logger
from app.features.auth.dependencies import CurrentUserEmail
from app.features.otel_api.service import OTELQueryService

router = APIRouter()


@router.get("/span-service-names")
async def get_service_names(current_user_email: CurrentUserEmail):
    """
    Get distinct service names.

    Auth required.
    """
    try:
        service_names = OTELQueryService.get_service_names()
        return {"service_names": service_names}
    except Exception as e:
        logger.error(f"Error getting service names: {e}")
        raise HTTPException(
            status_code=status.HTTP_500_INTERNAL_SERVER_ERROR,
            detail=str(e),
        )


@router.get("/service/{service_name}/root-spans")
async def get_root_spans(
    service_name: str,
    limit: int = Query(100, ge=1, le=1000),
    current_user_email: CurrentUserEmail = None,
):
    """
    Get root spans for a service.

    Auth required.
    """
    try:
        spans = OTELQueryService.get_root_spans(service_name, limit)
        return {"spans": spans}
    except Exception as e:
        logger.error(f"Error getting root spans: {e}")
        raise HTTPException(
            status_code=status.HTTP_500_INTERNAL_SERVER_ERROR,
            detail=str(e),
        )


@router.get("/service/{service_name}/root-spans-filtered")
async def get_root_spans_filtered(
    service_name: str,
    start_time: str = Query(..., description="Start time (ISO format)"),
    end_time: str = Query(..., description="End time (ISO format)"),
    limit: int = Query(100, ge=1, le=1000),
    current_user_email: CurrentUserEmail = None,
):
    """
    Get filtered root spans for a service.

    Auth required.
    """
    try:
        spans = OTELQueryService.get_root_spans_filtered(
            service_name, start_time, end_time, limit
        )
        return {"spans": spans}
    except Exception as e:
        logger.error(f"Error getting filtered root spans: {e}")
        raise HTTPException(
            status_code=status.HTTP_500_INTERNAL_SERVER_ERROR,
            detail=str(e),
        )


@router.get("/trace/{trace_id}/nested-spans")
async def get_nested_spans(trace_id: str, current_user_email: CurrentUserEmail):
    """
    Get all spans for a trace.

    Auth required.
    """
    try:
        spans = OTELQueryService.get_nested_spans(trace_id)
        return {"spans": spans}
    except Exception as e:
        logger.error(f"Error getting nested spans: {e}")
        raise HTTPException(
            status_code=status.HTTP_500_INTERNAL_SERVER_ERROR,
            detail=str(e),
        )


@router.get("/trace/{trace_id}/span/{span_id}")
async def get_span(
    trace_id: str,
    span_id: str,
    current_user_email: CurrentUserEmail,
):
    """
    Get a single span.

    Auth required.
    """
    try:
        span = OTELQueryService.get_span(trace_id, span_id)
        return span
    except ValueError as e:
        raise HTTPException(
            status_code=status.HTTP_404_NOT_FOUND,
            detail=str(e),
        )
    except Exception as e:
        logger.error(f"Error getting span: {e}")
        raise HTTPException(
            status_code=status.HTTP_500_INTERNAL_SERVER_ERROR,
            detail=str(e),
        )


@router.get("/spans/type/workflow/{service_name}")
async def get_workflow_spans(
    service_name: str,
    limit: int = Query(100, ge=1, le=1000),
    current_user_email: CurrentUserEmail = None,
):
    """
    Get workflow-type spans for a service.

    Auth required.
    """
    try:
        spans = OTELQueryService.get_workflow_spans(service_name, limit)
        return {"spans": spans}
    except Exception as e:
        logger.error(f"Error getting workflow spans: {e}")
        raise HTTPException(
            status_code=status.HTTP_500_INTERNAL_SERVER_ERROR,
            detail=str(e),
        )
```

**File**: `app/main.py` (add OTEL API router)

```python
# Add OTEL API router
from app.features.otel_api.router import router as otel_router
app.include_router(otel_router, prefix="/otel", tags=["otel"])
```

### Action Items

1. **Copy SQL Query Files**: Copy all `query_*.sql` files from `/backend/api/otel/` to `/python_backend/sql_queries/`

```bash
cp backend/api/otel/query_*.sql python_backend/sql_queries/
```

2. **Test Queries**: Verify all SQL queries work with DuckDB Python connector

## Health & Status Endpoints

### Implementation

**File**: `app/features/health/router.py`

```python
"""
Health check endpoints.
"""

from fastapi import APIRouter

router = APIRouter()


@router.get("/ping")
async def ping():
    """Simple health check endpoint."""
    return "pong"


@router.get("/health")
async def health():
    """
    Comprehensive health check.

    Checks:
    - Database connections (SQLite, DuckDB)
    - gRPC connections (ingestion client, auth server)
    """
    return {
        "status": "healthy",
        "services": {
            "sqlite": "ok",
            "duckdb": "ok",
            "grpc_client": "ok",
            "grpc_server": "ok",
        },
    }
```

**File**: `app/main.py` (add health router)

```python
from app.features.health.router import router as health_router
app.include_router(health_router, tags=["health"])
```

## Phase Completion Criteria

- [ ] All SQL query files copied to Python backend
- [ ] OTEL API endpoints implemented
- [ ] Health check endpoints implemented
- [ ] All endpoints return correct data
- [ ] JSON parsing works correctly for DuckDB JSON columns
- [ ] Error handling works
- [ ] Unit tests pass
- [ ] Integration tests pass
- [ ] Manual testing with frontend

## Testing

### Integration Tests

**File**: `tests/integration/features/otel_api/test_otel_endpoints.py`

```python
"""Integration tests for OTEL API endpoints."""

import pytest
from httpx import AsyncClient


@pytest.mark.integration
@pytest.mark.asyncio
async def test_get_service_names(authenticated_client: AsyncClient, duckdb_with_test_data):
    """Test GET /otel/span-service-names endpoint."""
    response = await authenticated_client.get("/otel/span-service-names")
    assert response.status_code == 200

    data = response.json()
    assert "service_names" in data
    assert isinstance(data["service_names"], list)


@pytest.mark.integration
@pytest.mark.asyncio
async def test_get_root_spans(authenticated_client: AsyncClient, duckdb_with_test_data):
    """Test GET /otel/service/{service_name}/root-spans endpoint."""
    response = await authenticated_client.get("/otel/service/test_service/root-spans")
    assert response.status_code == 200

    data = response.json()
    assert "spans" in data
    assert isinstance(data["spans"], list)
```

## Notes

1. **SQL Query Files**: Must be copied exactly from Go backend to maintain compatibility
2. **DuckDB Python API**: Uses native Python DuckDB connector
3. **JSON Parsing**: DuckDB returns JSON columns as strings, must parse them in Python
4. **Error Handling**: Wrap all queries in try/except for proper error responses
5. **Auth Required**: All OTEL endpoints require authentication
6. **Query Parameters**: Use FastAPI's `Query` for validation and documentation
7. **Response Format**: Match Go backend's response format for frontend compatibility

## Next Phase

Phase 10 will cover deployment strategy and cutover plan.
