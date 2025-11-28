# Test Suite for Omniphi Validator Orchestrator

**Comprehensive testing framework for the validator orchestrator backend.**

---

## Overview

This test suite covers:
- **API endpoints** - FastAPI route testing
- **Services** - Business logic and cloud providers
- **Integration** - End-to-end workflows
- **Performance** - Load and stress testing

---

## Quick Start

### Install Test Dependencies

```bash
cd validator-orchestrator/backend

# Activate virtual environment
source venv/bin/activate  # Linux/macOS
.\venv\Scripts\Activate.ps1  # Windows PowerShell

# Install pytest and dependencies
pip install pytest pytest-asyncio pytest-cov httpx
```

---

### Run Tests

```bash
# Run all tests
pytest

# Run with coverage
pytest --cov=app --cov-report=html

# Run specific test file
pytest tests/api/test_validators.py

# Run specific test class
pytest tests/api/test_validators.py::TestValidatorSetupEndpoints

# Run specific test
pytest tests/api/test_validators.py::TestValidatorSetupEndpoints::test_create_setup_request

# Run with verbose output
pytest -v

# Run with output (show print statements)
pytest -s
```

---

## Test Structure

```
tests/
├── __init__.py                       # Test package
├── conftest.py                       # Pytest fixtures and configuration
├── README.md                         # This file
│
├── api/                              # API endpoint tests
│   ├── test_validators.py           # Validator endpoints
│   ├── test_health.py                # Health check endpoints
│   └── test_auth.py                  # Authentication endpoints
│
├── services/                         # Service layer tests
│   ├── test_slashing_protection.py  # Slashing protection service
│   ├── test_auto_failover.py        # Auto-failover service
│   ├── test_aws_ec2.py               # AWS EC2 provider
│   └── test_digitalocean.py          # DigitalOcean provider
│
└── integration/                      # End-to-end tests
    ├── test_validator_lifecycle.py  # Full validator setup flow
    └── test_failover_scenarios.py   # Failover workflows
```

---

## Writing Tests

### Test Fixtures (conftest.py)

Available fixtures:
- `client` - TestClient for API testing
- `db_session` - Database session
- `db_engine` - Database engine
- `sample_validator_request` - Example validator request data
- `sample_heartbeat` - Example heartbeat data

### Example Test

```python
import pytest
from fastapi.testclient import TestClient

class TestMyFeature:
    """Tests for my feature."""

    def test_basic_functionality(self, client: TestClient):
        """Test basic functionality."""
        response = client.get("/api/v1/my-endpoint")

        assert response.status_code == 200
        data = response.json()
        assert data["status"] == "ok"

    @pytest.mark.asyncio
    async def test_async_functionality(self):
        """Test async functionality."""
        # Async test code here
        pass

    def test_with_fixture(self, sample_validator_request):
        """Test using custom fixture."""
        assert sample_validator_request["commission_rate"] == 0.10
```

---

## Test Categories

### 1. Unit Tests

**Location:** `tests/services/`

**Purpose:** Test individual functions and classes in isolation

**Example:**
```python
def test_validate_wallet_address():
    """Test wallet address validation."""
    valid = "omni1abc123..."
    invalid = "invalid_address"

    assert is_valid_wallet_address(valid) is True
    assert is_valid_wallet_address(invalid) is False
```

---

### 2. API Tests

**Location:** `tests/api/`

**Purpose:** Test HTTP endpoints and request/response handling

**Example:**
```python
def test_create_validator(client: TestClient):
    """Test validator creation endpoint."""
    response = client.post("/api/v1/validators", json={
        "name": "test-validator",
        "wallet": "omni1..."
    })

    assert response.status_code == 200
```

---

### 3. Integration Tests

**Location:** `tests/integration/`

**Purpose:** Test complete workflows across multiple components

**Example:**
```python
@pytest.mark.asyncio
async def test_full_validator_lifecycle():
    """Test complete validator setup to activation."""
    # 1. Create setup request
    # 2. Provision cloud instance
    # 3. Submit heartbeat
    # 4. Verify status
    pass
```

---

### 4. Performance Tests

**Location:** `tests/performance/` (create as needed)

**Purpose:** Load testing and benchmarking

**Example:**
```python
def test_api_performance(client: TestClient):
    """Test API can handle 100 concurrent requests."""
    import concurrent.futures

    def make_request():
        return client.get("/api/v1/health")

    with concurrent.futures.ThreadPoolExecutor(max_workers=100) as executor:
        futures = [executor.submit(make_request) for _ in range(100)]
        results = [f.result() for f in futures]

    assert all(r.status_code == 200 for r in results)
```

---

## Mocking External Services

### Mocking AWS

```python
import pytest
from unittest.mock import Mock, patch

@patch('boto3.client')
def test_aws_provisioning(mock_boto_client):
    """Test AWS EC2 provisioning with mocked boto3."""
    mock_ec2 = Mock()
    mock_boto_client.return_value = mock_ec2

    # Configure mock responses
    mock_ec2.run_instances.return_value = {
        'Instances': [{
            'InstanceId': 'i-test123',
            'PublicIpAddress': '1.2.3.4'
        }]
    }

    # Test provisioning
    # ...
```

---

### Mocking DigitalOcean

