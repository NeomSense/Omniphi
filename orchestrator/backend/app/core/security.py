"""Security utilities for authentication and authorization."""

import hmac
import secrets
from datetime import datetime, timedelta
from typing import Optional, Dict, Any

import jwt
from fastapi import HTTPException, Security, Depends
from fastapi.security import APIKeyHeader, HTTPBearer, HTTPAuthorizationCredentials
from passlib.context import CryptContext
from sqlalchemy.orm import Session

from app.core.config import settings
from app.db.session import get_db

# Password hashing
pwd_context = CryptContext(schemes=["bcrypt"], deprecated="auto")

# API Key header
api_key_header = APIKeyHeader(name="X-API-Key", auto_error=False)

# JWT Bearer
bearer_scheme = HTTPBearer(auto_error=False)

# JWT settings
ALGORITHM = "HS256"
ACCESS_TOKEN_EXPIRE_MINUTES = 60
REFRESH_TOKEN_EXPIRE_DAYS = 7


def create_access_token(
    subject: str,
    expires_delta: Optional[timedelta] = None,
    additional_claims: Optional[Dict[str, Any]] = None
) -> str:
    """
    Create a JWT access token.

    Args:
        subject: Subject identifier (usually user/validator ID)
        expires_delta: Optional expiration time delta
        additional_claims: Optional additional claims to include

    Returns:
        Encoded JWT token
    """
    if expires_delta:
        expire = datetime.utcnow() + expires_delta
    else:
        expire = datetime.utcnow() + timedelta(minutes=ACCESS_TOKEN_EXPIRE_MINUTES)

    to_encode = {
        "sub": subject,
        "exp": expire,
        "iat": datetime.utcnow(),
        "type": "access"
    }

    if additional_claims:
        to_encode.update(additional_claims)

    encoded_jwt = jwt.encode(to_encode, settings.SECRET_KEY, algorithm=ALGORITHM)
    return encoded_jwt


def create_refresh_token(subject: str) -> str:
    """
    Create a JWT refresh token.

    Args:
        subject: Subject identifier (usually user/validator ID)

    Returns:
        Encoded JWT refresh token
    """
    expire = datetime.utcnow() + timedelta(days=REFRESH_TOKEN_EXPIRE_DAYS)
    to_encode = {
        "sub": subject,
        "exp": expire,
        "iat": datetime.utcnow(),
        "type": "refresh"
    }
    encoded_jwt = jwt.encode(to_encode, settings.SECRET_KEY, algorithm=ALGORITHM)
    return encoded_jwt


def verify_token(token: str, token_type: str = "access") -> Dict[str, Any]:
    """
    Verify and decode a JWT token.

    Args:
        token: JWT token to verify
        token_type: Expected token type ("access" or "refresh")

    Returns:
        Decoded token payload

    Raises:
        HTTPException: If token is invalid or expired
    """
    try:
        payload = jwt.decode(token, settings.SECRET_KEY, algorithms=[ALGORITHM])

        # Verify token type
        if payload.get("type") != token_type:
            raise HTTPException(
                status_code=401,
                detail=f"Invalid token type. Expected {token_type}"
            )

        return payload

    except jwt.ExpiredSignatureError:
        raise HTTPException(
            status_code=401,
            detail="Token has expired"
        )
    except jwt.InvalidTokenError:
        raise HTTPException(
            status_code=401,
            detail="Invalid token"
        )


def generate_api_key() -> str:
    """
    Generate a secure random API key.

    Returns:
        Random API key string (32 bytes hex)
    """
    return secrets.token_hex(32)


def hash_api_key(api_key: str) -> str:
    """
    Hash an API key for secure storage.

    Args:
        api_key: Plain API key

    Returns:
        Hashed API key
    """
    return pwd_context.hash(api_key)


def verify_api_key(plain_key: str, hashed_key: str) -> bool:
    """
    Verify an API key against its hash.

    Args:
        plain_key: Plain API key
        hashed_key: Hashed API key from database

    Returns:
        True if key matches, False otherwise
    """
    return pwd_context.verify(plain_key, hashed_key)


async def get_current_user_from_token(
    credentials: HTTPAuthorizationCredentials = Security(bearer_scheme)
) -> Dict[str, Any]:
    """
    Dependency to get current user from JWT token.

    Args:
        credentials: HTTP authorization credentials

    Returns:
        Token payload containing user information

    Raises:
        HTTPException: If token is missing or invalid
    """
    if not credentials:
        raise HTTPException(
            status_code=401,
            detail="Missing authentication token",
            headers={"WWW-Authenticate": "Bearer"}
        )

    token = credentials.credentials
    payload = verify_token(token, token_type="access")
    return payload


