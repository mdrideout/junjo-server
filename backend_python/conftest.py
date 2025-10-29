"""Pytest configuration and fixtures.

Provides test database setup with temporary SQLite files.

IMPORTANT: Environment variables are set BEFORE any app imports to ensure
the production db_config.py (which creates engine at import time) points
to a test location instead of the real production database.
"""

import os
import tempfile

# Set test database path BEFORE importing app code
# This ensures production db_config.py creates engine pointing to test location
_test_base_dir = tempfile.mkdtemp(prefix="junjo_test_")
os.environ["DB_SQLITE_PATH"] = f"{_test_base_dir}/production_stub.db"

# NOW safe to import app code (db_config will use test path)
import pytest_asyncio
from sqlalchemy.ext.asyncio import create_async_engine, async_sessionmaker

from app.db_sqlite.base import Base


@pytest_asyncio.fixture(scope="function", autouse=True)
async def test_db():
    """Create test database in temporary directory for each test.

    Each test gets a completely isolated database file that is cleaned up after.

    Yields:
        async_sessionmaker: Session factory for test database
    """
    # Import models to register them with Base.metadata
    # Done here to ensure fresh registration for each test
    from app.db_sqlite import models  # noqa: F401

    # Create temporary database file
    temp_dir = tempfile.mkdtemp()
    db_path = os.path.join(temp_dir, "test.db")
    db_url = f"sqlite+aiosqlite:///{db_path}"

    # Create engine
    engine = create_async_engine(db_url, echo=False)

    # Create all tables
    async with engine.begin() as conn:
        await conn.run_sync(Base.metadata.create_all)

    # Create session factory
    async_session_test = async_sessionmaker(
        engine,
        expire_on_commit=False
    )

    # Override global session with test session
    import app.db_sqlite.db_config as db_config
    original_session = db_config.async_session
    original_engine = db_config.engine
    db_config.async_session = async_session_test
    db_config.engine = engine  # Also override engine for complete isolation

    yield async_session_test

    # Restore original session and engine
    db_config.async_session = original_session
    db_config.engine = original_engine

    # Cleanup
    await engine.dispose()

    # Remove temp database file
    try:
        if os.path.exists(db_path):
            os.remove(db_path)
        os.rmdir(temp_dir)
    except Exception:
        pass  # Ignore cleanup errors
