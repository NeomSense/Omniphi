"""Audit Log API endpoints."""

from datetime import datetime, timedelta
from typing import Optional
import logging
import random
import uuid

from fastapi import APIRouter, Depends, Query
from sqlalchemy.orm import Session

from app.db.session import get_db
from app.models import AuditLog, AuditAction

logger = logging.getLogger(__name__)

router = APIRouter()


def generate_mock_audit_logs(count: int = 25):
    """Generate mock audit log entries for development."""
    actions = [
        ("login", "Login"),
        ("logout", "Logout"),
        ("restart_node", "Restart Node"),
        ("stop_node", "Stop Node"),
        ("update_settings", "Update Settings"),
        ("retry_provisioning", "Retry Provisioning"),
        ("acknowledge_alert", "Acknowledge Alert")
    ]

    usernames = ["admin", "operator", "sysadmin"]

    logs = []
    now = datetime.utcnow()

    for i in range(count):
        action, label = random.choice(actions)
        username = random.choice(usernames)

        resource_type = None
        resource_id = None
        details = {}

        if action in ["restart_node", "stop_node"]:
            resource_type = "node"
            resource_id = f"node-{random.randint(1, 24):03d}"
            details = {"previous_status": "running" if action == "stop_node" else "stopped"}
        elif action == "update_settings":
            resource_type = "settings"
            details = {"changed_fields": ["max_parallel_jobs", "heartbeat_interval_seconds"]}
        elif action == "retry_provisioning":
            resource_type = "setup_request"
            resource_id = f"req-{random.randint(1000, 9999)}"
            details = {"attempt": random.randint(1, 3)}
        elif action == "acknowledge_alert":
            resource_type = "alert"
            resource_id = f"alert-{random.randint(100, 999)}"

        logs.append({
            "id": str(uuid.uuid4()),
            "user_id": username,
            "username": username,
            "action": action,
            "resource_type": resource_type,
            "resource_id": resource_id,
            "details": details,
            "ip_address": f"192.168.1.{random.randint(1, 254)}",
            "timestamp": (now - timedelta(hours=i, minutes=random.randint(0, 59))).isoformat()
        })

    return logs


@router.get("")
async def list_audit_logs(
    page: int = Query(1, ge=1),
    pageSize: int = Query(25, ge=1, le=100),
    action: Optional[str] = None,
    db: Session = Depends(get_db)
):
    """
    Get audit logs with pagination and filtering.

    Returns:
        Paginated list of audit log entries
    """
    query = db.query(AuditLog)

    # Apply filters
    if action:
        try:
            audit_action = AuditAction(action)
            query = query.filter(AuditLog.action == audit_action)
        except ValueError:
            pass

    # Get total count
    total = query.count()

    # Get paginated results
    offset = (page - 1) * pageSize
    logs = query.order_by(AuditLog.timestamp.desc()).offset(offset).limit(pageSize).all()

    # Transform to response format
    if logs:
        items = [
            {
                "id": str(log.id),
                "user_id": log.user_id,
                "username": log.username,
                "action": log.action.value if hasattr(log.action, 'value') else str(log.action),
                "resource_type": log.resource_type,
                "resource_id": log.resource_id,
                "details": log.details,
                "ip_address": log.ip_address,
                "timestamp": log.timestamp.isoformat()
            }
            for log in logs
        ]
    else:
        # Return mock data if no real logs exist
        items = generate_mock_audit_logs(pageSize)
        total = 150  # Mock total

    return {
        "items": items,
        "total": total,
        "page": page,
        "pageSize": pageSize,
        "totalPages": (total + pageSize - 1) // pageSize
    }


@router.get("/summary")
async def get_audit_summary(
    db: Session = Depends(get_db)
):
    """
    Get summary statistics for audit logs.
    """
    now = datetime.utcnow()
    today_start = now.replace(hour=0, minute=0, second=0, microsecond=0)

    # Try to get real stats
    try:
        total_logs = db.query(AuditLog).count()
        today_logins = db.query(AuditLog).filter(
            AuditLog.action == AuditAction.LOGIN,
            AuditLog.timestamp >= today_start
        ).count()
        today_restarts = db.query(AuditLog).filter(
            AuditLog.action.in_([AuditAction.RESTART_NODE, AuditAction.RETRY_PROVISIONING]),
            AuditLog.timestamp >= today_start
        ).count()
        today_settings = db.query(AuditLog).filter(
            AuditLog.action == AuditAction.UPDATE_SETTINGS,
            AuditLog.timestamp >= today_start
        ).count()
        today_failures = db.query(AuditLog).filter(
            AuditLog.action.in_([AuditAction.MARK_FAILED, AuditAction.DELETE_REQUEST]),
            AuditLog.timestamp >= today_start
        ).count()
    except Exception as e:
        logger.warning(f"Could not get audit stats: {e}")
        # Mock stats
        total_logs = 150
        today_logins = random.randint(3, 10)
        today_restarts = random.randint(1, 5)
        today_settings = random.randint(0, 3)
        today_failures = random.randint(0, 2)

    return {
        "total_logs": total_logs,
        "today": {
            "logins": today_logins,
            "restarts_retries": today_restarts,
            "config_changes": today_settings,
            "failures_deletes": today_failures
        }
    }
