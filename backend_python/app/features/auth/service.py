"""
Authentication service layer.

Contains business logic for authentication operations.
"""


from app.db_sqlite.users.repository import UserRepository
from app.db_sqlite.users.schemas import UserRead
from app.features.auth.utils import hash_password, verify_password


class AuthService:
    """Service for authentication operations."""

    @staticmethod
    async def db_has_users() -> bool:
        """
        Check if any users exist.

        Returns:
            True if users exist, False otherwise
        """
        return await UserRepository.db_has_users()

    @staticmethod
    async def create_first_user(email: str, password: str) -> UserRead:
        """
        Create the first user (only allowed if no users exist).

        Args:
            email: User email
            password: Plain text password

        Returns:
            Created user

        Raises:
            ValueError: If users already exist
            IntegrityError: If email already exists
        """
        # Check if any users exist
        if await AuthService.db_has_users():
            raise ValueError("Users already exist, cannot create first user")

        # Hash password and create user
        hashed_password = hash_password(password)
        return await UserRepository.create(email=email, password_hash=hashed_password)

    @staticmethod
    async def create_user(email: str, password: str) -> UserRead:
        """
        Create a new user.

        Args:
            email: User email
            password: Plain text password

        Returns:
            Created user

        Raises:
            IntegrityError: If email already exists
        """
        hashed_password = hash_password(password)
        return await UserRepository.create(email=email, password_hash=hashed_password)

    @staticmethod
    async def validate_credentials(email: str, password: str) -> UserRead | None:
        """
        Validate user credentials.

        Args:
            email: User email
            password: Plain text password

        Returns:
            User if credentials are valid, None otherwise
        """
        # Get user with password hash
        user_with_password = await UserRepository.get_by_email(email)
        if user_with_password is None:
            return None

        # Verify password
        if not verify_password(password, user_with_password.password_hash):
            return None

        # Return user without password hash
        user = await UserRepository.get_by_id(user_with_password.id)
        return user

    @staticmethod
    async def list_users() -> list[UserRead]:
        """
        List all users.

        Returns:
            List of all users
        """
        return await UserRepository.list_users()

    @staticmethod
    async def delete_user(user_id: str) -> bool:
        """
        Delete a user.

        Args:
            user_id: User ID to delete

        Returns:
            True if user was deleted, False if not found
        """
        return await UserRepository.delete_user(user_id)
