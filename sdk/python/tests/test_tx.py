"""Tests for the transaction builder module."""

from __future__ import annotations

import json

import pytest

from omniphi.constants import DENOM, MSG_TYPE_URLS
from omniphi.encoding import canonical_json, sort_json
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


class TestBuildSendMsg:
    """Tests for build_send_msg."""

    def test_structure(self) -> None:
        msg = build_send_msg("omni1sender", "omni1recipient", 1000)
        assert msg["@type"] == "/cosmos.bank.v1beta1.MsgSend"
        assert msg["from_address"] == "omni1sender"
        assert msg["to_address"] == "omni1recipient"
        assert msg["amount"] == [{"denom": DENOM, "amount": "1000"}]

    def test_custom_denom(self) -> None:
        msg = build_send_msg("omni1a", "omni1b", 500, denom="uatom")
        assert msg["amount"] == [{"denom": "uatom", "amount": "500"}]

    def test_amount_as_string(self) -> None:
        msg = build_send_msg("omni1a", "omni1b", "999")
        assert msg["amount"][0]["amount"] == "999"

    def test_amount_as_int(self) -> None:
        msg = build_send_msg("omni1a", "omni1b", 42)
        assert msg["amount"][0]["amount"] == "42"


class TestBuildDelegateMsg:
    """Tests for build_delegate_msg."""

    def test_structure(self) -> None:
        msg = build_delegate_msg("omni1del", "omnivaloper1val", 5000)
        assert msg["@type"] == "/cosmos.staking.v1beta1.MsgDelegate"
        assert msg["delegator_address"] == "omni1del"
        assert msg["validator_address"] == "omnivaloper1val"
        assert msg["amount"] == {"denom": DENOM, "amount": "5000"}

    def test_custom_denom(self) -> None:
        msg = build_delegate_msg("omni1a", "omnivaloper1b", 100, denom="stake")
        assert msg["amount"]["denom"] == "stake"


class TestBuildContributionMsg:
    """Tests for build_contribution_msg."""

    def test_structure_with_hex_hash(self) -> None:
        hash_hex = "a" * 64
        msg = build_contribution_msg("omni1dev", "code", "ipfs://Qm...", hash_hex)
        assert msg["@type"] == MSG_TYPE_URLS["submit_contribution"]
        assert msg["contributor"] == "omni1dev"
        assert msg["ctype"] == "code"
        assert msg["uri"] == "ipfs://Qm..."
        # Hash should be base64-encoded
        assert isinstance(msg["hash"], str)

    def test_structure_with_bytes_hash(self) -> None:
        hash_bytes = b"\xde\xad\xbe\xef" + b"\x00" * 28
        msg = build_contribution_msg("omni1dev", "record", "https://git.example.com", hash_bytes)
        assert msg["@type"] == MSG_TYPE_URLS["submit_contribution"]
        assert msg["hash"]  # Should be non-empty base64


class TestBuildEndorseMsg:
    """Tests for build_endorse_msg."""

    def test_structure(self) -> None:
        msg = build_endorse_msg("omni1val", 42, True)
        assert msg["@type"] == MSG_TYPE_URLS["endorse"]
        assert msg["validator"] == "omni1val"
        assert msg["contribution_id"] == "42"
        assert msg["decision"] is True

    def test_reject(self) -> None:
        msg = build_endorse_msg("omni1val", 1, False)
        assert msg["decision"] is False


class TestBuildWithdrawPocRewardsMsg:
    """Tests for build_withdraw_poc_rewards_msg."""

    def test_structure(self) -> None:
        msg = build_withdraw_poc_rewards_msg("omni1contributor")
        assert msg["@type"] == MSG_TYPE_URLS["withdraw_poc_rewards"]
        assert msg["address"] == "omni1contributor"


class TestBuildAdvisoryLinkMsg:
    """Tests for build_advisory_link_msg."""

    def test_structure(self) -> None:
        msg = build_advisory_link_msg(
            reporter="omni1reporter",
            proposal_id=7,
            uri="ipfs://QmReport",
            report_hash="b" * 64,
        )
        assert msg["@type"] == MSG_TYPE_URLS["submit_advisory_link"]
        assert msg["reporter"] == "omni1reporter"
        assert msg["proposal_id"] == "7"
        assert msg["uri"] == "ipfs://QmReport"
        assert msg["report_hash"] == "b" * 64


