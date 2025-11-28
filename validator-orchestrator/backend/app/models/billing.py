"""
Billing System Models

Database models for subscriptions, invoices, payment methods, and billing plans.
Supports Stripe and Coinbase Commerce payments.
"""

import enum
from datetime import datetime
from typing import Optional
from uuid import uuid4

from sqlalchemy import (
    Column,
    String,
    Integer,
    Float,
    Boolean,
    DateTime,
    ForeignKey,
    Enum,
    JSON,
    Text,
    Index,
)
from sqlalchemy.dialects.postgresql import UUID
from sqlalchemy.orm import relationship

from app.database import Base


class SubscriptionStatus(str, enum.Enum):
    """Subscription status"""
    ACTIVE = "active"
    PAST_DUE = "past_due"
    CANCELLED = "cancelled"
    SUSPENDED = "suspended"
    TRIALING = "trialing"
    EXPIRED = "expired"


class PaymentStatus(str, enum.Enum):
    """Payment status"""
    PENDING = "pending"
    PROCESSING = "processing"
    SUCCEEDED = "succeeded"
    FAILED = "failed"
    REFUNDED = "refunded"
    CANCELLED = "cancelled"


class InvoiceStatus(str, enum.Enum):
    """Invoice status"""
    DRAFT = "draft"
    OPEN = "open"
    PAID = "paid"
    OVERDUE = "overdue"
    VOID = "void"
    UNCOLLECTIBLE = "uncollectible"


class PaymentMethodType(str, enum.Enum):
    """Payment method types"""
    CARD = "card"
    BANK_ACCOUNT = "bank_account"
    CRYPTO = "crypto"


class BillingInterval(str, enum.Enum):
    """Billing interval"""
    MONTHLY = "monthly"
    QUARTERLY = "quarterly"
    YEARLY = "yearly"


class BillingPlan(Base):
    """
    Billing plan definition.

    Defines available subscription tiers with features and pricing.
    """
    __tablename__ = "billing_plans"

    id = Column(UUID(as_uuid=True), primary_key=True, default=uuid4)

    # Plan identification
    name = Column(String(100), nullable=False)
    code = Column(String(50), nullable=False, unique=True)  # e.g., "starter", "pro", "enterprise"
    description = Column(Text, nullable=True)

    # Pricing
    price_monthly = Column(Float, nullable=False)
    price_quarterly = Column(Float, nullable=True)
    price_yearly = Column(Float, nullable=True)
    currency = Column(String(3), nullable=False, default="USD")

    # Stripe integration
    stripe_price_id_monthly = Column(String(100), nullable=True)
    stripe_price_id_quarterly = Column(String(100), nullable=True)
    stripe_price_id_yearly = Column(String(100), nullable=True)
    stripe_product_id = Column(String(100), nullable=True)

    # Plan limits
    max_validators = Column(Integer, nullable=False, default=1)
    max_regions = Column(Integer, nullable=False, default=1)
    included_bandwidth_gb = Column(Integer, nullable=False, default=100)
    included_storage_gb = Column(Integer, nullable=False, default=50)

    # Features (JSON object)
    features = Column(JSON, nullable=False, default=dict)
    # e.g., {"priority_support": true, "auto_failover": true, "multi_region": false}

    # Status
    is_active = Column(Boolean, nullable=False, default=True)
    is_featured = Column(Boolean, nullable=False, default=False)
    sort_order = Column(Integer, nullable=False, default=0)

    # Metadata
    created_at = Column(DateTime, nullable=False, default=datetime.utcnow)
    updated_at = Column(DateTime, nullable=False, default=datetime.utcnow, onupdate=datetime.utcnow)

    # Relationships
    subscriptions = relationship("Subscription", back_populates="plan")

    def __repr__(self):
        return f"<BillingPlan {self.name} (${self.price_monthly}/mo)>"


