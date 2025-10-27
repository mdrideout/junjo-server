"""User repository using high-concurrency pattern.

Each method creates its own session for complete isolation.
See: PYTHON_BACKEND_HIGH_CONCURRENCY_DB_PATTERN.md
"""

from sqlalchemy import delete, func, select
from sqlalchemy.exc import SQLAlchemyError

from app.database import db_config
from app.database.users.models import UserTable
from app.database.users.schemas import UserRead, UserInDB


class UserRepository:
    """Repository for user database operations.

    All methods are static to avoid instance state and ensure thread-safety.
    Each method creates its own database session for complete isolation.
    """

    @staticmethod
    async def create(email: str, password_hash: str) -> UserRead:
        """Create a new user.

        Args:
            email: User email address
            password_hash: Hashed password (bcrypt)

        Returns:
            Created user (without password hash)

        Raises:
            SQLAlchemyError: If database operation fails

        Pattern:
            1. Create session
            2. Add object
            3. Commit
            4. Refresh (load generated fields)
            5. Validate to Pydantic BEFORE session closes
        """
        try:
            db_obj = UserTable(
                email=email,
                password_hash=password_hash
            )

            async with db_config.async_session() as session:
                session.add(db_obj)
                await session.commit()
                await session.refresh(db_obj)

                # Validate to Pydantic before session closes
                return UserRead.model_validate(db_obj)

        except SQLAlchemyError as e:
            raise e

    @staticmethod
    async def get_by_email(email: str) -> UserInDB | None:
        """Get user by email (including password hash for authentication).

        Args:
            email: User email address

        Returns:
            User with password hash if found, None otherwise

        Raises:
            SQLAlchemyError: If database operation fails
        """
        try:
            async with db_config.async_session() as session:
                stmt = select(UserTable).where(UserTable.email == email)
                result = await session.execute(stmt)
                db_obj = result.scalar_one_or_none()

                if db_obj:
                    return UserInDB.model_validate(db_obj)
                return None

        except SQLAlchemyError as e:
            raise e

    @staticmethod
    async def get_by_id(user_id: str) -> UserRead | None:
        """Get user by ID (no password hash).

        Args:
            user_id: Unique user identifier

        Returns:
            User if found, None otherwise

        Raises:
            SQLAlchemyError: If database operation fails
        """
        try:
            async with db_config.async_session() as session:
                stmt = select(UserTable).where(UserTable.id == user_id)
                result = await session.execute(stmt)
                db_obj = result.scalar_one_or_none()

                if db_obj:
                    return UserRead.model_validate(db_obj)
                return None

        except SQLAlchemyError as e:
            raise e

    @staticmethod
    async def count_users() -> int:
        """
        Count total number of users.

        Returns:
            Total number of users in database

        Raises:
            SQLAlchemyError: If database operation fails
        """
        try:
            async with db_config.async_session() as session:
                stmt = select(func.count()).select_from(UserTable)
                result = await session.execute(stmt)
                count = result.scalar_one()
                return count

        except SQLAlchemyError as e:
            raise e

    @staticmethod
    async def db_has_users() -> bool:
        """
        Check if any users exist.

        Returns:
            True if users exist, False otherwise

        Raises:
            SQLAlchemyError: If database operation fails
        """
        count = await UserRepository.count_users()
        return count > 0

    @staticmethod
    async def list_users() -> list[UserRead]:
        """
        List all users.

        Returns:
            List of all users (without password_hash)

        Raises:
            SQLAlchemyError: If database operation fails
        """
        try:
            async with db_config.async_session() as session:
                stmt = select(UserTable)
                result = await session.execute(stmt)
                db_objs = result.scalars().all()

                # Validate with Pydantic before session closes
                return [UserRead.model_validate(obj) for obj in db_objs]

        except SQLAlchemyError as e:
            raise e

    @staticmethod
    async def delete_user(user_id: str) -> bool:
        """
        Delete a user by ID.

        Args:
            user_id: User ID to delete

        Returns:
            True if user was deleted, False if user not found

        Raises:
            SQLAlchemyError: If database operation fails
        """
        try:
            async with db_config.async_session() as session:
                stmt = delete(UserTable).where(UserTable.id == user_id)
                result = await session.execute(stmt)
                await session.commit()

                # Check if any rows were deleted
                return result.rowcount > 0

        except SQLAlchemyError as e:
            raise e
