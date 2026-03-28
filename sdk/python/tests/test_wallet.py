"""Tests for the Omniphi Wallet class."""

from __future__ import annotations

import hashlib

import pytest
from ecdsa import SECP256k1, VerifyingKey

from omniphi.constants import BECH32_PREFIX
from omniphi.wallet import Wallet, _compress_public_key, _pubkey_to_address


class TestWalletCreate:
    """Tests for Wallet.create()."""

    def test_create_returns_wallet(self) -> None:
        wallet = Wallet.create()
        assert isinstance(wallet, Wallet)

    def test_create_generates_mnemonic(self) -> None:
        wallet = Wallet.create()
        assert wallet.mnemonic is not None
        words = wallet.mnemonic.split()
        assert len(words) == 24  # 256-bit entropy -> 24 words

    def test_create_12_word_mnemonic(self) -> None:
        wallet = Wallet.create(strength=128)
        assert wallet.mnemonic is not None
        words = wallet.mnemonic.split()
        assert len(words) == 12

    def test_create_generates_valid_address(self) -> None:
        wallet = Wallet.create()
        assert wallet.address.startswith(f"{BECH32_PREFIX}1")

    def test_create_generates_unique_wallets(self) -> None:
        w1 = Wallet.create()
        w2 = Wallet.create()
        assert w1.address != w2.address
        assert w1.private_key != w2.private_key

    def test_create_private_key_is_32_bytes(self) -> None:
        wallet = Wallet.create()
        assert len(wallet.private_key) == 32

    def test_create_public_key_is_33_bytes(self) -> None:
        wallet = Wallet.create()
        assert len(wallet.public_key) == 33
        # Compressed key starts with 02 or 03
        assert wallet.public_key[0] in (0x02, 0x03)


class TestWalletFromMnemonic:
    """Tests for Wallet.from_mnemonic()."""

    # Well-known test mnemonic (DO NOT use for real funds)
    TEST_MNEMONIC = (
        "abandon abandon abandon abandon abandon abandon "
        "abandon abandon abandon abandon abandon about"
    )

    def test_from_mnemonic_roundtrip(self) -> None:
        w1 = Wallet.create()
        assert w1.mnemonic is not None
        w2 = Wallet.from_mnemonic(w1.mnemonic)
        assert w1.address == w2.address
        assert w1.private_key == w2.private_key
        assert w1.public_key == w2.public_key

    def test_from_mnemonic_deterministic(self) -> None:
        w1 = Wallet.from_mnemonic(self.TEST_MNEMONIC)
        w2 = Wallet.from_mnemonic(self.TEST_MNEMONIC)
        assert w1.address == w2.address
        assert w1.private_key == w2.private_key

    def test_from_mnemonic_valid_address_format(self) -> None:
        wallet = Wallet.from_mnemonic(self.TEST_MNEMONIC)
        assert wallet.address.startswith("omni1")
        # bech32 addresses are lowercase alphanumeric (no 1, b, i, o)
        addr_data = wallet.address[len("omni1"):]
        assert all(c in "023456789acdefghjklmnpqrstuvwxyz" for c in addr_data)

    def test_from_mnemonic_no_stored_mnemonic(self) -> None:
        wallet = Wallet.from_mnemonic(self.TEST_MNEMONIC)
        assert wallet.mnemonic == self.TEST_MNEMONIC

    def test_invalid_mnemonic_raises(self) -> None:
        with pytest.raises(ValueError, match="Invalid BIP-39 mnemonic"):
            Wallet.from_mnemonic("invalid words that are not a mnemonic phrase at all")


class TestWalletFromPrivateKey:
    """Tests for Wallet.from_private_key()."""

    def test_from_private_key_roundtrip(self) -> None:
        w1 = Wallet.create()
        hex_key = w1.private_key_hex
        w2 = Wallet.from_private_key(hex_key)
        assert w1.address == w2.address
        assert w1.public_key == w2.public_key

    def test_from_private_key_no_mnemonic(self) -> None:
        w1 = Wallet.create()
        w2 = Wallet.from_private_key(w1.private_key_hex)
        assert w2.mnemonic is None

    def test_invalid_hex_raises(self) -> None:
        with pytest.raises(ValueError, match="Invalid hex"):
            Wallet.from_private_key("not_hex")

    def test_wrong_length_raises(self) -> None:
        with pytest.raises(ValueError, match="32 bytes"):
            Wallet.from_private_key("deadbeef")  # Only 4 bytes


class TestWalletSigning:
    """Tests for Wallet.sign() and verify()."""

    def test_sign_produces_64_bytes(self) -> None:
        wallet = Wallet.create()
        message = b"test message"
        signature = wallet.sign(message)
        assert isinstance(signature, bytes)
        assert len(signature) == 64

    def test_sign_verify_roundtrip(self) -> None:
        wallet = Wallet.create()
        message = b"hello omniphi blockchain"
        signature = wallet.sign(message)
        assert wallet.verify(message, signature) is True

    def test_verify_fails_for_wrong_message(self) -> None:
        wallet = Wallet.create()
        signature = wallet.sign(b"correct message")
        assert wallet.verify(b"wrong message", signature) is False

    def test_verify_fails_for_wrong_key(self) -> None:
        w1 = Wallet.create()
        w2 = Wallet.create()
        signature = w1.sign(b"test")
        assert w2.verify(b"test", signature) is False

    def test_sign_deterministic(self) -> None:
        wallet = Wallet.create()
        message = b"deterministic signing"
        sig1 = wallet.sign(message)
        sig2 = wallet.sign(message)
        # RFC 6979 deterministic signatures should be identical
        assert sig1 == sig2

    def test_signature_verifiable_with_raw_ecdsa(self) -> None:
        """Verify that signatures work with raw ecdsa library."""
        wallet = Wallet.create()
        message = b"cross-library verification"
        signature = wallet.sign(message)

        # Reconstruct verifying key from compressed public key
        compressed = wallet.public_key
        if compressed[0] == 0x02:
            prefix = b"\x02"
        else:
            prefix = b"\x03"

        vk = VerifyingKey.from_string(compressed, curve=SECP256k1)
        # Should not raise
        vk.verify(signature, message, hashfunc=hashlib.sha256)


class TestWalletProperties:
    """Tests for Wallet property accessors."""

    def test_private_key_hex_format(self) -> None:
        wallet = Wallet.create()
        hex_key = wallet.private_key_hex
        assert len(hex_key) == 64
        assert all(c in "0123456789abcdef" for c in hex_key)

    def test_public_key_hex_format(self) -> None:
        wallet = Wallet.create()
        hex_key = wallet.public_key_hex
        assert len(hex_key) == 66  # 33 bytes = 66 hex chars
        assert hex_key[:2] in ("02", "03")

    def test_repr(self) -> None:
        wallet = Wallet.create()
        r = repr(wallet)
        assert "Wallet" in r
        assert wallet.address in r

    def test_equality(self) -> None:
        w1 = Wallet.create()
        w2 = Wallet.from_private_key(w1.private_key_hex)
        assert w1 == w2

    def test_inequality(self) -> None:
        w1 = Wallet.create()
        w2 = Wallet.create()
        assert w1 != w2

    def test_hashable(self) -> None:
        w1 = Wallet.create()
        w2 = Wallet.from_private_key(w1.private_key_hex)
        wallet_set = {w1, w2}
        assert len(wallet_set) == 1

    def test_invalid_private_key_length(self) -> None:
        with pytest.raises(ValueError, match="32 bytes"):
            Wallet(b"\x00" * 16)
