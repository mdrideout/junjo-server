# Logging Migration Plan: Moving to slog

## Executive Summary

This document outlines the plan to migrate both `backend` and `ingestion-service` from Go's standard `log` package and ad-hoc `fmt.Println` statements to the modern `log/slog` structured logging system introduced in Go 1.21.

---

## Rationale

### Current State & Problems

**Current logging approach:**
- Using Go's standard `log` package (~85+ log statements)
- Mix of `log.Printf`, `log.Println`, `log.Fatalf`, and `fmt.Println`
- No structured logging (difficult to parse and query)
- No log level control (all logs print regardless of severity)
- Too verbose for production environments
- No distinction between debug, info, warn, and error logs
- Echo framework's built-in logger (separate from application logs)

**Key problems:**
1. **Production noise**: Debug-level logs pollute production output, making it hard to identify critical issues
2. **No filtering**: Cannot reduce log verbosity without code changes
3. **Unstructured**: Logs are plain text strings, not queryable JSON
4. **No context**: Missing correlation IDs, trace IDs, and structured metadata
5. **Inconsistent**: Different parts of the codebase use different logging approaches

### Why slog?

**slog (Structured Logging) is the official Go logging solution:**

1. ✅ **Zero external dependencies** - Part of the standard library since Go 1.21
2. ✅ **Structured logging** - Key-value pairs, machine-readable JSON output
3. ✅ **Log levels built-in** - Debug, Info, Warn, Error with filtering
4. ✅ **Context-aware** - First-class `context.Context` support (perfect for gRPC)
5. ✅ **Performance** - 40 B/op, minimal allocations, fastest in memory efficiency
6. ✅ **Future-proof** - Will be maintained by Go team forever
7. ✅ **Production-ready** - JSON output for log aggregation systems
8. ✅ **Developer-friendly** - Human-readable text output for development
9. ✅ **gRPC integration** - Official support in grpc-ecosystem/go-grpc-middleware v2
10. ✅ **Standard** - All future Go libraries will support slog

**Performance comparison** (from 2024-2025 benchmarks):
- **slog**: ~1200 ns/op, 40 B/op, fewest allocations
- **zap**: ~935 ns/op, 168 B/op, 3 allocs/op
- **zerolog**: ~656 ns/op, 40 B/op, minimal allocations

**Why performance doesn't matter for our use case:**
```
Typical span processing time breakdown:
- Protobuf unmarshaling:  ~1-5 µs    (1,000-5,000 ns)
- Database insert:        ~100-1000 µs (100,000-1,000,000 ns)
- gRPC network I/O:       ~100-500 µs
- Logging with slog:      ~1.2 µs     (1,200 ns)

Logging overhead: 0.0001% of total operation time
```

Our bottlenecks are database writes and network I/O, not logging. The 265 ns difference between slog and zap is negligible.

---

## Goals

### Primary Objectives

1. **Environment-based log control**: Use `LOG_LEVEL` environment variable to control verbosity
2. **Reduce production logs**: Default to `info` level, filter out debug logs
3. **Explicit debug logs**: Clearly mark debug-level information during refactoring
4. **Structured output**: Machine-readable JSON for production, human-readable text for development
5. **Context propagation**: Leverage existing `context.Context` usage for trace correlation
6. **gRPC request logging**: Automatic logging of gRPC requests/responses with metadata
7. **Maintainability**: Standard library means no dependency management overhead

### Log Level Strategy

**Default log level: `INFO`**

**Level definitions:**
- **DEBUG**: Detailed diagnostic information (span contents, internal state, step-by-step flow)
  - *Use sparingly, disabled in production by default*
  - Examples: "polling for new spans...", "received span bytes: [...]"

- **INFO**: Normal operational messages (significant events, milestones)
  - *Production default*
  - Examples: "received 10 spans", "database sync completed", "gRPC server started"

- **WARN**: Warning messages (potential issues, degraded state, recoverable errors)
  - *Always logged in production*
  - Examples: "CORS origin denied", "failed to sync database (retrying)", "high memory usage"

- **ERROR**: Error conditions (failures requiring attention)
  - *Always logged in production*
  - Examples: "failed to insert span", "database connection lost", "API key validation failed"

### Environment Variables

**New configuration:**

