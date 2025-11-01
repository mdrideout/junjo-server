"""
Main FastAPI application entry point.

Sets up the FastAPI app with:
- CORS middleware
- Loguru logging
- Health check endpoints
- Feature routers (will be added in later phases)
- gRPC server for internal authentication (runs concurrently)

Pattern from wt_api_v2 (validated for production use).
"""

import asyncio
from contextlib import asynccontextmanager
import os

from fastapi import FastAPI
from fastapi.middleware.cors import CORSMiddleware
from loguru import logger
from securecookies import SecureCookiesMiddleware
from starlette.middleware.sessions import SessionMiddleware

from app.common.responses import HealthResponse
from app.config.logger import setup_logging
from app.config.settings import settings
from app.features.api_keys.router import router as api_keys_router
from app.features.auth.router import router as auth_router
from app.features.llm_playground.router import router as llm_playground_router
from app.features.otel_spans.router import router as otel_spans_router
from app.grpc_server import start_grpc_server_background, stop_grpc_server


# Set up logging before anything else
setup_logging()


@asynccontextmanager
async def lifespan(app: FastAPI):
    """Application lifespan manager.

    Handles startup and shutdown events for the FastAPI application.
    Logs startup configuration, starts gRPC server, and performs cleanup on shutdown.

    Args:
        app: FastAPI application instance.

    Yields:
        Control to the application runtime.
    """
    # Startup
    logger.info("=" * 60)
    logger.info("Starting Junjo Server")
    logger.info("Python 3.13+ with Pydantic v2")
    logger.info(f"FastAPI Server: http://0.0.0.0:{settings.port}")
    logger.info(f"gRPC Server: [::]:{settings.GRPC_PORT}")
    logger.info(f"CORS origins: {settings.cors_origins}")
    logger.info("=" * 60)

    # Configure LiteLLM environment variables
    if settings.llm.openai_api_key:
        os.environ["OPENAI_API_KEY"] = settings.llm.openai_api_key
        logger.info("OpenAI API key configured")
    if settings.llm.anthropic_api_key:
        os.environ["ANTHROPIC_API_KEY"] = settings.llm.anthropic_api_key
        logger.info("Anthropic API key configured")
    if settings.llm.gemini_api_key:
        os.environ["GEMINI_API_KEY"] = settings.llm.gemini_api_key
        logger.info("Gemini API key configured")

    # Initialize DuckDB tables
    from app.db_duckdb.db_config import initialize_tables
    initialize_tables()
    logger.info("DuckDB tables initialized")

    # Validate deployment configuration
    logger.info("-" * 60)
    if settings.session_cookie.junjo_env == "production":
        # Production mode checks
        logger.info("🔒 Production mode detected")

        # Validate CORS origins
        if not settings.cors_origins or settings.cors_origins == ["*"]:
            logger.error(
                "❌ PRODUCTION ERROR: CORS_ORIGINS must be explicitly configured. "
                "Set CORS_ORIGINS environment variable to your frontend domain(s). "
                "See docs/DEPLOYMENT.md for details."
            )
            raise ValueError("CORS_ORIGINS required in production")

        logger.info("✅ HTTPS-only cookies enabled")
        logger.info(f"✅ CORS origins configured: {settings.cors_origins}")
        logger.info("✅ Session cookies: Encrypted (AES-256) + Signed (HMAC)")
        logger.info("✅ CSRF protection: SameSite=Strict")
        logger.info("")
        logger.info("⚠️  DEPLOYMENT REQUIREMENT:")
        logger.info("   Backend and frontend must be on same domain")
        logger.info("   (e.g., api.example.com + app.example.com)")
        logger.info("   See docs/DEPLOYMENT.md for setup instructions")
    else:
        # Development mode
        logger.info("🔧 Development mode")
        logger.info("⚠️  HTTPS-only cookies disabled (development only)")
        logger.info("✅ Session cookies: Encrypted (AES-256) + Signed (HMAC)")
        logger.info("✅ CSRF protection: SameSite=Strict")
    logger.info("-" * 60)

    # Start span ingestion poller as background task
    from app.features.span_ingestion.background_poller import span_ingestion_poller
    poller_task = asyncio.create_task(span_ingestion_poller())
    logger.info("Span ingestion poller task created")

    # Start gRPC server as background task
    grpc_task = asyncio.create_task(start_grpc_server_background())
    logger.info("gRPC server task created")

    yield

    # Shutdown
    logger.info("Shutting down application")

    # Stop span ingestion poller
    if not poller_task.done():
        poller_task.cancel()
        try:
            await poller_task
        except asyncio.CancelledError:
            logger.info("Span ingestion poller cancelled")

    # Stop gRPC server
    await stop_grpc_server()

    # Cancel gRPC task if still running
    if not grpc_task.done():
        grpc_task.cancel()
        try:
            await grpc_task
        except asyncio.CancelledError:
            logger.info("gRPC task cancelled")

    # Database cleanup
    from app.db_sqlite.db_config import checkpoint_wal, engine

    await checkpoint_wal()  # Checkpoint SQLite WAL
    logger.info("SQLite WAL checkpointed")
    await engine.dispose()  # Close database connections
    logger.info("Database connections closed")


