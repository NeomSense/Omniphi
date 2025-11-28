"""Main FastAPI application."""

import logging
from fastapi import FastAPI, Request
from fastapi.middleware.cors import CORSMiddleware
from slowapi import Limiter, _rate_limit_exceeded_handler
from slowapi.util import get_remote_address
from slowapi.errors import RateLimitExceeded

from app.core.config import settings
from app.api.v1 import (
    validators,
    health,
    auth,
    nodes,
    logs,
    settings as settings_api,
    audit,
    alerts,
    setup_requests,
    regions,
    upgrades,
    billing,
    providers,
    snapshots,
    migration,
    capacity,
)

# Configure logging
logging.basicConfig(
    level=logging.INFO if not settings.DEBUG else logging.DEBUG,
    format='%(asctime)s - %(name)s - %(levelname)s - %(message)s'
)

logger = logging.getLogger(__name__)

# Initialize rate limiter
limiter = Limiter(
    key_func=get_remote_address,
    default_limits=[f"{settings.RATE_LIMIT_PER_MINUTE}/minute", f"{settings.RATE_LIMIT_PER_HOUR}/hour"],
    enabled=settings.RATE_LIMIT_ENABLED
)

# Create FastAPI app
app = FastAPI(
    title=settings.PROJECT_NAME,
    version=settings.VERSION,
    description="Production-grade validator orchestration system for Omniphi blockchain",
    openapi_url=f"{settings.API_V1_STR}/openapi.json",
    docs_url="/docs",
    redoc_url="/redoc"
)

# Add rate limiter to app state
app.state.limiter = limiter
app.add_exception_handler(RateLimitExceeded, _rate_limit_exceeded_handler)

# Configure CORS
app.add_middleware(
    CORSMiddleware,
    allow_origins=["*"],  # Temporarily allow all origins for testing
    allow_credentials=True,
    allow_methods=["*"],
    allow_headers=["*"],
)

# Include API v1 routers
app.include_router(
    validators.router,
    prefix=f"{settings.API_V1_STR}/validators",
    tags=["validators"]
)

app.include_router(
    health.router,
    prefix=settings.API_V1_STR,
    tags=["health"]
)

app.include_router(
    auth.router,
    prefix=f"{settings.API_V1_STR}/auth",
    tags=["authentication"]
)

app.include_router(
    nodes.router,
    prefix=f"{settings.API_V1_STR}/nodes",
    tags=["nodes"]
)

app.include_router(
    logs.router,
    prefix=f"{settings.API_V1_STR}/logs",
    tags=["logs"]
)

app.include_router(
    settings_api.router,
    prefix=f"{settings.API_V1_STR}/settings",
    tags=["settings"]
)

app.include_router(
    audit.router,
    prefix=f"{settings.API_V1_STR}/audit",
    tags=["audit"]
)

app.include_router(
    alerts.router,
    prefix=f"{settings.API_V1_STR}/alerts",
    tags=["alerts"]
)

app.include_router(
    setup_requests.router,
    prefix=f"{settings.API_V1_STR}/setup-requests",
    tags=["setup-requests"]
)

# Multi-Region Infrastructure (Module 1)
app.include_router(
    regions.router,
    prefix=settings.API_V1_STR,
    tags=["regions"]
)

# Validator Upgrade Pipeline (Module 2)
app.include_router(
    upgrades.router,
    prefix=settings.API_V1_STR,
    tags=["upgrades"]
)

# Billing System (Module 4)
app.include_router(
    billing.router,
    prefix=settings.API_V1_STR,
    tags=["billing"]
)

# Provider Management (Module 3, 5, 6)
app.include_router(
    providers.router,
    prefix=settings.API_V1_STR,
    tags=["providers"]
)

# Snapshot Server (Module 10)
app.include_router(
    snapshots.router,
    prefix=settings.API_V1_STR,
    tags=["snapshots"]
)

# Migration & Failover (Module 8)
app.include_router(
    migration.router,
    prefix=settings.API_V1_STR,
    tags=["migration", "failover"]
)

# Autoscaling & Capacity Management (Module 7)
app.include_router(
    capacity.router,
    prefix=settings.API_V1_STR,
    tags=["capacity", "autoscaling"]
)


@app.get("/")
async def root():
    """
    Root endpoint.

    Returns basic API information and links to documentation.
    """
    logger.info("Root endpoint accessed")

    return {
        "message": "Omniphi Validator Orchestrator API",
        "version": settings.VERSION,
        "docs": "/docs",
        "redoc": "/redoc",
        "health": f"{settings.API_V1_STR}/health",
        "endpoints": {
            "validators": f"{settings.API_V1_STR}/validators",
            "health": f"{settings.API_V1_STR}/health",
            "auth": f"{settings.API_V1_STR}/auth",
            "nodes": f"{settings.API_V1_STR}/nodes",
            "logs": f"{settings.API_V1_STR}/logs",
            "settings": f"{settings.API_V1_STR}/settings",
            "audit": f"{settings.API_V1_STR}/audit",
            "alerts": f"{settings.API_V1_STR}/alerts",
            "setup_requests": f"{settings.API_V1_STR}/setup-requests",
            "regions": f"{settings.API_V1_STR}/regions",
            "upgrades": f"{settings.API_V1_STR}/upgrades",
            "billing": f"{settings.API_V1_STR}/billing",
            "providers": f"{settings.API_V1_STR}/providers",
            "snapshots": f"{settings.API_V1_STR}/snapshots",
            "migration": f"{settings.API_V1_STR}/migration",
            "failover": f"{settings.API_V1_STR}/failover",
            "capacity": f"{settings.API_V1_STR}/capacity",
        }
    }


@app.on_event("startup")
async def startup_event():
    """
    Application startup event.

    Initialize connections, start background workers, etc.
    """
    logger.info("=" * 60)
    logger.info(f"Starting {settings.PROJECT_NAME} v{settings.VERSION}")
    logger.info(f"Debug mode: {settings.DEBUG}")
    logger.info(f"API prefix: {settings.API_V1_STR}")
    logger.info(f"CORS Origins: {settings.BACKEND_CORS_ORIGINS}")
    logger.info("=" * 60)


@app.on_event("shutdown")
async def shutdown_event():
    """
    Application shutdown event.

    Clean up resources, close connections, etc.
    """
    logger.info("Shutting down Omniphi Validator Orchestrator")


if __name__ == "__main__":
    import uvicorn
    uvicorn.run(
        "app.main:app",
        host="0.0.0.0",
        port=8000,
        reload=settings.DEBUG,
        log_level="info" if not settings.DEBUG else "debug"
    )