```bash
# Log Level: debug, info, warn, error
# Default: info
# Controls minimum severity level for log output
LOG_LEVEL=info

# Log Format: json, text
# Default: json (production), text (development)
# - json: Machine-readable JSON output for production
# - text: Human-readable colored output for development
LOG_FORMAT=text
```

**Example configurations:**

```bash
# Local development - verbose, human-readable
LOG_LEVEL=debug
LOG_FORMAT=text

# Production - filtered, JSON for log aggregation
LOG_LEVEL=info
LOG_FORMAT=json

# Production troubleshooting - temporary debug
LOG_LEVEL=debug
LOG_FORMAT=json

# Production - errors only
LOG_LEVEL=error
LOG_FORMAT=json
```

---

## Implementation Plan

### Phase 1: Environment Configuration

#### 1.1 Update .env.example
Add new logging environment variables with documentation:

```bash
# === LOGGING CONFIGURATION ===================================================
# Log Level: debug, info, warn, error (default: info)
# Controls the minimum severity level for log output
# - debug: Show all logs including detailed diagnostics (use in development)
# - info:  Show informational messages and above (production default)
# - warn:  Show warnings and errors only
# - error: Show only error messages
LOG_LEVEL=info

# Log Format: json, text (default: json)
# Controls the output format of logs
# - json: Machine-readable JSON output (recommended for production)
# - text: Human-readable colored output (recommended for development)
LOG_FORMAT=text
```

#### 1.2 Update .env
Add same variables for local development (default to `text`/`debug` for better dev experience)

---

### Phase 2: Backend Service Implementation

#### 2.1 Create Logger Package
**File: `/backend/logger/logger.go`**

**Implementation requirements:**
- Export `InitLogger() *slog.Logger` function
- Read `LOG_LEVEL` environment variable, parse to slog.Level
- Read `LOG_FORMAT` environment variable
- Create appropriate handler (JSONHandler or TextHandler)
- Set as default global logger using `slog.SetDefault()`
- Return configured logger instance
- Add source location support for debug level

**Example structure:**
```go
package logger

import (
    "log/slog"
    "os"
    "strings"
)

func InitLogger() *slog.Logger {
    // Parse LOG_LEVEL from env (default: info)
    level := parseLogLevel(os.Getenv("LOG_LEVEL"))

    // Parse LOG_FORMAT from env (default: json)
    format := os.Getenv("LOG_FORMAT")
    if format == "" {
        format = "json"
    }

    // Create handler based on format
    var handler slog.Handler
    opts := &slog.HandlerOptions{
        Level: level,
        AddSource: level == slog.LevelDebug, // Add source location for debug
    }

    if format == "json" {
        handler = slog.NewJSONHandler(os.Stdout, opts)
    } else {
        handler = slog.NewTextHandler(os.Stdout, opts)
    }

    logger := slog.New(handler)
    slog.SetDefault(logger)

    return logger
}

func parseLogLevel(level string) slog.Level {
    switch strings.ToLower(level) {
    case "debug":
        return slog.LevelDebug
    case "warn", "warning":
        return slog.LevelWarn
    case "error":
        return slog.LevelError
    default:
        return slog.LevelInfo
    }
}
```

#### 2.2 Add gRPC Middleware Dependency
**File: `/backend/go.mod`**

Add dependency:
```bash
go get github.com/grpc-ecosystem/go-grpc-middleware/v2
```

#### 2.3 Create gRPC Logger Adapter
**File: `/backend/logger/grpc.go`**

**Implementation:**
- Export `InterceptorLogger(l *slog.Logger) logging.Logger` function
- Adapt slog.Logger to grpc-middleware logging.Logger interface
- Map logging.Level to slog.Level correctly

**Example structure:**
```go
package logger

import (
    "context"
    "log/slog"

    "github.com/grpc-ecosystem/go-grpc-middleware/v2/interceptors/logging"
)

// InterceptorLogger adapts slog.Logger to logging.Logger interface
func InterceptorLogger(l *slog.Logger) logging.Logger {
    return logging.LoggerFunc(func(ctx context.Context, lvl logging.Level, msg string, fields ...any) {
        l.Log(ctx, slog.Level(lvl), msg, fields...)
    })
}
```

