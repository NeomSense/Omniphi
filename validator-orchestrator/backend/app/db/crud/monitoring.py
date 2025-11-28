"""
Monitoring CRUD Repositories

Repositories for node metrics and incident operations.
"""

from datetime import datetime, timedelta
from typing import List, Optional, Tuple
from uuid import UUID

from sqlalchemy import and_, desc, func, or_
from sqlalchemy.orm import Session

from app.db.crud.base import BaseRepository
from app.db.models.node_metrics import NodeMetrics
from app.db.models.incident import Incident
from app.db.models.enums import IncidentSeverity, IncidentStatus


class NodeMetricsRepository(BaseRepository[NodeMetrics]):
    """Repository for NodeMetrics model operations."""

    def __init__(self, db: Session):
        super().__init__(NodeMetrics, db)

    def get_latest(self, validator_node_id: UUID) -> Optional[NodeMetrics]:
        """Get latest metrics for a node."""
        return (
            self.db.query(NodeMetrics)
            .filter(NodeMetrics.validator_node_id == validator_node_id)
            .order_by(desc(NodeMetrics.recorded_at))
            .first()
        )

    def get_by_node(
        self,
        validator_node_id: UUID,
        hours: int = 24,
    ) -> List[NodeMetrics]:
        """Get metrics history for a node."""
        threshold = datetime.utcnow() - timedelta(hours=hours)
        return (
            self.db.query(NodeMetrics)
            .filter(
                NodeMetrics.validator_node_id == validator_node_id,
                NodeMetrics.recorded_at >= threshold,
            )
            .order_by(NodeMetrics.recorded_at)
            .all()
        )

    def get_by_period(
        self,
        validator_node_id: UUID,
        period_type: str,
        limit: int = 100,
    ) -> List[NodeMetrics]:
        """Get metrics by aggregation period type."""
        return (
            self.db.query(NodeMetrics)
            .filter(
                NodeMetrics.validator_node_id == validator_node_id,
                NodeMetrics.period_type == period_type,
            )
            .order_by(desc(NodeMetrics.recorded_at))
            .limit(limit)
            .all()
        )

    def get_averages(
        self,
        validator_node_id: UUID,
        hours: int = 24,
    ) -> dict:
        """Get average metrics over time period."""
        threshold = datetime.utcnow() - timedelta(hours=hours)

        result = (
            self.db.query(
                func.avg(NodeMetrics.cpu_percent),
                func.avg(NodeMetrics.memory_percent),
                func.avg(NodeMetrics.disk_percent),
                func.avg(NodeMetrics.peer_count),
                func.avg(NodeMetrics.health_score),
                func.max(NodeMetrics.block_height),
            )
            .filter(
                NodeMetrics.validator_node_id == validator_node_id,
                NodeMetrics.recorded_at >= threshold,
            )
            .first()
        )

        return {
            "avg_cpu_percent": float(result[0] or 0),
            "avg_memory_percent": float(result[1] or 0),
            "avg_disk_percent": float(result[2] or 0),
            "avg_peer_count": float(result[3] or 0),
            "avg_health_score": float(result[4] or 0),
            "max_block_height": int(result[5] or 0),
        }

    def get_resource_usage(
        self,
        validator_node_id: UUID,
        hours: int = 1,
    ) -> dict:
        """Get current resource usage summary."""
        latest = self.get_latest(validator_node_id)
        averages = self.get_averages(validator_node_id, hours)

        return {
            "current": {
                "cpu_percent": latest.cpu_percent if latest else None,
                "memory_percent": latest.memory_percent if latest else None,
                "disk_percent": latest.disk_percent if latest else None,
                "peer_count": latest.peer_count if latest else None,
            },
            "averages": averages,
            "recorded_at": latest.recorded_at if latest else None,
        }

    def get_unhealthy_nodes(
        self,
        threshold: float = 50.0,
    ) -> List[Tuple[UUID, float]]:
        """Get nodes with low health scores."""
        subquery = (
            self.db.query(
                NodeMetrics.validator_node_id,
                func.max(NodeMetrics.recorded_at).label("latest"),
            )
            .group_by(NodeMetrics.validator_node_id)
            .subquery()
        )

        results = (
            self.db.query(NodeMetrics.validator_node_id, NodeMetrics.health_score)
            .join(
                subquery,
                and_(
                    NodeMetrics.validator_node_id == subquery.c.validator_node_id,
                    NodeMetrics.recorded_at == subquery.c.latest,
                ),
            )
            .filter(NodeMetrics.health_score < threshold)
            .all()
        )

        return [(r[0], r[1]) for r in results]

    def get_nodes_with_high_cpu(
        self,
        threshold: float = 90.0,
    ) -> List[Tuple[UUID, float]]:
        """Get nodes with high CPU usage."""
        threshold_time = datetime.utcnow() - timedelta(minutes=5)

        results = (
            self.db.query(
                NodeMetrics.validator_node_id,
                func.avg(NodeMetrics.cpu_percent),
            )
            .filter(
                NodeMetrics.recorded_at >= threshold_time,
                NodeMetrics.cpu_percent >= threshold,
            )
            .group_by(NodeMetrics.validator_node_id)
            .all()
        )

        return [(r[0], float(r[1])) for r in results]

    def cleanup_old(self, days: int = 30) -> int:
        """Remove metrics older than specified days."""
        threshold = datetime.utcnow() - timedelta(days=days)
        result = (
            self.db.query(NodeMetrics)
            .filter(NodeMetrics.recorded_at < threshold)
            .delete(synchronize_session=False)
        )
        self.db.commit()
        return result

    def aggregate_to_hourly(
        self,
        validator_node_id: UUID,
        hour_start: datetime,
    ) -> Optional[NodeMetrics]:
        """Aggregate minute metrics to hourly."""
        hour_end = hour_start + timedelta(hours=1)

        result = (
            self.db.query(
                func.avg(NodeMetrics.cpu_percent),
                func.avg(NodeMetrics.memory_percent),
                func.avg(NodeMetrics.disk_percent),
                func.avg(NodeMetrics.peer_count),
                func.max(NodeMetrics.block_height),
                func.avg(NodeMetrics.health_score),
            )
            .filter(
                NodeMetrics.validator_node_id == validator_node_id,
                NodeMetrics.period_type == "minute",
                NodeMetrics.recorded_at >= hour_start,
                NodeMetrics.recorded_at < hour_end,
            )
            .first()
        )

        if not result or result[0] is None:
            return None

        hourly = NodeMetrics(
            validator_node_id=validator_node_id,
            recorded_at=hour_start,
            period_type="hour",
            cpu_percent=float(result[0] or 0),
            memory_percent=float(result[1] or 0),
            disk_percent=float(result[2] or 0),
            peer_count=int(result[3] or 0),
            block_height=int(result[4] or 0),
            health_score=float(result[5] or 0),
        )

        self.db.add(hourly)
        self.db.commit()
        self.db.refresh(hourly)
        return hourly