async def verify_api_key_header(
    api_key: Optional[str] = Security(api_key_header),
    db: Session = Depends(get_db)
) -> bool:
    """
    Dependency to verify API key from header.

    This is a simplified version. In production, you would:
    1. Store API keys in database with metadata (owner, permissions, expiry)
    2. Implement rate limiting per API key
    3. Log API key usage

    Args:
        api_key: API key from X-API-Key header
        db: Database session

    Returns:
        True if API key is valid

    Raises:
        HTTPException: If API key is missing or invalid
    """
    if not api_key:
        raise HTTPException(
            status_code=401,
            detail="Missing API key",
            headers={"X-API-Key": "required"}
        )

    # SECURITY: Use constant-time comparison to prevent timing attacks
    # A timing attack could allow an attacker to determine the correct API key
    # by measuring response times for different inputs
    if not hmac.compare_digest(api_key, settings.MASTER_API_KEY):
        raise HTTPException(
            status_code=401,
            detail="Invalid API key"
        )

    return True


def create_validator_token(
    wallet_address: str,
    node_id: Optional[str] = None,
    expires_delta: Optional[timedelta] = None
) -> str:
    """
    Create a JWT token for validator authentication.

    Args:
        wallet_address: Validator wallet address
        node_id: Optional validator node ID
        expires_delta: Optional expiration time delta

    Returns:
        Encoded JWT token
    """
    additional_claims = {}
    if node_id:
        additional_claims["node_id"] = node_id

    return create_access_token(
        subject=wallet_address,
        expires_delta=expires_delta,
        additional_claims=additional_claims
    )


