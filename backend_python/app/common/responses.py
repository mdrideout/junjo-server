"""
Standard API response models.

Provides consistent response formats across all endpoints.
Uses Pydantic v2+ conventions.
"""

from typing import Generic, TypeVar
from pydantic import BaseModel, Field, ConfigDict

T = TypeVar("T")


class HealthResponse(BaseModel):
    """Health check response"""
    status: str = Field(default="ok", description="Health status")
    version: str = Field(default="0.1.0", description="API version")
    app_name: str = Field(description="Application name")

    model_config = ConfigDict(frozen=True)  # Immutable response


class ErrorResponse(BaseModel):
    """Error response"""
    error: str = Field(description="Error message")
    detail: str | None = Field(default=None, description="Detailed error information")


class SuccessResponse(BaseModel, Generic[T]):
    """Generic success response with data"""
    success: bool = Field(default=True)
    data: T = Field(description="Response data")