#### 2.4 Update main.go
**File: `/backend/main.go`**

**Changes required:**

1. **Import slog and logger package:**
```go
import (
    "log/slog"
    "junjo-server/logger"
    grpc_logging "github.com/grpc-ecosystem/go-grpc-middleware/v2/interceptors/logging"
)
```

2. **Initialize logger** at startup (line ~40, after godotenv.Load):
```go
log := logger.InitLogger()
log.Info("starting junjo backend service")
```

3. **Replace all log statements** (~17 occurrences):

**Current → New mapping:**

| Line | Current | New | Level |
|------|---------|-----|-------|
| 40 | `fmt.Println("Running main.go function")` | `log.Info("starting main function")` | INFO |
| 45 | `fmt.Printf("%v\n", err)` | `log.Warn("failed to load .env file", slog.Any("error", err))` | WARN |
| 60-61 | `log.Fatalf("duckdb err: %v", duck_err)` | `log.Error("failed to connect to duckdb", slog.Any("error", duck_err))` + `os.Exit(1)` | ERROR |
| 67-68 | `log.Fatalf("Failed to create ingestion client: %v", err)` | `log.Error("failed to create ingestion client", slog.Any("error", err))` + `os.Exit(1)` | ERROR |
| 81-82 | `log.Fatalf("Failed to load poller state: %v", err)` | `log.Error("failed to load poller state", slog.Any("error", err))` + `os.Exit(1)` | ERROR |
| 83 | `log.Println("No previous poller state found...")` | `log.Info("no previous poller state found, starting from beginning")` | INFO |
| 86 | `log.Printf("Resuming poller from last key: %x", lastKey)` | `log.Info("resuming poller", slog.String("last_key", fmt.Sprintf("%x", lastKey)))` | INFO |
| 90 | `log.Println("Polling for new spans...")` | `log.Debug("polling for new spans")` | DEBUG |
| 93 | `log.Printf("Error reading spans: %v", err)` | `log.Error("error reading spans", slog.Any("error", err))` | ERROR |
| 99 | `log.Printf("Received %d spans. Last key: %x", ...)` | `log.Info("received spans", slog.Int("count", len(spans)), slog.String("last_key", fmt.Sprintf("%x", lastKey)))` | INFO |
| 104 | `log.Printf("Error unmarshaling span: %v", err)` | `log.Warn("error unmarshaling span", slog.Any("error", err))` | WARN |
| 118 | `log.Printf("Error unmarshaling resource: %v", err)` | `log.Warn("error unmarshaling resource", slog.Any("error", err))` | WARN |
| 139 | `log.Printf("Error processing spans batch: %v", err)` | `log.Error("error processing spans batch", slog.Any("error", err))` | ERROR |
| 144 | `log.Printf("Failed to save poller state: %v", err)` | `log.Error("failed to save poller state", slog.Any("error", err))` | ERROR |
| 149 | `log.Println("No new spans found.")` | `log.Debug("no new spans found")` | DEBUG |
| 156 | `e.Logger.Printf("initialized echo with host:port %s", serverHostPort)` | `log.Info("initialized echo server", slog.String("host_port", serverHostPort))` | INFO |
| 207 | `log.Fatal("JUNJO_SESSION_SECRET environment variable is not set")` | `log.Error("JUNJO_SESSION_SECRET not set")` + `os.Exit(1)` | ERROR |
| 245 | `log.Fatalf("Failed to listen for internal gRPC: %v", err)` | `log.Error("failed to listen for internal grpc", slog.Any("error", err))` + `os.Exit(1)` | ERROR |
| 252 | `log.Printf("Internal gRPC server listening at %v", lis.Addr())` | `log.Info("internal grpc server listening", slog.String("address", lis.Addr().String()))` | INFO |
| 254 | `log.Fatalf("Failed to serve internal gRPC: %v", err)` | `log.Error("failed to serve internal grpc", slog.Any("error", err))` + `os.Exit(1)` | ERROR |

4. **Update Echo CORS logging** (lines 179, 187, 192, 198, 200):
   - Replace `e.Logger.Infof/Warnf/Printf` with `log.Info/Warn`
   - Add structured fields for origin