class IncidentRepository(BaseRepository[Incident]):
    """Repository for Incident model operations."""

    def __init__(self, db: Session):
        super().__init__(Incident, db)

    def get_by_number(self, incident_number: str) -> Optional[Incident]:
        """Get incident by number."""
        return (
            self.db.query(Incident)
            .filter(Incident.incident_number == incident_number)
            .first()
        )

    def get_by_node(self, validator_node_id: UUID) -> List[Incident]:
        """Get incidents for a validator node."""
        return (
            self.db.query(Incident)
            .filter(Incident.validator_node_id == validator_node_id)
            .order_by(desc(Incident.detected_at))
            .all()
        )

    def get_by_region(self, region_id: UUID) -> List[Incident]:
        """Get incidents in a region."""
        return (
            self.db.query(Incident)
            .filter(Incident.region_id == region_id)
            .order_by(desc(Incident.detected_at))
            .all()
        )

    def get_open(self) -> List[Incident]:
        """Get all open incidents."""
        return (
            self.db.query(Incident)
            .filter(
                Incident.status.notin_([
                    IncidentStatus.RESOLVED.value,
                    IncidentStatus.CLOSED.value,
                ])
            )
            .order_by(
                desc(Incident.severity == IncidentSeverity.CRITICAL.value),
                Incident.detected_at,
            )
            .all()
        )

    def get_critical(self) -> List[Incident]:
        """Get open critical incidents."""
        return (
            self.db.query(Incident)
            .filter(
                Incident.severity == IncidentSeverity.CRITICAL.value,
                Incident.status.notin_([
                    IncidentStatus.RESOLVED.value,
                    IncidentStatus.CLOSED.value,
                ]),
            )
            .order_by(Incident.detected_at)
            .all()
        )

    def get_by_severity(self, severity: IncidentSeverity) -> List[Incident]:
        """Get incidents by severity."""
        return (
            self.db.query(Incident)
            .filter(Incident.severity == severity.value)
            .order_by(desc(Incident.detected_at))
            .all()
        )

    def get_by_status(self, status: IncidentStatus) -> List[Incident]:
        """Get incidents by status."""
        return (
            self.db.query(Incident)
            .filter(Incident.status == status.value)
            .order_by(desc(Incident.detected_at))
            .all()
        )

    def get_unacknowledged(self) -> List[Incident]:
        """Get open incidents that haven't been acknowledged."""
        return (
            self.db.query(Incident)
            .filter(
                Incident.acknowledged_at.is_(None),
                Incident.status.notin_([
                    IncidentStatus.RESOLVED.value,
                    IncidentStatus.CLOSED.value,
                ]),
            )
            .order_by(
                desc(Incident.severity == IncidentSeverity.CRITICAL.value),
                Incident.detected_at,
            )
            .all()
        )

    def get_by_alert_type(self, alert_type: str) -> List[Incident]:
        """Get incidents by alert type."""
        return (
            self.db.query(Incident)
            .filter(Incident.alert_type == alert_type)
            .order_by(desc(Incident.detected_at))
            .all()
        )

    def get_recent(
        self,
        hours: int = 24,
        status: Optional[IncidentStatus] = None,
    ) -> List[Incident]:
        """Get recent incidents."""
        threshold = datetime.utcnow() - timedelta(hours=hours)
        q = self.db.query(Incident).filter(Incident.detected_at >= threshold)

        if status:
            q = q.filter(Incident.status == status.value)

        return q.order_by(desc(Incident.detected_at)).all()

    def acknowledge(
        self,
        id: UUID,
        acknowledged_by: str,
        notes: Optional[str] = None,
    ) -> Optional[Incident]:
        """Acknowledge an incident."""
        incident = self.get(id)
        if not incident:
            return None

        incident.acknowledge(acknowledged_by)
        if notes:
            incident.add_timeline_event(f"Notes: {notes}", acknowledged_by)

        self.db.commit()
        self.db.refresh(incident)
        return incident

    def resolve(
        self,
        id: UUID,
        resolved_by: str,
        resolution: str,
        resolution_type: str = "fixed",
        root_cause: Optional[str] = None,
    ) -> Optional[Incident]:
        """Resolve an incident."""
        incident = self.get(id)
        if not incident:
            return None

        incident.resolve(resolved_by, resolution, resolution_type)
        if root_cause:
            incident.root_cause = root_cause

        self.db.commit()
        self.db.refresh(incident)
        return incident

    def escalate(
        self,
        id: UUID,
        escalate_to: str,
        escalated_by: Optional[str] = None,
        reason: Optional[str] = None,
    ) -> Optional[Incident]:
        """Escalate an incident."""
        incident = self.get(id)
        if not incident:
            return None

        incident.escalate(escalate_to, escalated_by)
        if reason:
            incident.add_timeline_event(f"Escalation reason: {reason}", escalated_by)

        self.db.commit()
        self.db.refresh(incident)
        return incident

    def close(self, id: UUID, closed_by: str) -> Optional[Incident]:
        """Close an incident."""
        incident = self.get(id)
        if not incident:
            return None

        incident.set_status(IncidentStatus.CLOSED, closed_by)

        self.db.commit()
        self.db.refresh(incident)
        return incident

    def add_timeline_event(
        self,
        id: UUID,
        message: str,
        by: Optional[str] = None,
    ) -> Optional[Incident]:
        """Add event to incident timeline."""
        incident = self.get(id)
        if not incident:
            return None

        incident.add_timeline_event(message, by)

        self.db.commit()
        self.db.refresh(incident)
        return incident

    def get_stats(
        self,
        start_date: Optional[datetime] = None,
        end_date: Optional[datetime] = None,
    ) -> dict:
        """Get incident statistics."""
        q = self.db.query(Incident)

        if start_date:
            q = q.filter(Incident.detected_at >= start_date)
        if end_date:
            q = q.filter(Incident.detected_at <= end_date)

        # Count by severity
        severity_counts = (
            q.with_entities(Incident.severity, func.count(Incident.id))
            .group_by(Incident.severity)
            .all()
        )

        # Count by status
        status_counts = (
            q.with_entities(Incident.status, func.count(Incident.id))
            .group_by(Incident.status)
            .all()
        )

        # Calculate average resolution time
        avg_resolution = (
            q.filter(Incident.time_to_resolve_minutes.isnot(None))
            .with_entities(func.avg(Incident.time_to_resolve_minutes))
            .scalar()
        )

        # Calculate average acknowledgement time
        avg_ack = (
            q.filter(Incident.time_to_acknowledge_minutes.isnot(None))
            .with_entities(func.avg(Incident.time_to_acknowledge_minutes))
            .scalar()
        )

        # Count total
        total = q.count()

        # Count open
        open_count = q.filter(
            Incident.status.notin_([
                IncidentStatus.RESOLVED.value,
                IncidentStatus.CLOSED.value,
            ])
        ).count()

        # SLA compliance (simplified - incidents resolved within SLA)
        sla_met = q.filter(Incident.time_to_resolve_minutes.isnot(None)).count()
        sla_compliance = (sla_met / total * 100) if total > 0 else 100.0

        return {
            "total_incidents": total,
            "open_incidents": open_count,
            "critical_incidents": next(
                (c for s, c in severity_counts if s == IncidentSeverity.CRITICAL.value),
                0,
            ),
            "avg_time_to_acknowledge_minutes": float(avg_ack or 0),
            "avg_time_to_resolve_minutes": float(avg_resolution or 0),
            "sla_compliance_percent": round(sla_compliance, 2),
            "incidents_by_severity": {s: c for s, c in severity_counts},
            "incidents_by_status": {s: c for s, c in status_counts},
        }

    def generate_incident_number(self) -> str:
        """Generate a unique incident number."""
        today = datetime.utcnow().strftime("%Y%m%d")

        count = (
            self.db.query(func.count(Incident.id))
            .filter(Incident.incident_number.like(f"INC-{today}%"))
            .scalar()
        )

        return f"INC-{today}-{(count or 0) + 1:04d}"

    def create_incident(self, data: dict) -> Incident:
        """Create a new incident with auto-generated number."""
        if "incident_number" not in data:
            data["incident_number"] = self.generate_incident_number()

        if "detected_at" not in data:
            data["detected_at"] = datetime.utcnow()

        return self.create(data)
