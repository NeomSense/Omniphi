"""Authentication endpoints with wallet signature verification."""

import logging
import time
import hashlib
from datetime import timedelta
from typing import Optional

from fastapi import APIRouter, HTTPException, Depends, Security
from pydantic import BaseModel, field_validator
from sqlalchemy.orm import Session

from app.core.config import settings
from app.core.security import (
    create_access_token,
    create_refresh_token,
    verify_token,
    generate_api_key,
    create_validator_token,
    verify_api_key_header,
    verify_wallet_signature
)
from app.core.nonce_store import nonce_store
from app.db.session import get_db

logger = logging.getLogger(__name__)

router = APIRouter()

# Nonce expiry from settings (used for timestamp validation)
NONCE_EXPIRY_SECONDS = settings.NONCE_EXPIRY_SECONDS


# Request/Response Models
class TokenRequest(BaseModel):
    """
    Request model for token creation with wallet signature verification.

    SECURITY: Requires cryptographic proof of wallet ownership via signature.
    The client must sign a message containing the wallet address, nonce, and timestamp.
    The public key MUST be provided for non-circular signature verification.

    Message format to sign: "{wallet_address}:{nonce}:{timestamp}"
    """
    wallet_address: str
    node_id: Optional[str] = None
    signature: str  # Cosmos SDK amino signature (base64 encoded, 64 bytes r||s)
    pubkey: str  # Compressed public key (base64 encoded, 33 bytes) - REQUIRED
    nonce: str  # One-time nonce to prevent replay attacks
    timestamp: int  # Unix timestamp of signature creation

    @field_validator('timestamp')
    @classmethod
    def validate_timestamp(cls, v):
        """Ensure timestamp is within acceptable window (prevents replay)."""
        current_time = int(time.time())
        if abs(current_time - v) > NONCE_EXPIRY_SECONDS:
            raise ValueError('Timestamp outside acceptable window (±5 minutes)')
        return v

    @field_validator('wallet_address')
    @classmethod
    def validate_wallet_address(cls, v):
        """Validate wallet address format."""
        if not v.startswith('omni1'):
            raise ValueError('Invalid wallet address format (must start with omni1)')
        if len(v) < 39 or len(v) > 50:
            raise ValueError('Invalid wallet address length')
        return v


class TokenResponse(BaseModel):
    """Response model for token creation."""
    access_token: str
    refresh_token: str
    token_type: str = "bearer"
    expires_in: int


class RefreshTokenRequest(BaseModel):
    """Request model for token refresh."""
    refresh_token: str


class APIKeyResponse(BaseModel):
    """Response model for API key generation."""
    api_key: str
    message: str


class ChallengeResponse(BaseModel):
    """Response model for authentication challenge."""
    nonce: str
    message_to_sign: str
    expires_at: int


@router.get("/challenge/{wallet_address}", response_model=ChallengeResponse)
async def get_auth_challenge(wallet_address: str):
    """
    Get a challenge nonce for wallet authentication.

    The client must sign the returned message with their wallet's private key
    and submit it to /token to receive access tokens.

    Args:
        wallet_address: The wallet address requesting authentication

    Returns:
        Challenge nonce and message format to sign

    Example:
        ```bash
        curl http://localhost:8000/api/v1/auth/challenge/omni1abc...
        ```
    """
    # Validate address format
    if not wallet_address.startswith('omni1'):
        raise HTTPException(status_code=400, detail="Invalid wallet address format")

    # Generate challenge nonce
    nonce = hashlib.sha256(f"{wallet_address}{time.time()}{settings.SECRET_KEY}".encode()).hexdigest()[:32]
    timestamp = int(time.time())
    expires_at = timestamp + NONCE_EXPIRY_SECONDS

    # Message format that client must sign
    message_to_sign = f"{wallet_address}:{nonce}:{timestamp}"

    return ChallengeResponse(
        nonce=nonce,
        message_to_sign=message_to_sign,
        expires_at=expires_at
    )