5. **Update gRPC server creation** (line 248):
```go
grpcServer := grpc.NewServer(
    grpc.UnaryInterceptor(
        grpc_logging.UnaryServerInterceptor(
            logger.InterceptorLogger(log),
            grpc_logging.WithLogOnEvents(grpc_logging.FinishCall),
        ),
    ),
)
```

#### 2.5 Update Service Files

**File: `/backend/telemetry/otel_span_processor.go`**
- Import: `"log/slog"`
- Replace 5 `log.Printf` statements (lines 87, 104, 111, 272)
- Use structured logging with context:

| Line | Current | New | Level |
|------|---------|-----|-------|
| 87 | `log.Printf("Unsupported array element type...")` | `slog.Warn("unsupported array element type", slog.String("attribute", attr.Key))` | WARN |
| 104 | `log.Printf("Unsupported kvlist element type...")` | `slog.Warn("unsupported kvlist element type", slog.String("attribute", attr.Key))` | WARN |
| 111 | `log.Printf("Unsupported attribute type: %T...")` | `slog.Warn("unsupported attribute type", slog.String("type", fmt.Sprintf("%T", v)), slog.String("key", attr.Key))` | WARN |
| 272 | `log.Printf("Error inserting patch: %v", err)` | `slog.Error("error inserting patch", slog.Any("error", err))` | ERROR |

**File: `/backend/db/db_init.go`**
- Replace 8 log statements
- Add structured fields: db_path, migration_version

**File: `/backend/db_duckdb/duckdb_init.go`**
- Replace 1 log statement
- Add structured fields: db_path

**File: `/backend/auth/services.go`**
- Replace 8 log statements
- Add user_id, session_id to structured logs where available

**File: `/backend/telemetry/otel_api_key_interceptor.go`**
- Replace 7 log statements
- Use `slog.InfoContext(ctx, ...)` for context-aware logging

#### 2.6 Echo Framework Integration
**File: `/backend/middleware/logger.go`** (new file)

Create custom Echo middleware that uses slog:
- Log HTTP requests with: method, path, status, latency, error
- Use INFO level for successful requests (2xx, 3xx)
- Use WARN level for client errors (4xx)
- Use ERROR level for server errors (5xx)
- Skip sensitive headers and CSRF tokens
- Add request_id if available

**File: `/backend/main.go`**
- Replace `e.Use(middleware.Logger())` with custom `m.SlogLogger(log)`

---

### Phase 3: Ingestion Service Implementation

#### 3.1 Create Logger Package
**File: `/ingestion-service/logger/logger.go`**

Identical implementation to backend:
- `InitLogger() *slog.Logger`
- Parse LOG_LEVEL and LOG_FORMAT from env
- Set as default global logger

#### 3.2 Add gRPC Middleware Dependency
**File: `/ingestion-service/go.mod`**

Add dependency:
```bash
go get github.com/grpc-ecosystem/go-grpc-middleware/v2
```

#### 3.3 Create gRPC Logger Adapter
**File: `/ingestion-service/logger/grpc.go`**

Same adapter as backend:
- `InterceptorLogger(l *slog.Logger) logging.Logger`

#### 3.4 Update main.go
**File: `/ingestion-service/main.go`**

**Changes required (~22 log statements):**

