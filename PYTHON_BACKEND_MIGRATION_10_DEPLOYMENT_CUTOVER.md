# Phase 10: Deployment & Cutover

## Overview

This phase covers the deployment strategy and cutover plan for transitioning from the Go backend to the Python backend. The strategy prioritizes:
- **Zero Downtime**: Run both backends in parallel during transition
- **Gradual Migration**: Test in development/staging before production
- **Rollback Plan**: Easy rollback to Go backend if issues arise
- **Data Continuity**: Shared databases (SQLite, DuckDB) between Go and Python

## Deployment Architecture

### Current Architecture (Go Backend)

```
junjo-server-backend (Port 1323)
  ├─ SQLite (/dbdata/sqlite/junjo.db)
  ├─ DuckDB (/dbdata/duckdb/otel_data.db)
  ├─ gRPC Server (Port 50053) - API key validation
  └─ gRPC Client → junjo-server-ingestion:50052

junjo-server-ingestion (Port 50052)
  ├─ BadgerDB WAL
  └─ gRPC Client → junjo-server-backend:50053
```

### Target Architecture (Python Backend)

```
junjo-server-backend-python (Port 8000)
  ├─ SQLite (/dbdata/sqlite/junjo.db) - SHARED with Go
  ├─ DuckDB (/dbdata/duckdb/otel_data.db) - SHARED with Go
  ├─ gRPC Server (Port 50053) - API key validation
  └─ gRPC Client → junjo-server-ingestion:50052

junjo-server-ingestion (Port 50052) - UNCHANGED
  ├─ BadgerDB WAL
  └─ gRPC Client → junjo-server-backend:50053 OR :50054
```

**Key Points**:
- Python backend runs on different HTTP port (8000) from Go (1323)
- Both backends can share the same gRPC port (50053) by running at different times OR use different ports
- SQLite and DuckDB are shared (file-based databases)
- Ingestion service is unchanged and remains in Go

## Docker Configuration

### Python Backend Dockerfile

**File**: `python_backend/Dockerfile`

```dockerfile
# Use Python 3.14
FROM python:3.14-slim

# Set working directory
WORKDIR /app

# Install system dependencies
RUN apt-get update && apt-get install -y \\
    gcc \\
    g++ \\
    && rm -rf /var/lib/apt/lists/*

# Install uv
RUN pip install uv

# Copy dependency files
COPY pyproject.toml uv.lock ./

# Install dependencies
RUN uv sync --frozen

# Copy application code
COPY . .

# Copy SQL query files
COPY sql_queries /app/sql_queries

# Copy DuckDB schema files
COPY duckdb_schemas /app/duckdb_schemas

# Expose HTTP port
EXPOSE 8000

# Expose gRPC port (for internal auth service)
EXPOSE 50053

# Run the application
CMD ["uv", "run", "uvicorn", "app.main:app", "--host", "0.0.0.0", "--port", "8000"]
```

### Docker Compose Configuration

**File**: `docker-compose.yml` (updated to include Python backend)

