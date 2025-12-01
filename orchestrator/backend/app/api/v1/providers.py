"""
Omniphi Cloud Provider API Endpoints

Endpoints for provider management, pricing, and third-party onboarding.
"""

from datetime import datetime
from typing import List, Optional
from uuid import uuid4

from fastapi import APIRouter, HTTPException, Query, BackgroundTasks
from pydantic import BaseModel, Field

router = APIRouter(prefix="/providers", tags=["providers"])


# ============================================
# SCHEMAS
# ============================================

class ProviderResponse(BaseModel):
    """Provider response"""
    id: str
    code: str
    name: str
    display_name: str
    description: Optional[str]
    logo_url: Optional[str]
    is_first_party: bool
    is_community: bool
    status: str
    supported_regions: List[str]
    features: dict
    uptime_percent: Optional[float]
    avg_latency_ms: Optional[float]
    rating: Optional[float]
    review_count: int
    total_validators: int
    active_validators: int


class PricingTierResponse(BaseModel):
    """Pricing tier response"""
    id: str
    tier_code: str
    name: str
    description: Optional[str]
    cpu_cores: int
    memory_gb: int
    disk_gb: int
    hourly_price: float
    monthly_price: float
    setup_fee: float
    currency: str
    is_available: bool
    available_in_regions: List[str]


class ProviderMetricsResponse(BaseModel):
    """Provider metrics response"""
    provider_id: str
    region: str
    avg_latency_ms: float
    p95_latency_ms: float
    p99_latency_ms: float
    uptime_percent: float
    success_rate: float
    avg_provision_time_seconds: float
    total_validators: int
    active_validators: int
    recorded_at: str


class ProviderApplicationRequest(BaseModel):
    """Provider application request"""
    company_name: str
    contact_name: str
    contact_email: str  # Email address
    contact_phone: Optional[str] = None
    website_url: Optional[str] = None
    proposed_code: str
    description: str
    logo_url: Optional[str] = None
    api_endpoint: str
    api_documentation_url: Optional[str] = None
    supported_regions: List[str]
    proposed_pricing: List[dict]
    uptime_guarantee: float = 99.9
    response_time_sla_hours: int = 24


class ApplicationResponse(BaseModel):
    """Application response"""
    id: str
    company_name: str
    proposed_code: str
    status: str
    submitted_at: str
    reviewed_at: Optional[str]


class ReviewRequest(BaseModel):
    """Review request"""
    rating: float = Field(..., ge=1, le=5)
    title: Optional[str] = None
    comment: Optional[str] = None
    validators_hosted: Optional[int] = None
    months_used: Optional[int] = None


class ReviewResponse(BaseModel):
    """Review response"""
    id: str
    rating: float
    title: Optional[str]
    comment: Optional[str]
    is_verified: bool
    created_at: str


class ProvisionRequest(BaseModel):
    """Provision request for Omniphi Cloud"""
    tier_code: str
    region: str
    validator_name: str
    wallet_address: str
    commission_rate: float = 0.05


# ============================================
# MOCK DATA
# ============================================

def get_mock_providers():
    """Generate mock provider data."""
    return [
        {
            "id": "550e8400-e29b-41d4-a716-446655440001",
            "code": "omniphi-cloud",
            "name": "Omniphi Cloud",
            "display_name": "Omniphi Cloud",
            "description": "Official Omniphi Cloud validator hosting platform with enterprise-grade infrastructure",
            "logo_url": "https://omniphi.io/logo.png",
            "is_first_party": True,
            "is_community": False,
            "status": "active",
            "supported_regions": ["us-east", "us-west", "eu-central", "asia-pacific"],
            "features": {
                "auto_failover": True,
                "monitoring": True,
                "alerts": True,
                "backup": True,
                "api_access": True
            },
            "uptime_percent": 99.99,
            "avg_latency_ms": 12.5,
            "rating": 4.9,
            "review_count": 156,
            "total_validators": 520,
            "active_validators": 485
        },
        {
            "id": "550e8400-e29b-41d4-a716-446655440002",
            "code": "akash",
            "name": "Akash Network",
            "display_name": "Akash Network",
            "description": "Decentralized cloud computing marketplace",
            "logo_url": "https://akash.network/logo.png",
            "is_first_party": False,
            "is_community": True,
            "status": "active",
            "supported_regions": ["us-east", "eu-central"],
            "features": {
                "auto_failover": False,
                "monitoring": True,
                "alerts": True,
                "backup": False,
                "api_access": True
            },
            "uptime_percent": 99.5,
            "avg_latency_ms": 25.0,
            "rating": 4.5,
            "review_count": 42,
            "total_validators": 85,
            "active_validators": 78
        }
    ]


