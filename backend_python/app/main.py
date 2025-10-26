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

from app.config.settings import settings
from app.config.logger import setup_logging
from app.common.responses import HealthResponse


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
    # NOTE: In Phase 2, add database cleanup:
    # from app.database.db_config import checkpoint_wal, engine
    # await checkpoint_wal()  # Checkpoint SQLite WAL
    # await engine.dispose()  # Close database connections


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
