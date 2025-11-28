"""
Provider CRUD Repositories

Repositories for provider marketplace operations.
"""

from datetime import datetime, timedelta
from typing import List, Optional
from uuid import UUID

from sqlalchemy import and_, desc, func, or_
from sqlalchemy.orm import Session

from app.db.crud.base import BaseRepository
from app.db.models.provider import Provider
from app.db.models.provider_pricing_tier import ProviderPricingTier
from app.db.models.provider_metrics import ProviderMetrics
from app.db.models.provider_application import ProviderApplication
from app.db.models.provider_verification import ProviderVerification
from app.db.models.provider_sla import ProviderSLA
from app.db.models.provider_review import ProviderReview
from app.db.models.enums import ProviderStatus, ProviderType, ApplicationStatus


class ProviderRepository(BaseRepository[Provider]):
    """Repository for Provider model operations."""

    def __init__(self, db: Session):
        super().__init__(Provider, db)

    def get_by_code(self, code: str) -> Optional[Provider]:
        """Get provider by unique code."""
        return (
            self.db.query(Provider)
            .filter(Provider.code == code)
            .first()
        )

    def get_active(self) -> List[Provider]:
        """Get all active providers."""
        return (
            self.db.query(Provider)
            .filter(Provider.status == ProviderStatus.ACTIVE.value)
            .order_by(Provider.display_name)
            .all()
        )

    def get_by_type(self, provider_type: ProviderType) -> List[Provider]:
        """Get providers by type."""
        return (
            self.db.query(Provider)
            .filter(
                Provider.provider_type == provider_type.value,
                Provider.status == ProviderStatus.ACTIVE.value,
            )
            .order_by(Provider.display_name)
            .all()
        )

    def get_first_party(self) -> List[Provider]:
        """Get first-party (Omniphi) providers."""
        return (
            self.db.query(Provider)
            .filter(
                Provider.provider_type == ProviderType.FIRST_PARTY.value,
                Provider.status == ProviderStatus.ACTIVE.value,
            )
            .all()
        )

    def get_community(self) -> List[Provider]:
        """Get community (third-party) providers."""
        return (
            self.db.query(Provider)
            .filter(
                Provider.provider_type == ProviderType.COMMUNITY.value,
                Provider.status == ProviderStatus.ACTIVE.value,
            )
            .order_by(desc(Provider.rating))
            .all()
        )

    def get_by_region(self, region_code: str) -> List[Provider]:
        """Get providers available in a region."""
        return (
            self.db.query(Provider)
            .filter(
                Provider.status == ProviderStatus.ACTIVE.value,
                Provider.supported_regions.contains([region_code]),
            )
            .order_by(desc(Provider.rating))
            .all()
        )

    def get_top_rated(self, limit: int = 10) -> List[Provider]:
        """Get top-rated providers."""
        return (
            self.db.query(Provider)
            .filter(
                Provider.status == ProviderStatus.ACTIVE.value,
                Provider.review_count >= 5,
            )
            .order_by(desc(Provider.rating))
            .limit(limit)
            .all()
        )

    def search(
        self,
        query: Optional[str] = None,
        provider_type: Optional[ProviderType] = None,
        region_code: Optional[str] = None,
        min_rating: Optional[float] = None,
        features: Optional[List[str]] = None,
    ) -> List[Provider]:
        """Search providers with filters."""
        q = self.db.query(Provider).filter(
            Provider.status == ProviderStatus.ACTIVE.value
        )

        if query:
            q = q.filter(
                or_(
                    Provider.name.ilike(f"%{query}%"),
                    Provider.display_name.ilike(f"%{query}%"),
                    Provider.description.ilike(f"%{query}%"),
                )
            )

        if provider_type:
            q = q.filter(Provider.provider_type == provider_type.value)

        if region_code:
            q = q.filter(Provider.supported_regions.contains([region_code]))

        if min_rating:
            q = q.filter(Provider.rating >= min_rating)

        if features:
            for feature in features:
                q = q.filter(Provider.features.has_key(feature))

        return q.order_by(desc(Provider.rating)).all()

    def update_stats(self, id: UUID) -> Optional[Provider]:
        """Update provider statistics from validators."""
        provider = self.get(id)
        if not provider:
            return None

        # Count validators - would need to join with validators table
        # This is a placeholder for the actual implementation
        self.db.commit()
        self.db.refresh(provider)
        return provider

    def update_rating(self, id: UUID) -> Optional[Provider]:
        """Recalculate provider rating from reviews."""
        provider = self.get(id)
        if not provider:
            return None

        result = (
            self.db.query(
                func.avg(ProviderReview.overall_rating),
                func.count(ProviderReview.id),
            )
            .filter(
                ProviderReview.provider_id == id,
                ProviderReview.is_visible == True,
            )
            .first()
        )

        if result and result[0]:
            provider.rating = round(float(result[0]), 2)
            provider.review_count = result[1]
            self.db.commit()
            self.db.refresh(provider)

        return provider


