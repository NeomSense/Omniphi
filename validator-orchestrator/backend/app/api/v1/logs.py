"""Orchestrator Logs API endpoints."""

from datetime import datetime, timedelta
from typing import Optional
import logging
import random
import uuid

from fastapi import APIRouter, Depends, Query
from fastapi.responses import StreamingResponse, PlainTextResponse
from sqlalchemy.orm import Session

from app.db.session import get_db
from app.models import OrchestratorLog, LogLevel, LogSource

logger = logging.getLogger(__name__)

router = APIRouter()


def generate_mock_logs(count: int = 200, source: Optional[str] = None, level: Optional[str] = None):
    """Generate mock log entries for development."""
    levels = ["debug", "info", "info", "info", "warn", "error"]
    sources = ["orchestrator", "provisioning", "health", "docker", "chain"]

    messages_by_source = {
        "orchestrator": [
            "Starting orchestrator service",
            "Configuration loaded successfully",
            "Background worker started",
            "Processed 15 pending requests",
            "Database connection pool refreshed"
        ],
        "provisioning": [
            "Starting provisioning for request req-abc123",
            "Docker container created successfully",
            "Container startup completed in 45s",
            "Consensus key generated",
            "Provisioning failed: timeout after 300s"
        ],
        "health": [
            "Health check completed for 24 nodes",
            "Node node-001 responded in 23ms",
            "Node node-005 failed health check",
            "RPC endpoint check passed",
            "Detected 3 nodes with high CPU usage"
        ],
        "docker": [
            "Container omniphi-validator-001 started",
            "Pulling image omniphi/validator-node:latest",
            "Container logs attached",
            "Network omniphi-network created",
            "Volume mounted successfully"
        ],
        "chain": [
            "Connected to RPC endpoint",
            "Block height: 1,567,234",
            "Syncing: 98.5% complete",
            "New block received",
            "Validator set updated"
        ]
    }

    logs = []
    now = datetime.utcnow()

    for i in range(count):
        log_source = source if source else random.choice(sources)
        log_level = level if level else random.choice(levels)

        logs.append({
            "id": str(uuid.uuid4()),
            "level": log_level,
            "source": log_source,
            "message": random.choice(messages_by_source.get(log_source, ["Log message"])),
            "request_id": f"req-{random.randint(1000, 9999)}" if random.random() > 0.7 else None,
            "node_id": f"node-{random.randint(1, 24):03d}" if random.random() > 0.5 else None,
            "timestamp": (now - timedelta(minutes=count - i, seconds=random.randint(0, 59))).isoformat()
        })

    return logs


@router.get("")
async def list_logs(
    source: Optional[str] = None,
    level: Optional[str] = None,
    limit: int = Query(200, ge=1, le=1000),
    db: Session = Depends(get_db)
):
    """
    Get orchestrator logs with optional filtering.

    Returns:
        List of log entries
    """
    query = db.query(OrchestratorLog)

    # Apply filters
    if source:
        try:
            log_source = LogSource(source)
            query = query.filter(OrchestratorLog.source == log_source)
        except ValueError:
            pass

    if level:
        try:
            log_level = LogLevel(level)
            query = query.filter(OrchestratorLog.level == log_level)
        except ValueError:
            pass

    # Get logs ordered by timestamp
    logs = query.order_by(OrchestratorLog.timestamp.desc()).limit(limit).all()

    # Transform to response format
    if logs:
        return [
            {
                "id": str(log.id),
                "level": log.level.value if hasattr(log.level, 'value') else str(log.level),
                "source": log.source.value if hasattr(log.source, 'value') else str(log.source),
                "message": log.message,
                "request_id": log.request_id,
                "node_id": log.node_id,
                "timestamp": log.timestamp.isoformat()
            }
            for log in logs
        ]

    # Return mock data if no real logs exist
    return generate_mock_logs(limit, source, level)


@router.get("/stream")
async def stream_logs(
    source: Optional[str] = None,
    level: Optional[str] = None
):
    """
    Stream logs via Server-Sent Events (SSE).

    Note: This is a simplified implementation. In production,
    you would use a proper event queue or pub/sub system.
    """
    async def generate():
        """Generate SSE events."""
        levels = ["debug", "info", "info", "info", "warn", "error"]
        sources = ["orchestrator", "provisioning", "health", "docker", "chain"]

        messages = [
            "Processing request",
            "Health check completed",
            "Container started",
            "Block synced",
            "Connection established"
        ]

        import asyncio

        while True:
            # Wait for a bit between events
            await asyncio.sleep(random.uniform(0.5, 2.0))

            log_source = source if source else random.choice(sources)
            log_level = level if level else random.choice(levels)

            log_entry = {
                "id": str(uuid.uuid4()),
                "level": log_level,
                "source": log_source,
                "message": random.choice(messages),
                "request_id": f"req-{random.randint(1000, 9999)}" if random.random() > 0.7 else None,
                "node_id": f"node-{random.randint(1, 24):03d}" if random.random() > 0.5 else None,
                "timestamp": datetime.utcnow().isoformat()
            }

            import json
            yield f"data: {json.dumps(log_entry)}\n\n"

    return StreamingResponse(
        generate(),
        media_type="text/event-stream",
        headers={
            "Cache-Control": "no-cache",
            "Connection": "keep-alive",
            "X-Accel-Buffering": "no"
        }
    )


@router.get("/download")
async def download_logs(
    source: Optional[str] = None,
    level: Optional[str] = None,
    db: Session = Depends(get_db)
):
    """
    Download logs as a text file.
    """
    query = db.query(OrchestratorLog)

    if source:
        try:
            log_source = LogSource(source)
            query = query.filter(OrchestratorLog.source == log_source)
        except ValueError:
            pass

    if level:
        try:
            log_level = LogLevel(level)
            query = query.filter(OrchestratorLog.level == log_level)
        except ValueError:
            pass

    logs = query.order_by(OrchestratorLog.timestamp.desc()).limit(1000).all()

    # Format logs
    if logs:
        lines = [
            f"[{log.timestamp.isoformat()}] [{log.level.value.upper()}] [{log.source.value}] {log.message}"
            for log in logs
        ]
    else:
        # Generate mock logs for download
        mock_logs = generate_mock_logs(500, source, level)
        lines = [
            f"[{log['timestamp']}] [{log['level'].upper()}] [{log['source']}] {log['message']}"
            for log in mock_logs
        ]

    content = "\n".join(lines)

    return PlainTextResponse(
        content=content,
        media_type="text/plain",
        headers={
            "Content-Disposition": f"attachment; filename=orchestrator-logs-{datetime.utcnow().strftime('%Y%m%d-%H%M%S')}.log"
        }
    )