```yaml
version: '3.8'

services:
  # Python Backend (NEW)
  junjo-server-backend-python:
    build:
      context: ./python_backend
      dockerfile: Dockerfile
    container_name: junjo-server-backend-python
    ports:
      - "8000:8000"      # HTTP API
      - "50054:50053"    # gRPC (use different host port to avoid conflict with Go)
    volumes:
      - ./dbdata:/dbdata  # Shared database directory
    environment:
      - JUNJO_SESSION_SECRET=${JUNJO_SESSION_SECRET}
      - JUNJO_ENV=${JUNJO_ENV}
      - JUNJO_PROD_AUTH_DOMAIN=${JUNJO_PROD_AUTH_DOMAIN}
      - JUNJO_ALLOW_ORIGINS=${JUNJO_ALLOW_ORIGINS}
      - OPENAI_API_KEY=${OPENAI_API_KEY}
      - ANTHROPIC_API_KEY=${ANTHROPIC_API_KEY}
      - GEMINI_API_KEY=${GEMINI_API_KEY}
    networks:
      - junjo-network
    depends_on:
      - junjo-server-ingestion

  # Go Backend (EXISTING - keep for parallel operation)
  junjo-server-backend:
    build:
      context: ./backend
      dockerfile: Dockerfile
    container_name: junjo-server-backend
    ports:
      - "1323:1323"      # HTTP API
      - "50053:50053"    # gRPC
    volumes:
      - ./dbdata:/dbdata
    environment:
      - JUNJO_SESSION_SECRET=${JUNJO_SESSION_SECRET}
      - JUNJO_ENV=${JUNJO_ENV}
      - JUNJO_PROD_AUTH_DOMAIN=${JUNJO_PROD_AUTH_DOMAIN}
      - JUNJO_ALLOW_ORIGINS=${JUNJO_ALLOW_ORIGINS}
      - OPENAI_API_KEY=${OPENAI_API_KEY}
      - ANTHROPIC_API_KEY=${ANTHROPIC_API_KEY}
      - GEMINI_API_KEY=${GEMINI_API_KEY}
    networks:
      - junjo-network
    depends_on:
      - junjo-server-ingestion

  # Ingestion Service (UNCHANGED)
  junjo-server-ingestion:
    build:
      context: ./ingestion-service
      dockerfile: Dockerfile
    container_name: junjo-server-ingestion
    ports:
      - "50052:50052"    # gRPC
    volumes:
      - ./ingestion-data:/data
    environment:
      # Point to whichever backend is active
      - BACKEND_GRPC_ADDR=junjo-server-backend:50053
    networks:
      - junjo-network

networks:
  junjo-network:
    driver: bridge
```

## Migration Strategy

### Phase 1: Parallel Operation (Development)

**Goal**: Run both backends simultaneously for testing

**Steps**:
1. Build and start Python backend on port 8000
2. Keep Go backend running on port 1323
3. Frontend can switch between backends via environment variable:
   ```bash
   # Use Python backend
   VITE_API_BASE_URL=http://localhost:8000

   # Use Go backend
   VITE_API_BASE_URL=http://localhost:1323
   ```
4. Test all functionality with Python backend
5. Run integration test suite
6. Manual testing with frontend

**Duration**: 1-2 weeks

**Rollback**: Switch frontend back to Go backend (port 1323)

### Phase 2: Feature Parity Verification

**Goal**: Ensure Python backend has complete feature parity

**Checklist**:
- [ ] All API endpoints implemented
- [ ] Authentication works (session cookies)
- [ ] API key management works
- [ ] Span ingestion works (background polling)
- [ ] DuckDB queries work (OTEL API)
- [ ] LLM playground works (all providers)
- [ ] All tests pass (unit + integration)
- [ ] Manual testing completed
- [ ] Performance is acceptable
- [ ] Error handling works correctly

**Duration**: 1 week

### Phase 3: Staging Deployment

**Goal**: Deploy Python backend to staging environment

**Steps**:
1. Deploy Python backend to staging
2. Update frontend staging to use Python backend
3. Run smoke tests
4. Monitor for errors/issues
5. Performance testing
6. Load testing (if applicable)

**Duration**: 1 week

**Rollback**: Switch frontend staging back to Go backend

### Phase 4: Production Cutover (Blue-Green Deployment)

**Goal**: Switch production traffic to Python backend

**Strategy**: Blue-Green Deployment
- **Blue**: Go backend (current production)
- **Green**: Python backend (new version)

**Steps**:
1. Deploy Python backend to production (Green)
2. Run health checks
3. Switch 10% of traffic to Python backend (canary testing)
4. Monitor metrics:
   - Error rates
   - Response times
   - CPU/Memory usage
   - Database connection health
5. If stable after 24 hours, switch 50% of traffic
6. If stable after 48 hours, switch 100% of traffic
7. Monitor for 1 week before decommissioning Go backend

**Rollback Plan**:
- Switch traffic back to Go backend (Blue)
- Python backend remains deployed for debugging
- Fix issues and retry cutover

**Duration**: 2 weeks (gradual rollout)

### Phase 5: Decommissioning Go Backend

**Goal**: Remove Go backend from production