def get_mock_tiers():
    """Generate mock pricing tiers."""
    return [
        {
            "id": "550e8400-e29b-41d4-a716-446655440010",
            "tier_code": "starter",
            "name": "Starter",
            "description": "Perfect for getting started with validator operations",
            "cpu_cores": 2,
            "memory_gb": 4,
            "disk_gb": 100,
            "hourly_price": 0.05,
            "monthly_price": 29.0,
            "setup_fee": 0.0,
            "currency": "USD",
            "is_available": True,
            "available_in_regions": ["us-east", "us-west", "eu-central", "asia-pacific"]
        },
        {
            "id": "550e8400-e29b-41d4-a716-446655440011",
            "tier_code": "professional",
            "name": "Professional",
            "description": "For serious validators requiring higher performance",
            "cpu_cores": 4,
            "memory_gb": 8,
            "disk_gb": 250,
            "hourly_price": 0.12,
            "monthly_price": 89.0,
            "setup_fee": 0.0,
            "currency": "USD",
            "is_available": True,
            "available_in_regions": ["us-east", "us-west", "eu-central", "asia-pacific"]
        },
        {
            "id": "550e8400-e29b-41d4-a716-446655440012",
            "tier_code": "enterprise",
            "name": "Enterprise",
            "description": "Maximum performance for high-stake validators",
            "cpu_cores": 8,
            "memory_gb": 32,
            "disk_gb": 500,
            "hourly_price": 0.40,
            "monthly_price": 299.0,
            "setup_fee": 0.0,
            "currency": "USD",
            "is_available": True,
            "available_in_regions": ["us-east", "us-west", "eu-central", "asia-pacific"]
        }
    ]


def get_mock_metrics():
    """Generate mock provider metrics."""
    now = datetime.utcnow()
    return [
        {
            "provider_id": "550e8400-e29b-41d4-a716-446655440001",
            "region": "us-east",
            "avg_latency_ms": 10.5,
            "p95_latency_ms": 25.0,
            "p99_latency_ms": 45.0,
            "uptime_percent": 99.99,
            "success_rate": 99.95,
            "avg_provision_time_seconds": 180,
            "total_validators": 175,
            "active_validators": 160,
            "recorded_at": now.isoformat()
        },
        {
            "provider_id": "550e8400-e29b-41d4-a716-446655440001",
            "region": "us-west",
            "avg_latency_ms": 12.0,
            "p95_latency_ms": 28.0,
            "p99_latency_ms": 50.0,
            "uptime_percent": 99.98,
            "success_rate": 99.92,
            "avg_provision_time_seconds": 195,
            "total_validators": 130,
            "active_validators": 125,
            "recorded_at": now.isoformat()
        }
    ]


def get_mock_applications():
    """Generate mock provider applications."""
    return [
        {
            "id": "550e8400-e29b-41d4-a716-446655440020",
            "company_name": "CloudNodes Inc",
            "proposed_code": "cloudnodes",
            "status": "pending",
            "submitted_at": "2024-11-20T00:00:00Z",
            "reviewed_at": None
        },
        {
            "id": "550e8400-e29b-41d4-a716-446655440021",
            "company_name": "ValidatorPro",
            "proposed_code": "validatorpro",
            "status": "approved",
            "submitted_at": "2024-11-10T00:00:00Z",
            "reviewed_at": "2024-11-15T00:00:00Z"
        }
    ]


