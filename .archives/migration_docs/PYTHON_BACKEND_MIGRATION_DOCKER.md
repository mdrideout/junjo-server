# Python Backend Migration - Docker Integration Plan

## Current Status

✅ **Phase 1-3 Complete:**
- Dependencies installed
- Settings configured with session/cookie authentication
- Auth system fully implemented (8 endpoints)
- All 23 tests passing
- Test co-location complete

## Migration Strategy Options

### Option 1: Gradual Migration (RECOMMENDED)

Run both backends simultaneously during transition period.

**Advantages:**
- Safe rollback if issues arise
- Migrate features incrementally
- Test in production with real traffic
- Zero downtime migration

**Implementation:**
```yaml
# Both services run side-by-side
junjo-server-backend (Go):     Port 1323  # Existing features
junjo-server-backend-python:   Port 1324  # New features (auth)
```

**Frontend changes:**
```typescript
// config.ts
const GO_BACKEND = 'http://localhost:1323'
const PYTHON_BACKEND = 'http://localhost:1324'

// Route requests based on feature
export const API_HOST = {
  auth: PYTHON_BACKEND,      // /sign-in, /users, /auth-test
  legacy: GO_BACKEND,        // Other endpoints
}
```

### Option 2: Full Replacement

Replace Go backend entirely with Python backend.

**Advantages:**
- Simpler architecture (single backend)
- Cleaner deployment
- No routing complexity

**Risks:**
- Must migrate all features first
- Harder to rollback
- Requires complete feature parity

## Recommended Approach: Option 1 (Gradual)

Start with gradual migration, then switch to full replacement when feature parity is achieved.

---

## Implementation Plan - Option 1

### Step 1: Add Python Backend to Docker Compose

Update `docker-compose.yml`:

```yaml
services:
  # Existing Go backend (keep as-is)
  junjo-server-backend:
    container_name: junjo-server-backend
    # ... existing config ...
    ports:
      - "1323:1323"

  # NEW: Python backend
  junjo-server-backend-python:
    container_name: junjo-server-backend-python
    restart: unless-stopped
    build:
      context: ./backend_python
      target: ${JUNJO_BUILD_TARGET:-production}
    volumes:
      - ./backend_python:/app  # Hot reload in dev
      - ./.dbdata/sqlite:/dbdata/sqlite  # Share SQLite with Go (if needed)
    ports:
      - "1324:1324"
    networks:
      - junjo-network
    env_file:
      - .env
    healthcheck:
      test: ["CMD", "python", "-c", "import urllib.request; urllib.request.urlopen('http://localhost:1324/ping')"]
      interval: 5s
      timeout: 3s
      retries: 25
      start_period: 5s
```

### Step 2: Update Frontend Configuration

**Option A: Dual Backend (during migration)**
```typescript
// frontend/src/config.ts
const isDevelopment = import.meta.env.DEV

// Backend routing
export const BACKEND_HOSTS = {
  go: window.runtimeConfig?.GO_API_HOST || 'http://localhost:1323',
  python: window.runtimeConfig?.PYTHON_API_HOST || 'http://localhost:1324',
}

// Route by feature
export function getApiHost(feature: 'auth' | 'legacy'): string {
  return feature === 'auth' ? BACKEND_HOSTS.python : BACKEND_HOSTS.go
}
```

**Option B: Single Backend (after full migration)**
```typescript
// frontend/src/config.ts
export const API_HOST = window.runtimeConfig?.API_HOST || 'http://localhost:1324'
```

### Step 3: Update Environment Variables

Add to root `.env`:
```bash
# Backend hosts (for dual-backend mode)
GO_API_HOST=http://localhost:1323
PYTHON_API_HOST=http://localhost:1324

# Or single backend (after migration)
API_HOST=http://localhost:1324
```

### Step 4: Database Considerations

**SQLite Database:**
- Python uses: `./dbdata/junjo.db`
- Go uses: `/dbdata/sqlite/...`

**Options:**
1. **Separate databases** (recommended during migration)
   - Python: `./dbdata/junjo.db`
   - Go: `./dbdata/sqlite/legacy.db`

2. **Shared database** (after migration complete)
   - Both point to same SQLite file
   - Requires schema compatibility

### Step 5: Frontend Migration Steps

1. **Update auth-related fetch functions** to use Python backend:
   ```typescript
   // frontend/src/features/auth/fetch/post-sign-in.ts
   const response = await fetch(`${BACKEND_HOSTS.python}/sign-in`, {
     method: 'POST',
     body: JSON.stringify(credentials),
   })
   ```

2. **Test auth flow** end-to-end
3. **Gradually migrate other features** to Python backend
4. **Switch to single backend** when all features migrated

---

## Testing the Migration

### 1. Start Both Backends

```bash
# Option 1: Docker Compose
docker compose up junjo-server-backend junjo-server-backend-python

# Option 2: Locally (for development)
# Terminal 1: Go backend
cd backend && go run main.go

# Terminal 2: Python backend
cd backend_python && uv run uvicorn app.main:app --reload --port 1324

# Terminal 3: Frontend
cd frontend && npm run dev
```

### 2. Test Endpoints

```bash
# Python backend (auth)
curl http://localhost:1324/ping
curl http://localhost:1324/users/db-has-users

# Go backend (legacy)
curl http://localhost:1323/ping
```

### 3. Test Frontend Integration

1. Open browser: http://localhost:5151
2. Test sign-in flow (should hit Python backend at :1324)
3. Test other features (should hit Go backend at :1323)

---

## Rollback Plan

If issues arise:

1. **Immediate:** Stop Python backend container
2. **Frontend:** Revert frontend config to point only to Go backend
3. **Docker:** Comment out `junjo-server-backend-python` service

---

## Next Steps

**Phase 1: Docker Integration (This Phase)**
- [ ] Add Python backend to docker-compose.yml
- [ ] Update frontend config for dual backends
- [ ] Test full stack locally
- [ ] Deploy to development environment

**Phase 2: Migrate Remaining Features**
- [ ] Projects/Runs/Traces (read from DuckDB)
- [ ] Playground
- [ ] Settings
- [ ] API Keys

**Phase 3: Complete Migration**
- [ ] Achieve feature parity
- [ ] Switch frontend to single Python backend
- [ ] Remove Go backend from docker-compose
- [ ] Archive Go backend code

---

## Timeline Estimate

- **Docker Integration:** 1-2 hours
- **Frontend Updates:** 2-3 hours
- **Testing & Validation:** 2-4 hours
- **Per-feature migration:** 4-8 hours each

**Total for full migration:** 2-3 weeks (depending on feature complexity)
