# Docker Build Fix Applied ✅

## Issues Found & Fixed

### 1. Dockerfile Stage Name Mismatch
**Problem:** Dockerfile had stage named `dev` but compose files expected `development`

**Fix:** Renamed Dockerfile stage from `AS dev` to `AS development`
```dockerfile
# Stage 3: Development - Add dev dependencies and hot reload
FROM production AS development  # ← Changed from "dev"
```

### 2. docker-compose.dev.yml Mismatch
**Problem:** Referenced wrong stage name

**Fix:** Updated target from `dev` to `development`
```yaml
target: development  # Use development stage for hot reload
```

### 3. Missing Environment Variables
**Problem:** Main docker-compose.yml didn't override database paths

**Fix:** Added explicit environment variables to match dev setup
```yaml
environment:
  - DB_SQLITE_PATH=/dbdata/sqlite/junjo.db
  - DB_DUCKDB_PATH=/dbdata/duckdb/traces.duckdb
  - INGESTION_HOST=junjo-server-ingestion
  - INGESTION_PORT=50052
```

---

## Now Try Building

### Development Mode (Hot Reload)
```bash
# Your .env has JUNJO_BUILD_TARGET="development"
docker compose up --build

# Or explicitly:
JUNJO_BUILD_TARGET=development docker compose up --build
```

### Production Mode
```bash
JUNJO_BUILD_TARGET=production docker compose up --build
```

### Using Dev Compose File
```bash
docker compose -f docker-compose.dev.yml up --build
```

---

## What Should Happen

1. ✅ All services build successfully
2. ✅ Python backend starts on port 1324
3. ✅ Go backend starts on port 1323
4. ✅ Frontend starts on port 5151
5. ✅ Ingestion service starts on ports 50051/50052

---

## Verify It Worked

```bash
# Check all containers are running
docker ps

# Should see:
# - junjo-server-backend-python
# - junjo-server-backend (Go)
# - junjo-server-frontend
# - junjo-server-ingestion

# Test Python backend
curl http://localhost:1324/ping
# Response: "pong"

# Test Go backend
curl http://localhost:1323/ping

# Open frontend
open http://localhost:5151
```

---

## Notes

- **docker-compose.yml**: Main file, uses `JUNJO_BUILD_TARGET` env var (defaults to production)
- **docker-compose.dev.yml**: Development file, explicitly uses development target
- **Python backend database**: Will be at `/dbdata/sqlite/junjo.db` inside container
- **Shared volumes**: Both Go and Python backends can access same database files
