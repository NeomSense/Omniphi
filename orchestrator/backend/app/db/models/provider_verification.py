"""
Provider Verification Model

Tracks verification tests run against provider APIs during onboarding.
Each verification is a specific check of provider capabilities.

Table: provider_verifications
"""

import uuid
from datetime import datetime
from typing import Optional

from sqlalchemy import (
    Column,
    String,
    Integer,
    Float,
    Boolean,
    DateTime,
    ForeignKey,
    Text,
    Index,
)
from sqlalchemy.dialects.postgresql import UUID, JSONB

from app.db.database import Base
from app.db.models.enums import VerificationCheckType


class ProviderVerification(Base):
    """
    Provider verification check record.

    Tracks individual verification tests run against provider APIs
    during the application review process.
    """

    __tablename__ = "provider_verifications"

    # Primary key
    id = Column(
        UUID(as_uuid=True),
        primary_key=True,
        default=uuid.uuid4,
        index=True
    )

    # Foreign key
    application_id = Column(
        UUID(as_uuid=True),
        ForeignKey("provider_applications.id", ondelete="CASCADE"),
        nullable=False,
        index=True,
        doc="Parent application"
    )

    # Check identification
    check_type = Column(
        String(50),
        nullable=False,
        doc="Type of verification check"
    )
    check_name = Column(
        String(100),
        nullable=False,
        doc="Human-readable check name"
    )
    check_description = Column(
        Text,
        nullable=True,
        doc="Check description"
    )

    # Check parameters
    check_endpoint = Column(
        String(500),
        nullable=True,
        doc="Endpoint tested"
    )
    check_method = Column(
        String(10),
        nullable=True,
        doc="HTTP method used"
    )
    check_payload = Column(
        JSONB,
        nullable=True,
        doc="Request payload"
    )

    # Results
    passed = Column(
        Boolean,
        nullable=False,
        doc="Whether check passed"
    )
    result_code = Column(
        String(50),
        nullable=True,
        doc="Result code"
    )
    result_message = Column(
        Text,
        nullable=True,
        doc="Result message"
    )
    result_data = Column(
        JSONB,
        nullable=True,
        doc="Detailed result data"
    )

    # Performance
    duration_ms = Column(
        Float,
        nullable=True,
        doc="Check duration in milliseconds"
    )
    response_time_ms = Column(
        Float,
        nullable=True,
        doc="API response time"
    )

    # Error details (if failed)
    error_type = Column(
        String(100),
        nullable=True,
        doc="Error type if failed"
    )
    error_message = Column(
        Text,
        nullable=True,
        doc="Error message if failed"
    )
    error_stack = Column(
        Text,
        nullable=True,
        doc="Error stack trace"
    )

    # Retry info
    attempt_number = Column(
        Integer,
        nullable=False,
        default=1,
        doc="Attempt number"
    )
    max_attempts = Column(
        Integer,
        nullable=False,
        default=3,
        doc="Maximum attempts"
    )
    retried_at = Column(
        DateTime,
        nullable=True,
        doc="Last retry timestamp"
    )

    # Metadata
    executed_by = Column(
        String(100),
        nullable=True,
        doc="Who/what executed the check"
    )
    environment = Column(
        String(50),
        nullable=False,
        default="production",
        doc="Test environment"
    )

    # Timestamps
    executed_at = Column(
        DateTime,
        nullable=False,
        default=datetime.utcnow,
        index=True,
        doc="Execution timestamp"
    )
    created_at = Column(
        DateTime,
        nullable=False,
        default=datetime.utcnow
    )

    # Indexes
    __table_args__ = (
        Index("ix_provider_verifications_application", "application_id"),
        Index("ix_provider_verifications_type", "check_type", "passed"),
        Index("ix_provider_verifications_executed", "executed_at"),
    )

    def __repr__(self) -> str:
        result = "PASS" if self.passed else "FAIL"
        return f"<ProviderVerification {self.check_type}: {result}>"

    @property
    def can_retry(self) -> bool:
        """Check if verification can be retried."""
        return not self.passed and self.attempt_number < self.max_attempts

    @property
    def is_timeout(self) -> bool:
        """Check if failure was due to timeout."""
        return self.error_type == "timeout"

    @property
    def is_connection_error(self) -> bool:
        """Check if failure was connection error."""
        return self.error_type in ["connection_error", "dns_error", "ssl_error"]

    def record_result(
        self,
        passed: bool,
        message: Optional[str] = None,
        data: Optional[dict] = None,
        duration_ms: Optional[float] = None,
        error_type: Optional[str] = None,
        error_message: Optional[str] = None,
    ) -> None:
        """
        Record verification result.

        Args:
            passed: Whether check passed
            message: Result message
            data: Result data
            duration_ms: Check duration
            error_type: Error type if failed
            error_message: Error message if failed
        """
        self.passed = passed
        self.result_message = message
        self.result_data = data
        self.duration_ms = duration_ms
        self.error_type = error_type
        self.error_message = error_message
        self.executed_at = datetime.utcnow()

    def record_retry(self) -> bool:
        """
        Record a retry attempt.

        Returns:
            True if retry is allowed
        """
        if not self.can_retry:
            return False

        self.attempt_number += 1
        self.retried_at = datetime.utcnow()
        return True


# Import Integer that was missed
from sqlalchemy import Integer  # noqa: E402
