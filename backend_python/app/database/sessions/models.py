"""Session database model for cookie-based authentication."""

from datetime import datetime
from sqlalchemy import String, DateTime, ForeignKey, Text
from sqlalchemy.orm import Mapped, mapped_column
from sqlalchemy.sql import func

from app.database.base import Base
from app.common.utils import generate_id


class SessionTable(Base):
    """Session model for cookie-based authentication.

    Stores active user sessions for cookie-based authentication.
    """

    __tablename__ = "sessions"

    # Session ID (stored in cookie)
    id: Mapped[str] = mapped_column(
        String(22),
        primary_key=True,
        default=lambda: generate_id(size=22)
    )

    # User reference
    user_id: Mapped[str] = mapped_column(
        String(22),
        ForeignKey("users.id", ondelete="CASCADE"),
        nullable=False,
        index=True
    )

    # Session data (JSON string)
    data: Mapped[str | None] = mapped_column(
        Text,
        nullable=True
    )

    # Timestamps
    created_at: Mapped[datetime] = mapped_column(
        DateTime,
        server_default=func.now(),
        nullable=False
    )

    expires_at: Mapped[datetime] = mapped_column(
        DateTime,
        nullable=False,
        index=True  # Index for cleanup queries
    )
