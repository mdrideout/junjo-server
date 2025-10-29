"""API Key service layer with key generation.

Handles business logic for API key management including secure key generation.
"""

import nanoid

from app.db_sqlite.api_keys.repository import APIKeyRepository
from app.db_sqlite.api_keys.schemas import APIKeyRead


class APIKeyService:
    """Service for API key business logic.

    Static methods to avoid instance state and ensure thread-safety.
    """

    @staticmethod
    def generate_key() -> str:
        """Generate a secure 64-character alphanumeric API key.

        Uses nanoid with alphanumeric alphabet (matches Go implementation).

        Returns:
            64-character alphanumeric string
        """
        alphabet = "0123456789abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ"
        return nanoid.generate(alphabet, 64)

    @staticmethod
    def generate_id() -> str:
        """Generate a unique identifier for the API key.

        Uses nanoid with default length (21 characters).

        Returns:
            21-character nanoid
        """
        return nanoid.generate()

    @staticmethod
    async def create_api_key(name: str) -> APIKeyRead:
        """Create a new API key.

        Args:
            name: Human-readable name for the key

        Returns:
            Created API key with generated ID and key value

        Raises:
            SQLAlchemyError: If database operation fails
        """
        # Generate ID and key
        key_id = APIKeyService.generate_id()
        key_value = APIKeyService.generate_key()

        # Save to database
        return await APIKeyRepository.create(
            id=key_id,
            key=key_value,
            name=name
        )

    @staticmethod
    async def list_api_keys() -> list[APIKeyRead]:
        """List all API keys.

        Returns:
            List of all API keys, ordered by created_at descending

        Raises:
            SQLAlchemyError: If database operation fails
        """
        return await APIKeyRepository.list_all()

    @staticmethod
    async def delete_api_key(id: str) -> bool:
        """Delete an API key by ID.

        Args:
            id: API key ID to delete

        Returns:
            True if key was deleted, False if not found

        Raises:
            SQLAlchemyError: If database operation fails
        """
        return await APIKeyRepository.delete_by_id(id)
