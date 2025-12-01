"""
Billing CRUD Repositories

Repositories for billing accounts, plans, subscriptions, invoices, and payments.
"""

from datetime import datetime, timedelta
from typing import List, Optional
from uuid import UUID

from sqlalchemy import and_, desc, func, or_
from sqlalchemy.orm import Session

from app.db.crud.base import BaseRepository
from app.db.models.billing_account import BillingAccount
from app.db.models.billing_plan import BillingPlan
from app.db.models.billing_subscription import BillingSubscription
from app.db.models.billing_invoice import BillingInvoice
from app.db.models.billing_payment import BillingPayment
from app.db.models.billing_usage import BillingUsage
from app.db.models.enums import (
    SubscriptionStatus,
    InvoiceStatus,
    PaymentStatus,
    BillingPlanType,
)


class BillingAccountRepository(BaseRepository[BillingAccount]):
    """Repository for BillingAccount model operations."""

    def __init__(self, db: Session):
        super().__init__(BillingAccount, db)

    def get_by_wallet(self, wallet_address: str) -> Optional[BillingAccount]:
        """Get billing account by wallet address."""
        return (
            self.db.query(BillingAccount)
            .filter(BillingAccount.wallet_address == wallet_address)
            .first()
        )

    def get_by_stripe_customer(self, stripe_customer_id: str) -> Optional[BillingAccount]:
        """Get billing account by Stripe customer ID."""
        return (
            self.db.query(BillingAccount)
            .filter(BillingAccount.stripe_customer_id == stripe_customer_id)
            .first()
        )

    def get_by_email(self, email: str) -> Optional[BillingAccount]:
        """Get billing account by email."""
        return (
            self.db.query(BillingAccount)
            .filter(BillingAccount.billing_email == email)
            .first()
        )

    def get_or_create(self, wallet_address: str, **kwargs) -> BillingAccount:
        """Get existing account or create new one."""
        account = self.get_by_wallet(wallet_address)
        if account:
            return account

        data = {"wallet_address": wallet_address, **kwargs}
        return self.create(data)

    def add_credits(
        self,
        id: UUID,
        amount: float,
        reason: str = "manual",
    ) -> Optional[BillingAccount]:
        """Add credits to an account."""
        account = self.get(id)
        if not account:
            return None

        account.credits_balance = (account.credits_balance or 0) + amount
        self.db.commit()
        self.db.refresh(account)
        return account

    def deduct_credits(
        self,
        id: UUID,
        amount: float,
    ) -> Optional[BillingAccount]:
        """Deduct credits from an account."""
        account = self.get(id)
        if not account or (account.credits_balance or 0) < amount:
            return None

        account.credits_balance = (account.credits_balance or 0) - amount
        self.db.commit()
        self.db.refresh(account)
        return account

    def update_balance(self, id: UUID, amount: float) -> Optional[BillingAccount]:
        """Update account balance (positive = credit, negative = debit)."""
        account = self.get(id)
        if not account:
            return None

        account.balance = (account.balance or 0) + amount
        self.db.commit()
        self.db.refresh(account)
        return account

    def get_with_outstanding_balance(self) -> List[BillingAccount]:
        """Get accounts with outstanding balance."""
        return (
            self.db.query(BillingAccount)
            .filter(BillingAccount.balance > 0)
            .order_by(desc(BillingAccount.balance))
            .all()
        )

    def suspend(self, id: UUID, reason: str) -> Optional[BillingAccount]:
        """Suspend a billing account."""
        account = self.get(id)
        if not account:
            return None

        account.is_suspended = True
        account.suspended_at = datetime.utcnow()
        account.suspended_reason = reason
        self.db.commit()
        self.db.refresh(account)
        return account

    def unsuspend(self, id: UUID) -> Optional[BillingAccount]:
        """Unsuspend a billing account."""
        account = self.get(id)
        if not account:
            return None

        account.is_suspended = False
        account.suspended_at = None
        account.suspended_reason = None
        self.db.commit()
        self.db.refresh(account)
        return account


