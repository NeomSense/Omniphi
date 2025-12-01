"""Pytest configuration and fixtures for test suite."""

import pytest
import asyncio
from typing import Generator
from sqlalchemy import create_engine
from sqlalchemy.orm import sessionmaker
from fastapi.testclient import TestClient

from app.main import app
from app.db.base_class import Base
from app.db.session import get_db


# Test database URL (using SQLite for tests)
SQLALCHEMY_TEST_DATABASE_URL = "sqlite:///./test.db"


@pytest.fixture(scope="session")
def event_loop():
    """Create event loop for async tests."""
    loop = asyncio.get_event_loop_policy().new_event_loop()
    yield loop
    loop.close()


@pytest.fixture(scope="function")
def db_engine():
    """Create test database engine."""
    engine = create_engine(
        SQLALCHEMY_TEST_DATABASE_URL,
        connect_args={"check_same_thread": False}
    )
    Base.metadata.create_all(bind=engine)
    yield engine
    Base.metadata.drop_all(bind=engine)


@pytest.fixture(scope="function")
def db_session(db_engine):
    """Create test database session."""
    TestingSessionLocal = sessionmaker(
        autocommit=False,
        autoflush=False,
        bind=db_engine
    )
    session = TestingSessionLocal()
    yield session
    session.close()


@pytest.fixture(scope="function")
def client(db_session) -> Generator:
    """Create test client with database override."""
    def override_get_db():
        try:
            yield db_session
        finally:
            db_session.close()

    app.dependency_overrides[get_db] = override_get_db
    with TestClient(app) as test_client:
        yield test_client
    app.dependency_overrides.clear()


@pytest.fixture
def sample_validator_request():
    """Sample validator setup request data."""
    return {
        "wallet_address": "omni1test1234567890abcdefghijklmnopqrstuvwxyz",
        "validator_name": "Test Validator",
        "website": "https://test-validator.com",
        "description": "Test validator for unit tests",
        "commission_rate": 0.10,
        "run_mode": "cloud",
        "provider": "aws"
    }


@pytest.fixture
def sample_heartbeat():
    """Sample local validator heartbeat data."""
    return {
        "wallet_address": "omni1test1234567890abcdefghijklmnopqrstuvwxyz",
        "block_height": 12345,
        "peers": 25,
        "status": "running"
    }
