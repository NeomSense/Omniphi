"""
Billing Account Model

Customer billing account for tracking payment info and billing history.
Supports both Stripe and crypto payments.

Table: billing_accounts
"""

import uuid
from datetime import datetime
from typing import List, Optional, TYPE_CHECKING

from sqlalchemy import (
    Column,
    String,
    Integer,
    Float,
    Boolean,
    DateTime,
    Text,
    Index,
)
from sqlalchemy.dialects.postgresql import UUID, JSONB
from sqlalchemy.orm import relationship, Mapped

from app.db.database import Base

if TYPE_CHECKING:
    from app.db.models.billing_subscription import BillingSubscription
    from app.db.models.billing_invoice import BillingInvoice
    from app.db.models.billing_payment import BillingPayment


class BillingAccount(Base):
    """
    Customer billing account.

    Central record for all billing-related information including
    payment methods, subscriptions, and billing history.
    """

    __tablename__ = "billing_accounts"

    # Primary key
    id = Column(
        UUID(as_uuid=True),
        primary_key=True,
        default=uuid.uuid4,
        index=True
    )

    # Customer identification
    customer_id = Column(
        String(100),
        nullable=False,
        unique=True,
        index=True,
        doc="Internal customer ID"
    )
    wallet_address = Column(
        String(100),
        nullable=True,
        index=True,
        doc="Primary wallet address"
    )
    email = Column(
        String(255),
        nullable=True,
        index=True,
        doc="Billing email"
    )

    # Customer details
    name = Column(
        String(200),
        nullable=True,
        doc="Customer/organization name"
    )
    company = Column(
        String(200),
        nullable=True,
        doc="Company name"
    )
    tax_id = Column(
        String(100),
        nullable=True,
        doc="Tax ID/VAT number"
    )

    # Billing address
    billing_address_line1 = Column(
        String(255),
        nullable=True
    )
    billing_address_line2 = Column(
        String(255),
        nullable=True
    )
    billing_city = Column(
        String(100),
        nullable=True
    )
    billing_state = Column(
        String(100),
        nullable=True
    )
    billing_postal_code = Column(
        String(20),
        nullable=True
    )
    billing_country = Column(
        String(2),
        nullable=True,
        doc="ISO country code"
    )

    # Stripe integration
    stripe_customer_id = Column(
        String(100),
        nullable=True,
        unique=True,
        index=True,
        doc="Stripe customer ID"
    )
    stripe_payment_method_id = Column(
        String(100),
        nullable=True,
        doc="Default Stripe payment method"
    )

    # Crypto payment info
    crypto_payment_address = Column(
        String(255),
        nullable=True,
        doc="Crypto payment address"
    )
    preferred_crypto = Column(
        String(20),
        nullable=True,
        doc="Preferred cryptocurrency"
    )

    # Account balance and credits
    balance = Column(
        Float,
        nullable=False,
        default=0.0,
        doc="Current account balance (negative = owed)"
    )
    credits = Column(
        Float,
        nullable=False,
        default=0.0,
        doc="Available credits"
    )
    currency = Column(
        String(3),
        nullable=False,
        default="USD",
        doc="Account currency"
    )

    # Spending
    total_spent = Column(
        Float,
        nullable=False,
        default=0.0,
        doc="Total amount spent"
    )
    monthly_spending_limit = Column(
        Float,
        nullable=True,
        doc="Optional monthly spending limit"
    )
    current_month_spent = Column(
        Float,
        nullable=False,
        default=0.0,
        doc="Spending this month"
    )

    # Account status
    is_active = Column(
        Boolean,
        nullable=False,
        default=True,
        index=True
    )
    is_verified = Column(
        Boolean,
        nullable=False,
        default=False
    )
    is_delinquent = Column(
        Boolean,
        nullable=False,
        default=False,
        index=True,
        doc="Has overdue payments"
    )
    delinquent_since = Column(
        DateTime,
        nullable=True
    )

    # Billing preferences
    billing_cycle_day = Column(
        Integer,
        nullable=False,
        default=1,
        doc="Day of month for billing (1-28)"
    )
    auto_pay_enabled = Column(
        Boolean,
        nullable=False,
        default=True
    )
    invoice_email_enabled = Column(
        Boolean,
        nullable=False,
        default=True
    )
    payment_reminder_enabled = Column(
        Boolean,
        nullable=False,
        default=True
    )

    # Extra data
    extra_data = Column(
        JSONB,
        nullable=False,
        default=dict
    )
    notes = Column(
        Text,
        nullable=True,
        doc="Internal notes"
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
    verified_at = Column(
        DateTime,
        nullable=True
    )

    # Relationships
    subscriptions: Mapped[List["BillingSubscription"]] = relationship(
        "BillingSubscription",
        back_populates="account",
        cascade="all, delete-orphan",
        lazy="selectin"
    )
    invoices: Mapped[List["BillingInvoice"]] = relationship(
        "BillingInvoice",
        back_populates="account",
        cascade="all, delete-orphan",
        lazy="dynamic"
    )
    payments: Mapped[List["BillingPayment"]] = relationship(
        "BillingPayment",
        back_populates="account",
        cascade="all, delete-orphan",
        lazy="dynamic"
    )

    # Indexes
    __table_args__ = (
        Index("ix_billing_accounts_status", "is_active", "is_delinquent"),
        Index("ix_billing_accounts_stripe", "stripe_customer_id"),
    )

    def __repr__(self) -> str:
        return f"<BillingAccount {self.customer_id}>"

    @property
    def full_address(self) -> str:
        """Get formatted billing address."""
        parts = [
            self.billing_address_line1,
            self.billing_address_line2,
            f"{self.billing_city}, {self.billing_state} {self.billing_postal_code}",
            self.billing_country,
        ]
        return "\n".join(p for p in parts if p)

    @property
    def has_payment_method(self) -> bool:
        """Check if account has payment method."""
        return bool(self.stripe_payment_method_id or self.crypto_payment_address)

    @property
    def available_balance(self) -> float:
        """Get available balance (balance + credits)."""
        return self.balance + self.credits

    @property
    def active_subscriptions(self) -> List["BillingSubscription"]:
        """Get active subscriptions."""
        if not self.subscriptions:
            return []
        return [s for s in self.subscriptions if s.is_active]

    def add_credit(self, amount: float, reason: str = None) -> None:
        """
        Add credits to account.

        Args:
            amount: Credit amount
            reason: Reason for credit
        """
        self.credits += amount

    def use_credit(self, amount: float) -> float:
        """
        Use credits from account.

        Args:
            amount: Amount to use

        Returns:
            Amount of credits actually used
        """
        used = min(amount, self.credits)
        self.credits -= used
        return used

    def charge(self, amount: float) -> None:
        """
        Charge account (reduce balance).

        Args:
            amount: Amount to charge
        """
        self.balance -= amount
        self.total_spent += amount
        self.current_month_spent += amount

    def pay(self, amount: float) -> None:
        """
        Record payment (increase balance).

        Args:
            amount: Payment amount
        """
        self.balance += amount
        if self.balance >= 0 and self.is_delinquent:
            self.is_delinquent = False
            self.delinquent_since = None

    def mark_delinquent(self) -> None:
        """Mark account as delinquent."""
        if not self.is_delinquent:
            self.is_delinquent = True
            self.delinquent_since = datetime.utcnow()

    def reset_monthly_spending(self) -> None:
        """Reset monthly spending counter."""
        self.current_month_spent = 0.0