# Create FastAPI app
app = FastAPI(
    title="Junjo Server",
    description="LLM Observability Platform - Python Backend",
    version="0.1.0",
    lifespan=lifespan,
    # Disable Swagger UI and ReDoc (not needed for production deployments)
    docs_url=None,
    redoc_url=None,
)

# CORS middleware
app.add_middleware(
    CORSMiddleware,
    allow_origins=settings.cors_origins,
    allow_credentials=True,
    allow_methods=["*"],
    allow_headers=["*"],
)

# === SESSION/AUTH MIDDLEWARE (ORDER IS CRITICAL!) ===

# 1. Add ENCRYPTION middleware FIRST (outer layer)
#    This encrypts/decrypts all cookies before they reach SessionMiddleware
app.add_middleware(
    SecureCookiesMiddleware,
    secrets=[settings.session_cookie.secure_cookie_key],  # 32-byte encryption key
)

# 2. Add SESSION middleware SECOND (inner layer)
#    This signs/validates session data
#    https_only should be True in production, False in development/test
is_production = settings.session_cookie.junjo_env == "production"
app.add_middleware(
    SessionMiddleware,
    secret_key=settings.session_cookie.session_secret,  # Signing key
    max_age=86400 * 30,  # 30 days (matches Go implementation)
    https_only=is_production,  # HTTPS required in production only
    same_site="strict",  # CSRF protection
)

# === REQUEST/RESPONSE FLOW ===
# Incoming:  Browser → SecureCookiesMiddleware (decrypt) → SessionMiddleware (verify signature) → request.session populated
# Outgoing:  request.session modified → SessionMiddleware (sign) → SecureCookiesMiddleware (encrypt) → Browser

# === ROUTERS ===
app.include_router(auth_router, tags=["auth"])
app.include_router(api_keys_router)
app.include_router(llm_playground_router, prefix="/llm", tags=["llm"])
app.include_router(
    otel_spans_router, prefix="/api/v1/observability", tags=["observability"]
)


# Health check endpoints
@app.get("/ping", response_model=str, tags=["Health"])
async def ping() -> str:
    """Simple ping endpoint.

    Returns:
        String "pong" to confirm server is responding.
    """
    logger.debug("Ping endpoint called")
    return "pong"


@app.get("/health", response_model=HealthResponse, tags=["Health"])
async def health() -> HealthResponse:
    """Detailed health check endpoint.

    Returns:
        HealthResponse with status, version, and application name.
    """
    logger.debug("Health endpoint called")
    return HealthResponse(
        status="ok",
        version="0.1.0",
        app_name="Junjo Server",
    )


# Root endpoint
@app.get("/", tags=["Root"])
async def root() -> dict[str, str]:
    """Root endpoint with API information.

    Returns:
        Dictionary containing app name, version, and health check link.
    """
    return {
        "app": "Junjo Server",
        "version": "0.1.0",
        "health": "/health",
    }


if __name__ == "__main__":
    import uvicorn

    # Auto-reload in development only
    reload = settings.session_cookie.junjo_env == "development"

    uvicorn.run(
        "app.main:app",
        host="0.0.0.0",  # Listen on all interfaces (required for Docker)
        port=settings.port,
        reload=reload,
        log_config=None,  # Disable uvicorn logging (we use loguru)
    )