class ProviderPricingTierRepository(BaseRepository[ProviderPricingTier]):
    """Repository for ProviderPricingTier model operations."""

    def __init__(self, db: Session):
        super().__init__(ProviderPricingTier, db)

    def get_by_provider(self, provider_id: UUID) -> List[ProviderPricingTier]:
        """Get all pricing tiers for a provider."""
        return (
            self.db.query(ProviderPricingTier)
            .filter(ProviderPricingTier.provider_id == provider_id)
            .order_by(ProviderPricingTier.monthly_price_usd)
            .all()
        )

    def get_available_by_provider(self, provider_id: UUID) -> List[ProviderPricingTier]:
        """Get available pricing tiers for a provider."""
        return (
            self.db.query(ProviderPricingTier)
            .filter(
                ProviderPricingTier.provider_id == provider_id,
                ProviderPricingTier.is_available == True,
            )
            .order_by(ProviderPricingTier.monthly_price_usd)
            .all()
        )

    def get_by_code(self, provider_id: UUID, tier_code: str) -> Optional[ProviderPricingTier]:
        """Get specific tier by provider and code."""
        return (
            self.db.query(ProviderPricingTier)
            .filter(
                ProviderPricingTier.provider_id == provider_id,
                ProviderPricingTier.tier_code == tier_code,
            )
            .first()
        )

    def get_in_region(
        self, provider_id: UUID, region_code: str
    ) -> List[ProviderPricingTier]:
        """Get tiers available in a specific region."""
        return (
            self.db.query(ProviderPricingTier)
            .filter(
                ProviderPricingTier.provider_id == provider_id,
                ProviderPricingTier.is_available == True,
                ProviderPricingTier.available_regions.contains([region_code]),
            )
            .order_by(ProviderPricingTier.monthly_price_usd)
            .all()
        )

    def get_by_specs(
        self,
        min_cpu: Optional[int] = None,
        min_memory_gb: Optional[int] = None,
        min_storage_gb: Optional[int] = None,
        max_price: Optional[float] = None,
    ) -> List[ProviderPricingTier]:
        """Find tiers matching minimum specs."""
        q = self.db.query(ProviderPricingTier).filter(
            ProviderPricingTier.is_available == True
        )

        if min_cpu:
            q = q.filter(ProviderPricingTier.cpu_cores >= min_cpu)
        if min_memory_gb:
            q = q.filter(ProviderPricingTier.memory_gb >= min_memory_gb)
        if min_storage_gb:
            q = q.filter(ProviderPricingTier.storage_gb >= min_storage_gb)
        if max_price:
            q = q.filter(ProviderPricingTier.monthly_price_usd <= max_price)

        return q.order_by(ProviderPricingTier.monthly_price_usd).all()