| Line | Current | New | Level |
|------|---------|-----|-------|
| 18 | `fmt.Println("Starting ingestion service...")` | `log.Info("starting ingestion service")` | INFO |
| 26 | `log.Fatalf("Failed to get user home directory: %v", err)` | `log.Error("failed to get user home directory", slog.Any("error", err))` + `os.Exit(1)` | ERROR |
| 33 | `log.Fatalf("Failed to create database directory at %s: %v", dbPath, err)` | `log.Error("failed to create database directory", slog.String("path", dbPath), slog.Any("error", err))` + `os.Exit(1)` | ERROR |
| 36 | `log.Printf("Initializing BadgerDB at: %s", dbPath)` | `log.Info("initializing badgerdb", slog.String("path", dbPath))` | INFO |
| 39 | `log.Fatalf("Failed to initialize storage: %v", err)` | `log.Error("failed to initialize storage", slog.Any("error", err))` + `os.Exit(1)` | ERROR |
| 43 | `log.Println("Storage initialized successfully.")` | `log.Info("storage initialized successfully")` | INFO |
| 53 | `log.Fatalf("Failed to create backend auth client: %v", err)` | `log.Error("failed to create backend auth client", slog.Any("error", err))` + `os.Exit(1)` | ERROR |
| 62 | `log.Println("Waiting for backend to be ready...")` | `log.Info("waiting for backend to be ready")` | INFO |
| 64 | `log.Fatalf("Backend connection failed: %v", err)` | `log.Error("backend connection failed", slog.Any("error", err))` + `os.Exit(1)` | ERROR |
| 72 | `log.Fatalf("Failed to create public gRPC server: %v", err)` | `log.Error("failed to create public grpc server", slog.Any("error", err))` + `os.Exit(1)` | ERROR |
| 76 | `log.Printf("Public gRPC server listening at %v", publicLis.Addr())` | `log.Info("public grpc server listening", slog.String("address", publicLis.Addr().String()))` | INFO |
| 78 | `log.Fatalf("Failed to serve public gRPC: %v", err)` | `log.Error("failed to serve public grpc", slog.Any("error", err))` + `os.Exit(1)` | ERROR |
| 85 | `log.Fatalf("Failed to create internal gRPC server: %v", err)` | `log.Error("failed to create internal grpc server", slog.Any("error", err))` + `os.Exit(1)` | ERROR |
| 89 | `log.Printf("Internal gRPC server listening at %v", internalLis.Addr())` | `log.Info("internal grpc server listening", slog.String("address", internalLis.Addr().String()))` | INFO |
| 91 | `log.Fatalf("Failed to serve internal gRPC: %v", err)` | `log.Error("failed to serve internal grpc", slog.Any("error", err))` + `os.Exit(1)` | ERROR |
| 100 | `log.Println("Shutting down gRPC servers...")` | `log.Info("shutting down grpc servers")` | INFO |
| 103 | `log.Println("gRPC servers stopped.")` | `log.Info("grpc servers stopped")` | INFO |
| 105 | `log.Println("Attempting to sync database to disk...")` | `log.Info("syncing database to disk")` | INFO |
| 108 | `log.Printf("Warning: failed to sync database: %v", err)` | `log.Warn("failed to sync database", slog.Any("error", err))` | WARN |
| 110 | `log.Println("Database sync completed successfully.")` | `log.Info("database sync completed")` | INFO |
| 113 | `log.Println("Attempting to close database...")` | `log.Info("closing database")` | INFO |
| 115 | `log.Fatalf("FATAL: Failed to close database: %v", err)` | `log.Error("failed to close database", slog.Any("error", err))` + `os.Exit(1)` | ERROR |
| 117 | `log.Println("Database closed successfully.")` | `log.Info("database closed successfully")` | INFO |

#### 3.5 Update server/server.go
**File: `/ingestion-service/server/server.go`**

**Add gRPC logging interceptors:**

**Public gRPC Server** (NewGRPCServer, line 40):
```go
import grpc_logging "github.com/grpc-ecosystem/go-grpc-middleware/v2/interceptors/logging"

// Update signature to accept logger
func NewGRPCServer(store *storage.Storage, authClient *backend_client.AuthClient, log *slog.Logger) (*grpc.Server, net.Listener, error) {
    // ... existing code ...

    grpcServer := grpc.NewServer(
        grpc.ChainUnaryInterceptor(
            grpc_logging.UnaryServerInterceptor(
                logger.InterceptorLogger(log),
                grpc_logging.WithLogOnEvents(
                    grpc_logging.StartCall,
                    grpc_logging.FinishCall,
                ),
            ),
            ApiKeyAuthInterceptor(authClient),
        ),
    )

    // ... rest of code ...
}
```

**Internal gRPC Server** (NewInternalGRPCServer, line 65):
```go
func NewInternalGRPCServer(store *storage.Storage, log *slog.Logger) (*grpc.Server, net.Listener, error) {
    // ... existing code ...

    grpcServer := grpc.NewServer(
        grpc.UnaryInterceptor(
            grpc_logging.UnaryServerInterceptor(
                logger.InterceptorLogger(log),
                grpc_logging.WithLogOnEvents(grpc_logging.FinishCall),
                grpc_logging.WithDecider(func(fullMethodName string, err error) bool {
                    // Skip logging health checks
                    return !strings.Contains(fullMethodName, "grpc.health")
                }),
            ),
        ),
    )

    // ... rest of code ...
}
```

