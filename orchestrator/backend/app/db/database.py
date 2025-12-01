"""
Unified Database Configuration for Omniphi Cloud Database

This module provides the core database configuration including:
- Engine creation with connection pooling
- Session factory and dependency injection
- Base class for all models
- Support for both SQLite (MVP) and PostgreSQL (Production)

Usage:
    from app.db.database import get_db, engine, SessionLocal, Base

    # FastAPI dependency injection
    @app.get("/items")
    def read_items(db: Session = Depends(get_db)):
        ...
"""

from contextlib import contextmanager
from typing import Generator, Optional

from sqlalchemy import create_engine, event, text
from sqlalchemy.engine import Engine
from sqlalchemy.orm import Session, sessionmaker, declarative_base
from sqlalchemy.pool import StaticPool, QueuePool

from app.core.config import settings


def get_engine_config() -> dict:
    """
    Get SQLAlchemy engine configuration based on database type.

    Returns optimized settings for SQLite (dev/MVP) or PostgreSQL (production).
    """
    db_url = settings.SQLALCHEMY_DATABASE_URI
    is_sqlite = db_url.startswith("sqlite")

    if is_sqlite:
        # SQLite configuration for MVP/development
        return {
            "connect_args": {"check_same_thread": False},
            "poolclass": StaticPool,
            "echo": settings.DEBUG,
        }
    else:
        # PostgreSQL configuration for production
        return {
            "pool_pre_ping": True,  # Reconnect on stale connections
            "pool_size": 10,        # Base pool size
            "max_overflow": 20,     # Additional connections when needed
            "pool_timeout": 30,     # Seconds to wait for connection
            "pool_recycle": 1800,   # Recycle connections after 30 min
            "echo": settings.DEBUG,
            "poolclass": QueuePool,
        }


# Create SQLAlchemy engine with appropriate configuration
engine = create_engine(
    settings.SQLALCHEMY_DATABASE_URI,
    **get_engine_config()
)


# Enable foreign key constraints for SQLite
@event.listens_for(Engine, "connect")
def set_sqlite_pragma(dbapi_connection, connection_record):
    """Enable foreign key constraints for SQLite databases."""
    if settings.SQLALCHEMY_DATABASE_URI.startswith("sqlite"):
        cursor = dbapi_connection.cursor()
        cursor.execute("PRAGMA foreign_keys=ON")
        cursor.close()


# Create session factory
SessionLocal = sessionmaker(
    autocommit=False,
    autoflush=False,
    bind=engine,
    expire_on_commit=False,  # Don't expire objects after commit for better performance
)


# Base class for all ORM models
Base = declarative_base()


def get_db() -> Generator[Session, None, None]:
    """
    FastAPI dependency for database session injection.

    Yields a database session and ensures it's closed after use.

    Usage:
        @app.get("/items")
        def read_items(db: Session = Depends(get_db)):
            return db.query(Item).all()
    """
    db = SessionLocal()
    try:
        yield db
    finally:
        db.close()


@contextmanager
def get_db_context() -> Generator[Session, None, None]:
    """
    Context manager for database sessions outside of FastAPI routes.

    Usage:
        with get_db_context() as db:
            db.query(Model).all()
    """
    db = SessionLocal()
    try:
        yield db
        db.commit()
    except Exception:
        db.rollback()
        raise
    finally:
        db.close()


def init_db() -> None:
    """
    Initialize database by creating all tables.

    This should be called once at application startup for development.
    For production, use Alembic migrations.
    """
    # Import all models to register them with Base.metadata
    from app.models import (  # noqa: F401
        validator_setup_request,
        validator_node,
        local_validator_heartbeat,
        orchestrator_settings,
        audit_log,
        alert,
        orchestrator_log,
    )
    from app.db.models import (  # noqa: F401
        region,
        region_server,
        server_pool,
        provider,
        billing,
        snapshot,
        upgrade,
        node_metrics,
        incident,
    )

    Base.metadata.create_all(bind=engine)


def check_db_connection() -> bool:
    """
    Check if database connection is healthy.

    Returns:
        True if connection is successful, False otherwise.
    """
    try:
        with engine.connect() as conn:
            conn.execute(text("SELECT 1"))
        return True
    except Exception:
        return False


class DatabaseManager:
    """
    Database manager for advanced operations.

    Provides utilities for:
    - Connection health checks
    - Transaction management
    - Bulk operations
    """

    def __init__(self, session: Optional[Session] = None):
        self._session = session

    @property
    def session(self) -> Session:
        """Get current session or create new one."""
        if self._session is None:
            self._session = SessionLocal()
        return self._session

    def close(self) -> None:
        """Close session if exists."""
        if self._session:
            self._session.close()
            self._session = None

    def health_check(self) -> dict:
        """
        Perform comprehensive health check.

        Returns:
            Dictionary with health status and metrics.
        """
        result = {
            "healthy": False,
            "database_type": "postgresql" if not settings.SQLALCHEMY_DATABASE_URI.startswith("sqlite") else "sqlite",
            "pool_status": {},
            "error": None,
        }

        try:
            with engine.connect() as conn:
                conn.execute(text("SELECT 1"))

            result["healthy"] = True

            # Get pool status for PostgreSQL
            if hasattr(engine.pool, "status"):
                result["pool_status"] = {
                    "pool_size": engine.pool.size(),
                    "checked_in": engine.pool.checkedin(),
                    "checked_out": engine.pool.checkedout(),
                    "overflow": engine.pool.overflow(),
                }
        except Exception as e:
            result["error"] = str(e)

        return result

    def execute_raw(self, sql: str, params: Optional[dict] = None) -> list:
        """
        Execute raw SQL query.

        Args:
            sql: SQL query string
            params: Optional parameters for the query

        Returns:
            List of result rows
        """
        with engine.connect() as conn:
            result = conn.execute(text(sql), params or {})
            return [dict(row._mapping) for row in result]


# Export commonly used items
__all__ = [
    "engine",
    "SessionLocal",
    "Base",
    "get_db",
    "get_db_context",
    "init_db",
    "check_db_connection",
    "DatabaseManager",
]
