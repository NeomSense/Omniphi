"""
Billing System API Endpoints

Endpoints for subscriptions, invoices, and payment processing.
Supports Stripe and Coinbase Commerce.
"""

from datetime import datetime, timedelta
from typing import List, Optional
from uuid import uuid4

from fastapi import APIRouter, HTTPException, Query, Request, Header
from pydantic import BaseModel, Field

router = APIRouter(prefix="/billing", tags=["billing"])


# ============================================
# SCHEMAS
# ============================================

class BillingPlanResponse(BaseModel):
    """Billing plan response"""
    id: str
    name: str
    code: str
    description: Optional[str]
    price_monthly: float
    price_quarterly: Optional[float]
    price_yearly: Optional[float]
    currency: str
    max_validators: int
    max_regions: int
    features: dict
    is_featured: bool


class SubscriptionResponse(BaseModel):
    """Subscription response"""
    id: str
    user_id: str
    plan: BillingPlanResponse
    billing_interval: str
    current_period_start: str
    current_period_end: str
    status: str
    validators_used: int
    cancel_at_period_end: bool
    days_until_renewal: int
    created_at: str


class InvoiceResponse(BaseModel):
    """Invoice response"""
    id: str
    invoice_number: str
    description: Optional[str]
    subtotal: float
    tax: float
    discount: float
    total: float
    currency: str
    status: str
    invoice_date: str
    due_date: Optional[str]
    paid_at: Optional[str]
    hosted_invoice_url: Optional[str]
    pdf_url: Optional[str]
    line_items: List[dict]


class PaymentMethodResponse(BaseModel):
    """Payment method response"""
    id: str
    type: str
    card_brand: Optional[str]
    card_last4: Optional[str]
    card_exp_month: Optional[int]
    card_exp_year: Optional[int]
    crypto_currency: Optional[str]
    is_default: bool
    is_active: bool


class CreateSubscriptionRequest(BaseModel):
    """Request to create a subscription"""
    plan_code: str
    billing_interval: str = "monthly"
    payment_method_id: Optional[str] = None  # Stripe payment method ID


class CreateCryptoPaymentRequest(BaseModel):
    """Request to create a crypto payment"""
    invoice_id: Optional[str] = None
    amount: float
    currency: str = "USD"


class CryptoPaymentResponse(BaseModel):
    """Crypto payment response"""
    id: str
    coinbase_charge_id: str
    coinbase_code: str
    checkout_url: str
    fiat_amount: float
    fiat_currency: str
    status: str
    expires_at: Optional[str]


# ============================================
# MOCK DATA
# ============================================

def get_mock_plans():
    """Generate mock billing plans."""
    return [
        {
            "id": "550e8400-e29b-41d4-a716-446655440001",
            "name": "Starter",
            "code": "starter",
            "description": "Perfect for getting started with a single validator",
            "price_monthly": 29.0,
            "price_quarterly": 79.0,
            "price_yearly": 290.0,
            "currency": "USD",
            "max_validators": 1,
            "max_regions": 1,
            "features": {
                "monitoring": True,
                "alerts": True,
                "support": "email",
                "uptime_sla": 99.0
            },
            "is_featured": False
        },
        {
            "id": "550e8400-e29b-41d4-a716-446655440002",
            "name": "Professional",
            "code": "professional",
            "description": "For serious validators with multiple nodes",
            "price_monthly": 89.0,
            "price_quarterly": 239.0,
            "price_yearly": 890.0,
            "currency": "USD",
            "max_validators": 5,
            "max_regions": 2,
            "features": {
                "monitoring": True,
                "alerts": True,
                "support": "priority",
                "uptime_sla": 99.9,
                "auto_failover": True
            },
            "is_featured": True
        },
        {
            "id": "550e8400-e29b-41d4-a716-446655440003",
            "name": "Enterprise",
            "code": "enterprise",
            "description": "For large-scale validator operations",
            "price_monthly": 299.0,
            "price_quarterly": 799.0,
            "price_yearly": 2990.0,
            "currency": "USD",
            "max_validators": 50,
            "max_regions": 4,
            "features": {
                "monitoring": True,
                "alerts": True,
                "support": "dedicated",
                "uptime_sla": 99.99,
                "auto_failover": True,
                "custom_sla": True,
                "api_access": True
            },
            "is_featured": False
        }
    ]


