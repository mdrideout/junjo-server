"""
Application settings using Pydantic Settings v2.

Environment variables are loaded from .env file and can be overridden
by actual environment variables.

Pattern validated for high-concurrency asyncio environments.
"""

from pathlib import Path
from typing import Annotated

from pydantic import Field, computed_field, field_validator
from pydantic_settings import BaseSettings, SettingsConfigDict


class DatabaseSettings(BaseSettings):
    """Database configuration"""

    sqlite_path: Annotated[
        str,
        Field(
            default="../.dbdata/sqlite/junjo.db",
            description="Path to SQLite database file (relative to backend_python/)"
        )
    ]
    duckdb_path: Annotated[
        str,
        Field(
            default="../.dbdata/duckdb/traces.duckdb",
            description="Path to DuckDB database file (relative to backend_python/)"
        )
    ]

    @computed_field  # type: ignore[prop-decorator]
    @property
    def sqlite_url(self) -> str:
        """Computed SQLite async URL with absolute path.

        Returns:
            SQLite connection URL for async SQLAlchemy engine with absolute path.
        """
        # Resolve to absolute path (handles relative paths from any working directory)
        abs_path = Path(self.sqlite_path).resolve()
        # Ensure parent directory exists
        abs_path.parent.mkdir(parents=True, exist_ok=True)
        return f"sqlite+aiosqlite:///{abs_path}"

    model_config = SettingsConfigDict(
        env_prefix="DB_",
        env_file=".env",
        env_file_encoding="utf-8",
        extra="ignore",
    )


class IngestionServiceSettings(BaseSettings):
    """Ingestion service gRPC connection settings"""

    host: Annotated[
        str,
        Field(
            default="localhost",
            description="Ingestion service gRPC host"
        )
    ]
    port: Annotated[
        int,
        Field(
            default=50052,
            ge=1,
            le=65535,
            description="Ingestion service internal gRPC port"
        )
    ]

    @computed_field  # type: ignore[prop-decorator]
    @property
    def grpc_url(self) -> str:
        """Computed gRPC URL"""
        return f"{self.host}:{self.port}"

    model_config = SettingsConfigDict(
        env_prefix="INGESTION_",
        env_file=".env",
        env_file_encoding="utf-8",
        extra="ignore",
    )


class AppSettings(BaseSettings):
    """Main application settings"""

    # Application
    app_name: Annotated[
        str,
        Field(
            default="Junjo Server",
            description="Application name"
        )
    ]
    debug: Annotated[
        bool,
        Field(
            default=False,
            description="Enable debug mode"
        )
    ]

    # Server
    host: Annotated[
        str,
        Field(
            default="0.0.0.0",
            description="Server host"
        )
    ]
    port: Annotated[
        int,
        Field(
            default=1324,
            ge=1,
            le=65535,
            description="Server port (1324 for dev, 1323 for production)"
        )
    ]

    # CORS
    cors_origins: Annotated[
        list[str],
        Field(
            default=["http://localhost:5151"],
            description="Allowed CORS origins"
        )
    ]

    # Nested settings
    database: Annotated[
        DatabaseSettings,
        Field(
            default_factory=DatabaseSettings,
            description="Database settings"
        )
    ]
    ingestion: Annotated[
        IngestionServiceSettings,
        Field(
            default_factory=IngestionServiceSettings,
            description="Ingestion service settings"
        )
    ]

    @field_validator("cors_origins", mode="before")
    @classmethod
    def parse_cors_origins(cls, v: str | list[str]) -> list[str]:
        """Parse CORS origins from string or list.

        Args:
            v: Either a JSON string, comma-separated string, or list of origin URLs.

        Returns:
            List of validated CORS origin URLs.
        """
        if isinstance(v, str):
            # Handle JSON array string from env var
            import json
            try:
                return json.loads(v)
            except json.JSONDecodeError:
                # Fallback to comma-separated
                return [origin.strip() for origin in v.split(",")]
        return v

    model_config = SettingsConfigDict(
        env_file=".env",
        env_file_encoding="utf-8",
        case_sensitive=False,
        extra="ignore",
    )


# Global settings instance (singleton, loaded once at import)
settings = AppSettings()
