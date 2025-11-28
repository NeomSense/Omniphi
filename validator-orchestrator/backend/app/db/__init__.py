"""
Omniphi Cloud Database Layer

This package provides the complete database infrastructure for the
Omniphi Cloud + Validator Orchestrator ecosystem.

Modules:
- database: Core configuration (engine, session, Base)
- models: SQLAlchemy ORM models
- schemas: Pydantic request/response schemas
- crud: Repository classes for database operations
- seed: Default data seeding

Quick Start:
    # Database session dependency
    from app.db.database import get_db

    # Import models
    from app.db.models import Region, ValidatorNode, Provider

    # Import schemas
    from app.db.schemas import RegionResponse, ValidatorNodeCreate

    # Import repositories
    from app.db.crud import RegionRepository, ValidatorNodeRepository

    # Use in FastAPI endpoint
    @app.get("/regions")
    def list_regions(db: Session = Depends(get_db)):
        repo = RegionRepository(db)
        return repo.get_active()
"""

from app.db.database import (
    engine,
    SessionLocal,
    Base,
    get_db,
    get_db_context,
    init_db,
    check_db_connection,
    DatabaseManager,
)

__all__ = [
    # Core database
    "engine",
    "SessionLocal",
    "Base",
    "get_db",
    "get_db_context",
    "init_db",
    "check_db_connection",
    "DatabaseManager",
]
