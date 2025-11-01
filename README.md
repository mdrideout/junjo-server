# Junjo Server

> 順序 - order, sequence, procedure

**Junjo Server** is an OpenTelemetry ingestion server and AI graph workflow debugging interface for the [Junjo Python Library](https://github.com/mdrideout/junjo).

If you've struggled to understand what decisions your LLM is making in a chained sequence of events, and what data it is basing its decisions on, Junjo Server is for you. You will gain complete visibility to the state of the application, and every change LLMs make to the application state.

**Complex, mission critical AI workflows are made transparent and understandable with Junjo and Junjo Server.**

<img src="https://python-api.junjo.ai/_images/junjo-screenshot.png" width="600" />

_Junjo Server Frontend Screenshot_

### Key Features

- 🔍 **Real-time LLM Decision Visibility** - See every decision your LLM makes and the data it uses
- 📊 **OpenTelemetry Native** - Standards-based telemetry ingestion via gRPC
- 🎯 **Workflow Debugging Interface** - Visual debugging of AI graph workflows
- 🪶 **Prompt Playground** - Expirement with other models and prompt tweaks while you debug
- 🚀 **High-Performance Ingestion** - Decoupled ingestion service with BadgerDB WAL
- 🔒 **Production-Ready Security** - Session-based authentication with encrypted cookies
- 💾 **Low Resource Requirements** - Runs on shared vCPU with 1GB RAM

---

## Table of Contents

- [Quick Start](#quick-start)
- [Features](#features)
- [Architecture](#architecture)
- [Prerequisites](#prerequisites)
- [Configuration](#configuration)
- [Production Deployment](#production-deployment)
- [Advanced Topics](#advanced-topics)
- [Troubleshooting](#troubleshooting)
- [Resources](#resources)

---

## Quick Start

Get Junjo Server running on your local machine in 5 minutes using the **[Junjo Server Bare Bones](https://github.com/mdrideout/junjo-server-bare-bones)** repository. This repository provides a minimal, standalone setup using pre-built Docker images from Docker Hub, perfect for quick testing and production deployments of Junjo Server.

### Steps

1. **Clone the bare-bones repository**
   ```bash
   git clone https://github.com/mdrideout/junjo-server-bare-bones.git
   cd junjo-server-bare-bones
   ```

2. **Copy the environment configuration**
   ```bash
   cp .env.example .env
   ```

3. **Generate security secrets**
   ```bash
   # Generate session secret (copy this output)
   openssl rand -base64 32

   # Generate secure cookie key (copy this output)
   openssl rand -base64 32
   ```

   Open `.env` in a text editor and replace the placeholder values:
   - Replace `your_base64_secret_here` in `JUNJO_SESSION_SECRET` with the first generated value
   - Replace `your_base64_key_here` in `JUNJO_SECURE_COOKIE_KEY` with the second generated value

   _See the [Bare Bones template repository](https://github.com/mdrideout/junjo-server-bare-bones/blob/master/README.md) for in-depth configuration instructions._
 
4. **Create the Docker network** (first time only)
   ```bash
   docker network create junjo-network
   ```

5. **Start all services**
   ```bash
   docker compose up -d
   ```

6. **Access Junjo Server**
   - **Frontend**: http://localhost:5153
   - **Backend API**: http://localhost:1323
   - **OTLP Ingestion Endpoint**: grpc://localhost:50051

7. **Create your first user**
   - Navigate to http://localhost:5153
   - Follow the setup wizard to create your admin account

8. **Create an API key** (for sending telemetry from your Junjo app)
   - Sign in to the web UI
   - Navigate to **Settings → API Keys**
   - Click **Create API Key**
   - Copy the 64-character key (shown only once)
   - Use this key in your Junjo Python Library application

### Useful Docker Compose Commands

```bash
# View logs from all services
docker compose logs -f

# View logs from specific service
docker compose logs -f junjo-server-backend
docker compose logs -f junjo-server-ingestion
docker compose logs -f junjo-server-frontend

# Stop services (keeps data)
docker compose down

# Restart a specific service
docker compose restart junjo-server-backend

# View running containers and their status
docker compose ps

# Stop and remove all data (fresh start)
docker compose down -v
```

### Next Steps

Configure your [Junjo Python Library](https://github.com/mdrideout/junjo) application to send telemetry to `grpc://localhost:50051` using the API key you created.

**For source code development**: If you want to modify Junjo Server's source code (not just use it), see the development guides in `backend/README.md`, `frontend/README.md`, and `ingestion-service/README.md` in the main [junjo-server repository](https://github.com/mdrideout/junjo-server).

---

## Features

### What Can You Do With Junjo Server?

**Observability & Debugging:**
- View complete execution traces of AI workflows
- Inspect LLM prompts, responses, and reasoning
- Track state changes across workflow nodes
- Monitor performance and latency

**LLM Playground:**
- Test prompts with multiple providers (OpenAI, Anthropic, Google Gemini)
- Compare responses across models
- Experiment with temperature and reasoning modes

**OpenTelemetry Integration:**
- Standards-compliant OTLP/gRPC ingestion endpoint
- Automatic trace collection from Junjo Python Library
- Custom span attributes for AI-specific metadata

**Multi-Service Architecture:**
- Decoupled ingestion for high throughput
- Web UI for visualization
- REST API for programmatic access

---

## Architecture

The Junjo Server is composed of three primary services:

### 1. Backend (`junjo-server-backend`)
- **Tech Stack**: FastAPI (Python), SQLite, DuckDB
- **Responsibilities**:
  - HTTP REST API
  - User authentication & session management
  - LLM playground
  - Span querying & analytics

### 2. Ingestion Service (`junjo-server-ingestion`)
- **Tech Stack**: Go, gRPC, BadgerDB
- **Responsibilities**:
  - OpenTelemetry OTLP/gRPC endpoint (port 50051)
  - High-throughput span ingestion
  - Durable Write-Ahead Log (WAL) with BadgerDB

### 3. Frontend (`junjo-server-frontend`)
- **Tech Stack**: React, TypeScript
- **Responsibilities**:
  - Web UI for workflow visualization
  - LLM playground interface
  - User management

**Data Flow:**
```
Junjo Python App → Ingestion Service (gRPC) → BadgerDB WAL
                                                    ↓
Backend Service ← polls for new spans ← BadgerDB WAL
       ↓
    DuckDB (analytics)
    SQLite (metadata)
       ↓
    Frontend UI
```

The `backend` service polls the `ingestion-service`'s internal gRPC API to read batches of spans from the WAL, which it then indexes into DuckDB for analytics queries.

---

## Prerequisites

### Required
- **Docker** and **Docker Compose** (for both development and production)

### Optional (Development)
- **Go 1.21+** (for ingestion service development)
- **Python 3.13+** with **uv** (for backend development)
- **Node.js 18+** (for frontend development)
- **BadgerDB CLI** (for database inspection)

### For Production Deployment
- A domain or subdomain for hosting (see [Deployment Requirements](#deployment-requirements))
- SSL certificates (automatic with Caddy, Let's Encrypt, etc.)

---

## Configuration

### Environment Variables

Junjo Server uses a single `.env` file at the root of the project. All services read from this file.

#### Key Configuration Variables

```bash
# === Build & Environment ===========================================
# Build Target: development | production
JUNJO_BUILD_TARGET="development"

# Running Environment: development | production
# (affects cookie security, logging, etc.)
JUNJO_ENV="development"

# === Security (REQUIRED for production) ============================
# Generate both with: openssl rand -base64 32
JUNJO_SESSION_SECRET=your_base64_secret_here
JUNJO_SECURE_COOKIE_KEY=your_base64_key_here

# === CORS ==========================================================
# Comma-separated list of allowed origins
JUNJO_ALLOW_ORIGINS=http://localhost:5151,http://localhost:5153

# === Ports =========================================================
PORT=1323                    # Backend HTTP port
INGESTION_PORT=50051        # OTLP ingestion gRPC port (public)
GRPC_PORT=50053             # Backend internal gRPC port

# === Database Paths ================================================
DB_SQLITE_PATH=/dbdata/sqlite/junjo.db
DB_DUCKDB_PATH=/dbdata/duckdb/traces.duckdb
BADGERDB_PATH=/dbdata/badgerdb

# === Logging =======================================================
LOG_LEVEL=info              # debug | info | warn | error
LOG_FORMAT=text             # json | text

# === LLM API Keys (optional) =======================================
OPENAI_API_KEY=sk-...
ANTHROPIC_API_KEY=sk-ant-...
GEMINI_API_KEY=...
```

**See `.env.example` for complete configuration with detailed comments.**

### Creating API Keys

After starting Junjo Server:
1. Sign in to the web UI (http://localhost:5153)
2. Navigate to **Settings → API Keys**
3. Click **Create API Key**
4. Copy the 64-character key (shown only once)
5. Use this key in your Junjo Python Library application

---

## Production Deployment

### Deployment Requirements

⚠️ **IMPORTANT**: The backend API and frontend **MUST be deployed on the same domain** (sharing the same registrable domain).

**Supported configurations:**
- ✅ `api.example.com` + `app.example.com` (subdomain + subdomain)
- ✅ `api.example.com` + `example.com` (subdomain + apex)
- ✅ `example.com` + `api.example.com` (apex + subdomain)
- ❌ `app.example.com` + `service.run.app` (different domains - **will NOT work**)

**Why?** Junjo Server uses session cookies with `SameSite=Strict` for security (CSRF protection). Cross-domain deployments will cause authentication to fail.

**📖 See [docs/DEPLOYMENT.md](docs/DEPLOYMENT.md) for complete deployment guide**, including:
- Platform-specific examples (Google Cloud Run, AWS, Docker Compose)
- Environment configuration for production
- Security features and best practices
- Troubleshooting guide

### Turn-Key Example Repositories

#### Junjo Server Bare Bones
**[https://github.com/mdrideout/junjo-server-bare-bones](https://github.com/mdrideout/junjo-server-bare-bones)**

A minimal, standalone repository with just the core Junjo Server components using pre-built Docker images.

**Best for:**
- Quick testing of Junjo Server
- Simple production deployments
- Integration into existing infrastructure

#### Junjo Server Deployment Example
**[https://github.com/mdrideout/junjo-server-deployment-example](https://github.com/mdrideout/junjo-server-deployment-example)**

A complete, production-ready example that includes a Junjo Python Library application alongside the server infrastructure.

**Best for:**
- End-to-end deployment examples
- Learning how to configure your Junjo app with the server
- VM deployment guide (Digital Ocean Droplet, AWS EC2, etc.)
- Caddy reverse proxy setup for SSL

The [README](https://github.com/mdrideout/junjo-server-deployment-example/blob/master/README.md) provides step-by-step deployment instructions.

### Docker Compose - Production Images

Junjo Server is built and deployed to **Docker Hub** with each GitHub release:

- **Backend**: [mdrideout/junjo-server-backend](https://hub.docker.com/r/mdrideout/junjo-server-backend)
- **Ingestion Service**: [mdrideout/junjo-server-ingestion-service](https://hub.docker.com/r/mdrideout/junjo-server-ingestion-service)
- **Frontend**: [mdrideout/junjo-server-frontend](https://hub.docker.com/r/mdrideout/junjo-server-frontend)

**Example docker-compose.yml:**

```yaml
services:
  junjo-server-backend:
    image: mdrideout/junjo-server-backend:latest
    container_name: junjo-server-backend
    restart: unless-stopped
    volumes:
      - ./.dbdata/sqlite:/dbdata/sqlite
      - ./.dbdata/duckdb:/dbdata/duckdb
    ports:
      - "1323:1323"   # API server
      - "50053:50053" # Internal gRPC
    networks:
      - junjo-network
    env_file:
      - .env
    user: root
    healthcheck:
      test: ["CMD", "curl", "-f", "http://localhost:1323/ping"]
      interval: 5s
      timeout: 3s
      retries: 25
      start_period: 5s

  junjo-server-ingestion:
    image: mdrideout/junjo-server-ingestion-service:latest
    container_name: junjo-server-ingestion
    restart: unless-stopped
    volumes:
      - ./.dbdata/badgerdb:/dbdata/badgerdb
    ports:
      - "50051:50051"  # OTLP ingestion (public)
      - "50052:50052"  # Internal gRPC
    networks:
      - junjo-network
    env_file:
      - .env
    healthcheck:
      test: ["CMD", "grpc_health_probe", "-addr=localhost:50052"]
      interval: 5s
      timeout: 3s
      retries: 5
      start_period: 5s

  junjo-server-frontend:
    image: mdrideout/junjo-server-frontend:latest
    container_name: junjo-server-frontend
    restart: unless-stopped
    ports:
      - "5153:80"
    env_file:
      - .env
    networks:
      - junjo-network
    depends_on:
      junjo-server-backend:
        condition: service_healthy
      junjo-server-ingestion:
        condition: service_healthy

networks:
  junjo-network:
    driver: bridge
```

For a more complete example with reverse proxy, see the [Junjo Server Deployment Example Repository](https://github.com/mdrideout/junjo-server-deployment-example/blob/master/docker-compose.yml).

### VM Resource Requirements

Junjo Server is designed to be low resource:
- **Minimum**: Shared vCPU + 1GB RAM
- **Databases**: SQLite, DuckDB, BadgerDB (all embedded, low overhead)
- **Recommended**: 1 vCPU + 2GB RAM for production workloads

---

## Advanced Topics

### Database Access

#### Inspecting BadgerDB

BadgerDB is the Write-Ahead Log for ingested spans. You can inspect it using the BadgerDB CLI.

**Install BadgerDB CLI:**
```bash
go install github.com/dgraph-io/badger/v4/badger@latest
```

**Inspect data:**
```bash
# Read the database (requires ingestion service to be stopped)
badger stream --dir ./.dbdata/badgerdb

# Read with cleanup (use if database wasn't shut down properly)
badger stream --dir ./.dbdata/badgerdb --read_only=false
```

**Note**: BadgerDB is a directory-based database that creates multiple files (`value log`, `manifest`, etc.), unlike SQLite/DuckDB which are single-file databases.

#### Accessing SQLite and DuckDB

```bash
# SQLite (user data, API keys)
sqlite3 ./.dbdata/sqlite/junjo.db

# DuckDB (span analytics)
duckdb ./.dbdata/duckdb/traces.duckdb
```

### Performance Tuning

- **Ingestion throughput**: Adjust `SPAN_BATCH_SIZE` and `SPAN_POLL_INTERVAL` in `.env`
- **Database performance**: DuckDB and SQLite use WAL mode for better concurrency
- **Container resources**: Increase memory limits if processing high span volumes

---

## Troubleshooting

### Session Cookie / Authentication Issues

**Symptom**: Can't sign in, or immediately signed out after login.

**Causes & Solutions:**

1. **Multiple Junjo instances on localhost**
   - Old session cookies from another instance may interfere
   - **Fix**: Clear browser cookies for `localhost` and restart services

2. **Cross-domain deployment** (most common in production)
   - Frontend and backend on different top-level domains
   - **Fix**: Ensure both services share the same registrable domain (see [Deployment Requirements](#deployment-requirements))

3. **Missing or invalid secrets**
   - `JUNJO_SESSION_SECRET` or `JUNJO_SECURE_COOKIE_KEY` not set correctly
   - **Fix**: Generate new secrets with `openssl rand -base64 32`

4. **CORS misconfiguration**
   - Frontend URL not in `JUNJO_ALLOW_ORIGINS`
   - **Fix**: Add your frontend URL to the CORS origins list

**See [docs/DEPLOYMENT.md](docs/DEPLOYMENT.md#troubleshooting) for detailed troubleshooting guide.**

### Port Conflicts

**Symptom**: `Error: bind: address already in use`

**Solution:**
```bash
# Find process using the port
lsof -i :1323  # or :50051, :5153, etc.

# Kill the process
kill -9 <PID>

# Or change the port in .env
PORT=1324
```

### Container Startup Issues

**Symptom**: Services fail to start or health checks fail

**Solutions:**

1. **Check logs**
   ```bash
   docker compose logs backend
   docker compose logs ingestion
   docker compose logs frontend
   ```

2. **Ensure network exists**
   ```bash
   docker network create junjo-network
   ```

3. **Clear volumes and rebuild**
   ```bash
   docker compose down -v
   docker compose up --build
   ```

4. **Check .env file**
   - Ensure all required variables are set
   - Secrets must be base64-encoded 32-byte values

### Database Issues

**Symptom**: Database errors or corruption warnings

**Solution:**
```bash
# Stop services
docker compose down

# Backup and clear database files
mv .dbdata .dbdata.backup

# Restart (will create fresh databases)
docker compose up
```

---

## Resources

### Documentation
- **[Deployment Guide](docs/DEPLOYMENT.md)** - Complete production deployment instructions
- **[Junjo Python Library](https://github.com/mdrideout/junjo)** - AI Graph Workflow framework

### Example Repositories
- **[Junjo Server Bare Bones](https://github.com/mdrideout/junjo-server-bare-bones)** - Minimal setup with pre-built images
- **[Junjo Server Deployment Example](https://github.com/mdrideout/junjo-server-deployment-example)** - Complete production deployment with Caddy

### Docker Hub Images
- **[junjo-server-backend](https://hub.docker.com/r/mdrideout/junjo-server-backend)** - FastAPI backend
- **[junjo-server-ingestion-service](https://hub.docker.com/r/mdrideout/junjo-server-ingestion-service)** - Go gRPC ingestion service
- **[junjo-server-frontend](https://hub.docker.com/r/mdrideout/junjo-server-frontend)** - React frontend

### OpenTelemetry Resources
- **[OpenTelemetry Documentation](https://opentelemetry.io/docs/)** - OTLP specification
- **[OpenTelemetry Python](https://opentelemetry-python.readthedocs.io/)** - Python SDK

---

**Junjo Server** - Making AI workflows transparent and understandable.
