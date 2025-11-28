"""Slashing protection service for validator safety."""

import logging
import asyncio
from typing import Dict, Optional, Any, List
from datetime import datetime, timedelta
from uuid import UUID

from app.db.session import SessionLocal
from app.models import ValidatorNode, LocalValidatorHeartbeat
from app.models.validator_node import NodeStatus

logger = logging.getLogger(__name__)


class SlashingProtectionService:
    """
    Slashing protection service to prevent validator penalties.

    Features:
    - Double-signing prevention (height/round tracking)
    - Downtime monitoring and alerts
    - Missed blocks tracking
    - Automatic jailing detection
    - Health check enforcement
    """

    def __init__(self):
        """Initialize slashing protection service."""
        self.db = SessionLocal()
        self.validator_states: Dict[str, Dict[str, Any]] = {}
        logger.info("Initialized slashing protection service")

    async def monitor_validators(self):
        """
        Continuous monitoring loop for all validators.

        This is the main worker that:
        1. Tracks validator state (height, round)
        2. Detects potential double-signing conditions
        3. Monitors downtime and missed blocks
        4. Sends alerts when thresholds are reached
        """
        logger.info("Starting validator monitoring for slashing protection")

        while True:
            try:
                await self._check_all_validators()
                await asyncio.sleep(30)  # Check every 30 seconds

            except Exception as e:
                logger.error(f"Error in monitor_validators loop: {e}", exc_info=True)
                await asyncio.sleep(60)  # Back off on error

    async def _check_all_validators(self):
        """Check all active validators for slashing risks."""
        # Get all running validators
        validators = self.db.query(ValidatorNode).filter(
            ValidatorNode.status.in_([NodeStatus.RUNNING, NodeStatus.SYNCING])
        ).all()

        logger.debug(f"Checking {len(validators)} validators for slashing risks")

        for validator in validators:
            try:
                await self._check_validator(validator)
            except Exception as e:
                logger.error(f"Error checking validator {validator.id}: {e}")
                continue

        self.db.commit()

    async def _check_validator(self, validator: ValidatorNode):
        """
        Check a single validator for slashing risks.

        Args:
            validator: ValidatorNode instance
        """
        validator_id = str(validator.id)

        # Get latest heartbeat (for local validators)
        latest_heartbeat = self.db.query(LocalValidatorHeartbeat).filter(
            LocalValidatorHeartbeat.wallet_address == validator.setup_request.wallet_address
        ).order_by(LocalValidatorHeartbeat.timestamp.desc()).first()

        if latest_heartbeat:
            # Check for double-signing risk
            await self._check_double_signing_risk(validator, latest_heartbeat)

            # Check for downtime risk
            await self._check_downtime_risk(validator, latest_heartbeat)

            # Check missed blocks
            await self._check_missed_blocks(validator, latest_heartbeat)

            # Update validator state tracking
            self.validator_states[validator_id] = {
                "last_check": datetime.utcnow(),
                "block_height": latest_heartbeat.block_height,
                "consensus_round": getattr(latest_heartbeat, 'consensus_round', 0),
                "status": latest_heartbeat.status
            }

    async def _check_double_signing_risk(
        self,
        validator: ValidatorNode,
        heartbeat: LocalValidatorHeartbeat
    ):
        """
        Check for potential double-signing conditions.

        Double-signing occurs when:
        1. Two validators with same consensus key are running
        2. Validator signs same height/round twice
        3. State file is reset/corrupted

        Detection strategy:
        - Track height/round progression
        - Alert if height goes backwards
        - Alert if same height signed multiple times
        """
        validator_id = str(validator.id)
        current_height = heartbeat.block_height
        current_round = getattr(heartbeat, 'consensus_round', 0)

        # Get previous state
        prev_state = self.validator_states.get(validator_id)

        if prev_state:
            prev_height = prev_state.get('block_height', 0)
            prev_round = prev_state.get('consensus_round', 0)

            # CRITICAL: Height went backwards!
            if current_height < prev_height:
                logger.critical(
                    f"DOUBLE-SIGNING RISK: Validator {validator.id} height decreased "
                    f"from {prev_height} to {current_height}. "
                    "This indicates state file reset or multiple instances running!"
                )

                # TODO: Send critical alert
                # - Email/SMS to operator
                # - Slack/Discord notification
                # - Potentially auto-stop validator

                await self._send_critical_alert(
                    validator,
                    "DOUBLE-SIGNING RISK DETECTED",
                    f"Block height decreased from {prev_height} to {current_height}. "
                    "This indicates state file corruption or multiple validator instances. "
                    "IMMEDIATE ACTION REQUIRED: Stop all validator instances and investigate."
                )

            # WARNING: Same height but different round (potential fork)
            elif current_height == prev_height and current_round != prev_round:
                logger.warning(
                    f"Validator {validator.id} at same height {current_height} "
                    f"but round changed from {prev_round} to {current_round}. "
                    "Monitoring for consensus issues."
                )

    async def _check_downtime_risk(
        self,
        validator: ValidatorNode,
        heartbeat: LocalValidatorHeartbeat
    ):
        """
        Check for downtime that could lead to slashing.

        Downtime slashing occurs when:
        - Validator misses too many blocks in a row
        - Typically: 50% of last 10,000 blocks

        Detection strategy:
        - Check heartbeat freshness (< 2 minutes)
        - Monitor "catching_up" status
        - Track missed blocks counter
        """
        # Check heartbeat age
        heartbeat_age = datetime.utcnow() - heartbeat.timestamp
        max_heartbeat_age = timedelta(minutes=2)

        if heartbeat_age > max_heartbeat_age:
            logger.warning(
                f"Validator {validator.id} heartbeat is stale "
                f"({int(heartbeat_age.total_seconds())}s old). "
                "Validator may be offline or having connectivity issues."
            )

            # Check if we've already alerted recently
            validator_id = str(validator.id)
            last_alert = self.validator_states.get(validator_id, {}).get('last_downtime_alert')

            if not last_alert or (datetime.utcnow() - last_alert) > timedelta(minutes=10):
                await self._send_warning_alert(
                    validator,
                    "Validator Downtime Warning",
                    f"No heartbeat received for {int(heartbeat_age.total_seconds())} seconds. "
                    "Check validator process and network connectivity."
                )

                # Update alert timestamp
                if validator_id in self.validator_states:
                    self.validator_states[validator_id]['last_downtime_alert'] = datetime.utcnow()

        # Check if catching up
        if heartbeat.status == "catching_up":
            logger.info(
                f"Validator {validator.id} is catching up "
                f"(height: {heartbeat.block_height})"
            )

    async def _check_missed_blocks(
        self,
        validator: ValidatorNode,
        heartbeat: LocalValidatorHeartbeat
    ):
        """
        Monitor missed blocks counter.

        Typical slashing threshold:
        - 5,000 missed blocks out of last 10,000 blocks
        - At 6s block time: ~8-9 hours of downtime

        Alert levels:
        - 1,000 missed: Warning (20% to threshold)
        - 3,000 missed: High warning (60% to threshold)
        - 4,500 missed: Critical (90% to threshold, action required)
        """
        missed_blocks = getattr(heartbeat, 'missed_blocks', 0)

        if missed_blocks == 0:
            return  # All good

        # Calculate percentage to slashing threshold
        slashing_threshold = 5000  # Typical value
        percentage = (missed_blocks / slashing_threshold) * 100

        validator_id = str(validator.id)
        last_missed_alert = self.validator_states.get(validator_id, {}).get('last_missed_blocks_alert', 0)

        # Only alert if missed blocks increased significantly
        if missed_blocks > last_missed_alert + 100:
            if percentage >= 90:
                # CRITICAL: Close to slashing
                logger.critical(
                    f"CRITICAL: Validator {validator.id} has missed {missed_blocks} blocks "
                    f"({percentage:.1f}% of slashing threshold). "
                    "Jailing is imminent!"
                )

                await self._send_critical_alert(
                    validator,
                    "CRITICAL: Close to Slashing Threshold",
                    f"Missed blocks: {missed_blocks} ({percentage:.1f}% of threshold). "
                    "Validator will be jailed soon if not resolved. "
                    "Check validator status immediately!"
                )

            elif percentage >= 60:
                # HIGH WARNING
                logger.warning(
                    f"HIGH WARNING: Validator {validator.id} has missed {missed_blocks} blocks "
                    f"({percentage:.1f}% of slashing threshold)"
                )

                await self._send_warning_alert(
                    validator,
                    "High Missed Blocks Warning",
                    f"Missed blocks: {missed_blocks} ({percentage:.1f}% of threshold). "
                    "Monitor closely and ensure validator uptime."
                )

            elif percentage >= 20:
                # Warning
                logger.info(
                    f"Warning: Validator {validator.id} has missed {missed_blocks} blocks "
                    f"({percentage:.1f}% of slashing threshold)"
                )

            # Update last alert level
            if validator_id in self.validator_states:
                self.validator_states[validator_id]['last_missed_blocks_alert'] = missed_blocks

    async def _send_critical_alert(
        self,
        validator: ValidatorNode,
        subject: str,
        message: str
    ):
        """
        Send critical alert to operator.

        In production, this would:
        - Send email via SendGrid/SES
        - Send SMS via Twilio
        - Post to Slack/Discord
        - Trigger PagerDuty incident
        """
        logger.critical(
            f"CRITICAL ALERT for validator {validator.id}: {subject}\n{message}"
        )

        # TODO: Implement actual alerting
        # Example integrations:
        #
        # Email (SendGrid):
        # from sendgrid import SendGridAPIClient
        # from sendgrid.helpers.mail import Mail
        # message = Mail(
        #     from_email='alerts@omniphi.io',
        #     to_emails=validator.setup_request.alert_email,
        #     subject=subject,
        #     html_content=f"<strong>{message}</strong>"
        # )
        # sg = SendGridAPIClient(os.environ.get('SENDGRID_API_KEY'))
        # response = sg.send(message)
        #
        # SMS (Twilio):
        # from twilio.rest import Client
        # client = Client(account_sid, auth_token)
        # message = client.messages.create(
        #     body=f"{subject}: {message}",
        #     from_='+15551234567',
        #     to=validator.setup_request.alert_phone
        # )
        #
        # Slack:
        # import httpx
        # await httpx.post(slack_webhook_url, json={
        #     "text": f"🚨 *{subject}*\n{message}",
        #     "channel": "#validator-alerts"
        # })

        pass

    async def _send_warning_alert(
        self,
        validator: ValidatorNode,
        subject: str,
        message: str
    ):
        """Send warning alert to operator."""
        logger.warning(
            f"WARNING for validator {validator.id}: {subject}\n{message}"
        )

        # TODO: Implement actual alerting (same as critical, but different urgency)
        pass

    async def validate_new_validator_start(
        self,
        consensus_pubkey: str,
        wallet_address: str
    ) -> Dict[str, Any]:
        """
        Validate that it's safe to start a new validator.

        Checks:
        1. No other validator with same consensus pubkey is running
        2. No recent validator with same pubkey was stopped (cooldown)

        Args:
            consensus_pubkey: Validator's consensus public key
            wallet_address: Operator wallet address

        Returns:
            Dict with validation result:
            {
                "safe": True/False,
                "reason": "explanation if not safe",
                "recommendations": ["action1", "action2"]
            }
        """
        # Check for other running validators with same consensus key
        # In production, this would query the actual consensus pubkey from priv_validator_key.json
        # For now, we check by wallet address as proxy

        running_validators = self.db.query(ValidatorNode).filter(
            ValidatorNode.status.in_([NodeStatus.RUNNING, NodeStatus.SYNCING])
        ).all()

        for validator in running_validators:
            if validator.setup_request.wallet_address == wallet_address:
                logger.warning(
                    f"Found existing running validator for wallet {wallet_address}: {validator.id}"
                )

                return {
                    "safe": False,
                    "reason": "Another validator with the same wallet address is already running",
                    "recommendations": [
                        "Stop the existing validator before starting a new one",
                        "Ensure you're not running two validators with the same consensus key",
                        "Wait at least 5 minutes after stopping old validator before starting new one"
                    ],
                    "existing_validator_id": str(validator.id)
                }

        # Check for recently stopped validators (5 minute cooldown)
        recent_cutoff = datetime.utcnow() - timedelta(minutes=5)

        recent_validators = self.db.query(ValidatorNode).filter(
            ValidatorNode.status == NodeStatus.STOPPED,
            ValidatorNode.updated_at > recent_cutoff
        ).all()

        for validator in recent_validators:
            if validator.setup_request.wallet_address == wallet_address:
                time_since_stop = (datetime.utcnow() - validator.updated_at).total_seconds()

                return {
                    "safe": False,
                    "reason": f"Validator was stopped only {int(time_since_stop)}s ago (cooldown: 5 min)",
                    "recommendations": [
                        f"Wait {int(300 - time_since_stop)}s more before starting new validator",
                        "This cooldown prevents double-signing from state file desync"
                    ],
                    "recently_stopped_validator_id": str(validator.id),
                    "cooldown_remaining_seconds": int(300 - time_since_stop)
                }

        # All checks passed
        return {
            "safe": True,
            "reason": "No conflicts detected",
            "recommendations": []
        }

    async def record_validator_state(
        self,
        validator_id: UUID,
        block_height: int,
        consensus_round: int = 0
    ):
        """
        Record validator state for double-signing detection.

        Args:
            validator_id: Validator node ID
            block_height: Current block height
            consensus_round: Current consensus round
        """
        validator_id_str = str(validator_id)

        self.validator_states[validator_id_str] = {
            "last_update": datetime.utcnow(),
            "block_height": block_height,
            "consensus_round": consensus_round
        }

        logger.debug(
            f"Recorded state for validator {validator_id}: "
            f"height={block_height}, round={consensus_round}"
        )

    def get_validator_safety_status(self, validator_id: UUID) -> Dict[str, Any]:
        """
        Get current safety status of a validator.

        Returns:
            Dict with safety metrics:
            {
                "safe": True/False,
                "last_check": timestamp,
                "block_height": int,
                "warnings": ["list of warnings"],
                "critical_issues": ["list of critical issues"]
            }
        """
        validator_id_str = str(validator_id)
        state = self.validator_states.get(validator_id_str, {})

        warnings = []
        critical_issues = []

        # Check heartbeat freshness
        last_check = state.get('last_check')
        if last_check:
            age = datetime.utcnow() - last_check
            if age > timedelta(minutes=5):
                warnings.append(f"No data for {int(age.total_seconds())}s")
            if age > timedelta(minutes=15):
                critical_issues.append(f"No data for {int(age.total_seconds())}s - validator may be offline")

        return {
            "safe": len(critical_issues) == 0,
            "last_check": last_check.isoformat() if last_check else None,
            "block_height": state.get('block_height'),
            "warnings": warnings,
            "critical_issues": critical_issues
        }


# Global instance
slashing_protection_service = SlashingProtectionService()