class ProviderMetricsRepository(BaseRepository[ProviderMetrics]):
    """Repository for ProviderMetrics model operations."""

    def __init__(self, db: Session):
        super().__init__(ProviderMetrics, db)

    def get_latest(self, provider_id: UUID) -> Optional[ProviderMetrics]:
        """Get latest metrics for a provider."""
        return (
            self.db.query(ProviderMetrics)
            .filter(ProviderMetrics.provider_id == provider_id)
            .order_by(desc(ProviderMetrics.recorded_at))
            .first()
        )

    def get_by_region(
        self, provider_id: UUID, region_code: str
    ) -> List[ProviderMetrics]:
        """Get metrics for a provider in a region."""
        return (
            self.db.query(ProviderMetrics)
            .filter(
                ProviderMetrics.provider_id == provider_id,
                ProviderMetrics.region_code == region_code,
            )
            .order_by(desc(ProviderMetrics.recorded_at))
            .all()
        )

    def get_history(
        self,
        provider_id: UUID,
        hours: int = 24,
        region_code: Optional[str] = None,
    ) -> List[ProviderMetrics]:
        """Get metrics history for a time period."""
        threshold = datetime.utcnow() - timedelta(hours=hours)
        q = self.db.query(ProviderMetrics).filter(
            ProviderMetrics.provider_id == provider_id,
            ProviderMetrics.recorded_at >= threshold,
        )

        if region_code:
            q = q.filter(ProviderMetrics.region_code == region_code)

        return q.order_by(ProviderMetrics.recorded_at).all()

    def get_averages(
        self, provider_id: UUID, hours: int = 24
    ) -> dict:
        """Get average metrics over time period."""
        threshold = datetime.utcnow() - timedelta(hours=hours)

        result = (
            self.db.query(
                func.avg(ProviderMetrics.uptime_percent),
                func.avg(ProviderMetrics.latency_avg_ms),
                func.avg(ProviderMetrics.provision_success_rate),
                func.avg(ProviderMetrics.health_score),
            )
            .filter(
                ProviderMetrics.provider_id == provider_id,
                ProviderMetrics.recorded_at >= threshold,
            )
            .first()
        )

        return {
            "avg_uptime_percent": float(result[0] or 0),
            "avg_latency_ms": float(result[1] or 0),
            "avg_provision_success_rate": float(result[2] or 0),
            "avg_health_score": float(result[3] or 0),
        }

    def cleanup_old(self, days: int = 30) -> int:
        """Remove metrics older than specified days."""
        threshold = datetime.utcnow() - timedelta(days=days)
        result = (
            self.db.query(ProviderMetrics)
            .filter(ProviderMetrics.recorded_at < threshold)
            .delete(synchronize_session=False)
        )
        self.db.commit()
        return result


class ProviderApplicationRepository(BaseRepository[ProviderApplication]):
    """Repository for ProviderApplication model operations."""

    def __init__(self, db: Session):
        super().__init__(ProviderApplication, db)

    def get_by_email(self, email: str) -> List[ProviderApplication]:
        """Get applications by contact email."""
        return (
            self.db.query(ProviderApplication)
            .filter(ProviderApplication.contact_email == email)
            .order_by(desc(ProviderApplication.submitted_at))
            .all()
        )

    def get_by_status(self, status: ApplicationStatus) -> List[ProviderApplication]:
        """Get applications by status."""
        return (
            self.db.query(ProviderApplication)
            .filter(ProviderApplication.status == status.value)
            .order_by(ProviderApplication.submitted_at)
            .all()
        )

    def get_pending(self) -> List[ProviderApplication]:
        """Get pending applications."""
        return self.get_by_status(ApplicationStatus.PENDING)

    def get_under_review(self) -> List[ProviderApplication]:
        """Get applications under review."""
        return self.get_by_status(ApplicationStatus.UNDER_REVIEW)

    def set_status(
        self,
        id: UUID,
        status: ApplicationStatus,
        reason: Optional[str] = None,
        reviewed_by: Optional[str] = None,
    ) -> Optional[ProviderApplication]:
        """Update application status."""
        application = self.get(id)
        if not application:
            return None

        application.status = status.value
        application.status_reason = reason
        application.reviewed_by = reviewed_by
        application.reviewed_at = datetime.utcnow()

        if status == ApplicationStatus.APPROVED:
            application.approved_at = datetime.utcnow()

        self.db.commit()
        self.db.refresh(application)
        return application

    def approve(
        self,
        id: UUID,
        provider_id: UUID,
        reviewed_by: str,
    ) -> Optional[ProviderApplication]:
        """Approve application and link to created provider."""
        application = self.get(id)
        if not application:
            return None

        application.status = ApplicationStatus.APPROVED.value
        application.provider_id = provider_id
        application.reviewed_by = reviewed_by
        application.reviewed_at = datetime.utcnow()
        application.approved_at = datetime.utcnow()

        self.db.commit()
        self.db.refresh(application)
        return application


class ProviderVerificationRepository(BaseRepository[ProviderVerification]):
    """Repository for ProviderVerification model operations."""

    def __init__(self, db: Session):
        super().__init__(ProviderVerification, db)

    def get_by_application(self, application_id: UUID) -> List[ProviderVerification]:
        """Get all verifications for an application."""
        return (
            self.db.query(ProviderVerification)
            .filter(ProviderVerification.application_id == application_id)
            .order_by(ProviderVerification.executed_at)
            .all()
        )

    def get_latest_by_type(
        self, application_id: UUID, check_type: str
    ) -> Optional[ProviderVerification]:
        """Get latest verification of a specific type."""
        return (
            self.db.query(ProviderVerification)
            .filter(
                ProviderVerification.application_id == application_id,
                ProviderVerification.check_type == check_type,
            )
            .order_by(desc(ProviderVerification.executed_at))
            .first()
        )

    def all_passed(self, application_id: UUID) -> bool:
        """Check if all required verifications passed."""
        results = self.get_by_application(application_id)
        if not results:
            return False
        return all(v.passed for v in results)

    def get_failed(self, application_id: UUID) -> List[ProviderVerification]:
        """Get failed verifications for an application."""
        return (
            self.db.query(ProviderVerification)
            .filter(
                ProviderVerification.application_id == application_id,
                ProviderVerification.passed == False,
            )
            .all()
        )


