# Timestamp Handling: Go Backend vs Python Backend

## Summary of Differences

| Aspect | Go Backend | Python Backend (CURRENT) | Issue |
|--------|-----------|--------------------------|-------|
| **Indexing** | ✅ Stores `time.Time` objects directly | ✅ Stores `datetime` objects directly | **SAME** - Both correct |
| **DuckDB Storage** | `TIMESTAMPTZ` with nanosecond precision | `TIMESTAMPTZ` with microsecond precision | Minor precision loss in Python (acceptable) |
| **Query Format** | Returns timestamps as-is (driver auto-converts) | ❌ Used `::VARCHAR` → invalid format | **FIXED** - Now uses `strftime()` |
| **JSON Serialization** | Echo auto-converts to RFC3339/ISO8601 | DuckDB `::VARCHAR` produced `2025-11-01 13:53:29.882897+00` | **FIXED** - Now produces ISO8601 |

---

## Detailed Analysis

### 1. INDEXING (Writing to DuckDB)

#### Go Backend (`otel_span_processor.go:163-164`)

```go
// Convert OTLP nanoseconds to Go time.Time
startTime := time.Unix(0, int64(span.StartTimeUnixNano)).UTC()
endTime := time.Unix(0, int64(span.EndTimeUnixNano)).UTC()

// Insert directly into DuckDB
_, err = tx.ExecContext(ctx, spanInsertQuery,
    traceID, spanID, parentSpanID, service_name, span.Name, kindStr,
    startTime, endTime,  // ← time.Time objects
    // ... other fields
)
```

**Precision**: Nanosecond (Go `time.Time` supports nanoseconds)

#### Python Backend (`span_processor.py:310-312`)

```python
# Convert OTLP nanoseconds to Python datetime
start_time = convert_otlp_timestamp(span.start_time_unix_nano)
end_time = convert_otlp_timestamp(span.end_time_unix_nano)

def convert_otlp_timestamp(ts_nano: int) -> datetime:
    """Convert OTLP uint64 nanoseconds to timezone-aware datetime."""
    return datetime.fromtimestamp(ts_nano / 1e9, tz=UTC)

# Insert directly into DuckDB
conn.execute(span_insert_query, (
    trace_id, span_id, parent_span_id, service_name, span.name, kind_str,
    start_time, end_time,  # ← datetime objects
    # ... other fields
))
```

**Precision**: Microsecond (Python `datetime` supports only microseconds)

**Result**:
- ✅ Both backends store native timestamp objects in DuckDB
- ✅ DuckDB `TIMESTAMPTZ` column stores them correctly
- ⚠️ Python loses 3 decimal places of nanosecond precision (acceptable for most use cases)

---

### 2. QUERYING (Reading from DuckDB)

#### Go Backend (`query_spans_type_workflow.sql`)

```sql
SELECT *
FROM spans
WHERE junjo_span_type = 'workflow'
  AND service_name = ?
ORDER BY start_time DESC;
```

**Query Result**:
- Returns `TIMESTAMPTZ` columns directly
- Go SQL driver (`database/sql`) automatically converts DuckDB `TIMESTAMPTZ` to `time.Time`
- Echo JSON encoder automatically serializes `time.Time` to RFC3339 format

**JSON Output** (automatic):
```json
{
  "start_time": "2025-11-01T13:53:29.882897+00:00",
  "end_time": "2025-11-01T13:53:35.277934+00:00"
}
```

✅ **Valid ISO 8601 format with**:
- `T` separator
- Microsecond precision
- Timezone offset with colon (`+00:00`)

---

#### Python Backend - ORIGINAL (BROKEN)

```sql
SELECT
    trace_id, span_id, parent_span_id, service_name, name, kind,
    start_time::VARCHAR as start_time,  -- ❌ WRONG FORMAT
    end_time::VARCHAR as end_time,      -- ❌ WRONG FORMAT
    -- ... other fields
FROM spans
WHERE junjo_span_type = 'workflow'
  AND service_name = ?
ORDER BY start_time DESC
LIMIT ?
```

**Problem**: DuckDB's `::VARCHAR` cast produces:
```
2025-11-01 13:53:29.882897+00
```

❌ **Invalid format**:
- Missing `T` separator (space instead)
- Missing colon in timezone (`+00` instead of `+00:00`)
- Fails Zod `.datetime({ offset: true })` validation
- Causes negative duration calculations in frontend

---

#### Python Backend - FIXED (CURRENT)