def get_mock_reviews(provider_id: str):
    """Generate mock reviews."""
    return [
        {
            "id": "550e8400-e29b-41d4-a716-446655440030",
            "rating": 5.0,
            "title": "Excellent service",
            "comment": "Very reliable and easy to set up. Support is responsive.",
            "is_verified": True,
            "created_at": "2024-11-15T00:00:00Z"
        },
        {
            "id": "550e8400-e29b-41d4-a716-446655440031",
            "rating": 4.5,
            "title": "Great uptime",
            "comment": "No downtime in 6 months of operation.",
            "is_verified": True,
            "created_at": "2024-11-10T00:00:00Z"
        }
    ]


# ============================================
# ENDPOINTS
# ============================================

@router.get("", response_model=List[ProviderResponse])
async def list_providers(
    active_only: bool = Query(True, description="Only show active providers"),
    first_party_only: bool = Query(False, description="Only show first-party providers"),
):
    """
    List all cloud providers.

    Returns providers with their capabilities and metrics.
    """
    providers = get_mock_providers()

    if active_only:
        providers = [p for p in providers if p["status"] == "active"]
    if first_party_only:
        providers = [p for p in providers if p["is_first_party"]]

    return providers


@router.get("/omniphi-cloud", response_model=ProviderResponse)
async def get_omniphi_cloud():
    """
    Get Omniphi Cloud provider details.

    Returns the first-party Omniphi Cloud provider information.
    """
    providers = get_mock_providers()
    provider = next((p for p in providers if p["code"] == "omniphi-cloud"), None)

    if not provider:
        raise HTTPException(status_code=404, detail="Omniphi Cloud provider not found")

    return provider


@router.get("/omniphi-cloud/tiers", response_model=List[PricingTierResponse])
async def get_omniphi_cloud_tiers(
    region: Optional[str] = Query(None, description="Filter by region"),
):
    """
    Get Omniphi Cloud pricing tiers.

    Returns available machine types and pricing.
    """
    tiers = get_mock_tiers()

    if region:
        tiers = [t for t in tiers if region in t["available_in_regions"]]

    return tiers


@router.get("/omniphi-cloud/regions")
async def get_omniphi_cloud_regions():
    """
    Get available Omniphi Cloud regions.

    Returns regions where Omniphi Cloud validators can be deployed.
    """
    providers = get_mock_providers()
    provider = next((p for p in providers if p["code"] == "omniphi-cloud"), None)

    if not provider:
        raise HTTPException(status_code=404, detail="Omniphi Cloud provider not found")

    return {
        "provider": "omniphi-cloud",
        "regions": provider["supported_regions"],
    }


@router.get("/omniphi-cloud/metrics", response_model=List[ProviderMetricsResponse])
async def get_omniphi_cloud_metrics(
    region: Optional[str] = Query(None, description="Filter by region"),
    limit: int = Query(24, ge=1, le=168, description="Number of records (hours)"),
):
    """
    Get Omniphi Cloud performance metrics.

    Returns historical performance data.
    """
    metrics = get_mock_metrics()

    if region:
        metrics = [m for m in metrics if m["region"] == region]

    return metrics[:limit]


@router.post("/omniphi-cloud/provision")
async def provision_omniphi_cloud_validator(
    request: ProvisionRequest,
    background_tasks: BackgroundTasks,
):
    """
    Provision a new validator on Omniphi Cloud.

    Creates a new validator node in the specified region.
    """
    tiers = get_mock_tiers()
    tier = next((t for t in tiers if t["tier_code"] == request.tier_code), None)

    if not tier:
        raise HTTPException(status_code=404, detail=f"Tier '{request.tier_code}' not found")

    if request.region not in tier["available_in_regions"]:
        raise HTTPException(
            status_code=400,
            detail=f"Region '{request.region}' not available for tier '{request.tier_code}'"
        )

    provision_id = str(uuid4())

    return {
        "status": "provisioning",
        "provision_id": provision_id,
        "provider": "omniphi-cloud",
        "tier": request.tier_code,
        "region": request.region,
        "validator_name": request.validator_name,
        "estimated_time_seconds": 300,
    }


