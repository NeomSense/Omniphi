"""Transaction builder for the Omniphi blockchain.

Constructs, signs, and encodes transactions using Cosmos SDK Amino JSON
signing mode. Compatible with the Omniphi chain's custom modules (PoC,
Guard, etc.) as well as standard Cosmos SDK modules (bank, staking).
"""

from __future__ import annotations

from typing import Any

from omniphi.constants import (
    DEFAULT_FEE,
    DEFAULT_FEE_DENOM,
    DEFAULT_GAS,
    DENOM,
    MSG_TYPE_URLS,
)
from omniphi.encoding import bytes_to_base64, canonical_json
from omniphi.errors import OmniphiTxError
from omniphi.wallet import Wallet


def build_send_msg(
    from_addr: str,
    to_addr: str,
    amount: int | str,
    denom: str = DENOM,
) -> dict[str, Any]:
    """Build a Cosmos SDK bank MsgSend message.

    Args:
        from_addr: Bech32 sender address.
        to_addr: Bech32 recipient address.
        amount: Token amount in smallest denomination.
        denom: Token denomination (default: "omniphi").

    Returns:
        Message dict with ``@type`` and field values.
    """
    return {
        "@type": MSG_TYPE_URLS["send"],
        "from_address": from_addr,
        "to_address": to_addr,
        "amount": [{"denom": denom, "amount": str(amount)}],
    }


def build_delegate_msg(
    delegator: str,
    validator: str,
    amount: int | str,
    denom: str = DENOM,
) -> dict[str, Any]:
    """Build a Cosmos SDK staking MsgDelegate message.

    Args:
        delegator: Bech32 delegator address.
        validator: Bech32 validator operator address (omnivaloper1...).
        amount: Delegation amount in smallest denomination.
        denom: Token denomination (default: "omniphi").

    Returns:
        Message dict with ``@type`` and field values.
    """
    return {
        "@type": MSG_TYPE_URLS["delegate"],
        "delegator_address": delegator,
        "validator_address": validator,
        "amount": {"denom": denom, "amount": str(amount)},
    }


def build_contribution_msg(
    sender: str,
    ctype: str,
    uri: str,
    content_hash: bytes | str,
) -> dict[str, Any]:
    """Build a PoC MsgSubmitContribution message.

    Args:
        sender: Bech32 contributor address.
        ctype: Contribution type ("code", "record", "relay", "green").
        uri: Evidence URI (IPFS CID, Git URL, etc.).
        content_hash: Content hash as bytes or hex string.

    Returns:
        Message dict with ``@type`` and field values.
    """
    if isinstance(content_hash, str):
        hash_bytes = bytes.fromhex(content_hash)
    else:
        hash_bytes = content_hash

    return {
        "@type": MSG_TYPE_URLS["submit_contribution"],
        "contributor": sender,
        "ctype": ctype,
        "uri": uri,
        "hash": bytes_to_base64(hash_bytes),
    }


def build_endorse_msg(
    validator: str,
    contribution_id: int,
    decision: bool,
) -> dict[str, Any]:
    """Build a PoC MsgEndorse message.

    Args:
        validator: Bech32 validator address.
        contribution_id: ID of the contribution to endorse.
        decision: True to approve, False to reject.

    Returns:
        Message dict with ``@type`` and field values.
    """
    return {
        "@type": MSG_TYPE_URLS["endorse"],
        "validator": validator,
        "contribution_id": str(contribution_id),
        "decision": decision,
    }


def build_withdraw_poc_rewards_msg(address: str) -> dict[str, Any]:
    """Build a PoC MsgWithdrawPOCRewards message.

    Args:
        address: Bech32 address to withdraw rewards for.

    Returns:
        Message dict with ``@type`` and field values.
    """
    return {
        "@type": MSG_TYPE_URLS["withdraw_poc_rewards"],
        "address": address,
    }


def build_advisory_link_msg(
    reporter: str,
    proposal_id: int,
    uri: str,
    report_hash: str,
) -> dict[str, Any]:
    """Build a Guard MsgSubmitAdvisoryLink message.

    Args:
        reporter: Bech32 reporter address.
        proposal_id: Governance proposal ID.
        uri: IPFS CID or HTTP URI to the advisory report.
        report_hash: SHA256 hex hash of the report bytes.

    Returns:
        Message dict with ``@type`` and field values.
    """
    return {
        "@type": MSG_TYPE_URLS["submit_advisory_link"],
        "reporter": reporter,
        "proposal_id": str(proposal_id),
        "uri": uri,
        "report_hash": report_hash,
    }


