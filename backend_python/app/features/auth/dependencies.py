"""
FastAPI dependencies for authentication.

Provides dependency injection for auth-protected endpoints.
"""

from typing import Annotated

from fastapi import Depends, HTTPException, Request, status


async def get_current_user_email(request: Request) -> str:
    """
    Dependency to get current user email from session.

    Session is automatically decrypted and validated by middleware stack:
    1. SecureCookiesMiddleware decrypts the cookie
    2. SessionMiddleware validates signature and populates request.session

    Mirrors the Go middleware auth check:
    - Reads userEmail from request.session
    - Returns user email if present
    - Raises 401 if session is invalid or missing

    Args:
        request: FastAPI request object (with session populated by middleware)

    Returns:
        User email from session

    Raises:
        HTTPException: 401 if session is invalid or missing

    Example:
        ```python
        @router.get("/protected")
        async def protected_endpoint(
            current_user_email: CurrentUserEmail
        ):
            return {"user": current_user_email}
        ```
    """
    user_email = request.session.get("userEmail")

    if user_email is None:
        raise HTTPException(
            status_code=status.HTTP_401_UNAUTHORIZED,
            detail="Unauthorized: No valid session",
        )

    return user_email


# Type alias for dependency injection
CurrentUserEmail = Annotated[str, Depends(get_current_user_email)]