class Subscription(Base):
    """
    User subscription record.

    Tracks active subscriptions and their payment status.
    """
    __tablename__ = "subscriptions"

    id = Column(UUID(as_uuid=True), primary_key=True, default=uuid4)
    user_id = Column(String(100), nullable=False, index=True)
    plan_id = Column(UUID(as_uuid=True), ForeignKey("billing_plans.id"), nullable=False)

    # Stripe integration
    stripe_customer_id = Column(String(100), nullable=True)
    stripe_subscription_id = Column(String(100), nullable=True, unique=True)

    # Billing details
    billing_interval = Column(Enum(BillingInterval), nullable=False, default=BillingInterval.MONTHLY)
    current_period_start = Column(DateTime, nullable=False)
    current_period_end = Column(DateTime, nullable=False)
    cancel_at_period_end = Column(Boolean, nullable=False, default=False)

    # Status
    status = Column(Enum(SubscriptionStatus), nullable=False, default=SubscriptionStatus.ACTIVE)

    # Trial period
    trial_start = Column(DateTime, nullable=True)
    trial_end = Column(DateTime, nullable=True)

    # Usage tracking
    validators_used = Column(Integer, nullable=False, default=0)
    bandwidth_used_gb = Column(Float, nullable=False, default=0.0)
    storage_used_gb = Column(Float, nullable=False, default=0.0)

    # Payment failure tracking
    failed_payment_count = Column(Integer, nullable=False, default=0)
    last_payment_failure = Column(DateTime, nullable=True)
    grace_period_end = Column(DateTime, nullable=True)

    # Node suspension
    nodes_suspended_at = Column(DateTime, nullable=True)
    suspended_node_ids = Column(JSON, nullable=True, default=list)

    # Metadata
    metadata = Column(JSON, nullable=True, default=dict)
    created_at = Column(DateTime, nullable=False, default=datetime.utcnow)
    updated_at = Column(DateTime, nullable=False, default=datetime.utcnow, onupdate=datetime.utcnow)
    cancelled_at = Column(DateTime, nullable=True)

    # Relationships
    plan = relationship("BillingPlan", back_populates="subscriptions")
    invoices = relationship("Invoice", back_populates="subscription", cascade="all, delete-orphan")
    payment_methods = relationship("PaymentMethod", back_populates="subscription")

    __table_args__ = (
        Index("ix_subscriptions_user_status", "user_id", "status"),
        Index("ix_subscriptions_stripe", "stripe_subscription_id"),
    )

    def __repr__(self):
        return f"<Subscription {self.user_id} ({self.status.value})>"

    @property
    def is_active(self) -> bool:
        """Check if subscription is active"""
        return self.status in [SubscriptionStatus.ACTIVE, SubscriptionStatus.TRIALING]

    @property
    def days_until_renewal(self) -> int:
        """Calculate days until renewal"""
        if self.current_period_end:
            delta = self.current_period_end - datetime.utcnow()
            return max(0, delta.days)
        return 0


class Invoice(Base):
    """
    Invoice record.

    Tracks billing invoices and their payment status.
    """
    __tablename__ = "invoices"

    id = Column(UUID(as_uuid=True), primary_key=True, default=uuid4)
    subscription_id = Column(UUID(as_uuid=True), ForeignKey("subscriptions.id", ondelete="SET NULL"), nullable=True)
    user_id = Column(String(100), nullable=False, index=True)

    # Invoice details
    invoice_number = Column(String(50), nullable=False, unique=True)
    description = Column(Text, nullable=True)

    # Amounts
    subtotal = Column(Float, nullable=False)
    tax = Column(Float, nullable=False, default=0.0)
    discount = Column(Float, nullable=False, default=0.0)
    total = Column(Float, nullable=False)
    currency = Column(String(3), nullable=False, default="USD")

    # Stripe integration
    stripe_invoice_id = Column(String(100), nullable=True, unique=True)
    stripe_payment_intent_id = Column(String(100), nullable=True)
    hosted_invoice_url = Column(String(500), nullable=True)
    pdf_url = Column(String(500), nullable=True)

    # Status
    status = Column(Enum(InvoiceStatus), nullable=False, default=InvoiceStatus.DRAFT)

    # Dates
    invoice_date = Column(DateTime, nullable=False, default=datetime.utcnow)
    due_date = Column(DateTime, nullable=True)
    paid_at = Column(DateTime, nullable=True)

    # Period covered
    period_start = Column(DateTime, nullable=True)
    period_end = Column(DateTime, nullable=True)

    # Line items (JSON array)
    line_items = Column(JSON, nullable=False, default=list)
    # e.g., [{"description": "Pro Plan - Monthly", "quantity": 1, "unit_price": 99.00, "amount": 99.00}]

    # Metadata
    metadata = Column(JSON, nullable=True, default=dict)
    created_at = Column(DateTime, nullable=False, default=datetime.utcnow)
    updated_at = Column(DateTime, nullable=False, default=datetime.utcnow, onupdate=datetime.utcnow)

    # Relationships
    subscription = relationship("Subscription", back_populates="invoices")
    payments = relationship("PaymentHistory", back_populates="invoice")

    __table_args__ = (
        Index("ix_invoices_user_status", "user_id", "status"),
        Index("ix_invoices_stripe", "stripe_invoice_id"),
    )

    def __repr__(self):
        return f"<Invoice {self.invoice_number} (${self.total} - {self.status.value})>"


