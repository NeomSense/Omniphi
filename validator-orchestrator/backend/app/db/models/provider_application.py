"""
Provider Application Model

Tracks third-party provider applications to join the marketplace.
Includes application data, review process, and verification status.

Table: provider_applications
"""

import uuid
from datetime import datetime
from typing import Optional, TYPE_CHECKING

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
from sqlalchemy.orm import relationship, Mapped

from app.db.database import Base
from app.db.models.enums import ApplicationStatus

if TYPE_CHECKING:
    from app.db.models.provider import Provider


class ProviderApplication(Base):
    """
    Third-party provider application for joining the marketplace.

    Tracks the full lifecycle of provider onboarding from initial
    application through verification and approval.
    """

    __tablename__ = "provider_applications"

    # Primary key
    id = Column(
        UUID(as_uuid=True),
        primary_key=True,
        default=uuid.uuid4,
        index=True
    )

    # Applicant company info
    company_name = Column(
        String(200),
        nullable=False,
        doc="Company/organization name"
    )
    company_website = Column(
        String(500),
        nullable=True,
        doc="Company website"
    )
    company_description = Column(
        Text,
        nullable=True,
        doc="Company description"
    )
    company_size = Column(
        String(50),
        nullable=True,
        doc="Company size (startup, small, medium, enterprise)"
    )
    company_founded = Column(
        Integer,
        nullable=True,
        doc="Year company was founded"
    )

    # Contact information
    contact_name = Column(
        String(100),
        nullable=False,
        doc="Primary contact name"
    )
    contact_email = Column(
        String(255),
        nullable=False,
        index=True,
        doc="Primary contact email"
    )
    contact_phone = Column(
        String(50),
        nullable=True,
        doc="Contact phone number"
    )
    contact_role = Column(
        String(100),
        nullable=True,
        doc="Contact's role in company"
    )

    # Provider details
    proposed_code = Column(
        String(50),
        nullable=False,
        doc="Proposed provider code"
    )
    proposed_name = Column(
        String(100),
        nullable=False,
        doc="Proposed display name"
    )
    description = Column(
        Text,
        nullable=False,
        doc="Provider description"
    )
    logo_url = Column(
        String(500),
        nullable=True,
        doc="Logo URL"
    )

    # Technical details
    api_endpoint = Column(
        String(500),
        nullable=False,
        doc="API endpoint URL"
    )
    api_documentation_url = Column(
        String(500),
        nullable=True,
        doc="API documentation URL"
    )
    api_auth_type = Column(
        String(50),
        nullable=True,
        doc="API authentication type"
    )

    # Infrastructure details
    supported_regions = Column(
        JSONB,
        nullable=False,
        default=list,
        doc="Proposed supported regions"
    )
    proposed_pricing = Column(
        JSONB,
        nullable=False,
        default=list,
        doc="Proposed pricing tiers"
    )
    infrastructure_type = Column(
        String(50),
        nullable=True,
        doc="Infrastructure type (cloud, bare-metal, hybrid)"
    )
    data_centers = Column(
        JSONB,
        nullable=False,
        default=list,
        doc="List of data center locations"
    )

    # SLA commitments
    uptime_guarantee = Column(
        Float,
        nullable=False,
        default=99.9,
        doc="Committed uptime SLA percentage"
    )
    response_time_sla_hours = Column(
        Integer,
        nullable=False,
        default=24,
        doc="Support response time SLA"
    )
    resolution_time_sla_hours = Column(
        Integer,
        nullable=False,
        default=72,
        doc="Issue resolution time SLA"
    )
    data_retention_days = Column(
        Integer,
        nullable=False,
        default=30,
        doc="Data retention period"
    )

    # Security & compliance
    security_certifications = Column(
        JSONB,
        nullable=False,
        default=list,
        doc="Security certifications (SOC2, ISO27001, etc.)"
    )
    compliance_frameworks = Column(
        JSONB,
        nullable=False,
        default=list,
        doc="Compliance frameworks"
    )
    security_questionnaire = Column(
        JSONB,
        nullable=True,
        doc="Security questionnaire responses"
    )

    # Experience
    years_in_operation = Column(
        Integer,
        nullable=True,
        doc="Years in operation"
    )
    existing_customers = Column(
        Integer,
        nullable=True,
        doc="Number of existing customers"
    )
    validators_hosted = Column(
        Integer,
        nullable=True,
        doc="Validators currently hosted (other chains)"
    )
    references = Column(
        JSONB,
        nullable=False,
        default=list,
        doc="Customer references"
    )

    # Application status
    status = Column(
        String(50),
        nullable=False,
        default=ApplicationStatus.PENDING.value,
        index=True,
        doc="Application status"
    )
    status_reason = Column(
        Text,
        nullable=True,
        doc="Reason for current status"
    )

    # Review tracking
    reviewer_notes = Column(
        Text,
        nullable=True,
        doc="Internal reviewer notes"
    )
    reviewed_by = Column(
        String(100),
        nullable=True,
        doc="Reviewer name/ID"
    )
    reviewed_at = Column(
        DateTime,
        nullable=True,
        doc="Review completion timestamp"
    )

    # Verification results
    verification_results = Column(
        JSONB,
        nullable=False,
        default=dict,
        doc="Verification check results"
    )
    verification_score = Column(
        Float,
        nullable=True,
        doc="Overall verification score"
    )

    # Created provider (if approved)
    provider_id = Column(
        UUID(as_uuid=True),
        ForeignKey("providers.id", ondelete="SET NULL"),
        nullable=True,
        doc="Created provider record"
    )

    # Metadata
    source = Column(
        String(50),
        nullable=False,
        default="web",
        doc="Application source"
    )
    ip_address = Column(
        String(45),
        nullable=True,
        doc="Applicant IP address"
    )
    extra_data = Column(
        JSONB,
        nullable=False,
        default=dict,
        doc="Additional metadata"
    )

    # Timestamps
    submitted_at = Column(
        DateTime,
        nullable=False,
        default=datetime.utcnow,
        doc="Submission timestamp"
    )
    approved_at = Column(
        DateTime,
        nullable=True,
        doc="Approval timestamp"
    )
    rejected_at = Column(
        DateTime,
        nullable=True,
        doc="Rejection timestamp"
    )
    created_at = Column(
        DateTime,
        nullable=False,
        default=datetime.utcnow
    )
    updated_at = Column(
        DateTime,
        nullable=False,
        default=datetime.utcnow,
        onupdate=datetime.utcnow
    )

    # Relationships
    provider: Mapped[Optional["Provider"]] = relationship(
        "Provider",
        foreign_keys=[provider_id]
    )

    # Indexes
    __table_args__ = (
        Index("ix_provider_applications_status", "status"),
        Index("ix_provider_applications_email", "contact_email"),
        Index("ix_provider_applications_submitted", "submitted_at"),
    )

    def __repr__(self) -> str:
        return f"<ProviderApplication {self.company_name} ({self.status})>"

    @property
    def is_pending(self) -> bool:
        """Check if application is pending review."""
        return self.status == ApplicationStatus.PENDING.value

    @property
    def is_under_review(self) -> bool:
        """Check if application is being reviewed."""
        return self.status == ApplicationStatus.UNDER_REVIEW.value

    @property
    def is_approved(self) -> bool:
        """Check if application is approved."""
        return self.status == ApplicationStatus.APPROVED.value

    @property
    def is_rejected(self) -> bool:
        """Check if application is rejected."""
        return self.status == ApplicationStatus.REJECTED.value

    @property
    def can_resubmit(self) -> bool:
        """Check if application can be resubmitted."""
        return self.status == ApplicationStatus.REJECTED.value

    def set_status(
        self,
        status: ApplicationStatus,
        reason: Optional[str] = None,
        reviewer: Optional[str] = None,
    ) -> None:
        """
        Update application status.

        Args:
            status: New status
            reason: Reason for status change
            reviewer: Reviewer name/ID
        """
        self.status = status.value
        self.status_reason = reason

        if reviewer:
            self.reviewed_by = reviewer
            self.reviewed_at = datetime.utcnow()

        if status == ApplicationStatus.APPROVED:
            self.approved_at = datetime.utcnow()
        elif status == ApplicationStatus.REJECTED:
            self.rejected_at = datetime.utcnow()

    def add_verification_result(
        self,
        check_type: str,
        passed: bool,
        message: Optional[str] = None,
        data: Optional[dict] = None,
    ) -> None:
        """
        Add a verification check result.

        Args:
            check_type: Type of verification check
            passed: Whether check passed
            message: Result message
            data: Additional result data
        """
        self.verification_results[check_type] = {
            "passed": passed,
            "message": message,
            "data": data,
            "checked_at": datetime.utcnow().isoformat(),
        }
