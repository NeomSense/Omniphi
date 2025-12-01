"""
Seed Data Script for Omniphi Cloud Database

Creates default data for:
- Regions (global infrastructure)
- Omniphi Cloud provider (first-party)
- Billing plans (free, starter, professional, enterprise)
- Server pool templates

Usage:
    python -m app.db.seed
    # or
    from app.db.seed import seed_all
    seed_all()
"""

import uuid
from datetime import datetime, timedelta

from sqlalchemy.orm import Session

from app.db.database import SessionLocal, engine, Base
from app.db.models.region import Region
from app.db.models.server_pool import ServerPool
from app.db.models.provider import Provider
from app.db.models.provider_pricing_tier import ProviderPricingTier
from app.db.models.provider_sla import ProviderSLA
from app.db.models.billing_plan import BillingPlan
from app.db.models.enums import (
    RegionStatus,
    ProviderType,
    ProviderStatus,
    BillingPlanType,
)


# =============================================================================
# REGION DATA
# =============================================================================

REGIONS = [
    {
        "code": "us-east-1",
        "name": "US East (Virginia)",
        "display_name": "Virginia, USA",
        "continent": "North America",
        "country": "United States",
        "country_code": "US",
        "city": "Ashburn",
        "latitude": 39.0438,
        "longitude": -77.4874,
        "timezone": "America/New_York",
        "status": RegionStatus.ACTIVE.value,
        "is_default": True,
        "priority": 1,
        "cloud_provider": "omniphi",
        "cloud_region": "us-east-1",
        "cloud_zones": ["us-east-1a", "us-east-1b", "us-east-1c"],
        "max_validators": 5000,
        "features": {
            "snapshot_restore": True,
            "auto_scaling": True,
            "dedicated_servers": True,
            "ipv6": True,
        },
    },
    {
        "code": "us-west-2",
        "name": "US West (Oregon)",
        "display_name": "Oregon, USA",
        "continent": "North America",
        "country": "United States",
        "country_code": "US",
        "city": "Portland",
        "latitude": 45.5152,
        "longitude": -122.6784,
        "timezone": "America/Los_Angeles",
        "status": RegionStatus.ACTIVE.value,
        "is_default": False,
        "priority": 2,
        "cloud_provider": "omniphi",
        "cloud_region": "us-west-2",
        "cloud_zones": ["us-west-2a", "us-west-2b"],
        "max_validators": 3000,
        "features": {
            "snapshot_restore": True,
            "auto_scaling": True,
            "dedicated_servers": True,
            "ipv6": True,
        },
    },
    {
        "code": "eu-west-1",
        "name": "EU West (Ireland)",
        "display_name": "Dublin, Ireland",
        "continent": "Europe",
        "country": "Ireland",
        "country_code": "IE",
        "city": "Dublin",
        "latitude": 53.3498,
        "longitude": -6.2603,
        "timezone": "Europe/Dublin",
        "status": RegionStatus.ACTIVE.value,
        "is_default": False,
        "priority": 3,
        "cloud_provider": "omniphi",
        "cloud_region": "eu-west-1",
        "cloud_zones": ["eu-west-1a", "eu-west-1b"],
        "max_validators": 3000,
        "features": {
            "snapshot_restore": True,
            "auto_scaling": True,
            "dedicated_servers": True,
            "ipv6": True,
        },
    },
    {
        "code": "eu-central-1",
        "name": "EU Central (Frankfurt)",
        "display_name": "Frankfurt, Germany",
        "continent": "Europe",
        "country": "Germany",
        "country_code": "DE",
        "city": "Frankfurt",
        "latitude": 50.1109,
        "longitude": 8.6821,
        "timezone": "Europe/Berlin",
        "status": RegionStatus.ACTIVE.value,
        "is_default": False,
        "priority": 4,
        "cloud_provider": "omniphi",
        "cloud_region": "eu-central-1",
        "cloud_zones": ["eu-central-1a", "eu-central-1b"],
        "max_validators": 2000,
        "features": {
            "snapshot_restore": True,
            "auto_scaling": True,
            "dedicated_servers": False,
            "ipv6": True,
        },
    },
    {
        "code": "ap-southeast-1",
        "name": "Asia Pacific (Singapore)",
        "display_name": "Singapore",
        "continent": "Asia",
        "country": "Singapore",
        "country_code": "SG",
        "city": "Singapore",
        "latitude": 1.3521,
        "longitude": 103.8198,
        "timezone": "Asia/Singapore",
        "status": RegionStatus.ACTIVE.value,
        "is_default": False,
        "priority": 5,
        "cloud_provider": "omniphi",
        "cloud_region": "ap-southeast-1",
        "cloud_zones": ["ap-southeast-1a", "ap-southeast-1b"],
        "max_validators": 2000,
        "features": {
            "snapshot_restore": True,
            "auto_scaling": True,
            "dedicated_servers": False,
            "ipv6": True,
        },
    },
    {
        "code": "ap-northeast-1",
        "name": "Asia Pacific (Tokyo)",
        "display_name": "Tokyo, Japan",
        "continent": "Asia",
        "country": "Japan",
        "country_code": "JP",
        "city": "Tokyo",
        "latitude": 35.6762,
        "longitude": 139.6503,
        "timezone": "Asia/Tokyo",
        "status": RegionStatus.COMING_SOON.value,
        "is_default": False,
        "priority": 10,
        "cloud_provider": "omniphi",
        "cloud_region": "ap-northeast-1",
        "cloud_zones": ["ap-northeast-1a"],
        "max_validators": 1000,
        "features": {
            "snapshot_restore": True,
            "auto_scaling": False,
            "dedicated_servers": False,
            "ipv6": True,
        },
    },
]