def get_mock_subscription(user_id: str):
    """Generate mock subscription for a user."""
    plans = get_mock_plans()
    now = datetime.utcnow()
    return {
        "id": "550e8400-e29b-41d4-a716-446655440010",
        "user_id": user_id,
        "plan": plans[1],  # Professional plan
        "billing_interval": "monthly",
        "current_period_start": (now - timedelta(days=15)).isoformat(),
        "current_period_end": (now + timedelta(days=15)).isoformat(),
        "status": "active",
        "validators_used": 3,
        "cancel_at_period_end": False,
        "days_until_renewal": 15,
        "created_at": (now - timedelta(days=45)).isoformat()
    }


def get_mock_invoices(user_id: str):
    """Generate mock invoices."""
    now = datetime.utcnow()
    return [
        {
            "id": "550e8400-e29b-41d4-a716-446655440020",
            "invoice_number": "INV-20241101-ABC123",
            "description": "Professional Plan - Monthly",
            "subtotal": 89.0,
            "tax": 0.0,
            "discount": 0.0,
            "total": 89.0,
            "currency": "USD",
            "status": "paid",
            "invoice_date": (now - timedelta(days=15)).isoformat(),
            "due_date": (now - timedelta(days=8)).isoformat(),
            "paid_at": (now - timedelta(days=14)).isoformat(),
            "hosted_invoice_url": "https://pay.stripe.com/invoice/inv_abc123",
            "pdf_url": "https://pay.stripe.com/invoice/inv_abc123/pdf",
            "line_items": [
                {
                    "description": "Professional Plan (Monthly)",
                    "quantity": 1,
                    "unit_price": 89.0,
                    "amount": 89.0
                }
            ]
        },
        {
            "id": "550e8400-e29b-41d4-a716-446655440021",
            "invoice_number": "INV-20241001-DEF456",
            "description": "Professional Plan - Monthly",
            "subtotal": 89.0,
            "tax": 0.0,
            "discount": 0.0,
            "total": 89.0,
            "currency": "USD",
            "status": "paid",
            "invoice_date": (now - timedelta(days=45)).isoformat(),
            "due_date": (now - timedelta(days=38)).isoformat(),
            "paid_at": (now - timedelta(days=44)).isoformat(),
            "hosted_invoice_url": "https://pay.stripe.com/invoice/inv_def456",
            "pdf_url": "https://pay.stripe.com/invoice/inv_def456/pdf",
            "line_items": [
                {
                    "description": "Professional Plan (Monthly)",
                    "quantity": 1,
                    "unit_price": 89.0,
                    "amount": 89.0
                }
            ]
        }
    ]


def get_mock_payment_methods(user_id: str):
    """Generate mock payment methods."""
    return [
        {
            "id": "550e8400-e29b-41d4-a716-446655440030",
            "type": "card",
            "card_brand": "visa",
            "card_last4": "4242",
            "card_exp_month": 12,
            "card_exp_year": 2025,
            "crypto_currency": None,
            "is_default": True,
            "is_active": True
        },
        {
            "id": "550e8400-e29b-41d4-a716-446655440031",
            "type": "crypto",
            "card_brand": None,
            "card_last4": None,
            "card_exp_month": None,
            "card_exp_year": None,
            "crypto_currency": "BTC",
            "is_default": False,
            "is_active": True
        }
    ]


# ============================================
# ENDPOINTS
# ============================================

@router.get("/plans", response_model=List[BillingPlanResponse])
async def list_billing_plans(
    active_only: bool = Query(True, description="Only show active plans"),
):
    """
    List available billing plans.

    Returns all subscription tiers with pricing and features.
    """
    return get_mock_plans()


@router.get("/plans/{plan_code}", response_model=BillingPlanResponse)
async def get_billing_plan(plan_code: str):
    """Get details for a specific billing plan."""
    plans = get_mock_plans()
    plan = next((p for p in plans if p["code"] == plan_code), None)

    if not plan:
        raise HTTPException(status_code=404, detail="Plan not found")

    return plan


@router.post("/subscribe", response_model=SubscriptionResponse)
async def create_subscription(
    request: CreateSubscriptionRequest,
    user_id: str = Query(..., description="User ID"),
):
    """
    Create a new subscription.

    Subscribes the user to the specified plan.
    """
    plans = get_mock_plans()
    plan = next((p for p in plans if p["code"] == request.plan_code), None)

    if not plan:
        raise HTTPException(status_code=404, detail="Plan not found or inactive")

    valid_intervals = ["monthly", "quarterly", "yearly"]
    if request.billing_interval not in valid_intervals:
        raise HTTPException(
            status_code=400,
            detail=f"Invalid billing interval. Must be one of: {valid_intervals}"
        )

    now = datetime.utcnow()
    if request.billing_interval == "monthly":
        period_end = now + timedelta(days=30)
    elif request.billing_interval == "quarterly":
        period_end = now + timedelta(days=90)
    else:
        period_end = now + timedelta(days=365)

    return {
        "id": f"550e8400-e29b-41d4-a716-{uuid4().hex[:12]}",
        "user_id": user_id,
        "plan": plan,
        "billing_interval": request.billing_interval,
        "current_period_start": now.isoformat(),
        "current_period_end": period_end.isoformat(),
        "status": "active",
        "validators_used": 0,
        "cancel_at_period_end": False,
        "days_until_renewal": (period_end - now).days,
        "created_at": now.isoformat()
    }


