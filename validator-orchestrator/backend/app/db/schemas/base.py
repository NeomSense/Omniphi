"""
Base Pydantic Schemas

Common base schemas and utilities used across all domain schemas.
"""

from datetime import datetime
from typing import Any, Generic, List, Optional, TypeVar
from uuid import UUID

from pydantic import BaseModel, ConfigDict, Field


class BaseSchema(BaseModel):
    """
    Base schema with common configuration.

    All schemas should inherit from this to ensure consistent behavior.
    """

    model_config = ConfigDict(
        from_attributes=True,  # Enable ORM mode
        populate_by_name=True,  # Allow population by field name or alias
        str_strip_whitespace=True,  # Strip whitespace from strings
        validate_assignment=True,  # Validate on assignment
    )


class TimestampSchema(BaseSchema):
    """Schema mixin for timestamp fields."""

    created_at: datetime = Field(..., description="Creation timestamp")
    updated_at: datetime = Field(..., description="Last update timestamp")


class UUIDSchema(BaseSchema):
    """Schema mixin for UUID primary key."""

    id: UUID = Field(..., description="Unique identifier")


# Generic type for paginated responses
T = TypeVar("T")


class PaginatedResponse(BaseSchema, Generic[T]):
    """
    Generic paginated response schema.

    Usage:
        PaginatedResponse[RegionResponse]
    """

    items: List[T] = Field(..., description="List of items")
    total: int = Field(..., description="Total number of items", ge=0)
    page: int = Field(1, description="Current page number", ge=1)
    page_size: int = Field(20, description="Items per page", ge=1, le=100)
    pages: int = Field(..., description="Total number of pages", ge=0)
    has_next: bool = Field(..., description="Whether there are more pages")
    has_prev: bool = Field(..., description="Whether there are previous pages")

    @classmethod
    def create(
        cls,
        items: List[T],
        total: int,
        page: int = 1,
        page_size: int = 20,
    ) -> "PaginatedResponse[T]":
        """
        Create a paginated response.

        Args:
            items: List of items for current page
            total: Total number of items
            page: Current page number
            page_size: Items per page

        Returns:
            PaginatedResponse instance
        """
        pages = (total + page_size - 1) // page_size if total > 0 else 0
        return cls(
            items=items,
            total=total,
            page=page,
            page_size=page_size,
            pages=pages,
            has_next=page < pages,
            has_prev=page > 1,
        )


class SuccessResponse(BaseSchema):
    """Standard success response."""

    success: bool = Field(True, description="Operation success status")
    message: str = Field("Operation completed successfully", description="Success message")
    data: Optional[Any] = Field(None, description="Optional response data")


class ErrorResponse(BaseSchema):
    """Standard error response."""

    success: bool = Field(False, description="Operation success status")
    error: str = Field(..., description="Error message")
    error_code: Optional[str] = Field(None, description="Error code for programmatic handling")
    details: Optional[Any] = Field(None, description="Additional error details")


class BulkOperationResult(BaseSchema):
    """Result of a bulk operation."""

    total: int = Field(..., description="Total items processed")
    successful: int = Field(..., description="Successfully processed items")
    failed: int = Field(..., description="Failed items")
    errors: List[dict] = Field(default_factory=list, description="List of errors")


class StatusUpdate(BaseSchema):
    """Generic status update request."""

    status: str = Field(..., description="New status")
    reason: Optional[str] = Field(None, description="Reason for status change")


class FilterParams(BaseSchema):
    """Common filter parameters for list endpoints."""

    page: int = Field(1, ge=1, description="Page number")
    page_size: int = Field(20, ge=1, le=100, description="Items per page")
    sort_by: Optional[str] = Field(None, description="Field to sort by")
    sort_order: str = Field("desc", description="Sort order (asc/desc)")
    search: Optional[str] = Field(None, description="Search query")


class DateRangeFilter(BaseSchema):
    """Date range filter parameters."""

    start_date: Optional[datetime] = Field(None, description="Start date")
    end_date: Optional[datetime] = Field(None, description="End date")


class HealthCheckResponse(BaseSchema):
    """Health check response."""

    status: str = Field(..., description="Health status")
    version: str = Field(..., description="Application version")
    database: bool = Field(..., description="Database connection status")
    timestamp: datetime = Field(default_factory=datetime.utcnow)
    details: Optional[dict] = Field(None, description="Additional health details")
