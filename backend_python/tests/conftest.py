"""Pytest configuration and fixtures.

Provides test database setup with temporary SQLite files.
"""

import os
import tempfile
import pytest_asyncio
from sqlalchemy.ext.asyncio import create_async_engine, async_sessionmaker

from app.database.base import Base


@pytest_asyncio.fixture(scope="function", autouse=True)
async def test_db():
    """Create test database in temporary directory for each test.

    Each test gets a completely isolated database file that is cleaned up after.

    Yields:
        async_sessionmaker: Session factory for test database
    """
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
    import app.database.db_config as db_config
    original_session = db_config.async_session
    db_config.async_session = async_session_test

    yield async_session_test

    # Restore original session
    db_config.async_session = original_session

    # Cleanup
    await engine.dispose()

    # Remove temp database file
    try:
        if os.path.exists(db_path):
            os.remove(db_path)
        os.rmdir(temp_dir)
    except Exception:
        pass  # Ignore cleanup errors