#### 3.6 Update Service Files

**File: `/ingestion-service/storage/badger.go`**
- Replace 6 `log.Printf` statements (lines 57, 63, 69, 137, 147, 154)
- Add structured fields: db_path, operation, key_count

| Line | Current | New | Level |
|------|---------|-----|-------|
| 57 | `log.Printf("BadgerDB opened successfully at path: %s", path)` | `slog.Info("badgerdb opened", slog.String("path", path))` | INFO |
| 63 | `log.Println("Closing BadgerDB...")` | `slog.Info("closing badgerdb")` | INFO |
| 69 | `log.Println("Syncing BadgerDB to disk...")` | `slog.Info("syncing badgerdb to disk")` | INFO |
| 137 | `log.Printf("Error unmarshaling span data: %v", err)` | `slog.Warn("error unmarshaling span data", slog.Any("error", err))` | WARN |
| 147 | `log.Printf("Error marshaling span: %v", err)` | `slog.Warn("error marshaling span", slog.Any("error", err))` | WARN |
| 154 | `log.Printf("Error marshaling resource: %v", err)` | `slog.Warn("error marshaling resource", slog.Any("error", err))` | WARN |

**File: `/ingestion-service/backend_client/auth_client.go`**
- Replace 4 `log.Printf` statements (lines 37, 72, 83, 100)
- Use context-aware logging

| Line | Current | New | Level |
|------|---------|-----|-------|
| 37 | `log.Printf("Created gRPC client for backend auth service at %s", addr)` | `slog.Info("created grpc client for backend auth", slog.String("address", addr))` | INFO |
| 72 | `log.Printf("Failed to validate API key with backend: %v", err)` | `slog.Error("failed to validate api key with backend", slog.Any("error", err))` | ERROR |
| 83 | `log.Println("Waiting for backend gRPC server to be ready...")` | `slog.Info("waiting for backend grpc server to be ready")` | INFO |
| 100 | `log.Println("Backend gRPC server is ready!")` | `slog.Info("backend grpc server is ready")` | INFO |

**File: `/ingestion-service/server/otel_trace_service.go`**
- Replace 2 `log.Printf` statements (lines 32, 36)
- Use context-aware logging: `slog.InfoContext(ctx, ...)`

| Line | Current | New | Level |
|------|---------|-----|-------|
| 32 | `log.Printf("Received Span ID: %s, Trace ID: %s, Name: %s", spanID, traceID, span.Name)` | `slog.InfoContext(ctx, "received span", slog.String("span_id", spanID), slog.String("trace_id", traceID), slog.String("name", span.Name))` | INFO |
| 36 | `log.Printf("Error writing span to WAL: %v", err)` | `slog.ErrorContext(ctx, "error writing span to wal", slog.Any("error", err))` | ERROR |

**File: `/ingestion-service/server/otel_log_service.go`**
- Replace any log statements with structured slog

**File: `/ingestion-service/server/otel_metric_service.go`**
- Replace any log statements with structured slog

**File: `/ingestion-service/server/wal_reader_service.go`**
- Replace 5 `log.Printf` statements
- Add batch_size, start_key, span_count to structured logs

---

### Phase 4: Context-Aware Logging Enhancement

#### 4.1 Backend gRPC Handlers
For any gRPC service methods:
- Use `slog.InfoContext(ctx, ...)` instead of `slog.Info(...)`
- Automatically includes request metadata from interceptor
- Add trace_id correlation where available

#### 4.2 Ingestion Service Handlers
**Files: server/otel_*.go, server/wal_reader_service.go**
- Use `slog.InfoContext(ctx, ...)` for all logging
- Add trace/span correlation fields
- Log errors with full context

#### 4.3 Custom Fields in Context
Consider adding helper function to inject custom fields:
```go
func AddLogFields(ctx context.Context, fields ...slog.Attr) context.Context {
    // Add fields to context for automatic inclusion in logs
}
```

---

### Phase 5: Testing & Verification

#### 5.1 Log Level Testing

**Test different log levels:**