def _build_amino_sign_doc(
    msgs: list[dict[str, Any]],
    chain_id: str,
    account_number: int,
    sequence: int,
    fee_amount: int,
    fee_denom: str,
    gas: int,
    memo: str,
) -> dict[str, Any]:
    """Build an Amino JSON SignDoc for signing.

    This produces the canonical JSON structure that gets signed
    with secp256k1 in Cosmos SDK SIGN_MODE_LEGACY_AMINO_JSON.

    Args:
        msgs: List of message dicts.
        chain_id: Chain identifier.
        account_number: On-chain account number.
        sequence: Account sequence (nonce).
        fee_amount: Fee amount.
        fee_denom: Fee denomination.
        gas: Gas limit.
        memo: Transaction memo.

    Returns:
        SignDoc dict ready for canonical JSON encoding and signing.
    """
    return {
        "account_number": str(account_number),
        "chain_id": chain_id,
        "fee": {
            "amount": [{"amount": str(fee_amount), "denom": fee_denom}],
            "gas": str(gas),
        },
        "memo": memo,
        "msgs": msgs,
        "sequence": str(sequence),
    }


def _build_tx_body(
    msgs: list[dict[str, Any]],
    memo: str = "",
    timeout_height: int = 0,
) -> dict[str, Any]:
    """Build the TxBody portion of a Cosmos SDK transaction.

    Args:
        msgs: List of message dicts with @type annotations.
        memo: Transaction memo.
        timeout_height: Block height after which tx is invalid (0 = no timeout).

    Returns:
        TxBody dict.
    """
    body: dict[str, Any] = {
        "messages": msgs,
        "memo": memo,
    }
    if timeout_height > 0:
        body["timeout_height"] = str(timeout_height)
    return body


def _build_auth_info(
    public_key: bytes,
    sequence: int,
    fee_amount: int,
    fee_denom: str,
    gas: int,
) -> dict[str, Any]:
    """Build the AuthInfo portion of a Cosmos SDK transaction.

    Args:
        public_key: Compressed 33-byte secp256k1 public key.
        sequence: Account sequence (nonce).
        fee_amount: Fee amount.
        fee_denom: Fee denomination.
        gas: Gas limit.

    Returns:
        AuthInfo dict.
    """
    return {
        "signer_infos": [
            {
                "public_key": {
                    "@type": "/cosmos.crypto.secp256k1.PubKey",
                    "key": bytes_to_base64(public_key),
                },
                "mode_info": {
                    "single": {
                        "mode": "SIGN_MODE_LEGACY_AMINO_JSON",
                    },
                },
                "sequence": str(sequence),
            },
        ],
        "fee": {
            "amount": [{"amount": str(fee_amount), "denom": fee_denom}],
            "gas_limit": str(gas),
        },
    }


def sign_tx(
    msgs: list[dict[str, Any]],
    wallet: Wallet,
    chain_id: str,
    account_number: int,
    sequence: int,
    fee: int = DEFAULT_FEE,
    gas: int = DEFAULT_GAS,
    fee_denom: str = DEFAULT_FEE_DENOM,
    memo: str = "",
) -> dict[str, Any]:
    """Sign a transaction using Amino JSON signing mode.

    Constructs the SignDoc, signs it with the wallet's secp256k1 key,
    and returns the complete transaction envelope ready for broadcast
    via the ``/cosmos/tx/v1beta1/txs`` REST endpoint.

    Args:
        msgs: List of message dicts (each with ``@type``).
        wallet: Wallet instance used for signing.
        chain_id: Chain identifier string.
        account_number: On-chain account number.
        sequence: Account sequence number (nonce).
        fee: Fee amount in smallest denomination.
        gas: Gas limit.
        fee_denom: Fee denomination.
        memo: Transaction memo.

    Returns:
        Complete signed transaction dict suitable for JSON-encoded broadcast.

    Raises:
        OmniphiTxError: If signing fails.
    """
    try:
        # Build and sign the Amino JSON SignDoc
        sign_doc = _build_amino_sign_doc(
            msgs=msgs,
            chain_id=chain_id,
            account_number=account_number,
            sequence=sequence,
            fee_amount=fee,
            fee_denom=fee_denom,
            gas=gas,
            memo=memo,
        )
        sign_bytes = canonical_json(sign_doc)
        signature = wallet.sign(sign_bytes)

        # Build the full transaction envelope
        tx_body = _build_tx_body(msgs, memo=memo)
        auth_info = _build_auth_info(
            public_key=wallet.public_key,
            sequence=sequence,
            fee_amount=fee,
            fee_denom=fee_denom,
            gas=gas,
        )

        return {
            "tx": {
                "body": tx_body,
                "auth_info": auth_info,
                "signatures": [bytes_to_base64(signature)],
            },
            "mode": "BROADCAST_MODE_SYNC",
        }

    except Exception as exc:
        raise OmniphiTxError(
            f"Failed to sign transaction: {exc}",
            details={"chain_id": chain_id, "msgs_count": len(msgs)},
        ) from exc
