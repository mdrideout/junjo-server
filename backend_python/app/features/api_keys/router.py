"""
API Keys router.

Implements CRUD endpoints for API key management.
"""

from fastapi import APIRouter, HTTPException, status
from loguru import logger
from sqlalchemy.exc import IntegrityError

from app.database.api_keys.schemas import APIKeyCreate, APIKeyRead
from app.features.api_keys.service import APIKeyService

router = APIRouter(prefix="/api_keys", tags=["api_keys"])


@router.post("", response_model=APIKeyRead, status_code=status.HTTP_201_CREATED)
async def create_api_key(request: APIKeyCreate):
    """
    Create a new API key.

    Generates a unique 64-character alphanumeric key.

    Args:
        request: APIKeyCreate with name

    Returns:
        Created API key with id, key, name, created_at

    Raises:
        HTTPException: 500 if creation fails
    """
    logger.info(f"Creating API key: {request.name}")

    try:
        api_key = await APIKeyService.create_api_key(name=request.name)
        logger.info(f"Created API key: {api_key.id}")
        return api_key
    except IntegrityError:
        # Extremely rare - key collision (nanoid is cryptographically random)
        logger.error("API key collision (should be impossible)")
        raise HTTPException(
            status_code=status.HTTP_500_INTERNAL_SERVER_ERROR,
            detail="Failed to generate unique API key",
        )
    except Exception as e:
        logger.error(f"Failed to create API key: {e}")
        raise HTTPException(
            status_code=status.HTTP_500_INTERNAL_SERVER_ERROR,
            detail="Failed to create API key",
        )


@router.get("", response_model=list[APIKeyRead])
async def list_api_keys():
    """
    List all API keys, ordered by created_at descending.

    Returns:
        List of all API keys (or empty list if none exist)

    Raises:
        HTTPException: 500 if listing fails
    """
    logger.info("Listing API keys")

    try:
        api_keys = await APIKeyService.list_api_keys()
        logger.info(f"Found {len(api_keys)} API keys")
        return api_keys
    except Exception as e:
        logger.error(f"Failed to list API keys: {e}")
        raise HTTPException(
            status_code=status.HTTP_500_INTERNAL_SERVER_ERROR,
            detail="Failed to retrieve API keys",
        )


@router.delete("/{id}", status_code=status.HTTP_204_NO_CONTENT)
async def delete_api_key(id: str):
    """
    Delete an API key by ID.

    Args:
        id: API key ID to delete

    Returns:
        204 No Content if successful

    Raises:
        HTTPException: 404 if key not found, 500 if deletion fails
    """
    logger.info(f"Deleting API key: {id}")

    try:
        deleted = await APIKeyService.delete_api_key(id)

        if not deleted:
            logger.warning(f"API key not found: {id}")
            raise HTTPException(
                status_code=status.HTTP_404_NOT_FOUND,
                detail="API key not found",
            )

        logger.info(f"Deleted API key: {id}")
        return None  # FastAPI returns 204 automatically

    except HTTPException:
        # Re-raise HTTPExceptions (like 404)
        raise
    except Exception as e:
        logger.error(f"Failed to delete API key: {e}")
        raise HTTPException(
            status_code=status.HTTP_500_INTERNAL_SERVER_ERROR,
            detail="Failed to delete API key",
        )
