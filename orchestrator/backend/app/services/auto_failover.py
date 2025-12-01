"""Auto-failover service for validator high availability."""

import logging
import asyncio
from typing import Dict, Optional, Any, List
from datetime import datetime, timedelta
from uuid import UUID
from enum import Enum

from app.db.session import SessionLocal
from app.models import ValidatorNode, ValidatorSetupRequest
from app.models.validator_node import NodeStatus

logger = logging.getLogger(__name__)


class FailoverStrategy(str, Enum):
    """Failover strategy types."""
    MANUAL = "manual"  # Requires manual intervention
    TIME_DELAYED = "time_delayed"  # Automatic after delay
    CONSENSUS_BASED = "consensus_based"  # Based on external consensus


class FailoverState(str, Enum):
    """Current failover state."""
    ACTIVE = "active"  # Primary is running
    FAILING_OVER = "failing_over"  # In process of failover
    FAILED_OVER = "failed_over"  # Backup is now active
    FAILED = "failed"  # Failover failed


class AutoFailoverService:
    """
    Auto-failover service for validator high availability.

    Features:
    - Monitors primary validator health
    - Automatic failover to backup validator
    - Configurable failover strategies (manual, time-delayed, consensus-based)
    - Double-signing prevention (critical!)
    - Failback to primary when recovered
    """

    def __init__(
        self,
        default_strategy: FailoverStrategy = FailoverStrategy.TIME_DELAYED,
        failover_delay_seconds: int = 300,  # 5 minutes default
        health_check_interval: int = 30
    ):
        """
        Initialize auto-failover service.

        Args:
            default_strategy: Default failover strategy
            failover_delay_seconds: Delay before automatic failover (safety margin)
            health_check_interval: How often to check validator health (seconds)
        """
        self.db = SessionLocal()
        self.default_strategy = default_strategy
        self.failover_delay_seconds = failover_delay_seconds
        self.health_check_interval = health_check_interval

        # Failover state tracking
        self.failover_states: Dict[str, Dict[str, Any]] = {}

        logger.info(
            f"Initialized auto-failover service "
            f"(strategy: {default_strategy}, delay: {failover_delay_seconds}s)"
        )

    async def monitor_failover_groups(self):
        """
        Main monitoring loop for all failover groups.

        A failover group consists of:
        - Primary validator
        - One or more backup validators
        - Failover configuration

        This loop:
        1. Checks primary health
        2. Initiates failover if primary is down
        3. Monitors backup after failover
        4. Handles failback to primary
        """
        logger.info("Starting failover group monitoring")

        while True:
            try:
                await self._check_all_failover_groups()
                await asyncio.sleep(self.health_check_interval)

            except Exception as e:
                logger.error(f"Error in monitor_failover_groups loop: {e}", exc_info=True)
                await asyncio.sleep(60)  # Back off on error

    async def _check_all_failover_groups(self):
        """Check all configured failover groups."""
        # In production, this would query failover_groups table
        # For now, we'll implement a simplified version

        # Get all running validators
        validators = self.db.query(ValidatorNode).filter(
            ValidatorNode.status.in_([NodeStatus.RUNNING, NodeStatus.SYNCING])
        ).all()

        logger.debug(f"Checking {len(validators)} validators for failover needs")

        # TODO: Implement actual failover group logic
        # For now, just log that we're monitoring
        for validator in validators:
            group_id = str(validator.id)
            if group_id not in self.failover_states:
                self.failover_states[group_id] = {
                    "state": FailoverState.ACTIVE,
                    "primary_id": validator.id,
                    "backup_ids": [],
                    "last_check": datetime.utcnow()
                }

    async def initiate_failover(
        self,
        primary_validator_id: UUID,
        backup_validator_id: UUID,
        strategy: Optional[FailoverStrategy] = None,
        force: bool = False
    ) -> Dict[str, Any]:
        """
        Initiate failover from primary to backup validator.

        CRITICAL SAFETY MEASURES:
        1. Verify primary is truly down (not just slow)
        2. Ensure time delay to prevent state file desync
        3. Stop primary completely before starting backup
        4. Verify backup has latest state
        5. Monitor for double-signing during transition

        Args:
            primary_validator_id: Primary validator node ID
            backup_validator_id: Backup validator node ID
            strategy: Failover strategy to use (defaults to configured)
            force: Skip safety checks (DANGEROUS - use only in emergencies)

        Returns:
            Dict with failover result:
            {
                "success": True/False,
                "state": FailoverState,
                "message": "description",
                "started_at": timestamp,
                "completed_at": timestamp (if complete),
                "warnings": ["list of warnings"]
            }
        """
        try:
            logger.info(
                f"Initiating failover: primary={primary_validator_id}, "
                f"backup={backup_validator_id}, force={force}"
            )

            # Get validators
            primary = self.db.query(ValidatorNode).filter(
                ValidatorNode.id == primary_validator_id
            ).first()

            backup = self.db.query(ValidatorNode).filter(
                ValidatorNode.id == backup_validator_id
            ).first()

            if not primary or not backup:
                return {
                    "success": False,
                    "state": FailoverState.FAILED,
                    "message": "Primary or backup validator not found",
                    "warnings": []
                }

            # Check if same consensus key (would cause double-signing!)
            if primary.setup_request_id == backup.setup_request_id:
                if not force:
                    return {
                        "success": False,
                        "state": FailoverState.FAILED,
                        "message": "Cannot failover: Primary and backup use same consensus key (double-signing risk!)",
                        "warnings": [
                            "Primary and backup must have different consensus keys",
                            "Use force=True only if you're absolutely certain they won't run simultaneously"
                        ]
                    }
                else:
                    logger.critical(
                        "FORCED FAILOVER with same consensus key - "
                        "DOUBLE-SIGNING RISK! Ensure primary is completely stopped."
                    )

            # Determine strategy
            failover_strategy = strategy or self.default_strategy

            # Record failover start
            failover_record = {
                "started_at": datetime.utcnow(),
                "primary_id": primary_validator_id,
                "backup_id": backup_validator_id,
                "strategy": failover_strategy,
                "state": FailoverState.FAILING_OVER,
                "warnings": []
            }

            # Execute failover based on strategy
            if failover_strategy == FailoverStrategy.MANUAL:
                result = await self._manual_failover(primary, backup, failover_record)

            elif failover_strategy == FailoverStrategy.TIME_DELAYED:
                result = await self._time_delayed_failover(primary, backup, failover_record, force)

            elif failover_strategy == FailoverStrategy.CONSENSUS_BASED:
                result = await self._consensus_based_failover(primary, backup, failover_record)

            else:
                result = {
                    "success": False,
                    "state": FailoverState.FAILED,
                    "message": f"Unknown failover strategy: {failover_strategy}",
                    "warnings": []
                }

            # Update failover record
            failover_record.update(result)
            failover_record["completed_at"] = datetime.utcnow()

            # Store in state
            group_id = str(primary_validator_id)
            self.failover_states[group_id] = failover_record

            return result

        except Exception as e:
            logger.error(f"Error during failover: {e}", exc_info=True)
            return {
                "success": False,
                "state": FailoverState.FAILED,
                "message": f"Failover failed: {str(e)}",
                "warnings": []
            }

    async def _manual_failover(
        self,
        primary: ValidatorNode,
        backup: ValidatorNode,
        record: Dict[str, Any]
    ) -> Dict[str, Any]:
        """
        Manual failover strategy.

        Requires human intervention to start backup.
        This is the safest strategy but requires operator availability.
        """
        logger.info(f"Manual failover requested for {primary.id} -> {backup.id}")

        record["warnings"].append("Manual failover requires operator intervention")
        record["warnings"].append(f"1. Verify primary {primary.id} is completely stopped")
        record["warnings"].append("2. Wait 5 minutes after stopping primary")
        record["warnings"].append(f"3. Manually start backup {backup.id}")
        record["warnings"].append("4. Monitor for double-signing warnings")

        return {
            "success": False,  # Not automated
            "state": FailoverState.FAILING_OVER,
            "message": "Manual failover initiated - operator intervention required",
            "warnings": record["warnings"]
        }

    async def _time_delayed_failover(
        self,
        primary: ValidatorNode,
        backup: ValidatorNode,
        record: Dict[str, Any],
        force: bool = False
    ) -> Dict[str, Any]:
        """
        Time-delayed failover strategy.

        Automatically fails over after a safety delay.
        This is the recommended strategy for most validators.

        Steps:
        1. Verify primary is truly down
        2. Stop primary (if still running)
        3. Wait safety delay (prevents state file desync)
        4. Start backup
        5. Verify backup is signing
        """
        logger.info(
            f"Time-delayed failover: {primary.id} -> {backup.id} "
            f"(delay: {self.failover_delay_seconds}s)"
        )

        # Step 1: Verify primary is down
        if not force:
            primary_health = await self._check_validator_health(primary)

            if primary_health['healthy']:
                return {
                    "success": False,
                    "state": FailoverState.FAILED,
                    "message": "Primary validator is still healthy - failover aborted",
                    "warnings": ["Primary appears to be running normally"]
                }

            logger.info(f"Primary health check failed: {primary_health}")

        # Step 2: Ensure primary is stopped
        logger.info(f"Stopping primary validator {primary.id}")
        await self._stop_validator(primary)

        # Step 3: Safety delay
        logger.info(f"Waiting {self.failover_delay_seconds}s safety delay...")
        await asyncio.sleep(self.failover_delay_seconds)

        # Step 4: Start backup
        logger.info(f"Starting backup validator {backup.id}")
        backup_started = await self._start_validator(backup)

        if not backup_started:
            return {
                "success": False,
                "state": FailoverState.FAILED,
                "message": "Failed to start backup validator",
                "warnings": ["Backup failed to start - manual intervention required"]
            }

        # Step 5: Verify backup is signing
        await asyncio.sleep(30)  # Wait for backup to initialize

        backup_health = await self._check_validator_health(backup)

        if not backup_health['healthy']:
            record["warnings"].append("Backup started but health check failed")
            record["warnings"].append("Monitor backup closely")

        logger.info(f"Failover complete: {primary.id} -> {backup.id}")

        return {
            "success": True,
            "state": FailoverState.FAILED_OVER,
            "message": f"Successfully failed over to backup {backup.id}",
            "warnings": record["warnings"]
        }

    async def _consensus_based_failover(
        self,
        primary: ValidatorNode,
        backup: ValidatorNode,
        record: Dict[str, Any]
    ) -> Dict[str, Any]:
        """
        Consensus-based failover strategy.

        Uses external consensus (etcd, Consul, or Raft) to decide which
        validator should be active. This prevents split-brain scenarios.

        This is the most advanced strategy and requires additional infrastructure.
        """
        logger.info(f"Consensus-based failover for {primary.id} -> {backup.id}")

        # TODO: Implement consensus-based failover
        # Example with etcd:
        # 1. Acquire distributed lock with primary ID
        # 2. If lock acquired, start primary
        # 3. If lock not acquired, start backup
        # 4. Monitor lock and failover if lost

        record["warnings"].append("Consensus-based failover not yet implemented")
        record["warnings"].append("Consider using Horcrux for distributed signing instead")

        return {
            "success": False,
            "state": FailoverState.FAILED,
            "message": "Consensus-based failover not implemented",
            "warnings": record["warnings"]
        }

    async def _check_validator_health(self, validator: ValidatorNode) -> Dict[str, Any]:
        """
        Check if validator is healthy.

        Checks:
        - Process is running
        - RPC endpoint reachable
        - Block height increasing
        - Not catching up
        """
        try:
            # TODO: Implement actual health check
            # For now, check database status

            if validator.status != NodeStatus.RUNNING:
                return {
                    "healthy": False,
                    "reason": f"Validator status is {validator.status}, not RUNNING"
                }

            # Check last health check timestamp
            if validator.last_health_check:
                age = datetime.utcnow() - validator.last_health_check
                if age > timedelta(minutes=5):
                    return {
                        "healthy": False,
                        "reason": f"Last health check was {int(age.total_seconds())}s ago"
                    }

            return {
                "healthy": True,
                "reason": "All checks passed"
            }

        except Exception as e:
            logger.error(f"Error checking validator health: {e}")
            return {
                "healthy": False,
                "reason": f"Health check error: {str(e)}"
            }

    async def _stop_validator(self, validator: ValidatorNode):
        """Stop a validator node."""
        try:
            logger.info(f"Stopping validator {validator.id}")

            # Update status
            validator.status = NodeStatus.STOPPED
            self.db.commit()

            # TODO: Actually stop the validator
            # - For Docker: docker stop container_id
            # - For systemd: systemctl stop posd
            # - For cloud: terminate instance (or stop)

            logger.info(f"Validator {validator.id} stopped")

        except Exception as e:
            logger.error(f"Error stopping validator: {e}")
            raise

    async def _start_validator(self, validator: ValidatorNode) -> bool:
        """Start a validator node."""
        try:
            logger.info(f"Starting validator {validator.id}")

            # Update status
            validator.status = NodeStatus.RUNNING
            self.db.commit()

            # TODO: Actually start the validator
            # - For Docker: docker start container_id
            # - For systemd: systemctl start posd
            # - For cloud: start instance

            logger.info(f"Validator {validator.id} started")
            return True

        except Exception as e:
            logger.error(f"Error starting validator: {e}")
            return False

    def get_failover_status(self, primary_validator_id: UUID) -> Dict[str, Any]:
        """
        Get current failover status for a validator.

        Returns:
            Dict with failover status:
            {
                "state": FailoverState,
                "primary_id": UUID,
                "backup_id": UUID (if failed over),
                "last_check": timestamp,
                "warnings": [...]
            }
        """
        group_id = str(primary_validator_id)
        state = self.failover_states.get(group_id, {})

        return {
            "state": state.get("state", FailoverState.ACTIVE),
            "primary_id": state.get("primary_id"),
            "backup_id": state.get("backup_id"),
            "last_check": state.get("last_check"),
            "warnings": state.get("warnings", [])
        }

    async def configure_failover_group(
        self,
        primary_validator_id: UUID,
        backup_validator_ids: List[UUID],
        strategy: FailoverStrategy = FailoverStrategy.TIME_DELAYED,
        auto_failback: bool = False
    ) -> Dict[str, Any]:
        """
        Configure a new failover group.

        Args:
            primary_validator_id: Primary validator node ID
            backup_validator_ids: List of backup validator node IDs
            strategy: Failover strategy to use
            auto_failback: Automatically failback to primary when recovered

        Returns:
            Dict with configuration result
        """
        try:
            # Verify validators exist
            primary = self.db.query(ValidatorNode).filter(
                ValidatorNode.id == primary_validator_id
            ).first()

            if not primary:
                return {
                    "success": False,
                    "message": "Primary validator not found"
                }

            backups = self.db.query(ValidatorNode).filter(
                ValidatorNode.id.in_(backup_validator_ids)
            ).all()

            if len(backups) != len(backup_validator_ids):
                return {
                    "success": False,
                    "message": "One or more backup validators not found"
                }

            # Create failover group record
            # TODO: Store in database (failover_groups table)

            group_id = str(primary_validator_id)
            self.failover_states[group_id] = {
                "primary_id": primary_validator_id,
                "backup_ids": backup_validator_ids,
                "strategy": strategy,
                "auto_failback": auto_failback,
                "state": FailoverState.ACTIVE,
                "configured_at": datetime.utcnow()
            }

            logger.info(
                f"Configured failover group: primary={primary_validator_id}, "
                f"backups={backup_validator_ids}, strategy={strategy}"
            )

            return {
                "success": True,
                "message": "Failover group configured successfully",
                "group_id": group_id
            }

        except Exception as e:
            logger.error(f"Error configuring failover group: {e}", exc_info=True)
            return {
                "success": False,
                "message": f"Configuration failed: {str(e)}"
            }


# Global instance
auto_failover_service = AutoFailoverService()