# =============================================================================
# SERVER POOL TEMPLATES (per region)
# =============================================================================

SERVER_POOL_TEMPLATES = [
    {
        "tier_code": "small",
        "name": "Small",
        "machine_type": "small",
        "cpu_cores": 2,
        "memory_gb": 4,
        "storage_gb": 100,
        "storage_type": "nvme",
        "network_gbps": 1.0,
        "validators_per_server": 1,
        "hourly_price_usd": 0.05,
        "monthly_price_usd": 29.99,
        "features": {"backup": True, "monitoring": True},
    },
    {
        "tier_code": "medium",
        "name": "Medium",
        "machine_type": "medium",
        "cpu_cores": 4,
        "memory_gb": 8,
        "storage_gb": 200,
        "storage_type": "nvme",
        "network_gbps": 2.0,
        "validators_per_server": 2,
        "hourly_price_usd": 0.10,
        "monthly_price_usd": 59.99,
        "features": {"backup": True, "monitoring": True, "priority_support": True},
    },
    {
        "tier_code": "large",
        "name": "Large",
        "machine_type": "large",
        "cpu_cores": 8,
        "memory_gb": 16,
        "storage_gb": 500,
        "storage_type": "nvme",
        "network_gbps": 5.0,
        "validators_per_server": 4,
        "hourly_price_usd": 0.20,
        "monthly_price_usd": 119.99,
        "features": {"backup": True, "monitoring": True, "priority_support": True, "dedicated_ip": True},
    },
    {
        "tier_code": "xlarge",
        "name": "Extra Large",
        "machine_type": "xlarge",
        "cpu_cores": 16,
        "memory_gb": 32,
        "storage_gb": 1000,
        "storage_type": "nvme",
        "network_gbps": 10.0,
        "validators_per_server": 8,
        "hourly_price_usd": 0.40,
        "monthly_price_usd": 239.99,
        "features": {"backup": True, "monitoring": True, "priority_support": True, "dedicated_ip": True, "ddos_protection": True},
    },
]


# =============================================================================
# PROVIDER DATA
# =============================================================================

