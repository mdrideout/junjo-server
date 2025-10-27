"""
Main FastAPI application entry point.

Sets up the FastAPI app with:
- CORS middleware
- Loguru logging
- Health check endpoints
- Feature routers (will be added in later phases)

Pattern from wt_api_v2 (validated for production use).
"""

from contextlib import asynccontextmanager
from fastapi import FastAPI
from fastapi.middleware.cors import CORSMiddleware
from loguru import logger
from securecookies import SecureCookiesMiddleware
from starlette.middleware.sessions import SessionMiddleware

from app.config.settings import settings
from app.config.logger import setup_logging
from app.common.responses import HealthResponse
from app.features.auth import router as auth_router


# Set up logging before anything else
setup_logging()


@asynccontextmanager
async def lifespan(app: FastAPI):
    """Application lifespan manager.

    Handles startup and shutdown events for the FastAPI application.
    Logs startup configuration and performs cleanup on shutdown.

    Args:
        app: FastAPI application instance.

    Yields:
        Control to the application runtime.
    """
    # Startup
    logger.info("=" * 60)
    logger.info(f"Starting {settings.app_name}")
    logger.info(f"Python 3.14+ with Pydantic v2")
    logger.info(f"Debug mode: {settings.debug}")
    logger.info(f"Server: {settings.host}:{settings.port}")
    logger.info(f"CORS origins: {settings.cors_origins}")
    logger.info("=" * 60)

    yield

    # Shutdown
    logger.info("Shutting down application")
    # Database cleanup (Phase 2 complete)
    from app.database.db_config import checkpoint_wal, engine

    await checkpoint_wal()  # Checkpoint SQLite WAL
    logger.info("SQLite WAL checkpointed")
    await engine.dispose()  # Close database connections
    logger.info("Database connections closed")


# Create FastAPI app
app = FastAPI(
    title=settings.app_name,
    description="LLM Observability Platform - Python Backend",
    version="0.1.0",
    debug=settings.debug,
    lifespan=lifespan,
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
app.add_middleware(
    SessionMiddleware,
    secret_key=settings.session_cookie.session_secret,  # Signing key
    max_age=86400 * 30,  # 30 days (matches Go implementation)
    https_only=True,  # HTTPS in production (HTTP in dev is OK)
    same_site="strict",  # CSRF protection
)

# === REQUEST/RESPONSE FLOW ===
# Incoming:  Browser → SecureCookiesMiddleware (decrypt) → SessionMiddleware (verify signature) → request.session populated
# Outgoing:  request.session modified → SessionMiddleware (sign) → SecureCookiesMiddleware (encrypt) → Browser

# === ROUTERS ===
app.include_router(auth_router.router, tags=["auth"])


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
        app_name=settings.app_name,
    )


# Root endpoint
@app.get("/", tags=["Root"])
async def root() -> dict[str, str]:
    """Root endpoint with API information.

    Returns:
        Dictionary containing app name, version, and documentation links.
    """
    return {
        "app": settings.app_name,
        "version": "0.1.0",
        "docs": "/docs",
        "health": "/health",
    }


if __name__ == "__main__":
    import uvicorn

    uvicorn.run(
        "app.main:app",
        host=settings.host,
        port=settings.port,
        reload=settings.debug,
        log_config=None,  # Disable uvicorn logging (we use loguru)
    )