@router.post("/token", response_model=TokenResponse)
async def create_token(
    request: TokenRequest,
    db: Session = Depends(get_db)
):
    """
    Create JWT access and refresh tokens for a validator.

    SECURITY: This endpoint requires cryptographic proof of wallet ownership.
    The client must:
    1. Get a challenge from /challenge/{wallet_address}
    2. Sign the challenge message with their wallet private key
    3. Submit the signature here

    Args:
        request: Token creation request with wallet address, signature, nonce, and timestamp
        db: Database session

    Returns:
        Access token, refresh token, and expiration info

    Example:
        ```bash
        # Step 1: Get challenge
        CHALLENGE=$(curl -s http://localhost:8000/api/v1/auth/challenge/omni1abc...)

        # Step 2: Sign message with wallet (using posd CLI)
        MESSAGE=$(echo $CHALLENGE | jq -r '.message_to_sign')
        SIGNATURE=$(posd keys sign $MESSAGE --from validator)

        # Step 3: Submit for token
        curl -X POST http://localhost:8000/api/v1/auth/token \\
          -H "Content-Type: application/json" \\
          -d '{
            "wallet_address": "omni1...",
            "signature": "'$SIGNATURE'",
            "nonce": "'$(echo $CHALLENGE | jq -r '.nonce')'",
            "timestamp": '$(date +%s)'
          }'
        ```

    Raises:
        HTTPException 401: Invalid signature or nonce replay
        HTTPException 400: Invalid request format
    """
    logger.info(f"Token request for wallet: {request.wallet_address}")

    # SECURITY: Verify nonce hasn't been used (prevents replay attacks)
    # Uses Redis-backed storage for multi-instance support
    if not nonce_store.verify_and_consume(request.nonce, request.wallet_address):
        logger.warning(f"Nonce replay attempt detected for wallet: {request.wallet_address}")
        raise HTTPException(
            status_code=401,
            detail="Invalid or already used nonce. Request a new challenge."
        )

    # SECURITY: Verify wallet signature proves ownership
    # The pubkey is REQUIRED - without it, signature verification is circular
    message_to_verify = f"{request.wallet_address}:{request.nonce}:{request.timestamp}"

    try:
        is_valid = verify_wallet_signature(
            wallet_address=request.wallet_address,
            message=message_to_verify,
            signature=request.signature,
            pubkey=request.pubkey
        )
    except ValueError as e:
        logger.warning(f"Signature verification rejected: {e}")
        raise HTTPException(
            status_code=401,
            detail=str(e)
        )
    except Exception as e:
        logger.error(f"Signature verification error: {type(e).__name__}")
        raise HTTPException(
            status_code=401,
            detail="Signature verification failed"
        )

    if not is_valid:
        logger.warning(f"Invalid signature for wallet: {request.wallet_address}")
        raise HTTPException(
            status_code=401,
            detail="Invalid signature. Wallet ownership verification failed."
        )

    logger.info(f"Wallet ownership verified, creating token for: {request.wallet_address}")

    # Create tokens
    access_token = create_validator_token(
        wallet_address=request.wallet_address,
        node_id=request.node_id
    )
    refresh_token = create_refresh_token(subject=request.wallet_address)

    return TokenResponse(
        access_token=access_token,
        refresh_token=refresh_token,
        token_type="bearer",
        expires_in=settings.ACCESS_TOKEN_EXPIRE_MINUTES * 60
    )


@router.post("/token/refresh", response_model=TokenResponse)
async def refresh_token(
    request: RefreshTokenRequest,
    db: Session = Depends(get_db)
):
    """
    Refresh an access token using a refresh token.

    Args:
        request: Refresh token request
        db: Database session

    Returns:
        New access token and refresh token

    Raises:
        HTTPException: If refresh token is invalid or expired
    """
    # Verify refresh token
    payload = verify_token(request.refresh_token, token_type="refresh")
    wallet_address = payload.get("sub")

    if not wallet_address:
        raise HTTPException(
            status_code=401,
            detail="Invalid refresh token payload"
        )

    logger.info(f"Refreshing token for wallet: {wallet_address}")

    # Create new tokens
    node_id = payload.get("node_id")
    access_token = create_validator_token(
        wallet_address=wallet_address,
        node_id=node_id
    )
    new_refresh_token = create_refresh_token(subject=wallet_address)

    return TokenResponse(
        access_token=access_token,
        refresh_token=new_refresh_token,
        token_type="bearer",
        expires_in=settings.ACCESS_TOKEN_EXPIRE_MINUTES * 60
    )


@router.post("/api-key/generate", response_model=APIKeyResponse)
async def generate_new_api_key(
    _: bool = Security(verify_api_key_header),  # SECURITY: Requires master API key
    db: Session = Depends(get_db)
):
    """
    Generate a new API key for external integrations.

    **IMPORTANT**:
    - This endpoint requires the master API key (X-API-Key header)
    - Store the generated key securely. It will only be shown once.

    Returns:
        Generated API key

    Example:
        ```bash
        curl -X POST http://localhost:8000/api/v1/auth/api-key/generate \\
          -H "X-API-Key: your-master-api-key"
        ```

    Raises:
        HTTPException 401: Missing or invalid master API key
    """
    api_key = generate_api_key()

    logger.info("Generated new API key (first 8 chars): " + api_key[:8] + "...")

    # TODO: Store the hashed API key in database with metadata:
    # - Created timestamp
    # - Owner/description
    # - Permissions/scopes
    # - Expiration date
    # - Last used timestamp

    return APIKeyResponse(
        api_key=api_key,
        message="Store this key securely. It will not be shown again."
    )


@router.get("/verify")
async def verify_authentication(
    db: Session = Depends(get_db)
):
    """
    Verify that authentication is working.

    This is a public endpoint for testing authentication setup.

    Returns:
        Authentication status and configuration info
    """
    # Get nonce store health status
    nonce_health = nonce_store.health_check()

    return {
        "success": True,
        "message": "Authentication endpoints are available",
        "endpoints": {
            "get_challenge": "/api/v1/auth/challenge/{wallet_address}",
            "create_token": "/api/v1/auth/token",
            "refresh_token": "/api/v1/auth/token/refresh",
            "generate_api_key": "/api/v1/auth/api-key/generate (requires master API key)"
        },
        "token_expiration": f"{settings.ACCESS_TOKEN_EXPIRE_MINUTES} minutes",
        "rate_limiting_enabled": settings.RATE_LIMIT_ENABLED,
        "signature_verification": "required",
        "nonce_storage": nonce_health
    }