OMNIPHI_PROVIDER = {
    "code": "omniphi-cloud",
    "name": "Omniphi Cloud",
    "display_name": "Omniphi Cloud",
    "description": "Official Omniphi Cloud infrastructure. Enterprise-grade validator hosting with global coverage, automatic scaling, and 99.99% uptime guarantee.",
    "provider_type": ProviderType.FIRST_PARTY.value,
    "logo_url": "/static/images/omniphi-logo.svg",
    "website_url": "https://omniphi.io",
    "documentation_url": "https://docs.omniphi.io",
    "support_email": "support@omniphi.io",
    "support_url": "https://support.omniphi.io",
    "api_endpoint": "https://api.omniphi.io/v1",
    "api_version": "v1",
    "status": ProviderStatus.ACTIVE.value,
    "supported_regions": ["us-east-1", "us-west-2", "eu-west-1", "eu-central-1", "ap-southeast-1"],
    "supported_chains": ["omniphi-mainnet", "omniphi-testnet"],
    "features": {
        "auto_scaling": True,
        "snapshot_restore": True,
        "monitoring": True,
        "alerts": True,
        "api_access": True,
        "sso": True,
        "audit_logs": True,
        "dedicated_support": True,
        "sla_guarantee": True,
        "custom_configs": True,
    },
    "rating": 5.0,
    "review_count": 0,
    "uptime_percent": 99.99,
    "avg_provision_time_seconds": 180,
    "is_verified": True,
    "verified_at": datetime.utcnow(),
}

OMNIPHI_PRICING_TIERS = [
    {
        "tier_code": "validator-small",
        "name": "Small Validator",
        "description": "Perfect for testnet validators and light workloads",
        "cpu_cores": 2,
        "memory_gb": 4,
        "storage_gb": 100,
        "storage_type": "nvme",
        "bandwidth_gbps": 1.0,
        "hourly_price_usd": 0.05,
        "monthly_price_usd": 29.99,
        "setup_fee_usd": 0.0,
        "is_available": True,
        "is_recommended": False,
        "available_regions": ["us-east-1", "us-west-2", "eu-west-1", "eu-central-1", "ap-southeast-1"],
        "sort_order": 1,
        "features": {"backup": True, "monitoring": True},
    },
    {
        "tier_code": "validator-medium",
        "name": "Medium Validator",
        "description": "Recommended for mainnet validators with moderate stake",
        "cpu_cores": 4,
        "memory_gb": 8,
        "storage_gb": 200,
        "storage_type": "nvme",
        "bandwidth_gbps": 2.0,
        "hourly_price_usd": 0.10,
        "monthly_price_usd": 59.99,
        "setup_fee_usd": 0.0,
        "is_available": True,
        "is_recommended": True,
        "available_regions": ["us-east-1", "us-west-2", "eu-west-1", "eu-central-1", "ap-southeast-1"],
        "sort_order": 2,
        "features": {"backup": True, "monitoring": True, "priority_support": True},
    },
    {
        "tier_code": "validator-large",
        "name": "Large Validator",
        "description": "For high-stake validators requiring maximum performance",
        "cpu_cores": 8,
        "memory_gb": 16,
        "storage_gb": 500,
        "storage_type": "nvme",
        "bandwidth_gbps": 5.0,
        "hourly_price_usd": 0.20,
        "monthly_price_usd": 119.99,
        "setup_fee_usd": 0.0,
        "is_available": True,
        "is_recommended": False,
        "available_regions": ["us-east-1", "us-west-2", "eu-west-1", "eu-central-1", "ap-southeast-1"],
        "sort_order": 3,
        "features": {"backup": True, "monitoring": True, "priority_support": True, "dedicated_ip": True},
    },
    {
        "tier_code": "validator-enterprise",
        "name": "Enterprise Validator",
        "description": "Enterprise-grade infrastructure for institutional validators",
        "cpu_cores": 16,
        "memory_gb": 32,
        "storage_gb": 1000,
        "storage_type": "nvme",
        "bandwidth_gbps": 10.0,
        "hourly_price_usd": 0.40,
        "monthly_price_usd": 249.99,
        "setup_fee_usd": 0.0,
        "is_available": True,
        "is_recommended": False,
        "available_regions": ["us-east-1", "us-west-2", "eu-west-1"],
        "sort_order": 4,
        "features": {"backup": True, "monitoring": True, "priority_support": True, "dedicated_ip": True, "ddos_protection": True, "custom_configs": True},
    },
]

