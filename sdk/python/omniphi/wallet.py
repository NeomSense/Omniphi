"""Wallet implementation for the Omniphi blockchain.

Provides key generation, mnemonic derivation, bech32 address encoding,
and secp256k1 message signing compatible with Cosmos SDK.
"""

from __future__ import annotations

import hashlib
import hmac
import struct
from typing import Self

import bech32
from ecdsa import SECP256k1, SigningKey, VerifyingKey
from ecdsa.util import sigencode_string_canonize
from mnemonic import Mnemonic

from omniphi.constants import BECH32_PREFIX, COIN_TYPE, HD_PATH


def _hmac_sha512(key: bytes, data: bytes) -> bytes:
    """Compute HMAC-SHA512."""
    return hmac.new(key, data, hashlib.sha512).digest()


def _parse_hd_path(path: str) -> list[int]:
    """Parse a BIP-44 HD path string into integer components.

    Args:
        path: HD path like "m/44'/60'/0'/0/0".

    Returns:
        List of integer path components (hardened indices have 0x80000000 added).

    Raises:
        ValueError: If the path format is invalid.
    """
    if not path.startswith("m/"):
        raise ValueError(f"HD path must start with 'm/': {path}")

    components: list[int] = []
    for part in path[2:].split("/"):
        if part.endswith("'"):
            components.append(int(part[:-1]) + 0x80000000)
        else:
            components.append(int(part))
    return components


def _derive_child_key(parent_key: bytes, parent_chain_code: bytes, index: int) -> tuple[bytes, bytes]:
    """Derive a child key from a parent key using BIP-32.

    Args:
        parent_key: 32-byte parent private key.
        parent_chain_code: 32-byte parent chain code.
        index: Child index (hardened if >= 0x80000000).

    Returns:
        Tuple of (child_private_key, child_chain_code).
    """
    if index >= 0x80000000:
        # Hardened child: use 0x00 || parent_key || index
        data = b"\x00" + parent_key + struct.pack(">I", index)
    else:
        # Normal child: use compressed_pubkey || index
        sk = SigningKey.from_string(parent_key, curve=SECP256k1)
        vk = sk.get_verifying_key()
        compressed = _compress_public_key(vk)
        data = compressed + struct.pack(">I", index)

    hmac_result = _hmac_sha512(parent_chain_code, data)
    child_key_int = (
        int.from_bytes(hmac_result[:32], "big") + int.from_bytes(parent_key, "big")
    ) % SECP256k1.order
    child_key = child_key_int.to_bytes(32, "big")
    child_chain_code = hmac_result[32:]
    return child_key, child_chain_code


def _seed_to_master_key(seed: bytes) -> tuple[bytes, bytes]:
    """Derive the BIP-32 master key from a seed.

    Args:
        seed: 64-byte BIP-39 seed.

    Returns:
        Tuple of (master_private_key, master_chain_code).
    """
    hmac_result = _hmac_sha512(b"Bitcoin seed", seed)
    return hmac_result[:32], hmac_result[32:]


def _derive_key_from_path(seed: bytes, path: str) -> bytes:
    """Derive a private key from a seed along a BIP-44 path.

    Args:
        seed: 64-byte BIP-39 seed.
        path: HD derivation path.

    Returns:
        32-byte derived private key.
    """
    master_key, chain_code = _seed_to_master_key(seed)
    key = master_key
    cc = chain_code

    for index in _parse_hd_path(path):
        key, cc = _derive_child_key(key, cc, index)

    return key


def _compress_public_key(vk: VerifyingKey) -> bytes:
    """Compress a secp256k1 public key to 33 bytes.

    Args:
        vk: ecdsa VerifyingKey object.

    Returns:
        33-byte compressed public key.
    """
    x = vk.pubkey.point.x()
    y = vk.pubkey.point.y()
    prefix = b"\x02" if y % 2 == 0 else b"\x03"
    return prefix + x.to_bytes(32, "big")


def _pubkey_to_address(compressed_pubkey: bytes, prefix: str = BECH32_PREFIX) -> str:
    """Derive a bech32 address from a compressed public key.

    Cosmos SDK uses SHA256 + RIPEMD160 (same as Bitcoin's Hash160) to derive
    the 20-byte address from the compressed public key.

    Args:
        compressed_pubkey: 33-byte compressed secp256k1 public key.
        prefix: Bech32 human-readable prefix.

    Returns:
        Bech32-encoded address string.
    """
    sha256_hash = hashlib.sha256(compressed_pubkey).digest()
    ripemd160 = hashlib.new("ripemd160")
    ripemd160.update(sha256_hash)
    address_bytes = ripemd160.digest()

    # Convert 8-bit groups to 5-bit groups for bech32
    converted = bech32.convertbits(address_bytes, 8, 5)
    if converted is None:
        raise ValueError("Failed to convert address bytes to bech32")

    return bech32.bech32_encode(prefix, converted)