```python
# repository.py (all 6 query functions updated)
SELECT
    trace_id, span_id, parent_span_id, service_name, name, kind,
    strftime(start_time, '%Y-%m-%dT%H:%M:%S.%f') || '+00:00' as start_time,
    strftime(end_time, '%Y-%m-%dT%H:%M:%S.%f') || '+00:00' as end_time,
    -- ... other fields
FROM spans
WHERE junjo_span_type = 'workflow'
  AND service_name = ?
ORDER BY start_time DESC
LIMIT ?
```

**DuckDB `strftime()` format**:
- `%Y-%m-%d` → Date (YYYY-MM-DD)
- `T` → Literal T separator
- `%H:%M:%S` → Time (HH:MM:SS)
- `.%f` → Microseconds
- `|| '+00:00'` → Append timezone offset

**JSON Output**:
```json
{
  "start_time": "2025-11-01T13:53:29.882897+00:00",
  "end_time": "2025-11-01T13:53:35.277934+00:00"
}
```

✅ **Valid ISO 8601 format** - Matches Go backend output exactly!

---

## Root Cause Analysis

### Why Go Backend Worked Automatically

1. **Go's `database/sql` driver** automatically converts DuckDB `TIMESTAMPTZ` → `time.Time`
2. **Echo's JSON encoder** automatically serializes `time.Time` → RFC3339 string
3. **No manual formatting needed** in SQL queries

### Why Python Backend Needed Manual Formatting

1. **DuckDB Python driver** returns `TIMESTAMPTZ` as Python `datetime` objects
2. **FastAPI/Pydantic** can serialize `datetime` objects to ISO 8601
3. **BUT**: We were manually casting to `VARCHAR` in SQL, bypassing automatic conversion
4. **DuckDB's `::VARCHAR` cast** produces a format that's NOT ISO 8601 compliant

### The Fix

Changed from relying on DuckDB's `::VARCHAR` (which produces invalid format) to using `strftime()` to explicitly format as ISO 8601.

---

## Updated Functions

All 6 query functions in `backend/app/features/otel_spans/repository.py` were updated:

1. ✅ `get_service_spans` - All spans for a service
2. ✅ `get_root_spans` - Root spans (no parent)
3. ✅ `get_root_spans_with_llm` - Root spans from LLM traces
4. ✅ `get_workflow_spans` - Workflow-type spans
5. ✅ `get_trace_spans` - All spans for a trace
6. ✅ `get_span` - Single span by ID

---

## Verification

### Before Fix
```
GET /api/v1/observability/services/mbb.api.worker/workflows
```

Response:
```json
{
  "start_time": "2025-11-01 13:53:29.882897+00",  // ❌ Invalid
  "end_time": "2025-11-01 13:53:35.277934+00"     // ❌ Invalid
}
```

Frontend error: "Error loading workflow executions"
Negative durations: `-54496300µs`

### After Fix
```
GET /api/v1/observability/services/mbb.api.worker/workflows
```

Response:
```json
{
  "start_time": "2025-11-01T13:53:29.882897+00:00",  // ✅ Valid ISO 8601
  "end_time": "2025-11-01T13:53:35.277934+00:00"     // ✅ Valid ISO 8601
}
```

Frontend: ✅ Workflows load correctly
Durations: ✅ Positive values (e.g., `5.395s`)

---

## Lessons Learned

1. **Don't manually cast timestamps to strings in SQL** - Let the framework handle serialization
2. **Always use ISO 8601 for API responses** - Frontend validation expects it
3. **Test timestamp formats explicitly** - Subtle format differences cause validation failures
4. **DuckDB's `::VARCHAR` is NOT ISO 8601 compliant** - Use `strftime()` for explicit formatting

---

## Alternative Approaches (Not Taken)

### Option 1: Return datetime objects, let FastAPI serialize
```python
# Could have avoided SQL formatting entirely
SELECT start_time, end_time FROM spans  -- Returns datetime objects
# FastAPI would auto-serialize to ISO 8601
```

**Why not used**: Already had `::VARCHAR` pattern, easier to fix in SQL than refactor Python code.

### Option 2: Post-process in Python
```python
# Convert timestamps after query
for row in results:
    row['start_time'] = row['start_time'].isoformat()
```

**Why not used**: More efficient to format in SQL than in Python loop.

---

## References

- **Go Backend Archive**: `.archives/backend_go_archive_20251101.tar.gz`
- **Go Span Processor**: `backend/telemetry/otel_span_processor.go:163-164`
- **Python Span Processor**: `backend/app/features/span_ingestion/span_processor.py:61-82`
- **Python Repository**: `backend/app/features/otel_spans/repository.py`
- **DuckDB strftime docs**: https://duckdb.org/docs/sql/functions/dateformat
- **ISO 8601 spec**: https://www.w3.org/TR/NOTE-datetime
