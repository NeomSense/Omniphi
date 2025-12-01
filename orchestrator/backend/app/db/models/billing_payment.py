"""
Billing Payment Model

Payment transaction records for billing accounts.
Tracks all payment attempts and their outcomes.

Table: billing_payments
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
from app.db.models.enums import PaymentMethod, PaymentStatus

if TYPE_CHECKING:
    from app.db.models.billing_account import BillingAccount


class BillingPayment(Base):
    """
    Payment transaction record.

    Tracks individual payment transactions including
    card payments, crypto payments, and refunds.
    """

    __tablename__ = "billing_payments"

    # Primary key
    id = Column(
        UUID(as_uuid=True),
        primary_key=True,
        default=uuid.uuid4,
        index=True
    )

    # Foreign key
    account_id = Column(
        UUID(as_uuid=True),
        ForeignKey("billing_accounts.id", ondelete="CASCADE"),
        nullable=False,
        index=True
    )

    # Related invoice (optional)
    invoice_id = Column(
        UUID(as_uuid=True),
        ForeignKey("billing_invoices.id", ondelete="SET NULL"),
        nullable=True,
        index=True
    )

    # Payment identification
    payment_reference = Column(
        String(100),
        nullable=False,
        unique=True,
        index=True,
        doc="Internal payment reference"
    )
    external_reference = Column(
        String(255),
        nullable=True,
        doc="External payment reference (Stripe, blockchain tx)"
    )

    # Payment method
    payment_method = Column(
        String(50),
        nullable=False,
        default=PaymentMethod.STRIPE.value
    )
    payment_method_details = Column(
        JSONB,
        nullable=False,
        default=dict,
        doc="Payment method details"
    )
    # Example for card: {"brand": "visa", "last4": "4242", "exp_month": 12, "exp_year": 2025}
    # Example for crypto: {"network": "ethereum", "address": "0x...", "tx_hash": "0x..."}

    # Amounts
    amount = Column(
        Float,
        nullable=False,
        doc="Payment amount"
    )
    currency = Column(
        String(3),
        nullable=False,
        default="USD"
    )
    fee = Column(
        Float,
        nullable=False,
        default=0.0,
        doc="Processing fee"
    )
    net_amount = Column(
        Float,
        nullable=False,
        doc="Amount after fees"
    )

    # Crypto-specific
    crypto_amount = Column(
        Float,
        nullable=True,
        doc="Amount in cryptocurrency"
    )
    crypto_currency = Column(
        String(20),
        nullable=True
    )
    exchange_rate = Column(
        Float,
        nullable=True,
        doc="Exchange rate used"
    )
    blockchain_tx_hash = Column(
        String(255),
        nullable=True,
        index=True
    )
    blockchain_confirmations = Column(
        Integer,
        nullable=True
    )

    # Status
    status = Column(
        String(50),
        nullable=False,
        default=PaymentStatus.PENDING.value,
        index=True
    )
    failure_code = Column(
        String(100),
        nullable=True
    )
    failure_message = Column(
        Text,
        nullable=True
    )

    # Refund tracking
    is_refund = Column(
        Boolean,
        nullable=False,
        default=False,
        doc="True if this is a refund"
    )
    original_payment_id = Column(
        UUID(as_uuid=True),
        ForeignKey("billing_payments.id", ondelete="SET NULL"),
        nullable=True,
        doc="Original payment if refund"
    )
    refunded_amount = Column(
        Float,
        nullable=False,
        default=0.0,
        doc="Amount refunded from this payment"
    )
    refund_reason = Column(
        Text,
        nullable=True
    )

    # Stripe integration
    stripe_payment_intent_id = Column(
        String(100),
        nullable=True,
        index=True
    )
    stripe_charge_id = Column(
        String(100),
        nullable=True
    )
    stripe_refund_id = Column(
        String(100),
        nullable=True
    )

    # Risk and fraud
    risk_score = Column(
        Float,
        nullable=True,
        doc="Fraud risk score"
    )
    risk_level = Column(
        String(20),
        nullable=True,
        doc="Risk level (low, medium, high)"
    )

    # Metadata
    description = Column(
        Text,
        nullable=True
    )
    extra_data = Column(
        JSONB,
        nullable=False,
        default=dict
    )
    ip_address = Column(
        String(45),
        nullable=True
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
    completed_at = Column(
        DateTime,
        nullable=True
    )
    failed_at = Column(
        DateTime,
        nullable=True
    )

    # Relationships
    account: Mapped["BillingAccount"] = relationship(
        "BillingAccount",
        back_populates="payments"
    )

    # Indexes
    __table_args__ = (
        Index("ix_billing_payments_account_status", "account_id", "status"),
        Index("ix_billing_payments_created", "created_at"),
        Index("ix_billing_payments_stripe", "stripe_payment_intent_id"),
        Index("ix_billing_payments_blockchain", "blockchain_tx_hash"),
    )

    def __repr__(self) -> str:
        return f"<BillingPayment {self.payment_reference}: ${self.amount} ({self.status})>"

    @property
    def is_successful(self) -> bool:
        """Check if payment succeeded."""
        return self.status == PaymentStatus.SUCCEEDED.value

    @property
    def is_failed(self) -> bool:
        """Check if payment failed."""
        return self.status == PaymentStatus.FAILED.value

    @property
    def is_pending(self) -> bool:
        """Check if payment is pending."""
        return self.status in [PaymentStatus.PENDING.value, PaymentStatus.PROCESSING.value]

    @property
    def is_crypto(self) -> bool:
        """Check if this is a crypto payment."""
        return self.payment_method in [
            PaymentMethod.CRYPTO_ETH.value,
            PaymentMethod.CRYPTO_BTC.value,
            PaymentMethod.CRYPTO_USDC.value,
            PaymentMethod.CRYPTO_OMNI.value,
        ]

    @property
    def is_refundable(self) -> bool:
        """Check if payment can be refunded."""
        return (
            self.is_successful and
            not self.is_refund and
            self.refunded_amount < self.amount
        )

    @property
    def refundable_amount(self) -> float:
        """Get remaining refundable amount."""
        return max(0, self.amount - self.refunded_amount)

    def mark_succeeded(self, external_reference: str = None) -> None:
        """
        Mark payment as succeeded.

        Args:
            external_reference: External reference (Stripe charge ID, tx hash)
        """
        self.status = PaymentStatus.SUCCEEDED.value
        self.completed_at = datetime.utcnow()
        if external_reference:
            self.external_reference = external_reference

    def mark_failed(self, code: str = None, message: str = None) -> None:
        """
        Mark payment as failed.

        Args:
            code: Failure code
            message: Failure message
        """
        self.status = PaymentStatus.FAILED.value
        self.failed_at = datetime.utcnow()
        self.failure_code = code
        self.failure_message = message

    def record_refund(self, amount: float, reason: str = None) -> None:
        """
        Record a refund against this payment.

        Args:
            amount: Refund amount
            reason: Refund reason
        """
        if amount > self.refundable_amount:
            raise ValueError(f"Cannot refund more than {self.refundable_amount}")

        self.refunded_amount += amount
        self.refund_reason = reason

        if self.refunded_amount >= self.amount:
            self.status = PaymentStatus.REFUNDED.value
        else:
            self.status = PaymentStatus.PARTIALLY_REFUNDED.value


# Import Integer for confirmations
from sqlalchemy import Integer  # noqa: E402