class Wallet:
    """Omniphi blockchain wallet for key management and transaction signing.

    Supports creation from mnemonic phrases, raw private keys, or fresh
    generation. Uses secp256k1 for signing and BIP-44 derivation with
    coin type 60 (EIP-155 compatible).

    Examples:
        Create a new wallet::

            wallet = Wallet.create()
            print(wallet.address)       # "omni1..."
            print(wallet.mnemonic)      # 24-word BIP-39 phrase

        Restore from mnemonic::

            wallet = Wallet.from_mnemonic("abandon abandon ... about")

        Import from hex private key::

            wallet = Wallet.from_private_key("deadbeef...")
    """

    def __init__(
        self,
        private_key: bytes,
        mnemonic_phrase: str | None = None,
    ) -> None:
        """Initialize a wallet from a raw private key.

        Args:
            private_key: 32-byte secp256k1 private key.
            mnemonic_phrase: Optional BIP-39 mnemonic used to derive this key.

        Raises:
            ValueError: If the private key length is not 32 bytes.
        """
        if len(private_key) != 32:
            raise ValueError(f"Private key must be 32 bytes, got {len(private_key)}")

        self._private_key = private_key
        self._mnemonic = mnemonic_phrase
        self._signing_key = SigningKey.from_string(private_key, curve=SECP256k1)
        self._verifying_key = self._signing_key.get_verifying_key()
        self._compressed_pubkey = _compress_public_key(self._verifying_key)
        self._address = _pubkey_to_address(self._compressed_pubkey)

    @classmethod
    def create(cls, strength: int = 256) -> Self:
        """Generate a new wallet with a fresh mnemonic.

        Args:
            strength: Mnemonic entropy bits (128=12 words, 256=24 words).

        Returns:
            A new Wallet instance with a freshly generated key pair.
        """
        mnemo = Mnemonic("english")
        phrase = mnemo.generate(strength=strength)
        return cls.from_mnemonic(phrase)

    @classmethod
    def from_mnemonic(cls, mnemonic_phrase: str, hd_path: str = HD_PATH) -> Self:
        """Restore a wallet from a BIP-39 mnemonic phrase.

        Args:
            mnemonic_phrase: Space-separated BIP-39 mnemonic words.
            hd_path: BIP-44 derivation path (default: Omniphi standard).

        Returns:
            A Wallet derived from the mnemonic.

        Raises:
            ValueError: If the mnemonic is invalid.
        """
        mnemo = Mnemonic("english")
        if not mnemo.check(mnemonic_phrase):
            raise ValueError("Invalid BIP-39 mnemonic phrase")

        seed = mnemo.to_seed(mnemonic_phrase)
        private_key = _derive_key_from_path(seed, hd_path)
        return cls(private_key, mnemonic_phrase=mnemonic_phrase)

    @classmethod
    def from_private_key(cls, hex_key: str) -> Self:
        """Import a wallet from a hex-encoded private key.

        Args:
            hex_key: 64-character hex string representing a 32-byte private key.

        Returns:
            A Wallet using the provided private key (no mnemonic available).

        Raises:
            ValueError: If the hex string is not a valid 32-byte key.
        """
        try:
            key_bytes = bytes.fromhex(hex_key)
        except ValueError as exc:
            raise ValueError(f"Invalid hex private key: {exc}") from exc

        if len(key_bytes) != 32:
            raise ValueError(f"Private key must be 32 bytes (64 hex chars), got {len(key_bytes)}")

        return cls(key_bytes, mnemonic_phrase=None)

    @property
    def address(self) -> str:
        """Bech32-encoded Omniphi address (``omni1...``)."""
        return self._address

    @property
    def mnemonic(self) -> str | None:
        """BIP-39 mnemonic phrase, or ``None`` if imported from raw key."""
        return self._mnemonic

    @property
    def private_key(self) -> bytes:
        """Raw 32-byte private key."""
        return self._private_key

    @property
    def private_key_hex(self) -> str:
        """Hex-encoded private key string."""
        return self._private_key.hex()

    @property
    def public_key(self) -> bytes:
        """Compressed 33-byte secp256k1 public key."""
        return self._compressed_pubkey

    @property
    def public_key_hex(self) -> str:
        """Hex-encoded compressed public key string."""
        return self._compressed_pubkey.hex()

    def sign(self, message_bytes: bytes) -> bytes:
        """Sign a message using secp256k1 with SHA-256 hashing.

        Produces a 64-byte compact (r || s) signature with low-S normalization,
        as required by Cosmos SDK.

        Args:
            message_bytes: Raw message bytes to sign.

        Returns:
            64-byte signature (r || s, each 32 bytes).
        """
        return self._signing_key.sign(
            message_bytes,
            hashfunc=hashlib.sha256,
            sigencode=sigencode_string_canonize,
        )

    def verify(self, message_bytes: bytes, signature: bytes) -> bool:
        """Verify a signature against this wallet's public key.

        Args:
            message_bytes: Original signed message bytes.
            signature: 64-byte compact signature.

        Returns:
            True if the signature is valid, False otherwise.
        """
        try:
            self._verifying_key.verify(
                signature,
                message_bytes,
                hashfunc=hashlib.sha256,
            )
            return True
        except Exception:
            return False

    def __repr__(self) -> str:
        return f"Wallet(address={self._address!r})"

    def __eq__(self, other: object) -> bool:
        if not isinstance(other, Wallet):
            return NotImplemented
        return self._private_key == other._private_key

    def __hash__(self) -> int:
        return hash(self._private_key)
