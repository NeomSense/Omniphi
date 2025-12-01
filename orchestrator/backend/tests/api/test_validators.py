"""Tests for validator API endpoints."""

import pytest
from fastapi.testclient import TestClient


class TestValidatorSetupEndpoints:
    """Tests for validator setup request endpoints."""

    def test_create_setup_request(self, client: TestClient, sample_validator_request):
        """Test creating a new validator setup request."""
        response = client.post(
            "/api/v1/validators/setup-requests",
            json=sample_validator_request
        )

        assert response.status_code == 200
        data = response.json()

        assert data["wallet_address"] == sample_validator_request["wallet_address"]
        assert data["validator_name"] == sample_validator_request["validator_name"]
        assert data["status"] == "pending"
        assert "id" in data
        assert "created_at" in data

    def test_create_setup_request_invalid_wallet(self, client: TestClient):
        """Test creating setup request with invalid wallet address."""
        invalid_request = {
            "wallet_address": "invalid_address",
            "validator_name": "Test",
            "commission_rate": 0.10,
            "run_mode": "cloud",
            "provider": "aws"
        }

        response = client.post(
            "/api/v1/validators/setup-requests",
            json=invalid_request
        )

        # Should validate wallet address format
        assert response.status_code == 422

    def test_get_setup_request(self, client: TestClient, sample_validator_request):
        """Test retrieving a setup request by ID."""
        # Create a request first
        create_response = client.post(
            "/api/v1/validators/setup-requests",
            json=sample_validator_request
        )
        request_id = create_response.json()["id"]

        # Retrieve it
        response = client.get(f"/api/v1/validators/setup-requests/{request_id}")

        assert response.status_code == 200
        data = response.json()
        assert data["id"] == request_id

    def test_get_requests_by_wallet(self, client: TestClient, sample_validator_request):
        """Test retrieving setup requests by wallet address."""
        # Create a request
        client.post(
            "/api/v1/validators/setup-requests",
            json=sample_validator_request
        )

        # Get requests for this wallet
        response = client.get(
            f"/api/v1/validators/by-wallet/{sample_validator_request['wallet_address']}"
        )

        assert response.status_code == 200
        data = response.json()
        assert len(data) > 0
        assert data[0]["wallet_address"] == sample_validator_request["wallet_address"]


class TestLocalValidatorEndpoints:
    """Tests for local validator heartbeat endpoints."""

    def test_submit_heartbeat(self, client: TestClient, sample_heartbeat):
        """Test submitting a local validator heartbeat."""
        response = client.post(
            "/api/v1/validators/local/heartbeat",
            json=sample_heartbeat
        )

        assert response.status_code == 200
        data = response.json()

        assert data["wallet_address"] == sample_heartbeat["wallet_address"]
        assert data["block_height"] == sample_heartbeat["block_height"]
        assert "timestamp" in data

    def test_get_local_validators(self, client: TestClient, sample_heartbeat):
        """Test retrieving list of local validators."""
        # Submit a heartbeat first
        client.post(
            "/api/v1/validators/local/heartbeat",
            json=sample_heartbeat
        )

        # Get local validators
        response = client.get("/api/v1/validators/local")

        assert response.status_code == 200
        data = response.json()
        assert len(data) > 0

    def test_heartbeat_updates_existing(self, client: TestClient, sample_heartbeat):
        """Test that subsequent heartbeats update existing record."""
        # Submit first heartbeat
        response1 = client.post(
            "/api/v1/validators/local/heartbeat",
            json=sample_heartbeat
        )
        first_id = response1.json()["id"]

        # Submit second heartbeat with updated height
        updated_heartbeat = sample_heartbeat.copy()
        updated_heartbeat["block_height"] = 12346

        response2 = client.post(
            "/api/v1/validators/local/heartbeat",
            json=updated_heartbeat
        )
        second_id = response2.json()["id"]

        # Should update same record
        assert first_id == second_id
        assert response2.json()["block_height"] == 12346


class TestHealthEndpoints:
    """Tests for health check endpoints."""

    def test_health_check(self, client: TestClient):
        """Test health check endpoint."""
        response = client.get("/api/v1/health")

        assert response.status_code == 200
        data = response.json()

        assert data["status"] == "ok"
        assert "timestamp" in data
        assert "version" in data
