"""Omniphi Chain Client - Cosmos SDK compatible."""

import json
import logging
import httpx
from typing import Optional, Dict, Any
from datetime import datetime

from app.core.config import settings

logger = logging.getLogger(__name__)


class OmniphiChainClient:
    """Client for interacting with Omniphi blockchain."""

    def __init__(self):
        self.chain_id = settings.OMNIPHI_CHAIN_ID
        self.rpc_url = settings.OMNIPHI_RPC_URL
        self.rest_url = settings.OMNIPHI_REST_URL
        self.grpc_url = settings.OMNIPHI_GRPC_URL

    # ==================== Query Methods ====================

    async def get_validator_by_address(self, validator_address: str) -> Optional[Dict[str, Any]]:
        """
        Get validator information by operator address.

        Args:
            validator_address: Validator operator address (omnivaloper...)

        Returns:
            Validator information or None if not found
        """
        async with httpx.AsyncClient() as client:
            try:
                response = await client.get(
                    f"{self.rest_url}/cosmos/staking/v1beta1/validators/{validator_address}"
                )
                if response.status_code == 200:
                    data = response.json()
                    return data.get("validator")
                return None
            except Exception as e:
                logger.error(f"Error fetching validator: {e}")
                return None

    async def get_all_validators(self) -> list:
        """Get all validators."""
        async with httpx.AsyncClient() as client:
            try:
                response = await client.get(
                    f"{self.rest_url}/cosmos/staking/v1beta1/validators"
                )
                if response.status_code == 200:
                    data = response.json()
                    return data.get("validators", [])
                return []
            except Exception as e:
                logger.error(f"Error fetching validators: {e}")
                return []

    async def get_validator_by_consensus_pubkey(self, consensus_pubkey: str) -> Optional[Dict[str, Any]]:
        """
        Find validator by consensus public key.

        Args:
            consensus_pubkey: Tendermint consensus public key (base64)

        Returns:
            Validator info or None
        """
        validators = await self.get_all_validators()
        for val in validators:
            if val.get("consensus_pubkey", {}).get("key") == consensus_pubkey:
                return val
        return None

    async def get_block_height(self) -> int:
        """Get current block height."""
        async with httpx.AsyncClient() as client:
            try:
                response = await client.get(f"{self.rpc_url}/status")
                if response.status_code == 200:
                    data = response.json()
                    return int(data["result"]["sync_info"]["latest_block_height"])
                return 0
            except Exception as e:
                logger.debug(f"Failed to get block height: {e}")
                return 0

    async def get_signing_info(self, consensus_address: str) -> Optional[Dict[str, Any]]:
        """Get validator signing info (missed blocks, jailed status)."""
        async with httpx.AsyncClient() as client:
            try:
                response = await client.get(
                    f"{self.rest_url}/cosmos/slashing/v1beta1/signing_infos/{consensus_address}"
                )
                if response.status_code == 200:
                    return response.json().get("val_signing_info")
                return None
            except Exception as e:
                logger.debug(f"Failed to get signing info for {consensus_address}: {e}")
                return None

    # ==================== Transaction Building ====================

    def build_create_validator_tx(
        self,
        delegator_address: str,
        validator_address: str,
        consensus_pubkey: Dict[str, str],
        amount: str,
        moniker: str,
        website: str = "",
        identity: str = "",
        details: str = "",
        commission_rate: str = "0.10",
        commission_max_rate: str = "0.20",
        commission_max_change_rate: str = "0.01",
        min_self_delegation: str = "1"
    ) -> Dict[str, Any]:
        """
        Build MsgCreateValidator transaction.

        This returns the unsigned transaction body that must be signed by the user's wallet.

        Args:
            delegator_address: Delegator address (omni...)
            validator_address: Validator operator address (omnivaloper...)
            consensus_pubkey: Consensus public key dict {"@type": "...", "key": "..."}
            amount: Self-delegation amount (e.g., "100000000omniphi")
            moniker: Validator name
            website: Validator website
            identity: Keybase identity
            details: Validator description
            commission_rate: Commission rate (e.g., "0.10" for 10%)
            commission_max_rate: Max commission rate
            commission_max_change_rate: Max commission change rate
            min_self_delegation: Minimum self delegation

        Returns:
            Transaction body ready for signing
        """
        msg = {
            "@type": "/cosmos.staking.v1beta1.MsgCreateValidator",
            "description": {
                "moniker": moniker,
                "identity": identity,
                "website": website,
                "security_contact": "",
                "details": details
            },
            "commission": {
                "rate": commission_rate,
                "max_rate": commission_max_rate,
                "max_change_rate": commission_max_change_rate
            },
            "min_self_delegation": min_self_delegation,
            "delegator_address": delegator_address,
            "validator_address": validator_address,
            "pubkey": consensus_pubkey,
            "value": {
                "denom": "omniphi",
                "amount": amount
            }
        }

        tx_body = {
            "body": {
                "messages": [msg],
                "memo": "Created via Omniphi Validator Portal",
                "timeout_height": "0",
                "extension_options": [],
                "non_critical_extension_options": []
            },
            "auth_info": {
                "signer_infos": [],
                "fee": {
                    "amount": [{"denom": "omniphi", "amount": "5000"}],
                    "gas_limit": "200000",
                    "payer": "",
                    "granter": ""
                }
            },
            "signatures": []
        }

        return tx_body

    def build_edit_validator_tx(
        self,
        validator_address: str,
        moniker: Optional[str] = None,
        website: Optional[str] = None,
        identity: Optional[str] = None,
        details: Optional[str] = None,
        commission_rate: Optional[str] = None,
        min_self_delegation: Optional[str] = None
    ) -> Dict[str, Any]:
        """Build MsgEditValidator transaction."""
        description = {}
        if moniker:
            description["moniker"] = moniker
        if website:
            description["website"] = website
        if identity:
            description["identity"] = identity
        if details:
            description["details"] = details

        msg = {
            "@type": "/cosmos.staking.v1beta1.MsgEditValidator",
            "description": description,
            "validator_address": validator_address,
            "commission_rate": commission_rate or "",
            "min_self_delegation": min_self_delegation or ""
        }

        tx_body = {
            "body": {
                "messages": [msg],
                "memo": "Edited via Omniphi Validator Portal",
                "timeout_height": "0",
                "extension_options": [],
                "non_critical_extension_options": []
            },
            "auth_info": {
                "signer_infos": [],
                "fee": {
                    "amount": [{"denom": "omniphi", "amount": "5000"}],
                    "gas_limit": "200000",
                    "payer": "",
                    "granter": ""
                }
            },
            "signatures": []
        }

        return tx_body

    async def broadcast_tx(self, signed_tx: Dict[str, Any]) -> Dict[str, Any]:
        """
        Broadcast a signed transaction.

        Args:
            signed_tx: Fully signed transaction

        Returns:
            Broadcast result
        """
        async with httpx.AsyncClient() as client:
            try:
                response = await client.post(
                    f"{self.rest_url}/cosmos/tx/v1beta1/txs",
                    json=signed_tx
                )
                return response.json()
            except Exception as e:
                return {"error": str(e)}

    # ==================== Helper Methods ====================

    def build_delegate_tx(
        self,
        delegator_address: str,
        validator_address: str,
        amount: str
    ) -> Dict[str, Any]:
        """Build MsgDelegate transaction."""
        msg = {
            "@type": "/cosmos.staking.v1beta1.MsgDelegate",
            "delegator_address": delegator_address,
            "validator_address": validator_address,
            "amount": {
                "denom": "omniphi",
                "amount": amount
            }
        }

        return {
            "body": {
                "messages": [msg],
                "memo": "Delegated via Omniphi Portal",
                "timeout_height": "0",
                "extension_options": [],
                "non_critical_extension_options": []
            },
            "auth_info": {
                "signer_infos": [],
                "fee": {
                    "amount": [{"denom": "omniphi", "amount": "5000"}],
                    "gas_limit": "200000",
                    "payer": "",
                    "granter": ""
                }
            },
            "signatures": []
        }

    def build_undelegate_tx(
        self,
        delegator_address: str,
        validator_address: str,
        amount: str
    ) -> Dict[str, Any]:
        """Build MsgUndelegate transaction."""
        msg = {
            "@type": "/cosmos.staking.v1beta1.MsgUndelegate",
            "delegator_address": delegator_address,
            "validator_address": validator_address,
            "amount": {
                "denom": "omniphi",
                "amount": amount
            }
        }

        return {
            "body": {
                "messages": [msg],
                "memo": "Undelegated via Omniphi Portal",
                "timeout_height": "0",
                "extension_options": [],
                "non_critical_extension_options": []
            },
            "auth_info": {
                "signer_infos": [],
                "fee": {
                    "amount": [{"denom": "omniphi", "amount": "5000"}],
                    "gas_limit": "200000",
                    "payer": "",
                    "granter": ""
                }
            },
            "signatures": []
        }

    def address_to_valoper(self, address: str) -> str:
        """
        Convert wallet address to validator operator address using bech32.

        Args:
            address: Wallet address (omni...)

        Returns:
            Validator operator address (omnivaloper...)
        """
        try:
            import bech32
            # Decode the address
            hrp, data = bech32.bech32_decode(address)
            if data is None:
                # Fallback to simple replace
                return address.replace("omni", "omnivaloper", 1) if address.startswith("omni") else address

            # Convert to 5-bit array
            five_bit_data = bech32.convertbits(data, 5, 8, False)
            if five_bit_data is None:
                return address.replace("omni", "omnivaloper", 1) if address.startswith("omni") else address

            # Convert back and encode with new prefix
            new_five_bit = bech32.convertbits(five_bit_data, 8, 5)
            valoper_address = bech32.bech32_encode("omnivaloper", new_five_bit)
            return valoper_address if valoper_address else address.replace("omni", "omnivaloper", 1)
        except Exception as e:
            # Log the error for debugging, then fallback to simple string replacement
            logger.warning(f"Bech32 conversion failed for address {address[:15]}...: {e}")
            return address.replace("omni", "omnivaloper", 1) if address.startswith("omni") else address

    async def check_validator_exists(self, operator_address: str) -> bool:
        """Check if validator already exists on chain."""
        val = await self.get_validator_by_address(operator_address)
        return val is not None

    async def get_validator_status_summary(self, operator_address: str) -> Dict[str, Any]:
        """
        Get comprehensive validator status.

        Returns:
            Dict with validator info, voting power, jailed status, etc.
        """
        val = await self.get_validator_by_address(operator_address)
        if not val:
            return {"exists": False}

        return {
            "exists": True,
            "moniker": val.get("description", {}).get("moniker"),
            "operator_address": val.get("operator_address"),
            "jailed": val.get("jailed", False),
            "status": val.get("status"),
            "tokens": val.get("tokens"),
            "delegator_shares": val.get("delegator_shares"),
            "commission": val.get("commission", {}).get("commission_rates", {}),
            "unbonding_height": val.get("unbonding_height"),
            "unbonding_time": val.get("unbonding_time"),
        }


# Global instance
chain_client = OmniphiChainClient()
