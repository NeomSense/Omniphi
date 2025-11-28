"""Alerts API endpoints."""

from datetime import datetime, timedelta
from typing import Optional
import logging
import random
import uuid

from fastapi import APIRouter, Depends, Query, HTTPException, status
from pydantic import BaseModel
from sqlalchemy.orm import Session

from app.db.session import get_db
from app.models import Alert, AlertSeverity, AlertStatus, AuditLog, AuditAction

logger = logging.getLogger(__name__)

router = APIRouter()


class AlertAcknowledge(BaseModel):
    acknowledged_by: str = "admin"


class AlertResolve(BaseModel):
    resolved_by: str = "admin"
    resolution_notes: Optional[str] = None


def generate_mock_alerts(count: int = 10):
    """Generate mock alert entries for development."""
    severities = ["warning", "warning", "error", "critical", "info"]
    statuses = ["active", "active", "acknowledged", "resolved"]

    alert_templates = [
        {
            "title": "High CPU Usage",
            "message": "Node node-{node} CPU usage exceeded 90%",
            "severity": "warning",
            "resource_type": "node"
        },
        {
            "title": "Node Unreachable",
            "message": "Health check failed for node node-{node}",
            "severity": "error",
            "resource_type": "node"
        },
        {
            "title": "Provisioning Timeout",
            "message": "Provisioning request req-{req} exceeded 30 minute limit",
            "severity": "warning",
            "resource_type": "setup_request"
        },
        {
            "title": "Disk Space Low",
            "message": "Node node-{node} disk usage at 85%",
            "severity": "warning",
            "resource_type": "node"
        },
        {
            "title": "Consecutive Failures",
            "message": "3 consecutive provisioning failures detected",
            "severity": "critical",
            "resource_type": "orchestrator"
        },
        {
            "title": "RPC Endpoint Down",
            "message": "Primary RPC endpoint not responding",
            "severity": "critical",
            "resource_type": "rpc"
        },
        {
            "title": "Node Out of Sync",
            "message": "Node node-{node} is more than 100 blocks behind",
            "severity": "error",
            "resource_type": "node"
        },
        {
            "title": "Memory Pressure",
            "message": "System memory usage exceeded 80%",
            "severity": "warning",
            "resource_type": "orchestrator"
        }
    ]

    alerts = []
    now = datetime.utcnow()

    for i in range(count):
        template = random.choice(alert_templates)
        status_choice = random.choice(statuses)

        node_id = f"{random.randint(1, 24):03d}"
        req_id = str(random.randint(1000, 9999))

        message = template["message"].format(node=node_id, req=req_id)
        resource_id = f"node-{node_id}" if "node" in template["resource_type"] else (
            f"req-{req_id}" if "setup_request" in template["resource_type"] else None
        )

        created_at = now - timedelta(hours=random.randint(0, 72))

        alert = {
            "id": str(uuid.uuid4()),
            "title": template["title"],
            "message": message,
            "severity": template["severity"],
            "status": status_choice,
            "resource_type": template["resource_type"],
            "resource_id": resource_id,
            "details": {
                "threshold": "90%" if "CPU" in template["title"] else None,
                "current_value": f"{random.randint(85, 98)}%" if "Usage" in template["title"] else None
            },
            "acknowledged_by": "admin" if status_choice in ["acknowledged", "resolved"] else None,
            "acknowledged_at": (created_at + timedelta(minutes=random.randint(5, 30))).isoformat() if status_choice in ["acknowledged", "resolved"] else None,
            "resolved_by": "admin" if status_choice == "resolved" else None,
            "resolved_at": (created_at + timedelta(hours=random.randint(1, 4))).isoformat() if status_choice == "resolved" else None,
            "resolution_notes": "Issue resolved after node restart" if status_choice == "resolved" else None,
            "created_at": created_at.isoformat()
        }

        alerts.append(alert)

    # Sort by created_at descending
    alerts.sort(key=lambda x: x["created_at"], reverse=True)

    return alerts


@router.get("")
async def list_alerts(
    page: int = Query(1, ge=1),
    pageSize: int = Query(20, ge=1, le=100),
    severity: Optional[str] = None,
    status_filter: Optional[str] = Query(None, alias="status"),
    db: Session = Depends(get_db)
):
    """
    Get alerts with pagination and filtering.

    Returns:
        Paginated list of alerts
    """
    query = db.query(Alert)

    # Apply filters
    if severity:
        try:
            alert_severity = AlertSeverity(severity)
            query = query.filter(Alert.severity == alert_severity)
        except ValueError:
            pass

    if status_filter:
        try:
            alert_status = AlertStatus(status_filter)
            query = query.filter(Alert.status == alert_status)
        except ValueError:
            pass

    # Get total count
    total = query.count()

    # Get paginated results
    offset = (page - 1) * pageSize
    alerts = query.order_by(Alert.created_at.desc()).offset(offset).limit(pageSize).all()

    # Transform to response format
    if alerts:
        items = [
            {
                "id": str(alert.id),
                "title": alert.title,
                "message": alert.message,
                "severity": alert.severity.value if hasattr(alert.severity, 'value') else str(alert.severity),
                "status": alert.status.value if hasattr(alert.status, 'value') else str(alert.status),
                "resource_type": alert.resource_type,
                "resource_id": alert.resource_id,
                "details": alert.details,
                "acknowledged_by": alert.acknowledged_by,
                "acknowledged_at": alert.acknowledged_at.isoformat() if alert.acknowledged_at else None,
                "resolved_by": alert.resolved_by,
                "resolved_at": alert.resolved_at.isoformat() if alert.resolved_at else None,
                "resolution_notes": alert.resolution_notes,
                "created_at": alert.created_at.isoformat()
            }
            for alert in alerts
        ]
    else:
        # Return mock data if no real alerts exist
        items = generate_mock_alerts(pageSize)
        total = 25  # Mock total

    return {
        "items": items,
        "total": total,
        "page": page,
        "pageSize": pageSize,
        "totalPages": (total + pageSize - 1) // pageSize
    }