@router.post("/apply", response_model=ApplicationResponse)
async def apply_as_provider(request: ProviderApplicationRequest):
    """
    Apply to become a provider.

    Third-party providers can apply to join the marketplace.
    """
    # Check if code is taken
    providers = get_mock_providers()
    if any(p["code"] == request.proposed_code for p in providers):
        raise HTTPException(
            status_code=400,
            detail=f"Provider code '{request.proposed_code}' is already taken"
        )

    return {
        "id": f"550e8400-e29b-41d4-a716-{uuid4().hex[:12]}",
        "company_name": request.company_name,
        "proposed_code": request.proposed_code,
        "status": "pending",
        "submitted_at": datetime.utcnow().isoformat(),
        "reviewed_at": None,
    }


@router.get("/applications", response_model=List[ApplicationResponse])
async def list_applications(
    status: Optional[str] = Query(None, description="Filter by status"),
    limit: int = Query(20, ge=1, le=100),
    offset: int = Query(0, ge=0),
):
    """
    List provider applications (admin).

    Returns pending and processed applications.
    """
    applications = get_mock_applications()

    if status:
        applications = [a for a in applications if a["status"] == status]

    return applications[offset:offset + limit]


@router.post("/applications/{application_id}/approve")
async def approve_application(
    application_id: str,
    reviewed_by: str = Query(..., description="Reviewer ID"),
):
    """
    Approve a provider application (admin).

    Creates the provider and enables marketplace listing.
    """
    applications = get_mock_applications()
    application = next((a for a in applications if a["id"] == application_id), None)

    if not application:
        raise HTTPException(status_code=404, detail="Application not found")

    if application["status"] not in ["pending", "under_review"]:
        raise HTTPException(
            status_code=400,
            detail=f"Cannot approve application in status: {application['status']}"
        )

    return {
        "status": "approved",
        "application_id": application_id,
        "provider_id": f"550e8400-e29b-41d4-a716-{uuid4().hex[:12]}",
        "provider_code": application["proposed_code"],
    }


@router.post("/applications/{application_id}/reject")
async def reject_application(
    application_id: str,
    reason: str = Query(..., description="Rejection reason"),
    reviewed_by: str = Query(..., description="Reviewer ID"),
):
    """
    Reject a provider application (admin).
    """
    applications = get_mock_applications()
    application = next((a for a in applications if a["id"] == application_id), None)

    if not application:
        raise HTTPException(status_code=404, detail="Application not found")

    return {
        "status": "rejected",
        "application_id": application_id,
        "reason": reason,
    }


@router.get("/{provider_id}/reviews", response_model=List[ReviewResponse])
async def get_provider_reviews(
    provider_id: str,
    limit: int = Query(20, ge=1, le=100),
    offset: int = Query(0, ge=0),
):
    """Get reviews for a provider."""
    reviews = get_mock_reviews(provider_id)
    return reviews[offset:offset + limit]


@router.post("/{provider_id}/reviews", response_model=ReviewResponse)
async def submit_review(
    provider_id: str,
    request: ReviewRequest,
    user_id: str = Query(..., description="User ID"),
):
    """Submit a review for a provider."""
    providers = get_mock_providers()
    provider = next((p for p in providers if p["id"] == provider_id), None)

    if not provider:
        raise HTTPException(status_code=404, detail="Provider not found")

    return {
        "id": f"550e8400-e29b-41d4-a716-{uuid4().hex[:12]}",
        "rating": request.rating,
        "title": request.title,
        "comment": request.comment,
        "is_verified": False,
        "created_at": datetime.utcnow().isoformat(),
    }
