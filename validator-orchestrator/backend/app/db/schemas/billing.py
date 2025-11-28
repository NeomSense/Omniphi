"""
Billing Pydantic Schemas

Schemas for billing accounts, plans, subscriptions, invoices, and payments.
"""

from datetime import datetime
from typing import Any, Dict, List, Optional
from uuid import UUID

from pydantic import Field, field_validator

from app.db.schemas.base import BaseSchema, PaginatedResponse


# =============================================================================
# BILLING ACCOUNT SCHEMAS
# =============================================================================

class BillingAccountBase(BaseSchema):
    """Base schema for billing account."""

    customer_id: str = Field(..., min_length=1, max_length=100, description="Customer ID")
    email: Optional[str] = Field(None, max_length=255, description="Billing email")
    name: Optional[str] = Field(None, max_length=200, description="Customer name")


class BillingAccountCreate(BillingAccountBase):
    """Schema for creating a billing account."""

    wallet_address: Optional[str] = Field(None, max_length=100)
    company: Optional[str] = Field(None, max_length=200)
    tax_id: Optional[str] = Field(None, max_length=100)
    billing_address_line1: Optional[str] = Field(None, max_length=255)
    billing_address_line2: Optional[str] = Field(None, max_length=255)
    billing_city: Optional[str] = Field(None, max_length=100)
    billing_state: Optional[str] = Field(None, max_length=100)
    billing_postal_code: Optional[str] = Field(None, max_length=20)
    billing_country: Optional[str] = Field(None, max_length=2)
    currency: str = Field("USD", max_length=3)
    billing_cycle_day: int = Field(1, ge=1, le=28)
    auto_pay_enabled: bool = Field(True)


class BillingAccountUpdate(BaseSchema):
    """Schema for updating a billing account."""

    email: Optional[str] = Field(None, max_length=255)
    name: Optional[str] = Field(None, max_length=200)
    company: Optional[str] = Field(None, max_length=200)
    tax_id: Optional[str] = Field(None, max_length=100)
    billing_address_line1: Optional[str] = Field(None, max_length=255)
    billing_address_line2: Optional[str] = Field(None, max_length=255)
    billing_city: Optional[str] = Field(None, max_length=100)
    billing_state: Optional[str] = Field(None, max_length=100)
    billing_postal_code: Optional[str] = Field(None, max_length=20)
    billing_country: Optional[str] = Field(None, max_length=2)
    billing_cycle_day: Optional[int] = Field(None, ge=1, le=28)
    auto_pay_enabled: Optional[bool] = None
    monthly_spending_limit: Optional[float] = Field(None, ge=0)


class BillingAccountResponse(BillingAccountBase):
    """Schema for billing account response."""

    id: UUID
    wallet_address: Optional[str]
    company: Optional[str]
    tax_id: Optional[str]
    billing_address_line1: Optional[str]
    billing_address_line2: Optional[str]
    billing_city: Optional[str]
    billing_state: Optional[str]
    billing_postal_code: Optional[str]
    billing_country: Optional[str]
    stripe_customer_id: Optional[str]
    stripe_payment_method_id: Optional[str]
    crypto_payment_address: Optional[str]
    preferred_crypto: Optional[str]
    balance: float
    credits: float
    currency: str
    total_spent: float
    monthly_spending_limit: Optional[float]
    current_month_spent: float
    is_active: bool
    is_verified: bool
    is_delinquent: bool
    delinquent_since: Optional[datetime]
    billing_cycle_day: int
    auto_pay_enabled: bool
    created_at: datetime
    updated_at: datetime

    # Computed
    has_payment_method: bool
    available_balance: float


# =============================================================================
# BILLING PLAN SCHEMAS
# =============================================================================

class BillingPlanBase(BaseSchema):
    """Base schema for billing plan."""

    code: str = Field(..., min_length=2, max_length=50, description="Plan code")
    name: str = Field(..., min_length=2, max_length=100, description="Plan name")
    description: Optional[str] = Field(None, description="Plan description")


