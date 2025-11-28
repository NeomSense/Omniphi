"""Security utilities for authentication and authorization."""

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

    # For now, check against configured master API key
    # In production, query database for stored API keys
    if api_key != settings.MASTER_API_KEY:
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