class BillingPlanRepository(BaseRepository[BillingPlan]):
    """Repository for BillingPlan model operations."""

    def __init__(self, db: Session):
        super().__init__(BillingPlan, db)

    def get_by_code(self, code: str) -> Optional[BillingPlan]:
        """Get plan by unique code."""
        return (
            self.db.query(BillingPlan)
            .filter(BillingPlan.code == code)
            .first()
        )

    def get_active(self) -> List[BillingPlan]:
        """Get all active plans."""
        return (
            self.db.query(BillingPlan)
            .filter(BillingPlan.is_active == True)
            .order_by(BillingPlan.sort_order, BillingPlan.monthly_price_usd)
            .all()
        )

    def get_public(self) -> List[BillingPlan]:
        """Get public (visible) plans."""
        return (
            self.db.query(BillingPlan)
            .filter(
                BillingPlan.is_active == True,
                BillingPlan.is_public == True,
            )
            .order_by(BillingPlan.sort_order, BillingPlan.monthly_price_usd)
            .all()
        )

    def get_by_type(self, plan_type: BillingPlanType) -> List[BillingPlan]:
        """Get plans by type."""
        return (
            self.db.query(BillingPlan)
            .filter(
                BillingPlan.plan_type == plan_type.value,
                BillingPlan.is_active == True,
            )
            .order_by(BillingPlan.monthly_price_usd)
            .all()
        )

    def get_free_plan(self) -> Optional[BillingPlan]:
        """Get the free tier plan."""
        return (
            self.db.query(BillingPlan)
            .filter(
                BillingPlan.plan_type == BillingPlanType.FREE.value,
                BillingPlan.is_active == True,
            )
            .first()
        )

    def get_enterprise_plans(self) -> List[BillingPlan]:
        """Get enterprise plans."""
        return (
            self.db.query(BillingPlan)
            .filter(
                BillingPlan.plan_type == BillingPlanType.ENTERPRISE.value,
                BillingPlan.is_active == True,
            )
            .all()
        )


class BillingSubscriptionRepository(BaseRepository[BillingSubscription]):
    """Repository for BillingSubscription model operations."""

    def __init__(self, db: Session):
        super().__init__(BillingSubscription, db)

    def get_by_account(self, account_id: UUID) -> List[BillingSubscription]:
        """Get all subscriptions for an account."""
        return (
            self.db.query(BillingSubscription)
            .filter(BillingSubscription.billing_account_id == account_id)
            .order_by(desc(BillingSubscription.created_at))
            .all()
        )

    def get_active_by_account(self, account_id: UUID) -> Optional[BillingSubscription]:
        """Get active subscription for an account."""
        return (
            self.db.query(BillingSubscription)
            .filter(
                BillingSubscription.billing_account_id == account_id,
                BillingSubscription.status.in_([
                    SubscriptionStatus.ACTIVE.value,
                    SubscriptionStatus.TRIALING.value,
                ]),
            )
            .first()
        )

    def get_by_stripe_subscription(
        self, stripe_subscription_id: str
    ) -> Optional[BillingSubscription]:
        """Get subscription by Stripe subscription ID."""
        return (
            self.db.query(BillingSubscription)
            .filter(BillingSubscription.stripe_subscription_id == stripe_subscription_id)
            .first()
        )

    def get_expiring_soon(self, days: int = 7) -> List[BillingSubscription]:
        """Get subscriptions expiring within specified days."""
        threshold = datetime.utcnow() + timedelta(days=days)
        return (
            self.db.query(BillingSubscription)
            .filter(
                BillingSubscription.status == SubscriptionStatus.ACTIVE.value,
                BillingSubscription.current_period_end <= threshold,
                BillingSubscription.cancel_at_period_end == False,
            )
            .all()
        )

    def get_trials_ending(self, days: int = 3) -> List[BillingSubscription]:
        """Get trials ending within specified days."""
        threshold = datetime.utcnow() + timedelta(days=days)
        return (
            self.db.query(BillingSubscription)
            .filter(
                BillingSubscription.status == SubscriptionStatus.TRIALING.value,
                BillingSubscription.trial_end <= threshold,
            )
            .all()
        )

    def get_past_due(self) -> List[BillingSubscription]:
        """Get subscriptions with past due payments."""
        return (
            self.db.query(BillingSubscription)
            .filter(BillingSubscription.status == SubscriptionStatus.PAST_DUE.value)
            .all()
        )

    def set_status(
        self, id: UUID, status: SubscriptionStatus
    ) -> Optional[BillingSubscription]:
        """Update subscription status."""
        subscription = self.get(id)
        if not subscription:
            return None

        subscription.status = status.value

        if status == SubscriptionStatus.CANCELLED:
            subscription.cancelled_at = datetime.utcnow()
        elif status == SubscriptionStatus.ACTIVE:
            subscription.activated_at = datetime.utcnow()

        self.db.commit()
        self.db.refresh(subscription)
        return subscription

    def cancel(
        self,
        id: UUID,
        at_period_end: bool = True,
        reason: Optional[str] = None,
    ) -> Optional[BillingSubscription]:
        """Cancel a subscription."""
        subscription = self.get(id)
        if not subscription:
            return None

        subscription.cancel_at_period_end = at_period_end
        subscription.cancellation_reason = reason

        if not at_period_end:
            subscription.status = SubscriptionStatus.CANCELLED.value
            subscription.cancelled_at = datetime.utcnow()

        self.db.commit()
        self.db.refresh(subscription)
        return subscription

    def renew(self, id: UUID, new_period_end: datetime) -> Optional[BillingSubscription]:
        """Renew a subscription for a new period."""
        subscription = self.get(id)
        if not subscription:
            return None

        subscription.current_period_start = subscription.current_period_end
        subscription.current_period_end = new_period_end
        subscription.status = SubscriptionStatus.ACTIVE.value

        self.db.commit()
        self.db.refresh(subscription)
        return subscription