def verify_wallet_signature(
    wallet_address: str,
    message: str,
    signature: str,
    pubkey: Optional[str] = None
) -> bool:
    """
    Verify a Cosmos SDK wallet signature to prove ownership.

    SECURITY: This function requires the public key to be provided.
    Public-key-recovery-only authentication is cryptographically circular
    (the recovered key always verifies its own signature). The public key
    must be independently provided and verified against the wallet address.

    For Cosmos SDK amino signatures, the flow is:
    1. Client signs: signature = ECDSA_sign(SHA256(message), privkey)
    2. Client sends: {wallet_address, signature, pubkey}
    3. Server verifies: ECDSA_verify(SHA256(message), signature, pubkey)
    4. Server checks: RIPEMD160(SHA256(pubkey)) == address_bytes

    Args:
        wallet_address: The Cosmos bech32 wallet address (e.g., omni1...)
        message: The original message that was signed
        signature: Base64-encoded signature (64 bytes r||s)
        pubkey: Base64-encoded compressed public key (33 bytes) - REQUIRED

    Returns:
        True if signature is valid and matches the wallet address

    Raises:
        ValueError: If address format is invalid, pubkey missing, or verification fails
    """
    import base64
    import hashlib
    import logging

    logger = logging.getLogger(__name__)

    try:
        from bech32 import bech32_decode, convertbits
    except ImportError:
        raise ValueError("Missing bech32 library: pip install bech32")

    try:
        from ecdsa import VerifyingKey, SECP256k1, BadSignatureError
        from ecdsa.util import sigdecode_string
    except ImportError:
        raise ValueError("Missing ecdsa library: pip install ecdsa")

    # SECURITY: Public key MUST be provided for non-circular verification.
    # Without the pubkey, we can only do key recovery which is circular
    # (any signature recovers to some key that trivially verifies).
    if not pubkey:
        raise ValueError(
            "Public key is required for signature verification. "
            "Cosmos SDK amino signatures must include the signer's public key."
        )

    # Decode and validate wallet address
    hrp, data = bech32_decode(wallet_address)
    if hrp is None or data is None:
        raise ValueError(f"Invalid bech32 address: {wallet_address}")

    if hrp != "omni":
        raise ValueError(f"Invalid address prefix: {hrp}, expected 'omni'")

    # Convert from 5-bit to 8-bit to get the expected pubkey hash (20 bytes)
    expected_pubkey_hash = bytes(convertbits(data, 5, 8, False))
    if len(expected_pubkey_hash) != 20:
        raise ValueError(f"Invalid address length: expected 20 bytes, got {len(expected_pubkey_hash)}")

    # Decode signature
    try:
        signature_bytes = base64.b64decode(signature)
    except Exception:
        raise ValueError("Invalid base64 signature")

    if len(signature_bytes) not in (64, 65):
        raise ValueError(f"Invalid signature length: {len(signature_bytes)} (expected 64 or 65)")

    # Use only r||s portion (64 bytes) for ECDSA verification
    sig_r_s = signature_bytes[:64]

    # Validate r and s are non-zero and within curve order
    r = int.from_bytes(sig_r_s[:32], 'big')
    s = int.from_bytes(sig_r_s[32:], 'big')
    curve_order = SECP256k1.order

    if r == 0 or s == 0:
        logger.warning(f"Signature contains zero r or s component for {wallet_address}")
        return False
    if r >= curve_order or s >= curve_order:
        logger.warning(f"Signature r or s exceeds curve order for {wallet_address}")
        return False

    # STEP 1: Decode and validate the provided public key
    try:
        pubkey_bytes = base64.b64decode(pubkey)
    except Exception:
        raise ValueError("Invalid base64 public key")

    compressed_pubkey = None

    if len(pubkey_bytes) == 33:
        # Compressed public key (0x02 or 0x03 prefix)
        # The ecdsa library's from_string expects 64-byte raw (x||y),
        # so we must decompress: recover y from x and the prefix parity bit.
        prefix_byte = pubkey_bytes[0]
        if prefix_byte not in (0x02, 0x03):
            raise ValueError(f"Invalid compressed public key prefix: 0x{prefix_byte:02x}")

        x = int.from_bytes(pubkey_bytes[1:33], 'big')
        p = SECP256k1.curve.p()

        # secp256k1 curve equation: y^2 = x^3 + 7 (mod p)
        y_squared = (pow(x, 3, p) + 7) % p
        y = pow(y_squared, (p + 1) // 4, p)

        # Verify the square root is valid
        if pow(y, 2, p) != y_squared:
            raise ValueError("Invalid public key: x coordinate not on secp256k1 curve")

        # Choose y based on prefix parity (0x02 = even, 0x03 = odd)
        if (y % 2 == 0) != (prefix_byte == 0x02):
            y = p - y

        # Build uncompressed point bytes (64 bytes: x || y) for ecdsa library
        x_bytes = x.to_bytes(32, 'big')
        y_bytes = y.to_bytes(32, 'big')
        vk = VerifyingKey.from_string(x_bytes + y_bytes, curve=SECP256k1)
        compressed_pubkey = pubkey_bytes

    elif len(pubkey_bytes) == 65:
        # Uncompressed public key (0x04 prefix)
        if pubkey_bytes[0] != 0x04:
            raise ValueError(f"Invalid uncompressed public key prefix: 0x{pubkey_bytes[0]:02x}")
        # from_string expects 64-byte raw x||y (no 0x04 prefix)
        vk = VerifyingKey.from_string(pubkey_bytes[1:], curve=SECP256k1)
        # Compress for address derivation
        x = int.from_bytes(pubkey_bytes[1:33], 'big')
        y = int.from_bytes(pubkey_bytes[33:65], 'big')
        prefix = b'\x02' if y % 2 == 0 else b'\x03'
        compressed_pubkey = prefix + x.to_bytes(32, 'big')
    else:
        raise ValueError(f"Invalid public key length: {len(pubkey_bytes)} (expected 33 or 65)")

    # STEP 2: Verify the public key corresponds to the wallet address
    # This is the critical binding: pubkey -> RIPEMD160(SHA256(pubkey)) == address
    actual_pubkey_hash = _hash_pubkey(compressed_pubkey)
    if actual_pubkey_hash != expected_pubkey_hash:
        logger.warning(
            f"Public key does not match wallet address {wallet_address}: "
            f"expected hash {expected_pubkey_hash.hex()}, got {actual_pubkey_hash.hex()}"
        )
        return False

    # STEP 3: Verify the ECDSA signature against the message using the validated pubkey
    # Cosmos SDK signs SHA256(message), and ecdsa.verify with hashfunc hashes again,
    # so we pass the raw message bytes (not pre-hashed) to avoid double-hashing.
    message_bytes = message.encode('utf-8')

    try:
        vk.verify(sig_r_s, message_bytes, hashfunc=hashlib.sha256, sigdecode=sigdecode_string)
        logger.info(f"Signature verified successfully for {wallet_address}")
        return True
    except BadSignatureError:
        logger.warning(f"Invalid ECDSA signature for {wallet_address}")
        return False
    except Exception as e:
        logger.error(f"Signature verification error for {wallet_address}: {type(e).__name__}")
        return False


def _hash_pubkey(compressed_pubkey: bytes) -> bytes:
    """
    Compute the Cosmos SDK address hash from a compressed public key.

    Cosmos addresses are: RIPEMD160(SHA256(pubkey))

    Args:
        compressed_pubkey: 33-byte compressed secp256k1 public key

    Returns:
        20-byte address hash
    """
    import hashlib

    sha256_hash = hashlib.sha256(compressed_pubkey).digest()
    ripemd160 = hashlib.new('ripemd160')
    ripemd160.update(sha256_hash)
    return ripemd160.digest()