OMNIPHI_SLA = {
    "name": "Omniphi Cloud Standard SLA",
    "uptime_guarantee": 99.99,
    "response_time_hours": 1,
    "resolution_time_hours": 4,
    "credit_tiers": [
        {"below_percent": 99.99, "credit_percent": 10},
        {"below_percent": 99.9, "credit_percent": 25},
        {"below_percent": 99.0, "credit_percent": 50},
        {"below_percent": 95.0, "credit_percent": 100},
    ],
    "penalty_rate": 0.0,
    "max_monthly_credit_percent": 100.0,
    "exclusions": [
        "Scheduled maintenance",
        "Force majeure events",
        "Customer-caused issues",
        "Third-party service outages",
    ],
    "is_active": True,
    "effective_from": datetime.utcnow(),
}


# =============================================================================
# BILLING PLANS
# =============================================================================

BILLING_PLANS = [
    {
        "code": "free",
        "name": "Free",
        "display_name": "Free Tier",
        "description": "Get started with Omniphi Cloud for free. Perfect for testing and learning.",
        "plan_type": BillingPlanType.FREE.value,
        "monthly_price_usd": 0.0,
        "annual_price_usd": 0.0,
        "validators_included": 1,
        "max_validators": 1,
        "storage_gb_included": 10,
        "bandwidth_gb_included": 100,
        "support_level": "community",
        "features": {
            "testnet_access": True,
            "mainnet_access": False,
            "api_access": True,
            "monitoring_basic": True,
            "monitoring_advanced": False,
            "alerts_email": True,
            "alerts_webhook": False,
            "snapshot_restore": False,
            "priority_support": False,
        },
        "limits": {
            "api_requests_per_day": 1000,
            "regions": 1,
        },
        "overage_rates": {},
        "trial_days": 0,
        "is_active": True,
        "is_public": True,
        "sort_order": 1,
    },
    {
        "code": "starter",
        "name": "Starter",
        "display_name": "Starter Plan",
        "description": "For individual validators getting started on mainnet.",
        "plan_type": BillingPlanType.STARTER.value,
        "monthly_price_usd": 29.99,
        "annual_price_usd": 299.99,
        "annual_discount_percent": 17.0,
        "validators_included": 1,
        "max_validators": 3,
        "storage_gb_included": 100,
        "bandwidth_gb_included": 500,
        "support_level": "email",
        "features": {
            "testnet_access": True,
            "mainnet_access": True,
            "api_access": True,
            "monitoring_basic": True,
            "monitoring_advanced": False,
            "alerts_email": True,
            "alerts_webhook": True,
            "snapshot_restore": True,
            "priority_support": False,
        },
        "limits": {
            "api_requests_per_day": 10000,
            "regions": 2,
        },
        "overage_rates": {
            "validator": 29.99,
            "storage_gb": 0.10,
            "bandwidth_gb": 0.05,
        },
        "trial_days": 14,
        "is_active": True,
        "is_public": True,
        "sort_order": 2,
    },
    {
        "code": "professional",
        "name": "Professional",
        "display_name": "Professional Plan",
        "description": "For serious validators requiring reliability and advanced features.",
        "plan_type": BillingPlanType.PROFESSIONAL.value,
        "monthly_price_usd": 99.99,
        "annual_price_usd": 999.99,
        "annual_discount_percent": 17.0,
        "validators_included": 5,
        "max_validators": 20,
        "storage_gb_included": 500,
        "bandwidth_gb_included": 2000,
        "support_level": "priority",
        "features": {
            "testnet_access": True,
            "mainnet_access": True,
            "api_access": True,
            "monitoring_basic": True,
            "monitoring_advanced": True,
            "alerts_email": True,
            "alerts_webhook": True,
            "alerts_pagerduty": True,
            "snapshot_restore": True,
            "priority_support": True,
            "dedicated_ip": True,
        },
        "limits": {
            "api_requests_per_day": 100000,
            "regions": 5,
        },
        "overage_rates": {
            "validator": 19.99,
            "storage_gb": 0.08,
            "bandwidth_gb": 0.04,
        },
        "trial_days": 14,
        "is_active": True,
        "is_public": True,
        "sort_order": 3,
    },
    {
        "code": "business",
        "name": "Business",
        "display_name": "Business Plan",
        "description": "For validator businesses and staking providers.",
        "plan_type": BillingPlanType.BUSINESS.value,
        "monthly_price_usd": 299.99,
        "annual_price_usd": 2999.99,
        "annual_discount_percent": 17.0,
        "validators_included": 20,
        "max_validators": 100,
        "storage_gb_included": 2000,
        "bandwidth_gb_included": 10000,
        "support_level": "dedicated",
        "features": {
            "testnet_access": True,
            "mainnet_access": True,
            "api_access": True,
            "monitoring_basic": True,
            "monitoring_advanced": True,
            "alerts_email": True,
            "alerts_webhook": True,
            "alerts_pagerduty": True,
            "snapshot_restore": True,
            "priority_support": True,
            "dedicated_ip": True,
            "sso": True,
            "audit_logs": True,
            "custom_sla": True,
        },
        "limits": {
            "api_requests_per_day": 500000,
            "regions": "unlimited",
        },
        "overage_rates": {
            "validator": 14.99,
            "storage_gb": 0.06,
            "bandwidth_gb": 0.03,
        },
        "trial_days": 30,
        "is_active": True,
        "is_public": True,
        "sort_order": 4,
    },
    {
        "code": "enterprise",
        "name": "Enterprise",
        "display_name": "Enterprise Plan",
        "description": "Custom solutions for institutional validators and enterprises. Contact us for pricing.",
        "plan_type": BillingPlanType.ENTERPRISE.value,
        "monthly_price_usd": 0.0,  # Custom pricing
        "annual_price_usd": 0.0,
        "validators_included": 100,
        "max_validators": None,  # Unlimited
        "storage_gb_included": 10000,
        "bandwidth_gb_included": 50000,
        "support_level": "enterprise",
        "features": {
            "testnet_access": True,
            "mainnet_access": True,
            "api_access": True,
            "monitoring_basic": True,
            "monitoring_advanced": True,
            "alerts_email": True,
            "alerts_webhook": True,
            "alerts_pagerduty": True,
            "snapshot_restore": True,
            "priority_support": True,
            "dedicated_ip": True,
            "sso": True,
            "audit_logs": True,
            "custom_sla": True,
            "dedicated_account_manager": True,
            "custom_integration": True,
            "on_premise_option": True,
        },
        "limits": {
            "api_requests_per_day": "unlimited",
            "regions": "unlimited",
        },
        "overage_rates": {},  # Custom
        "trial_days": 0,  # Contact sales
        "is_active": True,
        "is_public": True,
        "sort_order": 5,
    },
]