class BillingInvoiceRepository(BaseRepository[BillingInvoice]):
    """Repository for BillingInvoice model operations."""

    def __init__(self, db: Session):
        super().__init__(BillingInvoice, db)

    def get_by_account(
        self,
        account_id: UUID,
        limit: int = 50,
        offset: int = 0,
    ) -> List[BillingInvoice]:
        """Get invoices for an account."""
        return (
            self.db.query(BillingInvoice)
            .filter(BillingInvoice.billing_account_id == account_id)
            .order_by(desc(BillingInvoice.created_at))
            .offset(offset)
            .limit(limit)
            .all()
        )

    def get_by_number(self, invoice_number: str) -> Optional[BillingInvoice]:
        """Get invoice by invoice number."""
        return (
            self.db.query(BillingInvoice)
            .filter(BillingInvoice.invoice_number == invoice_number)
            .first()
        )

    def get_by_stripe_invoice(self, stripe_invoice_id: str) -> Optional[BillingInvoice]:
        """Get invoice by Stripe invoice ID."""
        return (
            self.db.query(BillingInvoice)
            .filter(BillingInvoice.stripe_invoice_id == stripe_invoice_id)
            .first()
        )

    def get_by_status(self, status: InvoiceStatus) -> List[BillingInvoice]:
        """Get invoices by status."""
        return (
            self.db.query(BillingInvoice)
            .filter(BillingInvoice.status == status.value)
            .order_by(BillingInvoice.due_date)
            .all()
        )

    def get_unpaid(self) -> List[BillingInvoice]:
        """Get unpaid invoices."""
        return (
            self.db.query(BillingInvoice)
            .filter(
                BillingInvoice.status.in_([
                    InvoiceStatus.DRAFT.value,
                    InvoiceStatus.OPEN.value,
                ])
            )
            .order_by(BillingInvoice.due_date)
            .all()
        )

    def get_overdue(self) -> List[BillingInvoice]:
        """Get overdue invoices."""
        now = datetime.utcnow()
        return (
            self.db.query(BillingInvoice)
            .filter(
                BillingInvoice.status == InvoiceStatus.OPEN.value,
                BillingInvoice.due_date < now,
            )
            .order_by(BillingInvoice.due_date)
            .all()
        )

    def set_status(
        self, id: UUID, status: InvoiceStatus
    ) -> Optional[BillingInvoice]:
        """Update invoice status."""
        invoice = self.get(id)
        if not invoice:
            return None

        invoice.status = status.value

        if status == InvoiceStatus.PAID:
            invoice.paid_at = datetime.utcnow()
        elif status == InvoiceStatus.VOID:
            invoice.voided_at = datetime.utcnow()

        self.db.commit()
        self.db.refresh(invoice)
        return invoice

    def mark_paid(
        self,
        id: UUID,
        payment_id: Optional[UUID] = None,
    ) -> Optional[BillingInvoice]:
        """Mark invoice as paid."""
        invoice = self.get(id)
        if not invoice:
            return None

        invoice.status = InvoiceStatus.PAID.value
        invoice.paid_at = datetime.utcnow()
        if payment_id:
            invoice.payment_id = payment_id

        self.db.commit()
        self.db.refresh(invoice)
        return invoice

    def get_total_by_account(self, account_id: UUID) -> dict:
        """Get invoice totals by status for an account."""
        results = (
            self.db.query(
                BillingInvoice.status,
                func.sum(BillingInvoice.total_amount),
                func.count(BillingInvoice.id),
            )
            .filter(BillingInvoice.billing_account_id == account_id)
            .group_by(BillingInvoice.status)
            .all()
        )

        return {
            status: {"total": float(total or 0), "count": count}
            for status, total, count in results
        }


