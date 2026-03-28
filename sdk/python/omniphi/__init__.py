"""Omniphi Python SDK -- interact with the Omniphi blockchain.

Quick start::

    from omniphi import OmniphiClient, Wallet

    wallet = Wallet.create()
    client = OmniphiClient()
    height = client.get_block_height()
"""

from __future__ import annotations

from omniphi.client import OmniphiClient
from omniphi.constants import (
    BECH32_PREFIX,
    COIN_TYPE,
    DEFAULT_FEE,
    DEFAULT_GAS,
    DEFAULT_REST_URL,
    DEFAULT_RPC_URL,
    DENOM,
    HD_PATH,
    MODULE_NAMES,
    MSG_TYPE_URLS,
    REST_PATHS,
)
from omniphi.encoding import base64_to_bytes, bytes_to_base64, canonical_json, sort_json
from omniphi.errors import (
    OmniphiBroadcastError,
    OmniphiConnectionError,
    OmniphiError,
    OmniphiTxError,
)
from omniphi.tx import (
    build_advisory_link_msg,
    build_contribution_msg,
    build_delegate_msg,
    build_endorse_msg,
    build_send_msg,
    build_withdraw_poc_rewards_msg,
    sign_tx,
)
from omniphi.wallet import Wallet

__version__ = "0.1.0"

__all__ = [
    # Core classes
    "OmniphiClient",
    "Wallet",
    # Transaction builders
    "build_advisory_link_msg",
    "build_contribution_msg",
    "build_delegate_msg",
    "build_endorse_msg",
    "build_send_msg",
    "build_withdraw_poc_rewards_msg",
    "sign_tx",
    # Encoding utilities
    "base64_to_bytes",
    "bytes_to_base64",
    "canonical_json",
    "sort_json",
    # Errors
    "OmniphiBroadcastError",
    "OmniphiConnectionError",
    "OmniphiError",
    "OmniphiTxError",
    # Constants
    "BECH32_PREFIX",
    "COIN_TYPE",
    "DEFAULT_FEE",
    "DEFAULT_GAS",
    "DEFAULT_REST_URL",
    "DEFAULT_RPC_URL",
    "DENOM",
    "HD_PATH",
    "MODULE_NAMES",
    "MSG_TYPE_URLS",
    "REST_PATHS",
    # Version
    "__version__",
]
