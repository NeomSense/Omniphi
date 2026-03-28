"""Omniphi blockchain client for queries and transaction broadcast.

Provides a high-level interface to interact with the Omniphi chain's
REST (LCD) and Tendermint RPC endpoints. Supports all standard Cosmos SDK
queries as well as Omniphi-specific modules: PoC, tokenomics, feemarket,
guard, timelock, poseq, and more.
"""

from __future__ import annotations

import hashlib
from typing import Any

import httpx

from omniphi.constants import (
    DEFAULT_FEE,
    DEFAULT_FEE_DENOM,
    DEFAULT_GAS,
    DEFAULT_REST_URL,
    DEFAULT_RPC_URL,
    DENOM,
    MODULE_PARAM_PATHS,
    REST_PATHS,
)
from omniphi.errors import (
    OmniphiBroadcastError,
    OmniphiConnectionError,
    OmniphiError,
    OmniphiTxError,
)
from omniphi.tx import (
    build_contribution_msg,
    build_delegate_msg,
    build_send_msg,
    sign_tx,
)
from omniphi.wallet import Wallet


class OmniphiClient:
    """Client for the Omniphi blockchain.

    Communicates with a node via its REST (LCD) API on port 1317 and
    Tendermint RPC on port 26657. All query methods return parsed JSON
    dicts; transaction methods accept a ``Wallet`` for signing.

    Args:
        rpc_url: Tendermint RPC endpoint (default: ``http://localhost:26657``).
        rest_url: Cosmos REST/LCD endpoint (default: ``http://localhost:1317``).
        timeout: HTTP request timeout in seconds.

    Examples:
        Basic usage::

            client = OmniphiClient()
            height = client.get_block_height()
            balances = client.get_all_balances("omni1...")
    """

    def __init__(
        self,
        rpc_url: str = DEFAULT_RPC_URL,
        rest_url: str | None = None,
        timeout: float = 30.0,
    ) -> None:
        self._rpc_url = rpc_url.rstrip("/")
        self._rest_url = (rest_url or DEFAULT_REST_URL).rstrip("/")
        self._timeout = timeout
        self._http = httpx.Client(timeout=timeout)

    def close(self) -> None:
        """Close the underlying HTTP client and release resources."""
        self._http.close()

    def __enter__(self) -> OmniphiClient:
        return self

    def __exit__(self, *args: object) -> None:
        self.close()

    # ── Low-level HTTP helpers ────────────────────────────────────────

    def _rest_get(self, path: str, params: dict[str, Any] | None = None) -> dict[str, Any]:
        """Execute a GET request against the REST/LCD endpoint.

        Args:
            path: URL path (will be appended to rest_url).
            params: Optional query parameters.

        Returns:
            Parsed JSON response dict.

        Raises:
            OmniphiConnectionError: If the request fails.
            OmniphiError: If the response contains an error.
        """
        url = f"{self._rest_url}{path}"
        try:
            response = self._http.get(url, params=params)
        except httpx.HTTPError as exc:
            raise OmniphiConnectionError(
                f"REST request failed: {exc}",
                url=url,
            ) from exc

        data: dict[str, Any] = response.json()

        if response.status_code != 200:
            message = data.get("message", data.get("error", f"HTTP {response.status_code}"))
            raise OmniphiError(
                f"REST query error: {message}",
                details={"status_code": response.status_code, "url": url, "response": data},
            )

        return data

    def _rest_post(self, path: str, json_body: dict[str, Any]) -> dict[str, Any]:
        """Execute a POST request against the REST/LCD endpoint.

        Args:
            path: URL path.
            json_body: JSON request body.

        Returns:
            Parsed JSON response dict.

        Raises:
            OmniphiConnectionError: If the request fails.
            OmniphiError: If the response indicates an error.
        """
        url = f"{self._rest_url}{path}"
        try:
            response = self._http.post(url, json=json_body)
        except httpx.HTTPError as exc:
            raise OmniphiConnectionError(
                f"REST POST failed: {exc}",
                url=url,
            ) from exc

        data: dict[str, Any] = response.json()

        if response.status_code != 200:
            message = data.get("message", data.get("error", f"HTTP {response.status_code}"))
            raise OmniphiError(
                f"REST error: {message}",
                details={"status_code": response.status_code, "url": url, "response": data},
            )

        return data

    def _rpc_get(self, path: str, params: dict[str, Any] | None = None) -> dict[str, Any]:
        """Execute a GET request against the Tendermint RPC endpoint.

        Args:
            path: URL path (e.g., ``/status``, ``/block``).
            params: Optional query parameters.

        Returns:
            The ``result`` field from the JSON-RPC response.

        Raises:
            OmniphiConnectionError: If the request fails.
            OmniphiError: If the RPC response contains an error.
        """
        url = f"{self._rpc_url}{path}"
        try:
            response = self._http.get(url, params=params)
        except httpx.HTTPError as exc:
            raise OmniphiConnectionError(
                f"RPC request failed: {exc}",
                url=url,
            ) from exc

        data: dict[str, Any] = response.json()

        if "error" in data and data["error"]:
            error_info = data["error"]
            message = error_info.get("message", str(error_info)) if isinstance(error_info, dict) else str(error_info)
            raise OmniphiError(
                f"RPC error: {message}",
                details={"url": url, "error": error_info},
            )

        return data.get("result", data)

    # ── Chain info queries ────────────────────────────────────────────

    def get_block_height(self) -> int:
        """Get the latest committed block height.

        Returns:
            Current block height as an integer.
        """
        data = self._rest_get(REST_PATHS["latest_block"])
        block = data.get("block", {})
        header = block.get("header", {})
        return int(header.get("height", 0))

    def get_chain_id(self) -> str:
        """Get the chain ID from the node.

        Returns:
            Chain ID string (e.g., ``"omniphi-testnet-2"``).
        """
        data = self._rest_get(REST_PATHS["node_info"])
        node_info = data.get("default_node_info", data.get("node_info", {}))
        return str(node_info.get("network", ""))

    def get_node_info(self) -> dict[str, Any]:
        """Get detailed node information.

        Returns:
            Full node info dict including version, network, listen addresses.
        """
        return self._rest_get(REST_PATHS["node_info"])

    # ── Balance queries ───────────────────────────────────────────────

    def get_balance(self, address: str, denom: str = DENOM) -> dict[str, Any]:
        """Get the balance of a specific denomination for an address.

        Args:
            address: Bech32-encoded account address.
            denom: Token denomination to query (default: "omniphi").

        Returns:
            Dict with ``"balance"`` key containing ``{"denom": ..., "amount": ...}``.
        """
        path = REST_PATHS["balance"].format(address=address)
        return self._rest_get(path, params={"denom": denom})

    def get_all_balances(self, address: str) -> list[dict[str, str]]:
        """Get all token balances for an address.

        Args:
            address: Bech32-encoded account address.

        Returns:
            List of ``{"denom": ..., "amount": ...}`` dicts.
        """
        path = REST_PATHS["balances"].format(address=address)
        data = self._rest_get(path)
        return data.get("balances", [])

    # ── Account queries ───────────────────────────────────────────────

    def get_account(self, address: str) -> dict[str, Any]:
        """Get on-chain account information (number, sequence, etc.).

        Args:
            address: Bech32-encoded account address.

        Returns:
            Account dict with ``account_number`` and ``sequence``.
        """
        path = REST_PATHS["account"].format(address=address)
        data = self._rest_get(path)
        account = data.get("account", {})
        # Handle both BaseAccount and nested formats
        if "base_account" in account:
            return account["base_account"]
        return account

    def get_account_number_and_sequence(self, address: str) -> tuple[int, int]:
        """Get the account number and sequence for transaction signing.

        Args:
            address: Bech32-encoded account address.

        Returns:
            Tuple of (account_number, sequence).
        """
        account = self.get_account(address)
        return (
            int(account.get("account_number", 0)),
            int(account.get("sequence", 0)),
        )

    # ── Validator queries ─────────────────────────────────────────────

    def get_validators(self, status: str = "BOND_STATUS_BONDED") -> list[dict[str, Any]]:
        """Get the list of validators.

        Args:
            status: Filter by bonding status. Use ``"BOND_STATUS_BONDED"``
                for active validators, ``""`` for all.

        Returns:
            List of validator dicts.
        """
        params: dict[str, Any] = {}
        if status:
            params["status"] = status
        data = self._rest_get(REST_PATHS["validators"], params=params)
        return data.get("validators", [])

    # ── Transaction broadcast ─────────────────────────────────────────

    def broadcast_tx(self, tx_body: dict[str, Any]) -> dict[str, Any]:
        """Broadcast a signed transaction to the network.

        Args:
            tx_body: Complete signed transaction dict (as returned by ``sign_tx``).

        Returns:
            Broadcast response dict containing ``tx_response`` with
            ``txhash``, ``code``, ``raw_log``, etc.

        Raises:
            OmniphiBroadcastError: If the broadcast returns a non-zero code.
        """
        data = self._rest_post(REST_PATHS["tx_broadcast"], tx_body)
        tx_response = data.get("tx_response", {})

        code = int(tx_response.get("code", 0))
        if code != 0:
            raise OmniphiBroadcastError(
                f"Transaction failed with code {code}: {tx_response.get('raw_log', '')}",
                tx_hash=tx_response.get("txhash", ""),
                code=code,
                codespace=tx_response.get("codespace", ""),
                raw_log=tx_response.get("raw_log", ""),
            )

        return data

    def simulate_tx(self, tx_body: dict[str, Any]) -> dict[str, Any]:
        """Simulate a transaction to estimate gas usage.

        Args:
            tx_body: Transaction dict (same format as broadcast).

        Returns:
            Simulation response with ``gas_info`` and ``result``.
        """
        return self._rest_post(REST_PATHS["simulate"], tx_body)

    # ── High-level transaction methods ────────────────────────────────

    def _sign_and_broadcast(
        self,
        msgs: list[dict[str, Any]],
        wallet: Wallet,
        memo: str = "",
        fee: int = DEFAULT_FEE,
        gas: int = DEFAULT_GAS,
        fee_denom: str = DEFAULT_FEE_DENOM,
    ) -> dict[str, Any]:
        """Helper to sign and broadcast a list of messages.

        Fetches chain_id, account_number, and sequence automatically
        from the node, then signs and broadcasts.

        Args:
            msgs: List of message dicts.
            wallet: Signing wallet.
            memo: Transaction memo.
            fee: Fee amount.
            gas: Gas limit.
            fee_denom: Fee denomination.

        Returns:
            Broadcast response dict.
        """
        chain_id = self.get_chain_id()
        account_number, sequence = self.get_account_number_and_sequence(wallet.address)

        signed_tx = sign_tx(
            msgs=msgs,
            wallet=wallet,
            chain_id=chain_id,
            account_number=account_number,
            sequence=sequence,
            fee=fee,
            gas=gas,
            fee_denom=fee_denom,
            memo=memo,
        )

        return self.broadcast_tx(signed_tx)

    def send_tokens(
        self,
        sender: str,
        recipient: str,
        amount: int | str,
        denom: str = DENOM,
        wallet: Wallet | None = None,
        memo: str = "",
        fee: int = DEFAULT_FEE,
        gas: int = DEFAULT_GAS,
    ) -> dict[str, Any]:
        """Send tokens from one address to another.

        Args:
            sender: Bech32 sender address.
            recipient: Bech32 recipient address.
            amount: Token amount in smallest denomination.
            denom: Token denomination.
            wallet: Wallet for signing (must control ``sender``).
            memo: Optional transaction memo.
            fee: Fee amount.
            gas: Gas limit.

        Returns:
            Broadcast response dict.

        Raises:
            OmniphiTxError: If no wallet is provided.
        """
        if wallet is None:
            raise OmniphiTxError("Wallet required for send_tokens")

        msg = build_send_msg(sender, recipient, amount, denom)
        return self._sign_and_broadcast([msg], wallet, memo=memo, fee=fee, gas=gas)

    def delegate(
        self,
        delegator: str,
        validator: str,
        amount: int | str,
        wallet: Wallet | None = None,
        memo: str = "",
        fee: int = DEFAULT_FEE,
        gas: int = DEFAULT_GAS,
    ) -> dict[str, Any]:
        """Delegate tokens to a validator.

        Args:
            delegator: Bech32 delegator address.
            validator: Bech32 validator operator address (omnivaloper1...).
            amount: Delegation amount in smallest denomination.
            wallet: Wallet for signing (must control ``delegator``).
            memo: Optional transaction memo.
            fee: Fee amount.
            gas: Gas limit.

        Returns:
            Broadcast response dict.

        Raises:
            OmniphiTxError: If no wallet is provided.
        """
        if wallet is None:
            raise OmniphiTxError("Wallet required for delegate")

        msg = build_delegate_msg(delegator, validator, amount)
        return self._sign_and_broadcast([msg], wallet, memo=memo, fee=fee, gas=gas)

    def submit_contribution(
        self,
        sender: str,
        ctype: str,
        uri: str,
        content_hash: bytes | str,
        wallet: Wallet | None = None,
        memo: str = "",
        fee: int = DEFAULT_FEE,
        gas: int = DEFAULT_GAS,
    ) -> dict[str, Any]:
        """Submit a Proof-of-Contribution to the PoC module.

        Args:
            sender: Bech32 contributor address.
            ctype: Contribution type ("code", "record", "relay", "green").
            uri: Evidence URI (IPFS CID, Git URL, etc.).
            content_hash: SHA-256 hash of the content as bytes or hex string.
            wallet: Wallet for signing.
            memo: Optional transaction memo.
            fee: Fee amount.
            gas: Gas limit.

        Returns:
            Broadcast response dict.

        Raises:
            OmniphiTxError: If no wallet is provided.
        """
        if wallet is None:
            raise OmniphiTxError("Wallet required for submit_contribution")

        msg = build_contribution_msg(sender, ctype, uri, content_hash)
        return self._sign_and_broadcast([msg], wallet, memo=memo, fee=fee, gas=gas)

    # ── PoC module queries ────────────────────────────────────────────

    def query_contribution(self, contribution_id: int) -> dict[str, Any]:
        """Query a single contribution by its ID.

        Args:
            contribution_id: Numeric contribution ID.

        Returns:
            Contribution dict with fields: id, contributor, ctype, uri,
            hash, endorsements, verified, block_height, block_time, rewarded.
        """
        path = REST_PATHS["poc_contribution"].format(id=contribution_id)
        return self._rest_get(path)

    def query_contributions(
        self,
        contributor: str = "",
        ctype: str = "",
        verified: int = -1,
    ) -> dict[str, Any]:
        """Query contributions with optional filters.

        Args:
            contributor: Filter by contributor address (empty = all).
            ctype: Filter by contribution type (empty = all).
            verified: -1 = all, 0 = unverified, 1 = verified.

        Returns:
            Dict with ``"contributions"`` list and ``"pagination"`` info.
        """
        params: dict[str, Any] = {}
        if contributor:
            params["contributor"] = contributor
        if ctype:
            params["ctype"] = ctype
        if verified >= 0:
            params["verified"] = str(verified)
        return self._rest_get(REST_PATHS["poc_contributions"], params=params)

    def query_poc_credits(self, address: str) -> dict[str, Any]:
        """Query PoC credit balance and tier for an address.

        Args:
            address: Bech32 contributor address.

        Returns:
            Dict with ``"credits"`` and ``"tier"`` fields.
        """
        path = REST_PATHS["poc_credits"].format(address=address)
        return self._rest_get(path)

    # ── Poseq module queries ──────────────────────────────────────────

    def query_poseq_checkpoint(self, epoch: int) -> dict[str, Any]:
        """Query a poseq checkpoint anchor by epoch.

        Poseq module does not have REST gateway annotations; this queries
        via the ABCI query path through the RPC endpoint.

        Args:
            epoch: Epoch number to query.

        Returns:
            Checkpoint anchor data dict.
        """
        path = f"/abci_query"
        params = {
            "path": '"/store/poseq/key"',
            "data": f"0x03{epoch:016x}",
        }
        result = self._rpc_get(path, params=params)
        response = result.get("response", {})
        if response.get("code", 0) != 0:
            raise OmniphiError(
                f"Poseq checkpoint query failed: {response.get('log', '')}",
                details={"epoch": epoch, "response": response},
            )
        return response

    # ── Tokenomics module queries ─────────────────────────────────────

    def query_token_supply(self) -> dict[str, Any]:
        """Query current token supply metrics.

        Returns:
            Dict with total_supply_cap, current_total_supply, total_minted,
            total_burned, remaining_mintable, supply_pct_of_cap, net_inflation_rate.
        """
        return self._rest_get(REST_PATHS["tokenomics_supply"])

    def query_inflation(self) -> dict[str, Any]:
        """Query current inflation metrics.

        Returns:
            Dict with current_inflation_rate, inflation_min, inflation_max,
            annual_provisions, block_provisions, blocks_per_year.
        """
        return self._rest_get(REST_PATHS["tokenomics_inflation"])

    def query_emissions(self) -> dict[str, Any]:
        """Query current emission distribution.

        Returns:
            Dict with allocations list (staking, poc, sequencer, treasury)
            and total_annual_emissions.
        """
        return self._rest_get(REST_PATHS["tokenomics_emissions"])

    def query_treasury(self) -> dict[str, Any]:
        """Query the DAO treasury status.

        Returns:
            Dict with treasury_balance, total_treasury_inflows, from_inflation,
            from_burn_redirect, treasury_burn_redirect_pct, treasury_address.
        """
        return self._rest_get(REST_PATHS["tokenomics_treasury"])

    def query_tokenomics_burn_rate(self) -> dict[str, Any]:
        """Query the adaptive burn rate and trigger information.

        Returns:
            Dict with adaptive_burn_enabled, current_burn_ratio, trigger,
            min/max/default burn ratios, block_congestion, treasury_pct.
        """
        return self._rest_get(REST_PATHS["tokenomics_burn_rate"])

    # ── Feemarket module queries ──────────────────────────────────────

    def query_base_fee(self) -> dict[str, Any]:
        """Query the current EIP-1559 base fee.

        Returns:
            Dict with base_fee, min_gas_price, effective_gas_price.
        """
        return self._rest_get(REST_PATHS["feemarket_base_fee"])

    def query_block_utilization(self) -> dict[str, Any]:
        """Query current block utilization metrics.

        Returns:
            Dict with utilization, block_gas_used, max_block_gas, target_utilization.
        """
        return self._rest_get(REST_PATHS["feemarket_utilization"])

    # ── Guard module queries ──────────────────────────────────────────

    def query_risk_report(self, proposal_id: int) -> dict[str, Any]:
        """Query the guard risk report for a governance proposal.

        Args:
            proposal_id: Governance proposal ID.

        Returns:
            Risk report dict with risk_tier, risk_score, and analysis fields.
        """
        path = REST_PATHS["guard_risk_report"].format(proposal_id=proposal_id)
        return self._rest_get(path)

    def query_guard_queued(self, proposal_id: int) -> dict[str, Any]:
        """Query the guard queued execution state for a proposal.

        Args:
            proposal_id: Governance proposal ID.

        Returns:
            Queued execution state dict.
        """
        path = REST_PATHS["guard_queued"].format(proposal_id=proposal_id)
        return self._rest_get(path)

    # ── Timelock module queries ───────────────────────────────────────

    def query_timelock_operations(self) -> dict[str, Any]:
        """Query all timelock operations.

        Returns:
            Dict with ``"operations"`` list and pagination info.
        """
        return self._rest_get(REST_PATHS["timelock_operations"])

    def query_timelock_queued(self) -> dict[str, Any]:
        """Query all queued timelock operations.

        Returns:
            Dict with ``"operations"`` list of queued operations.
        """
        return self._rest_get(REST_PATHS["timelock_queued"])

    def query_timelock_executable(self) -> dict[str, Any]:
        """Query all timelock operations ready for execution.

        Returns:
            Dict with ``"operations"`` list of executable operations.
        """
        return self._rest_get(REST_PATHS["timelock_executable"])

    # ── Module params (generic) ───────────────────────────────────────

    def query_module_params(self, module_name: str) -> dict[str, Any]:
        """Query the parameters of any module by name.

        Args:
            module_name: Module name (e.g., "poc", "guard", "tokenomics",
                "feemarket", "timelock", "staking", "gov").

        Returns:
            Module parameters dict.

        Raises:
            OmniphiError: If the module name is not recognized.
        """
        path = MODULE_PARAM_PATHS.get(module_name)
        if path is None:
            known = ", ".join(sorted(MODULE_PARAM_PATHS.keys()))
            raise OmniphiError(
                f"Unknown module: {module_name!r}. Known modules: {known}",
            )
        return self._rest_get(path)

    # ── Cosmos bank supply ────────────────────────────────────────────

    def query_total_supply(self) -> list[dict[str, str]]:
        """Query the total supply of all denominations.

        Returns:
            List of ``{"denom": ..., "amount": ...}`` dicts.
        """
        data = self._rest_get(REST_PATHS["supply"])
        return data.get("supply", [])

    def __repr__(self) -> str:
        return f"OmniphiClient(rpc={self._rpc_url!r}, rest={self._rest_url!r})"
