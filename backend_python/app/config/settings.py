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


class SessionCookieSettings(BaseSettings):
    """Session cookie and authentication security settings"""

    secure_cookie_key: Annotated[
        str,
        Field(
            description="Base64-encoded 32-byte encryption key for SecureCookiesMiddleware (Fernet/AES-256). "
                        "Generate with: openssl rand -base64 32"
        )
    ]
    session_secret: Annotated[
        str,
        Field(
            description="Signing secret for SessionMiddleware (any length). "
                        "Generate with: openssl rand -base64 32"
        )
    ]
    junjo_env: Annotated[
        str,
        Field(
            default="development",
            description="Environment (development/production)"
        )
    ]
    junjo_prod_auth_domain: Annotated[
        str,
        Field(
            default="",
            description="Production auth domain for subdomain cookie support (e.g., 'junjo.io')"
        )
    ]

    @field_validator("secure_cookie_key", mode="before")
    @classmethod
    def validate_secure_cookie_key(cls, v: str) -> str:
        """
        Validate secure cookie key is a valid base64 string for Fernet.

        Fernet (used by SecureCookiesMiddleware) requires a base64url-encoded 32-byte key.
        The `openssl rand -base64 32` command generates 32 random bytes and base64-encodes them.

        Args:
            v: Base64-encoded string (from `openssl rand -base64 32`)

        Returns:
            The same base64 string (Fernet will decode it internally)

        Raises:
            ValueError: If key is not valid base64 or not 32 bytes when decoded
        """
        if not isinstance(v, str):
            raise TypeError(f"Expected str, got {type(v)}")

        # Validate it's valid base64 and decodes to 32 bytes
        import base64
        try:
            # Try standard base64 first (openssl rand -base64 uses standard, not urlsafe)
            decoded = base64.b64decode(v)
        except Exception as e:
            raise ValueError(
                f"JUNJO_SECURE_COOKIE_KEY must be a valid base64 string. "
                f"Generate with: openssl rand -base64 32"
            ) from e

        # Validate exactly 32 bytes for AES-256
        if len(decoded) != 32:
            raise ValueError(
                f"JUNJO_SECURE_COOKIE_KEY must decode to exactly 32 bytes for AES-256 encryption. "
                f"Got {len(decoded)} bytes. Generate with: openssl rand -base64 32"
            )

        return v

    model_config = SettingsConfigDict(
        env_prefix="JUNJO_",
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
    session_cookie: Annotated[
        SessionCookieSettings,
        Field(
            default_factory=SessionCookieSettings,
            description="Session cookie and authentication settings"
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