class BillingPlanCreate(BillingPlanBase):
    """Schema for creating a billing plan."""

    tagline: Optional[str] = Field(None, max_length=200)
    plan_type: str = Field("starter")
    tier_level: int = Field(1, ge=0)
    display_order: int = Field(0, ge=0)
    price_monthly: float = Field(0.0, ge=0)
    price_yearly: Optional[float] = Field(None, ge=0)
    currency: str = Field("USD", max_length=3)
    setup_fee: float = Field(0.0, ge=0)
    max_validators: Optional[int] = Field(None, ge=0)
    max_regions: Optional[int] = Field(None, ge=0)
    max_team_members: Optional[int] = Field(None, ge=0)
    cpu_cores_per_validator: int = Field(2, ge=1)
    memory_gb_per_validator: int = Field(4, ge=1)
    disk_gb_per_validator: int = Field(100, ge=10)
    included_validators: int = Field(1, ge=0)
    price_per_additional_validator: Optional[float] = Field(None, ge=0)
    features: Dict[str, bool] = Field(default_factory=dict)
    feature_list: List[str] = Field(default_factory=list)
    support_level: str = Field("standard")
    trial_days: int = Field(0, ge=0)
    is_active: bool = Field(True)
    is_public: bool = Field(True)


class BillingPlanUpdate(BaseSchema):
    """Schema for updating a billing plan."""

    name: Optional[str] = Field(None, min_length=2, max_length=100)
    description: Optional[str] = None
    tagline: Optional[str] = Field(None, max_length=200)
    price_monthly: Optional[float] = Field(None, ge=0)
    price_yearly: Optional[float] = Field(None, ge=0)
    is_active: Optional[bool] = None
    is_public: Optional[bool] = None
    is_featured: Optional[bool] = None
    features: Optional[Dict[str, bool]] = None
    feature_list: Optional[List[str]] = None


class BillingPlanResponse(BillingPlanBase):
    """Schema for billing plan response."""

    id: UUID
    tagline: Optional[str]
    plan_type: str
    tier_level: int
    display_order: int
    price_monthly: float
    price_yearly: Optional[float]
    price_hourly: Optional[float]
    currency: str
    setup_fee: float
    price_monthly_crypto: Optional[float]
    crypto_currency: Optional[str]
    billing_cycles: List[str]
    default_cycle: str
    max_validators: Optional[int]
    max_regions: Optional[int]
    max_team_members: Optional[int]
    max_api_requests_per_day: Optional[int]
    cpu_cores_per_validator: int
    memory_gb_per_validator: int
    disk_gb_per_validator: int
    bandwidth_tb_per_month: Optional[float]
    included_validators: int
    price_per_additional_validator: Optional[float]
    features: Dict[str, bool]
    feature_list: List[str]
    support_level: str
    support_response_hours: int
    trial_days: int
    trial_validators: Optional[int]
    is_active: bool
    is_public: bool
    is_featured: bool
    is_legacy: bool
    stripe_product_id: Optional[str]
    stripe_price_id_monthly: Optional[str]
    stripe_price_id_yearly: Optional[str]
    created_at: datetime
    updated_at: datetime

    # Computed
    is_free: bool
    yearly_savings: float
    yearly_savings_percent: float
    has_trial: bool


# =============================================================================
# BILLING SUBSCRIPTION SCHEMAS
# =============================================================================

class BillingSubscriptionCreate(BaseSchema):
    """Schema for creating a subscription."""

    account_id: UUID
    plan_id: UUID
    billing_cycle: str = Field("monthly", description="monthly or yearly")
    quantity: int = Field(1, ge=1, description="Number of validators")
    coupon_code: Optional[str] = Field(None, max_length=50)
    start_trial: bool = Field(False)


class BillingSubscriptionUpdate(BaseSchema):
    """Schema for updating a subscription."""

    plan_id: Optional[UUID] = None
    quantity: Optional[int] = Field(None, ge=1)
    billing_cycle: Optional[str] = None
    cancel_at_period_end: Optional[bool] = None


