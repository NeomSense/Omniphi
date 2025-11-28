"""
Base Model Mixin for Omniphi Cloud Database

Provides common functionality for all database models including:
- UUID primary keys
- Created/updated timestamps
- Soft delete support
- Common utility methods
"""

import uuid
from datetime import datetime
from typing import Any, Dict, Optional

from sqlalchemy import Column, DateTime, Boolean, event
from sqlalchemy.dialects.postgresql import UUID
from sqlalchemy.orm import declared_attr

from app.db.database import Base


class TimestampMixin:
    """Mixin for created_at and updated_at timestamps."""

    created_at = Column(
        DateTime,
        nullable=False,
        default=datetime.utcnow,
        doc="Record creation timestamp"
    )
    updated_at = Column(
        DateTime,
        nullable=False,
        default=datetime.utcnow,
        onupdate=datetime.utcnow,
        doc="Last update timestamp"
    )


class SoftDeleteMixin:
    """Mixin for soft delete functionality."""

    is_deleted = Column(
        Boolean,
        nullable=False,
        default=False,
        index=True,
        doc="Soft delete flag"
    )
    deleted_at = Column(
        DateTime,
        nullable=True,
        doc="Deletion timestamp"
    )

    def soft_delete(self) -> None:
        """Mark record as deleted without removing from database."""
        self.is_deleted = True
        self.deleted_at = datetime.utcnow()


class UUIDPrimaryKeyMixin:
    """Mixin for UUID primary key."""

    id = Column(
        UUID(as_uuid=True),
        primary_key=True,
        default=uuid.uuid4,
        index=True,
        doc="Unique identifier"
    )


class BaseModel(Base, UUIDPrimaryKeyMixin, TimestampMixin):
    """
    Abstract base class for all Omniphi database models.

    Provides:
    - UUID primary key
    - created_at and updated_at timestamps
    - Common utility methods

    Usage:
        class MyModel(BaseModel):
            __tablename__ = "my_table"
            name = Column(String, nullable=False)
    """

    __abstract__ = True

    def to_dict(self, exclude: Optional[set] = None) -> Dict[str, Any]:
        """
        Convert model instance to dictionary.

        Args:
            exclude: Set of field names to exclude

        Returns:
            Dictionary representation of the model
        """
        exclude = exclude or set()
        result = {}

        for column in self.__table__.columns:
            if column.name not in exclude:
                value = getattr(self, column.name)
                # Handle UUID serialization
                if isinstance(value, uuid.UUID):
                    value = str(value)
                # Handle datetime serialization
                elif isinstance(value, datetime):
                    value = value.isoformat()
                result[column.name] = value

        return result

    def update_from_dict(self, data: Dict[str, Any], exclude: Optional[set] = None) -> None:
        """
        Update model fields from dictionary.

        Args:
            data: Dictionary with field values
            exclude: Set of field names to not update
        """
        exclude = exclude or {"id", "created_at"}

        for key, value in data.items():
            if key not in exclude and hasattr(self, key):
                setattr(self, key, value)

    @classmethod
    def get_table_name(cls) -> str:
        """Get the table name for this model."""
        return cls.__tablename__

    def __repr__(self) -> str:
        """String representation of the model."""
        return f"<{self.__class__.__name__}(id={self.id})>"


class AuditableModel(BaseModel, SoftDeleteMixin):
    """
    Base class for models that need audit trail and soft delete.

    Provides all BaseModel features plus:
    - Soft delete functionality
    - Audit tracking fields
    """

    __abstract__ = True

    @declared_attr
    def created_by(cls):
        """User who created the record."""
        return Column(
            UUID(as_uuid=True),
            nullable=True,
            doc="User ID who created this record"
        )

    @declared_attr
    def updated_by(cls):
        """User who last updated the record."""
        return Column(
            UUID(as_uuid=True),
            nullable=True,
            doc="User ID who last updated this record"
        )


# Export all base classes
__all__ = [
    "Base",
    "BaseModel",
    "AuditableModel",
    "TimestampMixin",
    "SoftDeleteMixin",
    "UUIDPrimaryKeyMixin",
]
