"""Session Pydantic schemas."""

from datetime import datetime
from pydantic import BaseModel, ConfigDict


class SessionCreate(BaseModel):
    """Schema for creating a session.

    Attributes:
        user_id: User identifier this session belongs to
        expires_at: Timestamp when session expires
        data: Optional JSON data string
    """

    user_id: str
    expires_at: datetime
    data: str | None = None


class SessionRead(BaseModel):
    """Schema for reading a session.

    Attributes:
        id: Unique session identifier
        user_id: User identifier this session belongs to
        data: Optional JSON data string
        created_at: Timestamp when session was created
        expires_at: Timestamp when session expires
    """

    id: str
    user_id: str
    data: str | None
    created_at: datetime
    expires_at: datetime

    model_config = ConfigDict(from_attributes=True)