```bash
# Debug level - shows everything
LOG_LEVEL=debug LOG_FORMAT=text go run backend/main.go
# Should show: debug, info, warn, error logs

# Info level - normal operations (production default)
LOG_LEVEL=info LOG_FORMAT=json go run backend/main.go
# Should show: info, warn, error logs (no debug)

# Warn level - warnings and errors only
LOG_LEVEL=warn LOG_FORMAT=json go run backend/main.go
# Should show: warn, error logs only

# Error level - only errors
LOG_LEVEL=error LOG_FORMAT=json go run backend/main.go
# Should show: error logs only
```

**Verify:**
- ✅ Debug logs only appear when LOG_LEVEL=debug
- ✅ Info is the default level when LOG_LEVEL is not set
- ✅ Each level properly filters lower-severity logs

#### 5.2 Format Testing

**Test JSON output:**
```bash
LOG_LEVEL=info LOG_FORMAT=json go run backend/main.go
```
Verify:
- ✅ Output is valid JSON (parseable)
- ✅ Each log line is a JSON object
- ✅ Contains: time, level, msg, and structured fields
- ✅ Can be piped to `jq` for parsing

**Test text output:**
```bash
LOG_LEVEL=debug LOG_FORMAT=text go run backend/main.go
```
Verify:
- ✅ Output is human-readable
- ✅ Includes timestamp, level, message
- ✅ Structured fields are readable
- ✅ Color coding (if terminal supports it)

#### 5.3 gRPC Interceptor Testing

**Test automatic request logging:**
```bash
# Start ingestion service
LOG_LEVEL=info LOG_FORMAT=text go run ingestion-service/main.go

# Send test OTLP spans (use test client or curl)
```

Verify:
- ✅ gRPC requests automatically logged
- ✅ Request duration included
- ✅ Method name and status code logged
- ✅ No sensitive data (API keys) in logs
- ✅ Health checks filtered (if configured)

#### 5.4 Context Propagation Testing

**Test context-aware logging:**
```go
// In handler, should work:
slog.InfoContext(ctx, "processing request")
```

Verify:
- ✅ Context fields automatically included
- ✅ Trace ID correlation works (if added to context)
- ✅ Request-scoped metadata appears in logs

#### 5.5 Production Simulation

**Simulate production environment:**
```bash
LOG_LEVEL=info LOG_FORMAT=json docker-compose up
```

Verify:
- ✅ Only info, warn, error logs appear
- ✅ Debug logs are filtered out
- ✅ JSON output is consistent
- ✅ Log volume is manageable
- ✅ Critical events are captured

---

## Migration Checklist

### Pre-Migration
- [ ] Review current logging statements (~85 total)
- [ ] Identify which statements should be DEBUG vs INFO vs WARN vs ERROR
- [ ] Document any custom logging patterns to preserve
- [ ] Backup current codebase

### Backend Migration
- [ ] Create `/backend/logger/logger.go`
- [ ] Create `/backend/logger/grpc.go`
- [ ] Create `/backend/middleware/logger.go` (Echo middleware)
- [ ] Add grpc-middleware dependency
- [ ] Update `/backend/main.go` (17 log statements)
- [ ] Update `/backend/telemetry/otel_span_processor.go` (5 statements)
- [ ] Update `/backend/db/db_init.go` (8 statements)
- [ ] Update `/backend/db_duckdb/duckdb_init.go` (1 statement)
- [ ] Update `/backend/auth/services.go` (8 statements)
- [ ] Update `/backend/telemetry/otel_api_key_interceptor.go` (7 statements)
- [ ] Update gRPC server creation with interceptor

### Ingestion Service Migration
- [ ] Create `/ingestion-service/logger/logger.go`
- [ ] Create `/ingestion-service/logger/grpc.go`
- [ ] Add grpc-middleware dependency
- [ ] Update `/ingestion-service/main.go` (22 log statements)
- [ ] Update `/ingestion-service/server/server.go` (2 gRPC servers)
- [ ] Update `/ingestion-service/storage/badger.go` (6 statements)
- [ ] Update `/ingestion-service/backend_client/auth_client.go` (4 statements)
- [ ] Update `/ingestion-service/server/otel_trace_service.go` (2 statements)
- [ ] Update `/ingestion-service/server/otel_log_service.go`
- [ ] Update `/ingestion-service/server/otel_metric_service.go`
- [ ] Update `/ingestion-service/server/wal_reader_service.go` (5 statements)

