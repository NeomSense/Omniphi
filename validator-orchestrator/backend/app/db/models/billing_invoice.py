"""
Billing Invoice Model

Invoice records for billing accounts.
Tracks invoice lifecycle, line items, and payment status.

Table: billing_invoices
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
from app.db.models.enums import InvoiceStatus

if TYPE_CHECKING:
    from app.db.models.billing_account import BillingAccount


class BillingInvoice(Base):
    """
    Billing invoice record.

    Represents an invoice for services rendered, including
    line items, taxes, and payment tracking.
    """

    __tablename__ = "billing_invoices"

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

    # Invoice identification
    invoice_number = Column(
        String(50),
        nullable=False,
        unique=True,
        index=True,
        doc="Human-readable invoice number"
    )

    # Status
    status = Column(
        String(50),
        nullable=False,
        default=InvoiceStatus.DRAFT.value,
        index=True
    )

    # Amounts
    subtotal = Column(
        Float,
        nullable=False,
        default=0.0,
        doc="Amount before tax"
    )
    tax_amount = Column(
        Float,
        nullable=False,
        default=0.0
    )
    tax_rate = Column(
        Float,
        nullable=False,
        default=0.0,
        doc="Tax rate percentage"
    )
    discount_amount = Column(
        Float,
        nullable=False,
        default=0.0
    )
    credit_applied = Column(
        Float,
        nullable=False,
        default=0.0,
        doc="Credits applied"
    )
    total = Column(
        Float,
        nullable=False,
        default=0.0,
        doc="Total amount due"
    )
    amount_paid = Column(
        Float,
        nullable=False,
        default=0.0
    )
    amount_due = Column(
        Float,
        nullable=False,
        default=0.0
    )
    currency = Column(
        String(3),
        nullable=False,
        default="USD"
    )

    # Line items
    line_items = Column(
        JSONB,
        nullable=False,
        default=list,
        doc="Invoice line items"
    )
    # Example: [{"description": "Validator - Medium", "quantity": 2, "unit_price": 50.0, "amount": 100.0}]

    # Billing period
    period_start = Column(
        DateTime,
        nullable=True
    )
    period_end = Column(
        DateTime,
        nullable=True
    )

    # Dates
    invoice_date = Column(
        DateTime,
        nullable=False,
        default=datetime.utcnow
    )
    due_date = Column(
        DateTime,
        nullable=False
    )
    paid_at = Column(
        DateTime,
        nullable=True
    )
    voided_at = Column(
        DateTime,
        nullable=True
    )

    # Payment info
    payment_method = Column(
        String(50),
        nullable=True
    )
    payment_reference = Column(
        String(255),
        nullable=True,
        doc="Payment transaction reference"
    )

    # Billing details (snapshot at invoice time)
    billing_name = Column(
        String(200),
        nullable=True
    )
    billing_email = Column(
        String(255),
        nullable=True
    )
    billing_address = Column(
        Text,
        nullable=True
    )

    # Stripe integration
    stripe_invoice_id = Column(
        String(100),
        nullable=True,
        unique=True,
        index=True
    )
    stripe_payment_intent_id = Column(
        String(100),
        nullable=True
    )
    hosted_invoice_url = Column(
        String(500),
        nullable=True,
        doc="URL to hosted invoice page"
    )
    invoice_pdf_url = Column(
        String(500),
        nullable=True,
        doc="URL to download PDF"
    )

    # Auto-charge settings
    auto_charge_attempted = Column(
        Boolean,
        nullable=False,
        default=False
    )
    auto_charge_attempts = Column(
        Integer,
        nullable=False,
        default=0
    )
    last_charge_attempt_at = Column(
        DateTime,
        nullable=True
    )
    last_charge_error = Column(
        Text,
        nullable=True
    )

    # Notes
    notes = Column(
        Text,
        nullable=True,
        doc="Internal notes"
    )
    memo = Column(
        Text,
        nullable=True,
        doc="Customer-facing memo"
    )

    # Extra data
    extra_data = Column(
        JSONB,
        nullable=False,
        default=dict
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
    finalized_at = Column(
        DateTime,
        nullable=True,
        doc="When invoice was finalized"
    )

    # Relationships
    account: Mapped["BillingAccount"] = relationship(
        "BillingAccount",
        back_populates="invoices"
    )

    # Indexes
    __table_args__ = (
        Index("ix_billing_invoices_account_status", "account_id", "status"),
        Index("ix_billing_invoices_due_date", "due_date"),
        Index("ix_billing_invoices_stripe", "stripe_invoice_id"),
    )

    def __repr__(self) -> str:
        return f"<BillingInvoice {self.invoice_number}: ${self.total}>"

    @property
    def is_paid(self) -> bool:
        """Check if invoice is fully paid."""
        return self.status == InvoiceStatus.PAID.value

    @property
    def is_overdue(self) -> bool:
        """Check if invoice is overdue."""
        if self.is_paid:
            return False
        return datetime.utcnow() > self.due_date

    @property
    def days_overdue(self) -> int:
        """Get number of days overdue."""
        if not self.is_overdue:
            return 0
        delta = datetime.utcnow() - self.due_date
        return delta.days

    @property
    def can_be_paid(self) -> bool:
        """Check if invoice can be paid."""
        return self.status in [InvoiceStatus.OPEN.value, InvoiceStatus.DRAFT.value]

    def add_line_item(
        self,
        description: str,
        quantity: float,
        unit_price: float,
        metadata: dict = None,
    ) -> None:
        """
        Add a line item to the invoice.

        Args:
            description: Item description
            quantity: Quantity
            unit_price: Price per unit
            metadata: Additional metadata
        """
        item = {
            "description": description,
            "quantity": quantity,
            "unit_price": unit_price,
            "amount": round(quantity * unit_price, 2),
            "metadata": metadata or {},
        }
        self.line_items = [*self.line_items, item]
        self.recalculate_totals()

    def recalculate_totals(self) -> None:
        """Recalculate invoice totals from line items."""
        self.subtotal = sum(item.get("amount", 0) for item in self.line_items)
        self.tax_amount = round(self.subtotal * (self.tax_rate / 100), 2)
        self.total = self.subtotal + self.tax_amount - self.discount_amount - self.credit_applied
        self.amount_due = max(0, self.total - self.amount_paid)

    def mark_paid(self, payment_reference: str = None, payment_method: str = None) -> None:
        """
        Mark invoice as paid.

        Args:
            payment_reference: Payment transaction reference
            payment_method: Payment method used
        """
        self.status = InvoiceStatus.PAID.value
        self.amount_paid = self.total
        self.amount_due = 0
        self.paid_at = datetime.utcnow()
        self.payment_reference = payment_reference
        self.payment_method = payment_method

    def void(self) -> None:
        """Void the invoice."""
        self.status = InvoiceStatus.VOID.value
        self.voided_at = datetime.utcnow()

    def finalize(self) -> None:
        """Finalize the invoice (make it ready for payment)."""
        self.status = InvoiceStatus.OPEN.value
        self.finalized_at = datetime.utcnow()
        self.recalculate_totals()
