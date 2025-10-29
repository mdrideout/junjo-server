"""
Authentication router.

Implements all authentication endpoints, mirroring the Go implementation.
"""

from typing import Annotated

from fastapi import APIRouter, HTTPException, Request, status
from loguru import logger
from sqlalchemy.exc import IntegrityError

from app.db_sqlite.users.schemas import (
    AuthTestResponse,
    CreateUserRequest,
    DbHasUsersResponse,
    SignInRequest,
    UserRead,
    UserResponse,
)
from app.features.auth.dependencies import CurrentUserEmail
from app.features.auth.service import AuthService

router = APIRouter()

# Note: Session configuration (max_age, secure, samesite, etc.) is handled
# in main.py when adding SessionMiddleware. No need to configure here.


# --- Public Endpoints (no auth required) ---


@router.get("/users/db-has-users", response_model=DbHasUsersResponse)
async def db_has_users():
    """
    Check if any users exist.

    Returns:
        DbHasUsersResponse with users_exist boolean
    """
    exists = await AuthService.db_has_users()
    return DbHasUsersResponse(users_exist=exists)


@router.post("/users/create-first-user", response_model=UserResponse)
async def create_first_user(request: CreateUserRequest):
    """
    Create the first user (only allowed if no users exist).

    Mirrors Go implementation: only works if database has no users.

    Args:
        request: CreateUserRequest with email and password

    Returns:
        UserResponse with success message

    Raises:
        HTTPException: 400 if users already exist, 409 if email exists
    """
    try:
        await AuthService.create_first_user(request.email, request.password)
        return UserResponse(message="First user created successfully")
    except ValueError as e:
        raise HTTPException(
            status_code=status.HTTP_400_BAD_REQUEST,
            detail=str(e),
        )
    except IntegrityError:
        raise HTTPException(
            status_code=status.HTTP_409_CONFLICT,
            detail="A user with this email already exists",
        )


@router.post("/sign-in", response_model=UserResponse)
async def sign_in(sign_in_request: SignInRequest, request: Request):
    """
    Sign in with email and password.

    Sets userEmail in request.session, which is automatically:
    1. Signed by SessionMiddleware (integrity)
    2. Encrypted by SecureCookiesMiddleware (confidentiality)

    Mirrors Go implementation in /backend/auth/services.go:155-214.

    Args:
        sign_in_request: SignInRequest with email and password
        request: FastAPI request object

    Returns:
        UserResponse with success message

    Raises:
        HTTPException: 401 if credentials are invalid
    """
    logger.info(f"Sign-in request for email: {sign_in_request.email}")

    # Validate credentials
    user = await AuthService.validate_credentials(
        sign_in_request.email, sign_in_request.password
    )
    if user is None:
        logger.warning(f"Failed to validate credentials for: {sign_in_request.email}")
        raise HTTPException(
            status_code=status.HTTP_401_UNAUTHORIZED,
            detail="Invalid credentials",
        )

    # Set user email in session (middleware handles encryption + signing)
    request.session["userEmail"] = user.email

    logger.info(f"User signed in successfully: {user.email}")
    return UserResponse(message="signed in")


@router.post("/sign-out", response_model=UserResponse)
async def sign_out(request: Request):
    """
    Sign out by clearing the session.

    SessionMiddleware automatically deletes the cookie when session is cleared.
    Mirrors Go implementation.

    Args:
        request: FastAPI request object

    Returns:
        UserResponse with success message
    """
    # Clear session (middleware handles cookie deletion)
    request.session.clear()

    logger.info("User signed out")
    return UserResponse(message="signed out")


@router.get("/auth-test", response_model=AuthTestResponse)
async def auth_test(current_user_email: CurrentUserEmail):
    """
    Test authentication by returning the current user email.

    Protected by auth dependency - will return 401 if not authenticated.

    Args:
        current_user_email: Injected by CurrentUserEmail dependency

    Returns:
        AuthTestResponse with user email
    """
    return AuthTestResponse(user_email=current_user_email)


# --- Protected Endpoints (auth required via dependency) ---


@router.post("/users", response_model=UserResponse)
async def create_user(
    request: CreateUserRequest, current_user_email: CurrentUserEmail
):
    """
    Create a new user (auth required).

    Uses CurrentUserEmail dependency for auth check.

    Args:
        request: CreateUserRequest with email and password
        current_user_email: Injected by CurrentUserEmail dependency

    Returns:
        UserResponse with success message

    Raises:
        HTTPException: 409 if email already exists
    """
    try:
        await AuthService.create_user(request.email, request.password)
        logger.info(
            f"User created by {current_user_email}: {request.email}"
        )
        return UserResponse(message="User created successfully")
    except IntegrityError:
        raise HTTPException(
            status_code=status.HTTP_409_CONFLICT,
            detail="A user with this email already exists",
        )


@router.get("/users", response_model=list[UserRead])
async def list_users(current_user_email: CurrentUserEmail):
    """
    List all users (auth required).

    Uses CurrentUserEmail dependency for auth check.

    Args:
        current_user_email: Injected by CurrentUserEmail dependency

    Returns:
        List of UserRead objects
    """
    users = await AuthService.list_users()
    return users


@router.delete("/users/{user_id}", response_model=UserResponse)
async def delete_user(user_id: str, current_user_email: CurrentUserEmail):
    """
    Delete a user (auth required).

    Uses CurrentUserEmail dependency for auth check.

    Args:
        user_id: User ID to delete
        current_user_email: Injected by CurrentUserEmail dependency

    Returns:
        UserResponse with success message

    Raises:
        HTTPException: 404 if user not found
    """
    deleted = await AuthService.delete_user(user_id)
    if not deleted:
        raise HTTPException(
            status_code=status.HTTP_404_NOT_FOUND,
            detail="User not found",
        )
    logger.info(f"User deleted by {current_user_email}: {user_id}")
    return UserResponse(message="User deleted successfully")