# =============================================================================
# SEED FUNCTIONS
# =============================================================================

def seed_regions(db: Session) -> list:
    """Seed region data."""
    created = []
    for region_data in REGIONS:
        existing = db.query(Region).filter(Region.code == region_data["code"]).first()
        if not existing:
            region = Region(id=uuid.uuid4(), **region_data)
            db.add(region)
            created.append(region)
            print(f"  Created region: {region.code}")
        else:
            print(f"  Region exists: {region_data['code']}")
    db.commit()
    return created


def seed_server_pools(db: Session) -> list:
    """Seed server pool templates for each region."""
    created = []
    regions = db.query(Region).filter(Region.status == RegionStatus.ACTIVE.value).all()

    for region in regions:
        for template in SERVER_POOL_TEMPLATES:
            existing = (
                db.query(ServerPool)
                .filter(
                    ServerPool.region_id == region.id,
                    ServerPool.tier_code == template["tier_code"],
                )
                .first()
            )
            if not existing:
                pool = ServerPool(
                    id=uuid.uuid4(),
                    region_id=region.id,
                    **template,
                )
                db.add(pool)
                created.append(pool)
                print(f"  Created pool: {region.code}/{template['tier_code']}")

    db.commit()
    return created


def seed_provider(db: Session) -> Provider:
    """Seed Omniphi Cloud provider."""
    existing = db.query(Provider).filter(Provider.code == OMNIPHI_PROVIDER["code"]).first()
    if existing:
        print(f"  Provider exists: {OMNIPHI_PROVIDER['code']}")
        return existing

    provider = Provider(id=uuid.uuid4(), **OMNIPHI_PROVIDER)
    db.add(provider)
    db.commit()
    print(f"  Created provider: {provider.code}")
    return provider