class BillingPaymentRepository(BaseRepository[BillingPayment]):
    """Repository for BillingPayment model operations."""

    def __init__(self, db: Session):
        super().__init__(BillingPayment, db)

    def get_by_account(
        self,
        account_id: UUID,
        limit: int = 50,
        offset: int = 0,
    ) -> List[BillingPayment]:
        """Get payments for an account."""
        return (
            self.db.query(BillingPayment)
            .filter(BillingPayment.billing_account_id == account_id)
            .order_by(desc(BillingPayment.created_at))
            .offset(offset)
            .limit(limit)
            .all()
        )

    def get_by_stripe_payment(
        self, stripe_payment_intent_id: str
    ) -> Optional[BillingPayment]:
        """Get payment by Stripe payment intent ID."""
        return (
            self.db.query(BillingPayment)
            .filter(BillingPayment.stripe_payment_intent_id == stripe_payment_intent_id)
            .first()
        )

    def get_by_invoice(self, invoice_id: UUID) -> List[BillingPayment]:
        """Get payments for an invoice."""
        return (
            self.db.query(BillingPayment)
            .filter(BillingPayment.invoice_id == invoice_id)
            .order_by(BillingPayment.created_at)
            .all()
        )

    def get_by_status(self, status: PaymentStatus) -> List[BillingPayment]:
        """Get payments by status."""
        return (
            self.db.query(BillingPayment)
            .filter(BillingPayment.status == status.value)
            .order_by(desc(BillingPayment.created_at))
            .all()
        )

    def get_pending(self) -> List[BillingPayment]:
        """Get pending payments."""
        return self.get_by_status(PaymentStatus.PENDING)

    def get_successful(
        self,
        account_id: UUID,
        start_date: Optional[datetime] = None,
        end_date: Optional[datetime] = None,
    ) -> List[BillingPayment]:
        """Get successful payments for an account in date range."""
        q = self.db.query(BillingPayment).filter(
            BillingPayment.billing_account_id == account_id,
            BillingPayment.status == PaymentStatus.SUCCEEDED.value,
        )

        if start_date:
            q = q.filter(BillingPayment.created_at >= start_date)
        if end_date:
            q = q.filter(BillingPayment.created_at <= end_date)

        return q.order_by(desc(BillingPayment.created_at)).all()

    def set_status(
        self, id: UUID, status: PaymentStatus
    ) -> Optional[BillingPayment]:
        """Update payment status."""
        payment = self.get(id)
        if not payment:
            return None

        payment.status = status.value

        if status == PaymentStatus.SUCCEEDED:
            payment.paid_at = datetime.utcnow()
        elif status == PaymentStatus.FAILED:
            payment.failed_at = datetime.utcnow()

        self.db.commit()
        self.db.refresh(payment)
        return payment

    def record_refund(
        self,
        id: UUID,
        refund_amount: float,
        refund_reason: Optional[str] = None,
    ) -> Optional[BillingPayment]:
        """Record a refund on a payment."""
        payment = self.get(id)
        if not payment:
            return None

        payment.refunded_amount = (payment.refunded_amount or 0) + refund_amount
        payment.refund_reason = refund_reason
        payment.refunded_at = datetime.utcnow()

        if payment.refunded_amount >= payment.amount:
            payment.status = PaymentStatus.REFUNDED.value

        self.db.commit()
        self.db.refresh(payment)
        return payment