class PaymentMethod(Base):
    """
    Payment method record.

    Stores user payment methods (cards, bank accounts, crypto wallets).
    """
    __tablename__ = "payment_methods"

    id = Column(UUID(as_uuid=True), primary_key=True, default=uuid4)
    subscription_id = Column(UUID(as_uuid=True), ForeignKey("subscriptions.id", ondelete="CASCADE"), nullable=False)
    user_id = Column(String(100), nullable=False, index=True)

    # Payment method type
    type = Column(Enum(PaymentMethodType), nullable=False)

    # Stripe integration
    stripe_payment_method_id = Column(String(100), nullable=True, unique=True)

    # Card details (if applicable)
    card_brand = Column(String(20), nullable=True)  # visa, mastercard, etc.
    card_last4 = Column(String(4), nullable=True)
    card_exp_month = Column(Integer, nullable=True)
    card_exp_year = Column(Integer, nullable=True)

    # Crypto wallet (if applicable)
    crypto_currency = Column(String(10), nullable=True)  # BTC, ETH, USDC
    crypto_address = Column(String(100), nullable=True)

    # Status
    is_default = Column(Boolean, nullable=False, default=False)
    is_active = Column(Boolean, nullable=False, default=True)

    # Metadata
    created_at = Column(DateTime, nullable=False, default=datetime.utcnow)
    updated_at = Column(DateTime, nullable=False, default=datetime.utcnow, onupdate=datetime.utcnow)

    # Relationships
    subscription = relationship("Subscription", back_populates="payment_methods")

    __table_args__ = (
        Index("ix_payment_methods_user", "user_id"),
    )

    def __repr__(self):
        return f"<PaymentMethod {self.type.value} (default={self.is_default})>"


class PaymentHistory(Base):
    """
    Payment transaction history.

    Records all payment attempts and their outcomes.
    """
    __tablename__ = "payment_history"

    id = Column(UUID(as_uuid=True), primary_key=True, default=uuid4)
    invoice_id = Column(UUID(as_uuid=True), ForeignKey("invoices.id", ondelete="SET NULL"), nullable=True)
    user_id = Column(String(100), nullable=False, index=True)

    # Payment details
    amount = Column(Float, nullable=False)
    currency = Column(String(3), nullable=False, default="USD")
    payment_method_type = Column(Enum(PaymentMethodType), nullable=False)

    # Status
    status = Column(Enum(PaymentStatus), nullable=False)

    # Stripe details
    stripe_payment_intent_id = Column(String(100), nullable=True)
    stripe_charge_id = Column(String(100), nullable=True)

    # Coinbase Commerce details
    coinbase_charge_id = Column(String(100), nullable=True)
    coinbase_code = Column(String(20), nullable=True)
    crypto_amount = Column(Float, nullable=True)
    crypto_currency = Column(String(10), nullable=True)

    # Error tracking
    error_code = Column(String(50), nullable=True)
    error_message = Column(Text, nullable=True)

    # Timestamps
    created_at = Column(DateTime, nullable=False, default=datetime.utcnow)
    processed_at = Column(DateTime, nullable=True)

    # Relationships
    invoice = relationship("Invoice", back_populates="payments")

    __table_args__ = (
        Index("ix_payment_history_user", "user_id"),
        Index("ix_payment_history_status", "status"),
    )

    def __repr__(self):
        return f"<PaymentHistory ${self.amount} ({self.status.value})>"


class CryptoPayment(Base):
    """
    Cryptocurrency payment record.

    Tracks crypto payments via Coinbase Commerce.
    """
    __tablename__ = "crypto_payments"

    id = Column(UUID(as_uuid=True), primary_key=True, default=uuid4)
    user_id = Column(String(100), nullable=False, index=True)
    invoice_id = Column(UUID(as_uuid=True), nullable=True)

    # Coinbase Commerce details
    coinbase_charge_id = Column(String(100), nullable=False, unique=True)
    coinbase_code = Column(String(20), nullable=False)
    checkout_url = Column(String(500), nullable=True)

    # Amount details
    fiat_amount = Column(Float, nullable=False)
    fiat_currency = Column(String(3), nullable=False, default="USD")
    crypto_amount = Column(Float, nullable=True)
    crypto_currency = Column(String(10), nullable=True)  # BTC, ETH, USDC, etc.

    # Exchange rate at time of payment
    exchange_rate = Column(Float, nullable=True)

    # Status
    status = Column(String(20), nullable=False, default="created")
    # created, pending, completed, expired, unresolved, resolved, cancelled

    # Blockchain details
    tx_hash = Column(String(100), nullable=True)
    block_height = Column(Integer, nullable=True)
    confirmations = Column(Integer, nullable=True)

    # Timestamps
    expires_at = Column(DateTime, nullable=True)
    confirmed_at = Column(DateTime, nullable=True)
    created_at = Column(DateTime, nullable=False, default=datetime.utcnow)
    updated_at = Column(DateTime, nullable=False, default=datetime.utcnow, onupdate=datetime.utcnow)

    __table_args__ = (
        Index("ix_crypto_payments_coinbase", "coinbase_charge_id"),
        Index("ix_crypto_payments_user", "user_id"),
    )

    def __repr__(self):
        return f"<CryptoPayment {self.coinbase_code} (${self.fiat_amount} - {self.status})>"
