"""
API Key Management Service

Part of HIGH-1 Security Remediation: Automated API key rotation with
zero-downtime credential management, audit trails, and emergency revocation.

This service provides:
- Secure API key generation with cryptographically strong randomness
- bcrypt hashing for secure storage
- Constant-time validation
- Key lifecycle management (active, rotating, expired, revoked)
- Audit trail integration
- Zero-downtime rotation support
"""

import secrets
import string
from datetime import datetime, timedelta
from typing import Optional, List, Dict, Any, Tuple
import uuid

import bcrypt
from sqlalchemy.orm import Session
from sqlalchemy import and_, or_

from app.db.models.api_key import APIKey
from app.db.models.enums import APIKeyStatus
from app.models.audit_log import AuditLog, AuditAction


class APIKeyService:
    """Service for managing API key lifecycle and operations."""

    # API key format: ak_<32 hex characters>
    # Prefix identifies key type, followed by cryptographically random string
    KEY_PREFIX = "ak"
    KEY_LENGTH = 32  # Characters after prefix
    BCRYPT_ROUNDS = 12  # bcrypt work factor (2^12 iterations)

    @classmethod
    def generate_key(cls) -> str:
        """
        Generate a cryptographically secure API key.

        Format: ak_<32 random hex characters>
        Example: ak_a1b2c3d4e5f6g7h8i9j0k1l2m3n4o5p6

        Returns:
            Newly generated API key string (plaintext - only shown once!)
        """
        # Use secrets module for cryptographically strong random generation
        random_part = ''.join(
            secrets.choice(string.ascii_lowercase + string.digits)
            for _ in range(cls.KEY_LENGTH)
        )
        return f"{cls.KEY_PREFIX}_{random_part}"

    @classmethod
    def hash_key(cls, api_key: str) -> str:
        """
        Hash an API key using bcrypt.

        Args:
            api_key: Plaintext API key

        Returns:
            bcrypt hash string
        """
        # bcrypt requires bytes
        key_bytes = api_key.encode('utf-8')
        salt = bcrypt.gensalt(rounds=cls.BCRYPT_ROUNDS)
        hashed = bcrypt.hashpw(key_bytes, salt)
        return hashed.decode('utf-8')

    @classmethod
    def verify_key(cls, api_key: str, key_hash: str) -> bool:
        """
        Verify an API key against its hash using constant-time comparison.

        Args:
            api_key: Plaintext API key to verify
            key_hash: bcrypt hash to compare against

        Returns:
            True if key matches hash
        """
        try:
            key_bytes = api_key.encode('utf-8')
            hash_bytes = key_hash.encode('utf-8')
            return bcrypt.checkpw(key_bytes, hash_bytes)
        except Exception:
            # Any exception in verification = invalid key
            return False

    @classmethod
    def extract_prefix(cls, api_key: str) -> str:
        """
        Extract display prefix from API key.

        Args:
            api_key: Full API key

        Returns:
            First 8 characters (e.g., "ak_a1b2c")
        """
        return api_key[:8] if len(api_key) >= 8 else api_key

    @classmethod
    def create_api_key(
        cls,
        db: Session,
        name: str,
        scopes: List[str],
        created_by: uuid.UUID,
        expires_in_days: Optional[int] = None,
        metadata: Optional[Dict[str, Any]] = None,
        ip_address: Optional[str] = None
    ) -> Tuple[APIKey, str]:
        """
        Create a new API key.

        Args:
            db: Database session
            name: Human-readable name for the key
            scopes: List of permission scopes
            created_by: User ID creating the key
            expires_in_days: Days until expiration (None = never expires)
            metadata: Additional metadata
            ip_address: IP address of creator (for audit log)

        Returns:
            Tuple of (APIKey model, plaintext_key)
            IMPORTANT: plaintext_key is only available at creation time!
        """
        # Generate new key
        plaintext_key = cls.generate_key()
        key_hash = cls.hash_key(plaintext_key)
        key_prefix = cls.extract_prefix(plaintext_key)

        # Calculate expiration
        expires_at = None
        if expires_in_days:
            expires_at = datetime.utcnow() + timedelta(days=expires_in_days)

        # Create database record
        api_key = APIKey(
            key_hash=key_hash,
            key_prefix=key_prefix,
            name=name,
            status=APIKeyStatus.ACTIVE.value,
            scopes=scopes or [],
            expires_at=expires_at,
            metadata=metadata or {},
            created_by=created_by,
            updated_by=created_by,
        )

        db.add(api_key)
        db.flush()  # Get ID without committing

        # Create audit log
        audit = AuditLog(
            user_id=str(created_by),
            username=metadata.get('username', 'unknown') if metadata else 'unknown',
            action=AuditAction.GENERATE_API_KEY,
            resource_type='api_key',
            resource_id=str(api_key.id),
            details={
                'key_prefix': key_prefix,
                'name': name,
                'scopes': scopes,
                'expires_at': expires_at.isoformat() if expires_at else None,
            },
            ip_address=ip_address or 'unknown',
        )
        db.add(audit)

        return api_key, plaintext_key

    @classmethod
    def validate_api_key(
        cls,
        db: Session,
        api_key: str,
        required_scopes: Optional[List[str]] = None,
        ip_address: Optional[str] = None
    ) -> Optional[APIKey]:
        """
        Validate an API key and check permissions.

        Args:
            db: Database session
            api_key: Plaintext API key to validate
            required_scopes: Required permission scopes (checks if any match)
            ip_address: IP address making request (for tracking)

        Returns:
            APIKey model if valid, None if invalid
        """
        # Extract prefix for efficient lookup
        key_prefix = cls.extract_prefix(api_key)

        # Query for keys with matching prefix
        # This reduces the number of bcrypt comparisons needed
        candidate_keys = db.query(APIKey).filter(
            and_(
                APIKey.key_prefix == key_prefix,
                APIKey.is_deleted == False,
                or_(
                    APIKey.status == APIKeyStatus.ACTIVE.value,
                    APIKey.status == APIKeyStatus.ROTATING.value
                )
            )
        ).all()

        # Try each candidate (constant-time comparison)
        for key_record in candidate_keys:
            if cls.verify_key(api_key, key_record.key_hash):
                # Found matching key - check if still valid
                if not key_record.is_valid():
                    return None

                # Check expiration
                if key_record.expires_at and key_record.expires_at < datetime.utcnow():
                    # Mark as expired
                    key_record.mark_expired()
                    db.commit()
                    return None

                # Check scopes if required
                if required_scopes:
                    key_scopes = set(key_record.scopes or [])
                    if not any(scope in key_scopes for scope in required_scopes):
                        return None  # Insufficient permissions

                # Valid key - update tracking
                key_record.mark_used(ip_address)
                db.commit()

                return key_record

        # No matching key found
        return None

    @classmethod
    def revoke_api_key(
        cls,
        db: Session,
        key_id: uuid.UUID,
        reason: str,
        revoked_by: uuid.UUID,
        ip_address: Optional[str] = None
    ) -> bool:
        """
        Revoke an API key immediately.

        Args:
            db: Database session
            key_id: ID of key to revoke
            reason: Reason for revocation
            revoked_by: User ID performing revocation
            ip_address: IP address of revoker

        Returns:
            True if revoked, False if not found
        """
        api_key = db.query(APIKey).filter(APIKey.id == key_id).first()
        if not api_key:
            return False

        # Revoke the key
        api_key.revoke(reason)
        api_key.updated_by = revoked_by

        # Create audit log
        audit = AuditLog(
            user_id=str(revoked_by),
            username='unknown',  # Would need to look up
            action=AuditAction.REVOKE_API_KEY,
            resource_type='api_key',
            resource_id=str(key_id),
            details={
                'key_prefix': api_key.key_prefix,
                'reason': reason,
                'name': api_key.name,
            },
            ip_address=ip_address or 'unknown',
        )
        db.add(audit)

        db.commit()
        return True

    @classmethod
    def list_api_keys(
        cls,
        db: Session,
        created_by: Optional[uuid.UUID] = None,
        status: Optional[APIKeyStatus] = None,
        include_deleted: bool = False
    ) -> List[APIKey]:
        """
        List API keys with optional filters.

        Args:
            db: Database session
            created_by: Filter by creator
            status: Filter by status
            include_deleted: Include soft-deleted keys

        Returns:
            List of APIKey models
        """
        query = db.query(APIKey)

        if not include_deleted:
            query = query.filter(APIKey.is_deleted == False)

        if created_by:
            query = query.filter(APIKey.created_by == created_by)

        if status:
            query = query.filter(APIKey.status == status.value)

        return query.order_by(APIKey.created_at.desc()).all()

    @classmethod
    def cleanup_expired_keys(cls, db: Session) -> int:
        """
        Mark expired keys as EXPIRED (background job).

        Args:
            db: Database session

        Returns:
            Number of keys marked as expired
        """
        now = datetime.utcnow()

        expired_keys = db.query(APIKey).filter(
            and_(
                APIKey.status == APIKeyStatus.ACTIVE.value,
                APIKey.expires_at != None,
                APIKey.expires_at < now,
                APIKey.is_deleted == False
            )
        ).all()

        for key in expired_keys:
            key.mark_expired()

        db.commit()
        return len(expired_keys)

    @classmethod
    def rotate_api_key(
        cls,
        db: Session,
        old_key_id: uuid.UUID,
        rotated_by: uuid.UUID,
        overlap_days: int = 7,
        ip_address: Optional[str] = None
    ) -> Tuple[Optional[APIKey], Optional[str]]:
        """
        Rotate an API key with zero-downtime overlap period.

        Creates a new key that will eventually replace the old one.
        Both keys remain valid during overlap period.

        Args:
            db: Database session
            old_key_id: ID of key to rotate
            rotated_by: User performing rotation
            overlap_days: Days both keys remain valid
            ip_address: IP address of requester

        Returns:
            Tuple of (new APIKey, plaintext_new_key) or (None, None) if failed
        """
        # Get old key
        old_key = db.query(APIKey).filter(APIKey.id == old_key_id).first()
        if not old_key:
            return None, None

        # Create new key with same properties
        new_key, plaintext_key = cls.create_api_key(
            db=db,
            name=f"{old_key.name} (rotated)",
            scopes=old_key.scopes,
            created_by=rotated_by,
            expires_in_days=None,  # New key doesn't expire
            metadata={
                **old_key.metadata,
                'rotated_from': str(old_key_id),
                'rotation_date': datetime.utcnow().isoformat(),
            },
            ip_address=ip_address
        )

        # Link keys for rotation tracking
        new_key.replaces_key_id = old_key_id

        # Mark old key as rotating (will expire after overlap period)
        old_key.status = APIKeyStatus.ROTATING.value
        old_key.expires_at = datetime.utcnow() + timedelta(days=overlap_days)
        old_key.updated_by = rotated_by

        # Audit log
        audit = AuditLog(
            user_id=str(rotated_by),
            username='unknown',
            action=AuditAction.ROTATE_API_KEY,
            resource_type='api_key',
            resource_id=str(old_key_id),
            details={
                'old_key_prefix': old_key.key_prefix,
                'new_key_prefix': new_key.key_prefix,
                'overlap_days': overlap_days,
            },
            ip_address=ip_address or 'unknown',
        )
        db.add(audit)

        db.commit()
        return new_key, plaintext_key