class BillingUsageRepository(BaseRepository[BillingUsage]):
    """Repository for BillingUsage model operations."""

    def __init__(self, db: Session):
        super().__init__(BillingUsage, db)

    def get_by_account(
        self,
        account_id: UUID,
        start_date: Optional[datetime] = None,
        end_date: Optional[datetime] = None,
    ) -> List[BillingUsage]:
        """Get usage records for an account."""
        q = self.db.query(BillingUsage).filter(
            BillingUsage.billing_account_id == account_id
        )

        if start_date:
            q = q.filter(BillingUsage.period_start >= start_date)
        if end_date:
            q = q.filter(BillingUsage.period_end <= end_date)

        return q.order_by(desc(BillingUsage.period_start)).all()

    def get_by_subscription(
        self,
        subscription_id: UUID,
        period_start: Optional[datetime] = None,
    ) -> List[BillingUsage]:
        """Get usage for a subscription period."""
        q = self.db.query(BillingUsage).filter(
            BillingUsage.subscription_id == subscription_id
        )

        if period_start:
            q = q.filter(BillingUsage.period_start == period_start)

        return q.order_by(BillingUsage.metric_name).all()

    def get_current_period(
        self, subscription_id: UUID
    ) -> List[BillingUsage]:
        """Get usage for current billing period."""
        now = datetime.utcnow()
        return (
            self.db.query(BillingUsage)
            .filter(
                BillingUsage.subscription_id == subscription_id,
                BillingUsage.period_start <= now,
                BillingUsage.period_end >= now,
            )
            .all()
        )

    def get_by_metric(
        self,
        account_id: UUID,
        metric_name: str,
        limit: int = 12,
    ) -> List[BillingUsage]:
        """Get usage history for a specific metric."""
        return (
            self.db.query(BillingUsage)
            .filter(
                BillingUsage.billing_account_id == account_id,
                BillingUsage.metric_name == metric_name,
            )
            .order_by(desc(BillingUsage.period_start))
            .limit(limit)
            .all()
        )

    def record_usage(
        self,
        account_id: UUID,
        subscription_id: UUID,
        metric_name: str,
        quantity: float,
        period_start: datetime,
        period_end: datetime,
        **kwargs,
    ) -> BillingUsage:
        """Record or update usage for a metric."""
        existing = (
            self.db.query(BillingUsage)
            .filter(
                BillingUsage.billing_account_id == account_id,
                BillingUsage.subscription_id == subscription_id,
                BillingUsage.metric_name == metric_name,
                BillingUsage.period_start == period_start,
            )
            .first()
        )

        if existing:
            existing.quantity = quantity
            existing.updated_at = datetime.utcnow()
            for key, value in kwargs.items():
                if hasattr(existing, key):
                    setattr(existing, key, value)
            self.db.commit()
            self.db.refresh(existing)
            return existing

        data = {
            "billing_account_id": account_id,
            "subscription_id": subscription_id,
            "metric_name": metric_name,
            "quantity": quantity,
            "period_start": period_start,
            "period_end": period_end,
            **kwargs,
        }
        return self.create(data)

    def get_overage_summary(self, subscription_id: UUID) -> dict:
        """Get overage summary for current period."""
        usage_records = self.get_current_period(subscription_id)

        summary = {
            "total_overage_amount": 0.0,
            "metrics_with_overage": [],
        }

        for usage in usage_records:
            if usage.overage_quantity and usage.overage_quantity > 0:
                overage_cost = usage.overage_quantity * (usage.overage_rate or 0)
                summary["total_overage_amount"] += overage_cost
                summary["metrics_with_overage"].append({
                    "metric": usage.metric_name,
                    "overage_quantity": usage.overage_quantity,
                    "overage_cost": overage_cost,
                })

        return summary