class BillingSubscriptionResponse(BaseSchema):
    """Schema for subscription response."""

    id: UUID
    account_id: UUID
    plan_id: UUID
    subscription_number: str
    status: str
    is_active: bool
    billing_cycle: str
    billing_anchor_day: int
    price_at_signup: float
    currency: str
    discount_percent: float
    discount_amount: float
    coupon_code: Optional[str]
    quantity: int
    max_quantity: Optional[int]
    current_period_start: datetime
    current_period_end: datetime
    next_billing_date: Optional[datetime]
    trial_start: Optional[datetime]
    trial_end: Optional[datetime]
    is_trial: bool
    cancel_at_period_end: bool
    cancelled_at: Optional[datetime]
    cancellation_reason: Optional[str]
    is_paused: bool
    paused_at: Optional[datetime]
    resume_at: Optional[datetime]
    stripe_subscription_id: Optional[str]
    usage_this_period: Dict[str, Any]
    created_at: datetime
    updated_at: datetime
    ended_at: Optional[datetime]

    # Computed
    is_in_trial: bool
    trial_days_remaining: int
    days_until_renewal: int
    effective_price: float
    total_price: float


# =============================================================================
# BILLING INVOICE SCHEMAS
# =============================================================================

class BillingInvoiceResponse(BaseSchema):
    """Schema for invoice response."""

    id: UUID
    account_id: UUID
    invoice_number: str
    status: str
    subtotal: float
    tax_amount: float
    tax_rate: float
    discount_amount: float
    credit_applied: float
    total: float
    amount_paid: float
    amount_due: float
    currency: str
    line_items: List[Dict[str, Any]]
    period_start: Optional[datetime]
    period_end: Optional[datetime]
    invoice_date: datetime
    due_date: datetime
    paid_at: Optional[datetime]
    voided_at: Optional[datetime]
    payment_method: Optional[str]
    payment_reference: Optional[str]
    billing_name: Optional[str]
    billing_email: Optional[str]
    billing_address: Optional[str]
    stripe_invoice_id: Optional[str]
    hosted_invoice_url: Optional[str]
    invoice_pdf_url: Optional[str]
    memo: Optional[str]
    created_at: datetime
    finalized_at: Optional[datetime]

    # Computed
    is_paid: bool
    is_overdue: bool
    days_overdue: int


# =============================================================================
# BILLING PAYMENT SCHEMAS
# =============================================================================

class BillingPaymentCreate(BaseSchema):
    """Schema for creating a payment."""

    account_id: UUID
    invoice_id: Optional[UUID] = None
    payment_method: str = Field(..., description="Payment method")
    amount: float = Field(..., gt=0, description="Payment amount")
    currency: str = Field("USD", max_length=3)
    description: Optional[str] = None

    # Crypto payment fields
    crypto_amount: Optional[float] = Field(None, gt=0)
    crypto_currency: Optional[str] = Field(None, max_length=20)
    blockchain_tx_hash: Optional[str] = Field(None, max_length=255)


class BillingPaymentResponse(BaseSchema):
    """Schema for payment response."""

    id: UUID
    account_id: UUID
    invoice_id: Optional[UUID]
    payment_reference: str
    external_reference: Optional[str]
    payment_method: str
    payment_method_details: Dict[str, Any]
    amount: float
    currency: str
    fee: float
    net_amount: float
    crypto_amount: Optional[float]
    crypto_currency: Optional[str]
    exchange_rate: Optional[float]
    blockchain_tx_hash: Optional[str]
    blockchain_confirmations: Optional[int]
    status: str
    failure_code: Optional[str]
    failure_message: Optional[str]
    is_refund: bool
    original_payment_id: Optional[UUID]
    refunded_amount: float
    refund_reason: Optional[str]
    stripe_payment_intent_id: Optional[str]
    stripe_charge_id: Optional[str]
    risk_score: Optional[float]
    risk_level: Optional[str]
    description: Optional[str]
    created_at: datetime
    completed_at: Optional[datetime]
    failed_at: Optional[datetime]

    # Computed
    is_successful: bool
    is_failed: bool
    is_pending: bool
    is_crypto: bool
    is_refundable: bool
    refundable_amount: float
