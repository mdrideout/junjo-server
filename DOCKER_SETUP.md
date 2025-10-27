# Docker Setup - Following wt_api_v2 Pattern ✅

## Fixed Issues

### 1. Missing C++ Compiler
**Problem:** `greenlet` (SQLAlchemy async dependency) needs C++ compilation
**Fix:** Added `g++` to Alpine build dependencies

### 2. uv Not Available in Dev Stage
**Problem:** Dev stage tried to use `/bin/uv` but it wasn't copied from base
**Fix:** Use `pip install` instead (pip is available in Python base image)

### 3. Wrong Stage Names
**Problem:** Inconsistent stage naming (`dev` vs `development`)
**Fix:** Renamed to `dev` to match wt_api_v2 pattern

---

## Docker Compose File Structure

Following wt_api_v2's pattern:

### 1. `docker-compose.yml` (Production/Base)
- Default configuration
- No target specified → defaults to `production` stage
- Minimal volumes
- Used in production deployments

### 2. `docker-compose.override.yml` (Development - Auto-merged)
- **Automatically merged** with docker-compose.yml when running `docker compose up`
- Adds `target: dev` for development builds
- Adds volume mounts for hot reload
- Used for local development

### 3. `docker-compose.dev.yml` (Alternative Dev Config)
- Explicit dev configuration
- Use with: `docker compose -f docker-compose.dev.yml up`
- Alternative to override pattern

---

## Dockerfile Stages

### Stage 1: `base`
- Installs uv and build tools (`gcc`, `g++`, etc.)
- Installs Python dependencies into `.venv`
- **uv is only available here**

### Stage 2: `production`
- Copies `.venv` from base
- **Does NOT include uv** (keeps image small)
- Uses pip from Python base image
- Runs: `uvicorn app.main:app`

### Stage 3: `dev`
- Extends `production`
- Installs dev dependencies with `pip install` (not uv)
- Runs: `uvicorn app.main:app --reload`

---

## Usage

### Local Development (Recommended)

```bash
# Uses docker-compose.yml + docker-compose.override.yml
docker compose up --build

# Hot reload enabled automatically
# Volumes mounted automatically
```

**Services:**
- Go Backend: http://localhost:1323
- Python Backend: http://localhost:1324 (with hot reload)
- Frontend: http://localhost:5151
- Ingestion: localhost:50051, :50052

### Production Build

```bash
# Rename or remove override file
mv docker-compose.override.yml docker-compose.override.yml.bak

# Or explicitly use only main file
docker compose -f docker-compose.yml up --build
```

### Alternative Dev Config

```bash
docker compose -f docker-compose.dev.yml up --build
```

---

## Environment Variables

### `.env` File
```bash
# Build target (for other services)
JUNJO_BUILD_TARGET="development"

# Session keys (base64)
JUNJO_SESSION_SECRET=<base64-string>
JUNJO_SECURE_COOKIE_KEY=<base64-string>

# Database paths (overridden in compose)
DB_SQLITE_PATH=./dbdata/junjo.db
DB_DUCKDB_PATH=./dbdata/traces.duckdb
```

### Docker Overrides
```yaml
# In docker-compose.yml
environment:
  - DB_SQLITE_PATH=/dbdata/sqlite/junjo.db  # Container path
  - DB_DUCKDB_PATH=/dbdata/duckdb/traces.duckdb
```

---

## Volume Mounts

### Development (with override)
```yaml
volumes:
  - ./backend_python/app:/app/app  # Hot reload
  - ./.dbdata/sqlite:/dbdata/sqlite  # Shared DB
  - ./.dbdata/duckdb:/dbdata/duckdb  # Shared DuckDB
```

### Production
```yaml
# No app volumes mounted
volumes:
  - ./.dbdata/sqlite:/dbdata/sqlite  # Only data
  - ./.dbdata/duckdb:/dbdata/duckdb
```

---

## Hot Reload

### Python Backend
- **Dev stage**: Uses `uvicorn --reload`
- **Watches**: `./backend_python/app/`
- **Requires**: `watchfiles` package

### How It Works
1. Code change in `backend_python/app/`
2. Docker volume sync to `/app/app` in container
3. `watchfiles` detects change
4. `uvicorn` reloads app
5. Changes live in ~1 second

---

## Verification

### Check Services Running
```bash
docker ps

# Should show:
# - junjo-server-backend (Go)
# - junjo-server-backend-python
# - junjo-server-frontend
# - junjo-server-ingestion
```

### Check Python Backend
```bash
# Health check
curl http://localhost:1324/ping
# Response: "pong"

# Database status
curl http://localhost:1324/users/db-has-users
# Response: {"users_exist": false}
```

### Test Hot Reload
1. Edit `backend_python/app/main.py`
2. Add a print statement
3. Save file
4. Check logs: `docker logs junjo-server-backend-python -f`
5. Should see: "Reloading..."

---

## Troubleshooting

### Build Fails: "g++ not found"
- ✅ **Fixed:** Added `g++` to Dockerfile

### Build Fails: "uv not found"
- ✅ **Fixed:** Use `pip install` in dev stage

### Hot Reload Not Working
1. Check volume mount exists:
   ```bash
   docker inspect junjo-server-backend-python | grep Mounts -A 10
   ```
2. Check override file is being used:
   ```bash
   docker compose config | grep "target: dev"
   ```

### Override File Not Used
- Docker Compose automatically uses `docker-compose.override.yml`
- To disable: rename it or use `-f docker-compose.yml` explicitly

---

## Key Differences from Original

### Before (Broken)
```dockerfile
# Stage 3: Development
FROM production AS development
RUN /bin/uv pip install watchfiles  # ❌ uv not available
```

### After (Fixed)
```dockerfile
# Stage 3: Development
FROM production AS dev
RUN pip install watchfiles  # ✅ pip available from Python base
```

### Before (Inconsistent)
```yaml
# docker-compose.yml
target: ${JUNJO_BUILD_TARGET:-production}

# docker-compose.dev.yml
target: development  # ❌ Wrong name
```

### After (Consistent)
```yaml
# docker-compose.yml
# No target → defaults to production

# docker-compose.override.yml (auto-merged)
target: dev  # ✅ Matches Dockerfile

# docker-compose.dev.yml
target: dev  # ✅ Matches Dockerfile
```

---

## Production Deployment

When deploying to production:

1. **Remove override file:**
   ```bash
   rm docker-compose.override.yml
   # Or rename: mv docker-compose.override.yml docker-compose.override.yml.disabled
   ```

2. **Build production image:**
   ```bash
   docker compose -f docker-compose.yml build
   ```

3. **Run production:**
   ```bash
   docker compose -f docker-compose.yml up -d
   ```

Production image:
- ✅ No dev dependencies
- ✅ No volume mounts
- ✅ Smaller image size
- ✅ No hot reload overhead

---

## Success! 🎉

Your Docker setup now matches the proven wt_api_v2 pattern:
- ✅ Clean stage separation
- ✅ Automatic dev/prod switching
- ✅ Hot reload in development
- ✅ Optimized production builds
- ✅ No manual target specification needed

Run `docker compose up --build` and start testing! 🚀