```python
import pytest
from unittest.mock import AsyncMock, patch

@pytest.mark.asyncio
@patch('httpx.AsyncClient')
async def test_digitalocean_provisioning(mock_client):
    """Test DigitalOcean provisioning with mocked httpx."""
    mock_response = AsyncMock()
    mock_response.status_code = 202
    mock_response.json.return_value = {
        'droplet': {
            'id': 12345,
            'name': 'test-droplet'
        }
    }

    mock_client.return_value.__aenter__.return_value.post.return_value = mock_response

    # Test provisioning
    # ...
```

---

## Coverage Reports

### Generate Coverage Report

```bash
# Run tests with coverage
pytest --cov=app --cov-report=html --cov-report=term

# Open HTML report
# Linux/macOS:
open htmlcov/index.html

# Windows:
start htmlcov/index.html
```

### Coverage Goals

- **Overall:** > 80%
- **Critical paths:** > 95%
  - Slashing protection
  - Validator provisioning
  - API authentication

---

## Continuous Integration

### GitHub Actions Example

**.github/workflows/tests.yml:**
```yaml
name: Tests

on: [push, pull_request]

jobs:
  test:
    runs-on: ubuntu-latest

    steps:
      - uses: actions/checkout@v3

      - name: Set up Python
        uses: actions/setup-python@v4
        with:
          python-version: '3.11'

      - name: Install dependencies
        run: |
          cd validator-orchestrator/backend
          pip install -r requirements.txt
          pip install pytest pytest-cov pytest-asyncio

      - name: Run tests
        run: |
          cd validator-orchestrator/backend
          pytest --cov=app --cov-report=xml

      - name: Upload coverage
        uses: codecov/codecov-action@v3
        with:
          files: ./validator-orchestrator/backend/coverage.xml
```

---

## Testing Best Practices

### 1. Test Naming

```python
# Good
def test_create_validator_success():
    """Test successful validator creation."""
    pass

def test_create_validator_invalid_wallet():
    """Test validator creation fails with invalid wallet."""
    pass

# Bad
def test_validator():  # Too vague
    pass

def test_1():  # Not descriptive
    pass
```

---

### 2. Arrange-Act-Assert Pattern

```python
def test_heartbeat_updates_block_height():
    """Test heartbeat updates block height."""
    # Arrange
    initial_height = 1000
    new_height = 1001

    # Act
    response = submit_heartbeat(height=new_height)

    # Assert
    assert response.block_height == new_height
    assert response.block_height > initial_height
```

---

### 3. Use Fixtures for Setup

```python
@pytest.fixture
def validator_with_data():
    """Create validator with test data."""
    validator = create_validator()
    validator.block_height = 5000
    validator.peers = 25
    return validator

def test_validator_health(validator_with_data):
    """Test validator health check."""
    assert validator_with_data.is_healthy() is True
```

---

### 4. Test Edge Cases

```python
def test_heartbeat_validation():
    """Test heartbeat validation edge cases."""
    # Zero height
    with pytest.raises(ValueError):
        submit_heartbeat(block_height=0)

    # Negative peers
    with pytest.raises(ValueError):
        submit_heartbeat(peers=-1)

    # Very large numbers
    submit_heartbeat(block_height=999999999)  # Should work
```

---

## Debugging Failed Tests

### Run Single Test with Debug Output

```bash
# With pytest debugging
pytest tests/api/test_validators.py::test_create_setup_request -s -v

# With Python debugger
pytest --pdb tests/api/test_validators.py::test_create_setup_request
```

### View Detailed Error Output

```bash
# Show full diff on assertion failures
pytest -vv

# Show local variables on failure
pytest -l
```

---

## Test Data Management

### Using Factories (Optional)

```python
# tests/factories.py
from factory import Factory, Faker
from app.models import ValidatorSetupRequest

class ValidatorSetupRequestFactory(Factory):
    class Meta:
        model = ValidatorSetupRequest

    wallet_address = Faker('sha256')
    validator_name = Faker('company')
    commission_rate = 0.10

# In tests:
def test_with_factory():
    validator = ValidatorSetupRequestFactory()
    assert validator.commission_rate == 0.10
```

---

## Running Specific Test Suites

```bash
# Run only fast tests (mark with @pytest.mark.fast)
pytest -m fast

# Run only slow tests
pytest -m slow

# Skip integration tests
pytest -m "not integration"

# Run only critical tests
pytest -m critical
```

---

## Summary

**Test Command Reference:**

```bash
# Development
pytest -v                      # Verbose output
pytest -s                      # Show print statements
pytest --pdb                   # Debug on failure

# Coverage
pytest --cov=app              # Basic coverage
pytest --cov=app --cov-report=html  # HTML report

# Specific tests
pytest tests/api/              # All API tests
pytest -k "test_create"        # Tests matching name

# Continuous Integration
pytest --cov=app --cov-report=xml  # For CI/CD
```

**Coverage Goals:**
- API endpoints: > 90%
- Critical services: > 95%
- Overall: > 80%

**Best Practices:**
- Write tests first (TDD when possible)
- Test edge cases and error conditions
- Use descriptive test names
- Keep tests fast and isolated
- Mock external services

---

**For more information:**
- [Pytest Documentation](https://docs.pytest.org/)
- [FastAPI Testing](https://fastapi.tiangolo.com/tutorial/testing/)
- [Coverage.py](https://coverage.readthedocs.io/)

---

**Last Updated:** 2025-11-20