class TestCanonicalJson:
    """Tests for canonical JSON encoding (Amino signing)."""

    def test_sort_json_flat(self) -> None:
        obj = {"z": 1, "a": 2, "m": 3}
        sorted_obj = sort_json(obj)
        keys = list(sorted_obj.keys())
        assert keys == ["a", "m", "z"]

    def test_sort_json_nested(self) -> None:
        obj = {"b": {"z": 1, "a": 2}, "a": 1}
        sorted_obj = sort_json(obj)
        outer_keys = list(sorted_obj.keys())
        inner_keys = list(sorted_obj["b"].keys())
        assert outer_keys == ["a", "b"]
        assert inner_keys == ["a", "z"]

    def test_sort_json_with_lists(self) -> None:
        obj = {"items": [{"z": 1, "a": 2}, {"y": 3, "b": 4}]}
        sorted_obj = sort_json(obj)
        assert list(sorted_obj["items"][0].keys()) == ["a", "z"]
        assert list(sorted_obj["items"][1].keys()) == ["b", "y"]

    def test_canonical_json_deterministic(self) -> None:
        obj = {"z": 1, "a": 2, "m": {"c": 3, "b": 4}}
        result = canonical_json(obj)
        assert result == b'{"a":2,"m":{"b":4,"c":3},"z":1}'

    def test_canonical_json_no_spaces(self) -> None:
        obj = {"key": "value", "num": 42}
        result = canonical_json(obj)
        assert b" " not in result

    def test_sort_json_preserves_primitives(self) -> None:
        assert sort_json(42) == 42
        assert sort_json("hello") == "hello"
        assert sort_json(True) is True
        assert sort_json(None) is None

    def test_sort_json_preserves_list_order(self) -> None:
        """Lists maintain element order (only dict keys are sorted)."""
        result = sort_json([3, 1, 2])
        assert result == [3, 1, 2]


class TestSignTx:
    """Tests for sign_tx."""

    def test_sign_tx_returns_broadcast_format(self) -> None:
        wallet = Wallet.create()
        msg = build_send_msg(wallet.address, "omni1recipient", 100)

        result = sign_tx(
            msgs=[msg],
            wallet=wallet,
            chain_id="omniphi-testnet-2",
            account_number=0,
            sequence=0,
        )

        # Should have the broadcast envelope structure
        assert "tx" in result
        assert "mode" in result
        assert result["mode"] == "BROADCAST_MODE_SYNC"

        tx = result["tx"]
        assert "body" in tx
        assert "auth_info" in tx
        assert "signatures" in tx

    def test_sign_tx_body_contains_messages(self) -> None:
        wallet = Wallet.create()
        msg = build_send_msg(wallet.address, "omni1other", 200)

        result = sign_tx(
            msgs=[msg],
            wallet=wallet,
            chain_id="test-chain",
            account_number=5,
            sequence=3,
        )

        body = result["tx"]["body"]
        assert len(body["messages"]) == 1
        assert body["messages"][0]["@type"] == "/cosmos.bank.v1beta1.MsgSend"

    def test_sign_tx_auth_info_structure(self) -> None:
        wallet = Wallet.create()
        msg = build_send_msg(wallet.address, "omni1other", 100)

        result = sign_tx(
            msgs=[msg],
            wallet=wallet,
            chain_id="test-chain",
            account_number=1,
            sequence=0,
            fee=10000,
            gas=300000,
        )

        auth_info = result["tx"]["auth_info"]
        assert len(auth_info["signer_infos"]) == 1

        signer = auth_info["signer_infos"][0]
        assert signer["public_key"]["@type"] == "/cosmos.crypto.secp256k1.PubKey"
        assert signer["sequence"] == "0"

        fee = auth_info["fee"]
        assert fee["gas_limit"] == "300000"
        assert fee["amount"][0]["amount"] == "10000"

    def test_sign_tx_signature_is_base64(self) -> None:
        wallet = Wallet.create()
        msg = build_send_msg(wallet.address, "omni1other", 100)

        result = sign_tx(
            msgs=[msg],
            wallet=wallet,
            chain_id="test-chain",
            account_number=0,
            sequence=0,
        )

        sig = result["tx"]["signatures"][0]
        # Base64 characters only
        import base64
        decoded = base64.b64decode(sig)
        assert len(decoded) == 64  # secp256k1 compact signature

    def test_sign_tx_deterministic(self) -> None:
        wallet = Wallet.create()
        msg = build_send_msg(wallet.address, "omni1other", 100)

        r1 = sign_tx(msgs=[msg], wallet=wallet, chain_id="c", account_number=0, sequence=0)
        r2 = sign_tx(msgs=[msg], wallet=wallet, chain_id="c", account_number=0, sequence=0)

        assert r1["tx"]["signatures"] == r2["tx"]["signatures"]

    def test_sign_tx_with_memo(self) -> None:
        wallet = Wallet.create()
        msg = build_send_msg(wallet.address, "omni1other", 100)

        result = sign_tx(
            msgs=[msg],
            wallet=wallet,
            chain_id="test-chain",
            account_number=0,
            sequence=0,
            memo="sent via omniphi python sdk",
        )

        assert result["tx"]["body"]["memo"] == "sent via omniphi python sdk"

    def test_sign_tx_multiple_messages(self) -> None:
        wallet = Wallet.create()
        msg1 = build_send_msg(wallet.address, "omni1a", 100)
        msg2 = build_send_msg(wallet.address, "omni1b", 200)

        result = sign_tx(
            msgs=[msg1, msg2],
            wallet=wallet,
            chain_id="test-chain",
            account_number=0,
            sequence=0,
        )

        assert len(result["tx"]["body"]["messages"]) == 2
