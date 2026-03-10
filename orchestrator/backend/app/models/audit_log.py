"""Audit Log Model."""

import uuid
from datetime import datetime
from typing import Optional
import enum

from sqlalchemy import Column, String, DateTime, Enum as SQLEnum, JSON
from sqlalchemy.dialects.postgresql import UUID

from app.db.base_class import Base


class AuditAction(str, enum.Enum):
    """Audit action types."""
    LOGIN = "login"
    LOGOUT = "logout"
    CREATE_REQUEST = "create_request"
    RETRY_PROVISIONING = "retry_provisioning"
    MARK_FAILED = "mark_failed"
    DELETE_REQUEST = "delete_request"
    RESTART_NODE = "restart_node"
    STOP_NODE = "stop_node"
    UPDATE_SETTINGS = "update_settings"
    ACKNOWLEDGE_ALERT = "acknowledge_alert"
    # API Key & Credential Management (HIGH-1 remediation)
    GENERATE_API_KEY = "generate_api_key"
    REVOKE_API_KEY = "revoke_api_key"
    ROTATE_API_KEY = "rotate_api_key"
    START_CREDENTIAL_ROTATION = "start_credential_rotation"
    COMPLETE_CREDENTIAL_ROTATION = "complete_credential_rotation"
    ROLLBACK_CREDENTIAL_ROTATION = "rollback_credential_rotation"
    EMERGENCY_CREDENTIAL_REVOCATION = "emergency_credential_revocation"


class AuditLog(Base):
    """Audit log table for tracking admin actions."""

    __tablename__ = "audit_logs"

    # Primary key
    id = Column(UUID(as_uuid=True), primary_key=True, default=uuid.uuid4, index=True)

    # Who performed the action
    user_id = Column(String, nullable=False, index=True)
    username = Column(String, nullable=False)

    # What action was performed
    action = Column(SQLEnum(AuditAction), nullable=False, index=True)

    # What resource was affected
    resource_type = Column(String, nullable=True)  # e.g., "setup_request", "node", "settings"
    resource_id = Column(String, nullable=True, index=True)

    # Additional details
    details = Column(JSON, nullable=False, default=dict)

    # Request metadata
    ip_address = Column(String, nullable=False)
    user_agent = Column(String, nullable=True)

    # Timestamp
    timestamp = Column(DateTime, nullable=False, default=datetime.utcnow, index=True)

    def __repr__(self):
        return f"<AuditLog {self.action} by {self.username} at {self.timestamp}>"