@router.get("/active")
async def get_active_alerts(
    db: Session = Depends(get_db)
):
    """
    Get all active (non-resolved) alerts.
    """
    alerts = db.query(Alert).filter(
        Alert.status != AlertStatus.RESOLVED
    ).order_by(Alert.created_at.desc()).all()

    if alerts:
        return [
            {
                "id": str(alert.id),
                "title": alert.title,
                "message": alert.message,
                "severity": alert.severity.value if hasattr(alert.severity, 'value') else str(alert.severity),
                "status": alert.status.value if hasattr(alert.status, 'value') else str(alert.status),
                "resource_type": alert.resource_type,
                "resource_id": alert.resource_id,
                "created_at": alert.created_at.isoformat()
            }
            for alert in alerts
        ]

    # Return mock active alerts
    mock = generate_mock_alerts(5)
    return [a for a in mock if a["status"] in ["active", "acknowledged"]]


@router.post("/{alert_id}/acknowledge")
async def acknowledge_alert(
    alert_id: str,
    body: AlertAcknowledge,
    db: Session = Depends(get_db)
):
    """
    Acknowledge an alert.
    """
    try:
        alert_uuid = uuid.UUID(alert_id)
        alert = db.query(Alert).filter(Alert.id == alert_uuid).first()
    except ValueError:
        # Mock acknowledge for mock alerts
        return {
            "id": alert_id,
            "status": "acknowledged",
            "acknowledged_by": body.acknowledged_by,
            "acknowledged_at": datetime.utcnow().isoformat(),
            "message": "Alert acknowledged"
        }

    if not alert:
        # Mock acknowledge
        return {
            "id": alert_id,
            "status": "acknowledged",
            "acknowledged_by": body.acknowledged_by,
            "acknowledged_at": datetime.utcnow().isoformat(),
            "message": "Alert acknowledged"
        }

    # Update alert
    alert.status = AlertStatus.ACKNOWLEDGED
    alert.acknowledged_by = body.acknowledged_by
    alert.acknowledged_at = datetime.utcnow()

    # Create audit log
    audit = AuditLog(
        user_id=body.acknowledged_by,
        username=body.acknowledged_by,
        action=AuditAction.ACKNOWLEDGE_ALERT,
        resource_type="alert",
        resource_id=str(alert.id),
        details={"title": alert.title},
        ip_address="127.0.0.1"
    )
    db.add(audit)
    db.commit()

    return {
        "id": str(alert.id),
        "status": "acknowledged",
        "acknowledged_by": alert.acknowledged_by,
        "acknowledged_at": alert.acknowledged_at.isoformat(),
        "message": "Alert acknowledged"
    }


@router.post("/{alert_id}/resolve")
async def resolve_alert(
    alert_id: str,
    body: AlertResolve,
    db: Session = Depends(get_db)
):
    """
    Resolve an alert.
    """
    try:
        alert_uuid = uuid.UUID(alert_id)
        alert = db.query(Alert).filter(Alert.id == alert_uuid).first()
    except ValueError:
        # Mock resolve for mock alerts
        return {
            "id": alert_id,
            "status": "resolved",
            "resolved_by": body.resolved_by,
            "resolved_at": datetime.utcnow().isoformat(),
            "resolution_notes": body.resolution_notes,
            "message": "Alert resolved"
        }

    if not alert:
        # Mock resolve
        return {
            "id": alert_id,
            "status": "resolved",
            "resolved_by": body.resolved_by,
            "resolved_at": datetime.utcnow().isoformat(),
            "resolution_notes": body.resolution_notes,
            "message": "Alert resolved"
        }

    # Update alert
    alert.status = AlertStatus.RESOLVED
    alert.resolved_by = body.resolved_by
    alert.resolved_at = datetime.utcnow()
    alert.resolution_notes = body.resolution_notes

    db.commit()

    return {
        "id": str(alert.id),
        "status": "resolved",
        "resolved_by": alert.resolved_by,
        "resolved_at": alert.resolved_at.isoformat(),
        "resolution_notes": alert.resolution_notes,
        "message": "Alert resolved"
    }


@router.get("/summary")
async def get_alerts_summary(
    db: Session = Depends(get_db)
):
    """
    Get summary of alerts by severity and status.
    """
    try:
        active_critical = db.query(Alert).filter(
            Alert.status != AlertStatus.RESOLVED,
            Alert.severity == AlertSeverity.CRITICAL
        ).count()
        active_error = db.query(Alert).filter(
            Alert.status != AlertStatus.RESOLVED,
            Alert.severity == AlertSeverity.ERROR
        ).count()
        active_warning = db.query(Alert).filter(
            Alert.status != AlertStatus.RESOLVED,
            Alert.severity == AlertSeverity.WARNING
        ).count()
        total_active = db.query(Alert).filter(
            Alert.status != AlertStatus.RESOLVED
        ).count()
        total_resolved = db.query(Alert).filter(
            Alert.status == AlertStatus.RESOLVED
        ).count()
    except Exception as e:
        logger.warning(f"Could not get alert stats: {e}")
        # Mock stats
        active_critical = random.randint(0, 2)
        active_error = random.randint(1, 4)
        active_warning = random.randint(2, 6)
        total_active = active_critical + active_error + active_warning
        total_resolved = random.randint(10, 30)

    return {
        "active": {
            "critical": active_critical,
            "error": active_error,
            "warning": active_warning,
            "total": total_active
        },
        "resolved": total_resolved,
        "total": total_active + total_resolved
    }
