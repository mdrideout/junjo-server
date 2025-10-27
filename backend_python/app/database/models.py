"""Central import location for all database models.

This module imports all SQLAlchemy models to ensure they are registered with Base.metadata.
Both Alembic migrations (env.py) and tests (conftest.py) import from this module.

Pattern: Add new model imports here as you create them.
"""

# Import all models here
from app.database.users.models import UserTable  # noqa: F401
from app.database.api_keys.models import APIKeyTable  # noqa: F401

# Add future models here:
# from app.database.projects.models import ProjectTable  # noqa: F401
