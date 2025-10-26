"""Session repository using high-concurrency pattern."""

from datetime import datetime
from sqlalchemy import select, delete
from sqlalchemy.exc import SQLAlchemyError

from app.database.db_config import async_session
from app.database.sessions.models import SessionTable
from app.database.sessions.schemas import SessionRead


class SessionRepository:
    """Repository for session database operations.

    All methods are static to avoid instance state and ensure thread-safety.
    Each method creates its own database session for complete isolation.
    """

    @staticmethod
    async def create(user_id: str, expires_at: datetime, data: str | None = None) -> SessionRead:
        """Create a new session.

        Args:
            user_id: User identifier this session belongs to
            expires_at: Timestamp when session expires
            data: Optional JSON data string

        Returns:
            Created session

        Raises:
            SQLAlchemyError: If database operation fails
        """
        try:
            db_obj = SessionTable(
                user_id=user_id,
                expires_at=expires_at,
                data=data
            )

            async with async_session() as session:
                session.add(db_obj)
                await session.commit()
                await session.refresh(db_obj)
                return SessionRead.model_validate(db_obj)

        except SQLAlchemyError as e:
            raise e

    @staticmethod
    async def get_by_id(session_id: str) -> SessionRead | None:
        """Get session by ID.

        Args:
            session_id: Unique session identifier

        Returns:
            Session if found, None otherwise

        Raises:
            SQLAlchemyError: If database operation fails
        """
        try:
            async with async_session() as session:
                stmt = select(SessionTable).where(SessionTable.id == session_id)
                result = await session.execute(stmt)
                db_obj = result.scalar_one_or_none()

                if db_obj:
                    return SessionRead.model_validate(db_obj)
                return None

        except SQLAlchemyError as e:
            raise e

    @staticmethod
    async def delete_by_id(session_id: str) -> bool:
        """Delete session by ID.

        Args:
            session_id: Unique session identifier

        Returns:
            True if session was deleted, False if not found

        Raises:
            SQLAlchemyError: If database operation fails
        """
        try:
            async with async_session() as session:
                stmt = delete(SessionTable).where(SessionTable.id == session_id)
                result = await session.execute(stmt)
                await session.commit()
                return result.rowcount > 0

        except SQLAlchemyError as e:
            raise e

    @staticmethod
    async def delete_expired() -> int:
        """Delete all expired sessions (cleanup task).

        Returns:
            Number of sessions deleted

        Raises:
            SQLAlchemyError: If database operation fails
        """
        try:
            async with async_session() as session:
                stmt = delete(SessionTable).where(
                    SessionTable.expires_at < datetime.utcnow()
                )
                result = await session.execute(stmt)
                await session.commit()
                return result.rowcount

        except SQLAlchemyError as e:
            raise e
