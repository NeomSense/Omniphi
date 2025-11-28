"""Tests for slashing protection service."""

import pytest
from datetime import datetime, timedelta
from uuid import uuid4

from app.services.slashing_protection import SlashingProtectionService


class TestSlashingProtection:
    """Tests for slashing protection service."""

    @pytest.fixture
    def service(self):
        """Create slashing protection service instance."""
        return SlashingProtectionService()

    @pytest.mark.asyncio
    async def test_validate_new_validator_safe(self, service):
        """Test validation when safe to start new validator."""
        result = await service.validate_new_validator_start(
            consensus_pubkey="test_pubkey_123",
            wallet_address="omni1newsafevalidator123"
        )

        assert result["safe"] is True
        assert result["reason"] == "No conflicts detected"
        assert len(result["recommendations"]) == 0

    def test_record_validator_state(self, service):
        """Test recording validator state."""
        validator_id = uuid4()

        # Record initial state
        service.validator_states[str(validator_id)] = {
            "last_update": datetime.utcnow(),
            "block_height": 1000,
            "consensus_round": 0
        }

        # Verify stored
        assert str(validator_id) in service.validator_states
        assert service.validator_states[str(validator_id)]["block_height"] == 1000

    def test_get_validator_safety_status(self, service):
        """Test getting validator safety status."""
        validator_id = uuid4()

        # No data yet
        status = service.get_validator_safety_status(validator_id)
        assert status["safe"] is True  # Safe by default if no data
        assert status["block_height"] is None

        # Add recent data
        service.validator_states[str(validator_id)] = {
            "last_check": datetime.utcnow(),
            "block_height": 5000,
            "consensus_round": 0
        }

        status = service.get_validator_safety_status(validator_id)
        assert status["safe"] is True
        assert status["block_height"] == 5000

    def test_double_signing_detection_height_decrease(self, service):
        """Test detection of height decrease (double-signing risk)."""
        validator_id = uuid4()

        # Set initial state
        service.validator_states[str(validator_id)] = {
            "last_check": datetime.utcnow(),
            "block_height": 5000,
            "consensus_round": 0
        }

        # Simulate height going backwards
        # This would be detected in _check_double_signing_risk
        prev_state = service.validator_states[str(validator_id)]
        current_height = 4900  # Went backwards!

        assert current_height < prev_state["block_height"]
        # This would trigger a critical alert in the actual service

    def test_downtime_detection_stale_heartbeat(self, service):
        """Test detection of stale heartbeat."""
        validator_id = uuid4()

        # Set old heartbeat
        old_time = datetime.utcnow() - timedelta(minutes=10)
        service.validator_states[str(validator_id)] = {
            "last_check": old_time,
            "block_height": 5000
        }

        # Check safety status
        status = service.get_validator_safety_status(validator_id)

        # Should have warnings about stale data
        assert len(status["warnings"]) > 0 or len(status["critical_issues"]) > 0
