"""Local Validator Heartbeat Model."""

import uuid
from datetime import datetime

from sqlalchemy import Column, String, DateTime, Integer
from sqlalchemy.dialects.postgresql import UUID

from app.db.base_class import Base


class LocalValidatorHeartbeat(Base):
    """Local validator heartbeat tracking table."""

    __tablename__ = "local_validator_heartbeats"

    # Primary key
    id = Column(UUID(as_uuid=True), primary_key=True, default=uuid.uuid4, index=True)

    # Identity
    wallet_address = Column(String, nullable=False, index=True)
    consensus_pubkey = Column(String, nullable=False, unique=True, index=True)

    # Status
    block_height = Column(Integer, nullable=False, default=0)
    uptime_seconds = Column(Integer, nullable=False, default=0)

    # Connection info
    local_rpc_port = Column(Integer, nullable=True)
    local_p2p_port = Column(Integer, nullable=True)

    # Timestamps
    first_seen = Column(DateTime, nullable=False, default=datetime.utcnow)
    last_seen = Column(DateTime, nullable=False, default=datetime.utcnow)

    def __repr__(self):
        return f"<LocalValidatorHeartbeat {self.consensus_pubkey[:16]}... height={self.block_height}>"
