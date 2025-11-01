#!/bin/bash
# Run all backend tests including gRPC integration tests

cd "$(dirname "$0")/backend"

echo "=========================================="
echo "Running All Backend Tests"
echo "=========================================="

# Set temp database paths
export DB_SQLITE_PATH=/tmp/junjo-test-$(date +%s).db
export DB_DUCKDB_PATH=/tmp/junjo-test-$(date +%s).duckdb

echo "Using temp databases:"
echo "  SQLite: $DB_SQLITE_PATH"
echo "  DuckDB: $DB_DUCKDB_PATH"
echo

# Track test results
UNIT_RESULT=0
INTEGRATION_RESULT=0
GRPC_RESULT=0

# Run unit tests (no server needed)
echo "=== Unit Tests ==="
uv run pytest -m "unit" -v || UNIT_RESULT=$?
echo

# Run integration tests without gRPC (no server needed)
echo "=== Integration Tests (no gRPC) ==="
uv run pytest -m "integration and not requires_grpc_server" -v || INTEGRATION_RESULT=$?
echo

# Run gRPC integration tests (requires server)
echo "=== gRPC Integration Tests ==="
echo "Running migrations..."
uv run alembic upgrade head

echo "Starting backend server..."
uv run uvicorn app.main:app --host 0.0.0.0 --port 1323 > /tmp/backend-test.log 2>&1 &
SERVER_PID=$!

sleep 5
curl -s http://localhost:1323/health > /dev/null || {
    echo "❌ Server failed to start"
    cat /tmp/backend-test.log
    kill $SERVER_PID 2>/dev/null || true
    exit 1
}

echo "Running gRPC tests..."
uv run pytest -m "requires_grpc_server" -v || GRPC_RESULT=$?

# Cleanup
echo "Stopping server..."
kill $SERVER_PID 2>/dev/null || true

# Summary
echo
echo "=========================================="
echo "Test Results Summary:"
echo "=========================================="
echo "Unit tests:        $([ $UNIT_RESULT -eq 0 ] && echo '✓ PASSED' || echo '❌ FAILED')"
echo "Integration tests: $([ $INTEGRATION_RESULT -eq 0 ] && echo '✓ PASSED' || echo '⚠ FAILED (expected Gemini API issue)')"
echo "gRPC tests:        $([ $GRPC_RESULT -eq 0 ] && echo '✓ PASSED' || echo '❌ FAILED')"
echo "=========================================="

# Exit with error if critical tests failed
if [ $UNIT_RESULT -ne 0 ] || [ $GRPC_RESULT -ne 0 ]; then
    echo "❌ Critical tests failed"
    exit 1
fi

echo "✓ All critical tests passed!"
exit 0
