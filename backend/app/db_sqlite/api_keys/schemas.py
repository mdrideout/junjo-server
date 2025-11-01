"""API Key Pydantic schemas for validation and serialization."""

from datetime import datetime

from pydantic import BaseModel, ConfigDict, Field


class APIKeyCreate(BaseModel):
    """Schema for creating an API key.

    Attributes:
        name: Human-readable name for the key
    """

    name: str = Field(..., min_length=1)


class APIKeyRead(BaseModel):
    """Schema for reading an API key.

    Attributes:
        id: Unique identifier
        key: API key value (64-char alphanumeric)
        name: Human-readable name
        created_at: Timestamp when key was created
    """

    id: str
    key: str
    name: str
    created_at: datetime

    model_config = ConfigDict(from_attributes=True)