class ProviderSLARepository(BaseRepository[ProviderSLA]):
    """Repository for ProviderSLA model operations."""

    def __init__(self, db: Session):
        super().__init__(ProviderSLA, db)

    def get_active(self, provider_id: UUID) -> Optional[ProviderSLA]:
        """Get active SLA for a provider."""
        return (
            self.db.query(ProviderSLA)
            .filter(
                ProviderSLA.provider_id == provider_id,
                ProviderSLA.is_active == True,
            )
            .first()
        )

    def get_by_provider(self, provider_id: UUID) -> List[ProviderSLA]:
        """Get all SLAs for a provider (including historical)."""
        return (
            self.db.query(ProviderSLA)
            .filter(ProviderSLA.provider_id == provider_id)
            .order_by(desc(ProviderSLA.effective_from))
            .all()
        )

    def deactivate_current(self, provider_id: UUID) -> None:
        """Deactivate current SLA before adding new one."""
        current = self.get_active(provider_id)
        if current:
            current.is_active = False
            current.effective_until = datetime.utcnow()
            self.db.commit()


class ProviderReviewRepository(BaseRepository[ProviderReview]):
    """Repository for ProviderReview model operations."""

    def __init__(self, db: Session):
        super().__init__(ProviderReview, db)

    def get_by_provider(
        self,
        provider_id: UUID,
        limit: int = 50,
        offset: int = 0,
    ) -> List[ProviderReview]:
        """Get reviews for a provider."""
        return (
            self.db.query(ProviderReview)
            .filter(
                ProviderReview.provider_id == provider_id,
                ProviderReview.is_visible == True,
            )
            .order_by(desc(ProviderReview.created_at))
            .offset(offset)
            .limit(limit)
            .all()
        )

    def get_by_user(self, user_id: str) -> List[ProviderReview]:
        """Get reviews by a user."""
        return (
            self.db.query(ProviderReview)
            .filter(ProviderReview.user_id == user_id)
            .order_by(desc(ProviderReview.created_at))
            .all()
        )

    def get_user_review(
        self, provider_id: UUID, user_id: str
    ) -> Optional[ProviderReview]:
        """Get a user's review for a specific provider."""
        return (
            self.db.query(ProviderReview)
            .filter(
                ProviderReview.provider_id == provider_id,
                ProviderReview.user_id == user_id,
            )
            .first()
        )

    def get_verified(self, provider_id: UUID) -> List[ProviderReview]:
        """Get verified reviews for a provider."""
        return (
            self.db.query(ProviderReview)
            .filter(
                ProviderReview.provider_id == provider_id,
                ProviderReview.is_visible == True,
                ProviderReview.is_verified == True,
            )
            .order_by(desc(ProviderReview.created_at))
            .all()
        )

    def get_rating_distribution(self, provider_id: UUID) -> dict:
        """Get rating distribution for a provider."""
        results = (
            self.db.query(
                ProviderReview.overall_rating,
                func.count(ProviderReview.id),
            )
            .filter(
                ProviderReview.provider_id == provider_id,
                ProviderReview.is_visible == True,
            )
            .group_by(ProviderReview.overall_rating)
            .all()
        )

        distribution = {1: 0, 2: 0, 3: 0, 4: 0, 5: 0}
        for rating, count in results:
            if rating:
                distribution[int(rating)] = count

        return distribution

    def vote_helpful(self, id: UUID, helpful: bool) -> Optional[ProviderReview]:
        """Record a helpful/unhelpful vote on a review."""
        review = self.get(id)
        if not review:
            return None

        if helpful:
            review.helpful_votes = (review.helpful_votes or 0) + 1
        else:
            review.unhelpful_votes = (review.unhelpful_votes or 0) + 1

        self.db.commit()
        self.db.refresh(review)
        return review
