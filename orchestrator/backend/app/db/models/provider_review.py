"""
Provider Review Model

User ratings and reviews for providers in the marketplace.
Supports verified reviews and detailed usage context.

Table: provider_reviews
"""

import uuid
from datetime import datetime
from typing import TYPE_CHECKING

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

if TYPE_CHECKING:
    from app.db.models.provider import Provider


class ProviderReview(Base):
    """
    Provider review from users.

    User ratings and reviews for providers with usage context
    and verification status.
    """

    __tablename__ = "provider_reviews"

    # Primary key
    id = Column(
        UUID(as_uuid=True),
        primary_key=True,
        default=uuid.uuid4,
        index=True
    )

    # Foreign key
    provider_id = Column(
        UUID(as_uuid=True),
        ForeignKey("providers.id", ondelete="CASCADE"),
        nullable=False,
        index=True,
        doc="Reviewed provider"
    )

    # Reviewer info
    user_id = Column(
        String(100),
        nullable=False,
        index=True,
        doc="User identifier"
    )
    wallet_address = Column(
        String(100),
        nullable=True,
        doc="Reviewer's wallet address"
    )
    reviewer_name = Column(
        String(100),
        nullable=True,
        doc="Reviewer display name"
    )

    # Rating (1-5 stars)
    rating = Column(
        Float,
        nullable=False,
        doc="Overall rating (1-5)"
    )

    # Detailed ratings (optional)
    rating_reliability = Column(
        Float,
        nullable=True,
        doc="Reliability rating (1-5)"
    )
    rating_performance = Column(
        Float,
        nullable=True,
        doc="Performance rating (1-5)"
    )
    rating_support = Column(
        Float,
        nullable=True,
        doc="Support rating (1-5)"
    )
    rating_value = Column(
        Float,
        nullable=True,
        doc="Value for money rating (1-5)"
    )

    # Review content
    title = Column(
        String(200),
        nullable=True,
        doc="Review title"
    )
    comment = Column(
        Text,
        nullable=True,
        doc="Review text"
    )
    pros = Column(
        JSONB,
        nullable=False,
        default=list,
        doc="List of pros"
    )
    cons = Column(
        JSONB,
        nullable=False,
        default=list,
        doc="List of cons"
    )

    # Usage context
    validators_hosted = Column(
        Integer,
        nullable=True,
        doc="Number of validators hosted"
    )
    months_used = Column(
        Integer,
        nullable=True,
        doc="Months using provider"
    )
    region_used = Column(
        String(50),
        nullable=True,
        doc="Primary region used"
    )
    plan_used = Column(
        String(100),
        nullable=True,
        doc="Plan/tier used"
    )
    use_case = Column(
        String(100),
        nullable=True,
        doc="Primary use case"
    )

    # Verification status
    is_verified = Column(
        Boolean,
        nullable=False,
        default=False,
        index=True,
        doc="Whether reviewer is verified customer"
    )
    verified_at = Column(
        DateTime,
        nullable=True,
        doc="Verification timestamp"
    )
    verification_method = Column(
        String(50),
        nullable=True,
        doc="How review was verified"
    )

    # Visibility and moderation
    is_visible = Column(
        Boolean,
        nullable=False,
        default=True,
        index=True,
        doc="Whether review is publicly visible"
    )
    is_featured = Column(
        Boolean,
        nullable=False,
        default=False,
        doc="Whether review is featured"
    )
    moderation_status = Column(
        String(50),
        nullable=False,
        default="approved",
        doc="Moderation status"
    )
    moderation_notes = Column(
        Text,
        nullable=True,
        doc="Moderation notes"
    )
    moderated_by = Column(
        String(100),
        nullable=True,
        doc="Moderator ID"
    )
    moderated_at = Column(
        DateTime,
        nullable=True,
        doc="Moderation timestamp"
    )

    # Provider response
    provider_response = Column(
        Text,
        nullable=True,
        doc="Provider's response to review"
    )
    provider_responded_at = Column(
        DateTime,
        nullable=True,
        doc="Response timestamp"
    )

    # Helpfulness voting
    helpful_count = Column(
        Integer,
        nullable=False,
        default=0,
        doc="Number of 'helpful' votes"
    )
    not_helpful_count = Column(
        Integer,
        nullable=False,
        default=0,
        doc="Number of 'not helpful' votes"
    )

    # Metadata
    source = Column(
        String(50),
        nullable=False,
        default="web",
        doc="Review source"
    )
    ip_address = Column(
        String(45),
        nullable=True,
        doc="Reviewer IP"
    )
    user_agent = Column(
        String(500),
        nullable=True,
        doc="Reviewer user agent"
    )

    # Timestamps
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
    provider: Mapped["Provider"] = relationship(
        "Provider",
        back_populates="reviews"
    )

    # Indexes
    __table_args__ = (
        Index("ix_provider_reviews_provider_rating", "provider_id", "rating"),
        Index("ix_provider_reviews_provider_visible", "provider_id", "is_visible"),
        Index("ix_provider_reviews_user", "user_id"),
        Index("ix_provider_reviews_verified", "is_verified", "is_visible"),
    )

    def __repr__(self) -> str:
        return f"<ProviderReview {self.provider_id} ({self.rating}/5)>"

    @property
    def has_detailed_ratings(self) -> bool:
        """Check if review has detailed ratings."""
        return any([
            self.rating_reliability,
            self.rating_performance,
            self.rating_support,
            self.rating_value,
        ])

    @property
    def average_detailed_rating(self) -> float:
        """Calculate average of detailed ratings."""
        ratings = [
            r for r in [
                self.rating_reliability,
                self.rating_performance,
                self.rating_support,
                self.rating_value,
            ] if r is not None
        ]
        if not ratings:
            return self.rating
        return sum(ratings) / len(ratings)

    @property
    def helpfulness_score(self) -> float:
        """Calculate helpfulness score (0-1)."""
        total = self.helpful_count + self.not_helpful_count
        if total == 0:
            return 0.5
        return self.helpful_count / total

    @property
    def is_long_term_user(self) -> bool:
        """Check if reviewer is a long-term user (6+ months)."""
        return (self.months_used or 0) >= 6

    def add_helpful_vote(self, is_helpful: bool) -> None:
        """
        Add a helpfulness vote.

        Args:
            is_helpful: Whether vote is 'helpful'
        """
        if is_helpful:
            self.helpful_count += 1
        else:
            self.not_helpful_count += 1

    def verify(self, method: str = "system") -> None:
        """
        Mark review as verified.

        Args:
            method: Verification method
        """
        self.is_verified = True
        self.verified_at = datetime.utcnow()
        self.verification_method = method

    def moderate(
        self,
        status: str,
        moderator: str,
        notes: str = None,
        visible: bool = True,
    ) -> None:
        """
        Moderate the review.

        Args:
            status: Moderation status
            moderator: Moderator ID
            notes: Moderation notes
            visible: Whether to make visible
        """
        self.moderation_status = status
        self.moderated_by = moderator
        self.moderated_at = datetime.utcnow()
        self.moderation_notes = notes
        self.is_visible = visible

    def add_provider_response(self, response: str) -> None:
        """
        Add provider's response to review.

        Args:
            response: Response text
        """
        self.provider_response = response
        self.provider_responded_at = datetime.utcnow()