**Steps**:
1. Verify Python backend has been stable for 1+ week
2. Stop Go backend container
3. Archive Go backend code (don't delete)
4. Update documentation to reflect Python backend
5. Celebrate! 🎉

**Duration**: 1 day

## Database Migration Considerations

### SQLite (Shared State)

**Current Tables**:
- `users` - User accounts
- `api_keys` - API keys
- `poller_state` - Span polling state

**Migration Strategy**:
- **No schema changes required** - Both Go and Python can use same schema
- **WAL Mode**: Both backends use WAL mode, safe for concurrent access
- **Shared File**: Both backends access `/dbdata/sqlite/junjo.db`
- **Atomic Operations**: SQLite handles concurrent writes safely

**Important Notes**:
- When running both backends in parallel, both will access same database
- Poller state is shared - only one backend should run the poller at a time
- During transition, disable background poller in one backend

### DuckDB (Shared Analytics)

**Current Tables**:
- `spans` - OpenTelemetry spans
- `state_patches` - Workflow state patches

**Migration Strategy**:
- **No schema changes required** - Both Go and Python use same schema
- **Shared File**: Both backends access `/dbdata/duckdb/otel_data.db`
- **Read-Only in Go**: During parallel operation, Go backend can read, Python writes

**Important Notes**:
- Only one backend should write to DuckDB (span ingestion)
- During transition, run background poller in only one backend
- DuckDB handles concurrent reads safely

## Environment Variables

### Required Environment Variables

```bash
# Session secret (REQUIRED)
JUNJO_SESSION_SECRET=<generate with: python -c "import secrets; print(secrets.token_urlsafe(32))">

# Environment
JUNJO_ENV=production  # or development

# Production auth domain (optional)
JUNJO_PROD_AUTH_DOMAIN=yourdomain.com

# CORS origins
JUNJO_ALLOW_ORIGINS=https://yourdomain.com,https://app.yourdomain.com

# LLM API Keys
OPENAI_API_KEY=sk-...
ANTHROPIC_API_KEY=sk-ant-...
GEMINI_API_KEY=...

# Database paths (optional, defaults work)
SQLITE_DB_PATH=/dbdata/sqlite/junjo.db
DUCKDB_PATH=/dbdata/duckdb/otel_data.db

# gRPC addresses (optional, defaults work)
INGESTION_GRPC_ADDRESS=junjo-server-ingestion:50052
AUTH_GRPC_LISTEN_ADDRESS=0.0.0.0:50053

# Span polling settings (optional)
SPAN_POLL_INTERVAL=5  # seconds
SPAN_BATCH_SIZE=100
```

## Monitoring & Observability

### Metrics to Monitor

**Application Metrics**:
- HTTP request rate
- HTTP error rate
- HTTP response time (p50, p95, p99)
- Active sessions count

**Database Metrics**:
- SQLite connection pool size
- DuckDB query performance
- Database file sizes

**gRPC Metrics**:
- gRPC call rate
- gRPC error rate
- gRPC response time

**Background Tasks**:
- Span polling rate
- Span processing errors
- Span processing lag

**LLM Metrics** (from LiteLLM OpenTelemetry):
- LLM request rate
- LLM error rate
- LLM token usage
- LLM cost

### Health Check Endpoints

```bash
# Simple health check
curl http://localhost:8000/ping
# Response: "pong"

# Detailed health check
curl http://localhost:8000/health
# Response: {"status": "healthy", "services": {...}}
```

### Logging

**Log Levels**:
- **ERROR**: Application errors, exceptions
- **WARNING**: Degraded performance, retries
- **INFO**: Important state changes, startup/shutdown
- **DEBUG**: Detailed debugging information (disabled in production)

**Log Format**: JSON (structured logging via loguru)

**Log Aggregation**: Send logs to your logging service (e.g., CloudWatch, Datadog, etc.)

## Rollback Procedures

### Emergency Rollback (Production)

**If Python backend has critical issues**:

1. **Immediate**: Switch traffic back to Go backend
   ```bash
   # Update environment variable
   export API_BASE_URL=http://junjo-server-backend:1323

   # Or update load balancer / proxy configuration
   ```

2. **Stop Python backend** (to prevent database contention):
   ```bash
   docker stop junjo-server-backend-python
   ```

3. **Verify Go backend is healthy**:
   ```bash
   curl http://localhost:1323/ping
   ```

4. **Monitor metrics** to confirm traffic is back to normal

5. **Debug Python backend** offline:
   - Check logs
   - Run tests
   - Fix issues

6. **Retry cutover** once issues are resolved

**Rollback Time**: < 5 minutes

### Planned Rollback (Staging)

**If issues discovered during staging**:

1. Switch frontend staging to Go backend
2. Keep Python backend running for debugging
3. Fix issues and redeploy
4. Retest in staging

**No production impact** - can take time to fix properly

## Success Criteria

### Technical Success

- [ ] All API endpoints working correctly
- [ ] Zero data loss
- [ ] No increase in error rates
- [ ] Response times within acceptable range
- [ ] All tests passing
- [ ] Background tasks running correctly
- [ ] Database integrity maintained

### Business Success

- [ ] Users experience no disruption
- [ ] All features working as expected
- [ ] No increase in support tickets
- [ ] Positive feedback from users (if applicable)

### Performance Success

- [ ] HTTP response time p95 < 500ms
- [ ] HTTP error rate < 1%
- [ ] Database query performance acceptable
- [ ] No memory leaks
- [ ] CPU usage reasonable

## Post-Migration Tasks

### Immediate (Week 1)

- [ ] Monitor all metrics closely
- [ ] Respond to any issues quickly
- [ ] Gather user feedback
- [ ] Document any issues encountered
- [ ] Update runbooks with Python-specific information

### Short-term (Month 1)

- [ ] Optimize performance if needed
- [ ] Add missing features (if any discovered)
- [ ] Improve error handling based on real usage
- [ ] Update documentation
- [ ] Train team on Python backend

### Long-term (Month 2+)

- [ ] Decommission Go backend
- [ ] Archive Go code (don't delete)
- [ ] Update CI/CD pipelines to remove Go backend
- [ ] Celebrate successful migration! 🎉

## Troubleshooting Guide

### Common Issues

**Issue**: Python backend can't connect to DuckDB

**Solution**:
- Check file permissions on `/dbdata/duckdb/otel_data.db`
- Verify DuckDB file is not corrupted
- Check disk space

**Issue**: SQLite database locked

**Solution**:
- Ensure WAL mode is enabled
- Check if both backends are writing simultaneously (disable poller in one)
- Verify no other processes have database locked

**Issue**: gRPC connection refused

**Solution**:
- Check gRPC server is running (port 50053 or 50054)
- Verify firewall rules allow gRPC traffic
- Check ingestion service can reach backend

**Issue**: LLM API errors

**Solution**:
- Verify API keys are set correctly
- Check API key has sufficient quota
- Verify LiteLLM is configured correctly

**Issue**: Background span poller not running

**Solution**:
- Check logs for errors
- Verify ingestion client connection is established
- Check DuckDB connection is working

## Appendix: Useful Commands

### Start Python Backend (Development)

```bash
cd python_backend
uv run uvicorn app.main:app --reload --host 0.0.0.0 --port 8000
```

### Run Tests

```bash
cd python_backend
uv run pytest tests/ -v
```

### Run Migrations

```bash
cd python_backend
uv run alembic upgrade head
```

### Generate Proto Files

```bash
cd python_backend
uv run python scripts/generate_protos.py
```

### Check Logs (Docker)

```bash
# Python backend logs
docker logs -f junjo-server-backend-python

# Go backend logs
docker logs -f junjo-server-backend

# Ingestion service logs
docker logs -f junjo-server-ingestion
```

### Database Operations

```bash
# Connect to SQLite
sqlite3 /dbdata/sqlite/junjo.db

# Connect to DuckDB
duckdb /dbdata/duckdb/otel_data.db

# Check WAL mode
sqlite3 /dbdata/sqlite/junjo.db "PRAGMA journal_mode;"
```

## Conclusion

This migration strategy provides a safe, gradual path from Go to Python backend with:
- **Zero downtime** through parallel operation
- **Easy rollback** at any stage
- **Data continuity** via shared databases
- **Risk mitigation** through gradual traffic shifting
- **Clear success criteria** for each phase

The migration can be completed in approximately **5-7 weeks** with proper testing and validation at each stage.

Good luck with your migration! 🚀
