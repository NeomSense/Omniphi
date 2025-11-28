"""Authentication endpoints."""

import logging
from datetime import timedelta
from typing import Optional

from fastapi import APIRouter, HTTPException, Depends
from pydantic import BaseModel
from sqlalchemy.orm import Session

from app.core.config import settings
from app.core.security import (
    create_access_token,
    create_refresh_token,
    verify_token,
    generate_api_key,
    create_validator_token
)
from app.db.session import get_db

logger = logging.getLogger(__name__)

router = APIRouter()


# Request/Response Models
class TokenRequest(BaseModel):
    """Request model for token creation."""
    wallet_address: str
    node_id: Optional[str] = None


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


@router.post("/token", response_model=TokenResponse)
async def create_token(
    request: TokenRequest,
    db: Session = Depends(get_db)
):
    """
    Create JWT access and refresh tokens for a validator.

    This endpoint allows validators to authenticate and receive tokens
    for accessing protected endpoints.

    Args:
        request: Token creation request with wallet address and optional node ID
        db: Database session

    Returns:
        Access token, refresh token, and expiration info

    Example:
        ```bash
        curl -X POST http://localhost:8000/api/v1/auth/token \\
          -H "Content-Type: application/json" \\
          -d '{"wallet_address": "omni1...", "node_id": "node123"}'
        ```
    """
    logger.info(f"Creating token for wallet: {request.wallet_address}")

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
        expires_in=settings.ACCESS_TOKEN_EXPIRE_MINUTES * 60  # in seconds
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

    Example:
        ```bash
        curl -X POST http://localhost:8000/api/v1/auth/token/refresh \\
          -H "Content-Type: application/json" \\
          -d '{"refresh_token": "eyJ..."}'
        ```
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
    db: Session = Depends(get_db)
):
    """
    Generate a new API key for external integrations.

    **IMPORTANT**: Store this key securely. It will only be shown once.

    In production, this endpoint should be protected with admin authentication.

    Returns:
        Generated API key

    Example:
        ```bash
        curl -X POST http://localhost:8000/api/v1/auth/api-key/generate \\
          -H "X-API-Key: your-master-api-key"
        ```
    """
    api_key = generate_api_key()

    logger.info("Generated new API key (first 8 chars): " + api_key[:8] + "...")

    # In production, store the hashed API key in database with metadata:
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
    return {
        "success": True,
        "message": "Authentication endpoints are available",
        "endpoints": {
            "create_token": "/api/v1/auth/token",
            "refresh_token": "/api/v1/auth/token/refresh",
            "generate_api_key": "/api/v1/auth/api-key/generate"
        },
        "token_expiration": f"{settings.ACCESS_TOKEN_EXPIRE_MINUTES} minutes",
        "rate_limiting_enabled": settings.RATE_LIMIT_ENABLED
    }