def seed_provider_pricing(db: Session, provider: Provider) -> list:
    """Seed provider pricing tiers."""
    created = []
    for tier_data in OMNIPHI_PRICING_TIERS:
        existing = (
            db.query(ProviderPricingTier)
            .filter(
                ProviderPricingTier.provider_id == provider.id,
                ProviderPricingTier.tier_code == tier_data["tier_code"],
            )
            .first()
        )
        if not existing:
            tier = ProviderPricingTier(
                id=uuid.uuid4(),
                provider_id=provider.id,
                **tier_data,
            )
            db.add(tier)
            created.append(tier)
            print(f"  Created pricing tier: {tier_data['tier_code']}")

    db.commit()
    return created


def seed_provider_sla(db: Session, provider: Provider) -> ProviderSLA:
    """Seed provider SLA."""
    existing = (
        db.query(ProviderSLA)
        .filter(
            ProviderSLA.provider_id == provider.id,
            ProviderSLA.is_active == True,
        )
        .first()
    )
    if existing:
        print(f"  SLA exists for provider: {provider.code}")
        return existing

    sla = ProviderSLA(
        id=uuid.uuid4(),
        provider_id=provider.id,
        **OMNIPHI_SLA,
    )
    db.add(sla)
    db.commit()
    print(f"  Created SLA for provider: {provider.code}")
    return sla


def seed_billing_plans(db: Session) -> list:
    """Seed billing plans."""
    created = []
    for plan_data in BILLING_PLANS:
        existing = db.query(BillingPlan).filter(BillingPlan.code == plan_data["code"]).first()
        if not existing:
            plan = BillingPlan(id=uuid.uuid4(), **plan_data)
            db.add(plan)
            created.append(plan)
            print(f"  Created billing plan: {plan_data['code']}")
        else:
            print(f"  Billing plan exists: {plan_data['code']}")

    db.commit()
    return created


def seed_all(db: Session = None) -> dict:
    """
    Run all seed functions.

    Returns:
        Dictionary with counts of created items.
    """
    close_session = False
    if db is None:
        db = SessionLocal()
        close_session = True

    try:
        print("\n=== Seeding Omniphi Cloud Database ===\n")

        print("1. Seeding Regions...")
        regions = seed_regions(db)

        print("\n2. Seeding Server Pools...")
        pools = seed_server_pools(db)

        print("\n3. Seeding Omniphi Cloud Provider...")
        provider = seed_provider(db)

        print("\n4. Seeding Provider Pricing Tiers...")
        tiers = seed_provider_pricing(db, provider)

        print("\n5. Seeding Provider SLA...")
        sla = seed_provider_sla(db, provider)

        print("\n6. Seeding Billing Plans...")
        plans = seed_billing_plans(db)

        print("\n=== Seeding Complete ===\n")

        return {
            "regions": len(regions),
            "server_pools": len(pools),
            "provider": 1 if provider else 0,
            "pricing_tiers": len(tiers),
            "sla": 1 if sla else 0,
            "billing_plans": len(plans),
        }

    finally:
        if close_session:
            db.close()


def reset_and_seed(db: Session = None) -> dict:
    """
    Drop all tables, recreate them, and seed data.

    WARNING: This will delete all existing data!
    """
    close_session = False
    if db is None:
        db = SessionLocal()
        close_session = True

    try:
        print("\n=== Resetting Database ===\n")
        print("Dropping all tables...")
        Base.metadata.drop_all(bind=engine)

        print("Creating all tables...")
        Base.metadata.create_all(bind=engine)

        return seed_all(db)

    finally:
        if close_session:
            db.close()


if __name__ == "__main__":
    import sys

    if "--reset" in sys.argv:
        print("WARNING: This will delete all existing data!")
        confirm = input("Type 'yes' to confirm: ")
        if confirm.lower() == "yes":
            result = reset_and_seed()
        else:
            print("Aborted.")
            sys.exit(0)
    else:
        result = seed_all()

    print(f"Created: {result}")