@router.get("/subscription", response_model=SubscriptionResponse)
async def get_subscription(
    user_id: str = Query(..., description="User ID"),
):
    """Get the current subscription for a user."""
    return get_mock_subscription(user_id)


@router.post("/cancel")
async def cancel_subscription(
    user_id: str = Query(..., description="User ID"),
    immediately: bool = Query(False, description="Cancel immediately vs at period end"),
):
    """
    Cancel a subscription.

    By default, cancels at the end of the current billing period.
    """
    subscription = get_mock_subscription(user_id)
    now = datetime.utcnow()

    return {
        "status": "cancelled" if immediately else "cancel_scheduled",
        "subscription_id": subscription["id"],
        "cancel_at": now.isoformat() if immediately else subscription["current_period_end"],
    }


@router.get("/invoices", response_model=List[InvoiceResponse])
async def list_invoices(
    user_id: str = Query(..., description="User ID"),
    status: Optional[str] = Query(None, description="Filter by status"),
    limit: int = Query(20, ge=1, le=100),
    offset: int = Query(0, ge=0),
):
    """List invoices for a user."""
    invoices = get_mock_invoices(user_id)

    if status:
        invoices = [i for i in invoices if i["status"] == status]

    return invoices[offset:offset + limit]


@router.get("/invoices/{invoice_id}", response_model=InvoiceResponse)
async def get_invoice(invoice_id: str):
    """Get details for a specific invoice."""
    invoices = get_mock_invoices("mock-user")
    invoice = next((i for i in invoices if i["id"] == invoice_id), None)

    if not invoice:
        raise HTTPException(status_code=404, detail="Invoice not found")

    return invoice


@router.post("/pay-crypto", response_model=CryptoPaymentResponse)
async def create_crypto_payment(
    request: CreateCryptoPaymentRequest,
    user_id: str = Query(..., description="User ID"),
):
    """
    Create a cryptocurrency payment via Coinbase Commerce.

    Returns a checkout URL for the user to complete payment.
    """
    charge_id = f"charge_{uuid4().hex[:24]}"
    code = uuid4().hex[:8].upper()
    checkout_url = f"https://commerce.coinbase.com/checkout/{code}"

    return {
        "id": f"550e8400-e29b-41d4-a716-{uuid4().hex[:12]}",
        "coinbase_charge_id": charge_id,
        "coinbase_code": code,
        "checkout_url": checkout_url,
        "fiat_amount": request.amount,
        "fiat_currency": request.currency,
        "status": "created",
        "expires_at": (datetime.utcnow() + timedelta(hours=1)).isoformat()
    }


@router.post("/webhooks/stripe")
async def stripe_webhook(
    request: Request,
    stripe_signature: str = Header(None, alias="stripe-signature"),
):
    """
    Handle Stripe webhook events.

    Processes payment successes, failures, and subscription updates.
    """
    try:
        event = await request.json()
    except Exception:
        raise HTTPException(status_code=400, detail="Invalid payload")

    event_type = event.get("type", "")

    # In production, verify webhook signature and process events
    # For mock, just acknowledge receipt
    return {"received": True, "event_type": event_type}


@router.post("/webhooks/coinbase")
async def coinbase_webhook(request: Request):
    """
    Handle Coinbase Commerce webhook events.

    Processes cryptocurrency payment confirmations.
    """
    try:
        event = await request.json()
    except Exception:
        raise HTTPException(status_code=400, detail="Invalid payload")

    event_type = event.get("event", {}).get("type", "")

    # In production, process Coinbase events
    # For mock, just acknowledge receipt
    return {"received": True, "event_type": event_type}


@router.get("/payment-methods", response_model=List[PaymentMethodResponse])
async def list_payment_methods(
    user_id: str = Query(..., description="User ID"),
):
    """List payment methods for a user."""
    return get_mock_payment_methods(user_id)