### Environment Configuration
- [ ] Update `.env.example` with LOG_LEVEL and LOG_FORMAT
- [ ] Update `.env` with development defaults
- [ ] Document environment variables in README

### Testing
- [ ] Test LOG_LEVEL=debug (shows all logs)
- [ ] Test LOG_LEVEL=info (filters debug logs)
- [ ] Test LOG_LEVEL=warn (shows only warnings and errors)
- [ ] Test LOG_LEVEL=error (shows only errors)
- [ ] Test LOG_FORMAT=json (valid JSON output)
- [ ] Test LOG_FORMAT=text (human-readable output)
- [ ] Test gRPC interceptor logging
- [ ] Test context-aware logging
- [ ] Test production simulation (info + json)
- [ ] Verify no regressions in functionality

### Documentation
- [ ] Update README with logging configuration
- [ ] Document log levels and when to use each
- [ ] Document structured logging best practices
- [ ] Add examples of common logging patterns

---

## Summary

### Files to Create
1. `/backend/logger/logger.go`
2. `/backend/logger/grpc.go`
3. `/backend/middleware/logger.go`
4. `/ingestion-service/logger/logger.go`
5. `/ingestion-service/logger/grpc.go`

### Files to Modify
**Backend (15 files):**
1. `/backend/main.go` (~17 log statements)
2. `/backend/telemetry/otel_span_processor.go` (5)
3. `/backend/db/db_init.go` (8)
4. `/backend/db_duckdb/duckdb_init.go` (1)
5. `/backend/auth/services.go` (8)
6. `/backend/telemetry/otel_api_key_interceptor.go` (7)
7. `/backend/go.mod` (add dependency)

**Ingestion Service (8 files):**
8. `/ingestion-service/main.go` (~22 log statements)
9. `/ingestion-service/server/server.go` (2 gRPC servers)
10. `/ingestion-service/storage/badger.go` (6)
11. `/ingestion-service/backend_client/auth_client.go` (4)
12. `/ingestion-service/server/otel_trace_service.go` (2)
13. `/ingestion-service/server/otel_log_service.go`
14. `/ingestion-service/server/otel_metric_service.go`
15. `/ingestion-service/server/wal_reader_service.go` (5)
16. `/ingestion-service/go.mod` (add dependency)

**Configuration:**
17. `.env.example` (2 new environment variables)

### Dependencies to Add
- `github.com/grpc-ecosystem/go-grpc-middleware/v2` (only external dependency)

### Total Log Statements to Migrate
- **~85+ log statements** across both services

### Key Benefits
✅ **Zero dependencies** (except gRPC middleware for gRPC logging)
✅ **Future-proof** - standard library, supported forever
✅ **Environment-based control** - LOG_LEVEL and LOG_FORMAT
✅ **Production-ready** - JSON output, filtered logs
✅ **Developer-friendly** - text output, debug logs
✅ **Context-aware** - automatic request correlation
✅ **Structured** - machine-readable, queryable logs
✅ **Performance** - 40 B/op, minimal allocations
✅ **Intelligent log levels** - debug/info/warn/error properly categorized
✅ **Default to INFO** - production-safe out of the box

---

## Next Steps

1. **Review this plan** - Ensure all stakeholders agree on approach
2. **Create feature branch** - `git checkout -b feature/slog-migration`
3. **Implement Phase 1** - Environment configuration
4. **Implement Phase 2** - Backend service migration
5. **Test Backend** - Verify all log levels and formats
6. **Implement Phase 3** - Ingestion service migration
7. **Test Ingestion** - Verify all log levels and formats
8. **Integration Testing** - Test both services together
9. **Documentation** - Update README and docs
10. **Code Review** - Team review of changes
11. **Merge to main** - After approval and testing
12. **Deploy to production** - With LOG_LEVEL=info LOG_FORMAT=json

---

**Estimated Effort:** 4-6 hours for implementation, 2-3 hours for testing and documentation

**Risk Level:** Low (standard library, well-tested, backwards compatible with log output)

**Breaking Changes:** None (environment variables optional, sensible defaults)
