# HIGH-1 Security Remediation: Automated API Key Rotation System

**Status**: ✅ IMPLEMENTED
**Date**: February 2, 2026
**Security Level**: HIGH
**Component**: Orchestrator Backend - Authentication & Authorization

---

## Table of Contents

1. [Overview](#overview)
2. [Architecture](#architecture)
3. [Database Schema](#database-schema)
4. [Security Features](#security-features)
5. [API Endpoints](#api-endpoints)
6. [Usage Examples](#usage-examples)
7. [Automated Rotation](#automated-rotation)
8. [Emergency Procedures](#emergency-procedures)
9. [Monitoring & Alerts](#monitoring--alerts)
10. [Migration Guide](#migration-guide)

---

## Overview

The API Key Rotation System implements **HIGH-1 security remediation** from the audit report, providing:

- ✅ **Automated key generation** with cryptographically strong randomness
- ✅ **bcrypt hashing** for secure storage (never stores plaintext)
- ✅ **Zero-downtime rotation** with configurable overlap periods
- ✅ **Comprehensive audit trails** for all key operations
- ✅ **Emergency revocation** capabilities
- ✅ **Scope-based permissions** for granular access control
- ✅ **Expiration management** with automatic cleanup

### Key Improvements Over Previous System

| Feature | Before | After HIGH-1 |
|---------|--------|--------------|
| Key Storage | ⚠️ Hardcoded in env vars | ✅ Database with bcrypt hashing |
| Rotation | ❌ Manual, requires downtime | ✅ Automated, zero-downtime |
| Audit Trail | ❌ None | ✅ Full audit log |
| Expiration | ❌ None | ✅ Configurable with auto-cleanup |
| Scopes | ❌ All-or-nothing | ✅ Granular permissions |
| Emergency Revocation | ❌ Config file edits | ✅ Immediate API revocation |

---

## Architecture

### Component Diagram

```
┌─────────────────────────────────────────────────────────────────┐
│                     CLIENT APPLICATION                           │
│  - CI/CD Pipeline                                                │
│  - External Integrations                                         │
│  - Monitoring Tools                                              │
└────────────────┬────────────────────────────────────────────────┘
                 │ X-API-Key: ak_...
                 ▼
┌─────────────────────────────────────────────────────────────────┐
│                  FASTAPI MIDDLEWARE                              │
│                                                                  │
│  ┌──────────────────────────────────────────────────────────┐  │
│  │  verify_api_key_header()                                  │  │
│  │  - Extract key from X-API-Key header                      │  │
│  │  - Validate via APIKeyService                             │  │
│  │  - Check scopes & expiration                              │  │
│  └──────────────────────────────────────────────────────────┘  │
└────────────────┬────────────────────────────────────────────────┘
                 │
                 ▼
┌─────────────────────────────────────────────────────────────────┐
│                    APIKeyService                                 │
│  ┌──────────────────────────────────────────────────────────┐  │
│  │  validate_api_key()                                       │  │
│  │  1. Extract key_prefix for efficient lookup               │  │
│  │  2. Query candidates from DB (prefix match)               │  │
│  │  3. bcrypt verify (constant-time comparison)              │  │
│  │  4. Check status, expiration, scopes                      │  │
│  │  5. Update last_used_at, usage_count                      │  │
│  └──────────────────────────────────────────────────────────┘  │
│                                                                  │
│  ┌──────────────────────────────────────────────────────────┐  │
│  │  create_api_key()                                         │  │
│  │  1. Generate cryptographically secure key                 │  │
│  │  2. Hash with bcrypt (work factor: 12)                    │  │
│  │  3. Store in database                                     │  │
│  │  4. Create audit log                                      │  │
│  │  5. Return plaintext (only shown once!)                   │  │
│  └──────────────────────────────────────────────────────────┘  │
│                                                                  │
│  ┌──────────────────────────────────────────────────────────┐  │
│  │  rotate_api_key()                                         │  │
│  │  1. Create new key with same scopes                       │  │
│  │  2. Mark old key as ROTATING                              │  │
│  │  3. Set old key expiration (overlap period)               │  │
│  │  4. Link keys via replaces_key_id                         │  │
│  └──────────────────────────────────────────────────────────┘  │
└────────────────┬────────────────────────────────────────────────┘
                 │
                 ▼
┌─────────────────────────────────────────────────────────────────┐
│                  PostgreSQL Database                             │
│                                                                  │
│  ┌──────────────────┐        ┌──────────────────────────────┐  │
│  │   api_keys       │        │  credential_rotations        │  │
│  │                  │        │                              │  │
│  │ - id (UUID)      │◄───────┤ - id (UUID)                 │  │
│  │ - key_hash       │        │ - credential_type           │  │
│  │ - key_prefix     │        │ - old_credential_id         │  │
│  │ - name           │        │ - new_credential_id         │  │
│  │ - status         │        │ - status (pending->active)  │  │
│  │ - scopes (JSON)  │        │ - scheduled_at              │  │
│  │ - expires_at     │        │ - overlap_duration          │  │
│  │ - last_used_at   │        │ - validation_tests (JSON)   │  │
│  │ - usage_count    │        └──────────────────────────────┘  │
│  │ - rotation_id ───┼────┐                                     │
│  └──────────────────┘    │                                     │
│                          └───► Foreign Key Link                │
└─────────────────────────────────────────────────────────────────┘
```

### Key Generation Flow

```
generate_api_key()
    │
    ├─► Generate random string (32 chars)
    │   └─► Use secrets.choice() (cryptographically strong)
    │
    ├─► Format: "ak_<32 random chars>"
    │   └─► Example: ak_a1b2c3d4e5f6g7h8i9j0k1l2m3n4o5p6
    │
    ├─► Hash with bcrypt (work factor: 12)
    │   └─► ~2^12 = 4096 iterations
    │
    ├─► Store in database
    │   ├─► key_hash: bcrypt hash
    │   ├─► key_prefix: "ak_a1b2c" (first 8 chars)
    │   ├─► status: ACTIVE
    │   └─► metadata: name, scopes, expiration
    │
    └─► Return plaintext key (ONLY ONCE!)
        └─► Client MUST store securely
```

### Zero-Downtime Rotation Flow

```
┌─────────────────────────────────────────────────────────────┐
│  BEFORE ROTATION                                             │
│  ────────────────                                            │
│                                                              │
│  Old Key: ak_old123...                                       │
│  Status: ACTIVE                                              │
│  Expires: Never                                              │
│                                                              │
│  All requests use: X-API-Key: ak_old123...                   │
└─────────────────────────────────────────────────────────────┘

                     ▼ rotate_api_key()

┌─────────────────────────────────────────────────────────────┐
│  DURING OVERLAP PERIOD (7 days default)                      │
│  ──────────────────────────────────────────────────          │
│                                                              │
│  Old Key: ak_old123...          New Key: ak_new456...        │
│  Status: ROTATING               Status: ACTIVE               │
│  Expires: 7 days from now       Expires: Never               │
│                                                              │
│  BOTH keys work! Client can migrate gradually:               │
│  Day 1-3: Still using ak_old123...                           │
│  Day 4-6: Switching to ak_new456...                          │
│  Day 7:   Must use ak_new456...                              │
└─────────────────────────────────────────────────────────────┘

                     ▼ After overlap period expires

┌─────────────────────────────────────────────────────────────┐
│  AFTER ROTATION COMPLETE                                     │
│  ────────────────────────                                    │
│                                                              │
│  Old Key: ak_old123...          New Key: ak_new456...        │
│  Status: EXPIRED ❌             Status: ACTIVE ✅            │
│                                                              │
│  Only new key works: X-API-Key: ak_new456...                 │
└─────────────────────────────────────────────────────────────┘
```

---

## Database Schema

### `api_keys` Table

```sql
CREATE TABLE api_keys (
    -- Primary key and audit fields (from AuditableModel)
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    created_at TIMESTAMP NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMP NOT NULL DEFAULT NOW(),
    created_by UUID,
    updated_by UUID,
    is_deleted BOOLEAN NOT NULL DEFAULT FALSE,
    deleted_at TIMESTAMP,

    -- Key identification and storage
    key_hash VARCHAR(255) NOT NULL,  -- bcrypt hash (NEVER plaintext!)
    key_prefix VARCHAR(8) NOT NULL,  -- First 8 chars for display/logging

    -- Key metadata
    name VARCHAR(255) NOT NULL,
    status VARCHAR(50) NOT NULL DEFAULT 'active',  -- active, rotating, expired, revoked

    -- Lifecycle timestamps
    expires_at TIMESTAMP,  -- NULL = never expires
    last_used_at TIMESTAMP,
    revoked_at TIMESTAMP,
    revoked_reason VARCHAR(500),

    -- Permissions and scopes
    scopes JSONB NOT NULL DEFAULT '[]',  -- ["read:validators", "write:providers"]

    -- Usage tracking
    usage_count INTEGER NOT NULL DEFAULT 0,
    last_used_ip VARCHAR(45),

    -- Rotation chain
    rotation_id UUID REFERENCES credential_rotations(id) ON DELETE SET NULL,
    replaces_key_id UUID REFERENCES api_keys(id) ON DELETE SET NULL,

    -- Metadata
    metadata JSONB NOT NULL DEFAULT '{}'
);

-- Indexes
CREATE INDEX ix_api_keys_key_hash ON api_keys(key_hash);
CREATE INDEX ix_api_keys_key_prefix ON api_keys(key_prefix);
CREATE INDEX ix_api_keys_status ON api_keys(status);
CREATE INDEX ix_api_keys_status_expires ON api_keys(status, expires_at);
CREATE INDEX ix_api_keys_created_by_status ON api_keys(created_by, status);
```

### `credential_rotations` Table

```sql
CREATE TABLE credential_rotations (
    -- Primary key and audit fields
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    created_at TIMESTAMP NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMP NOT NULL DEFAULT NOW(),
    created_by UUID,
    updated_by UUID,
    is_deleted BOOLEAN NOT NULL DEFAULT FALSE,
    deleted_at TIMESTAMP,

    -- Rotation identification
    rotation_name VARCHAR(255) NOT NULL,

    -- Credential type and target
    credential_type VARCHAR(50) NOT NULL,  -- api_key, aws_iam, etc.
    resource_type VARCHAR(100),
    resource_id VARCHAR(255),

    -- Rotation status
    status VARCHAR(50) NOT NULL DEFAULT 'pending',
    -- pending → generating → deploying → testing → active → finalizing → completed

    -- Credential references
    old_credential_id UUID,
    new_credential_id UUID,

    -- Timing
    scheduled_at TIMESTAMP,
    started_at TIMESTAMP,
    completed_at TIMESTAMP,
    overlap_duration INTERVAL,  -- e.g., '7 days'

    -- Rotation trigger
    rotation_reason VARCHAR(500) NOT NULL,
    triggered_by UUID,

    -- Error tracking
    error_message VARCHAR(2000),
    error_stage VARCHAR(100),
    retry_count INTEGER NOT NULL DEFAULT 0,
    max_retries INTEGER NOT NULL DEFAULT 3,

    -- Rollback support
    can_rollback BOOLEAN NOT NULL DEFAULT TRUE,
    rolled_back_at TIMESTAMP,
    rollback_reason VARCHAR(500),

    -- Validation and testing
    validation_tests JSONB NOT NULL DEFAULT '[]',

    -- Metadata
    metadata JSONB NOT NULL DEFAULT '{}'
);

-- Indexes
CREATE INDEX ix_rotations_status_scheduled ON credential_rotations(status, scheduled_at);
CREATE INDEX ix_rotations_type_resource ON credential_rotations(credential_type, resource_id);
CREATE INDEX ix_rotations_created_status ON credential_rotations(created_at, status);
```

---

## Security Features

### 1. Cryptographically Secure Generation

```python
# Uses Python's secrets module (cryptographically strong PRNG)
random_part = ''.join(
    secrets.choice(string.ascii_lowercase + string.digits)
    for _ in range(32)
)
api_key = f"ak_{random_part}"

# Entropy: 36^32 = ~1.7 × 10^49 possible keys
# Brute force time: ~10^39 years at 1 billion attempts/second
```

### 2. bcrypt Hashing (Work Factor: 12)

```python
# Hash generation
salt = bcrypt.gensalt(rounds=12)  # 2^12 = 4096 iterations
hashed = bcrypt.hashpw(api_key.encode('utf-8'), salt)

# Verification (constant-time to prevent timing attacks)
is_valid = bcrypt.checkpw(api_key.encode('utf-8'), stored_hash.encode('utf-8'))
```

**Why bcrypt?**
- Adaptive work factor (can increase as hardware improves)
- Salt automatically generated and stored with hash
- Resistant to rainbow table attacks
- Industry standard for password/key storage

### 3. Constant-Time Comparison

All key verification uses constant-time comparison to prevent timing attacks:

```python
# BAD (timing attack vulnerable):
if api_key == stored_key:
    return True

# GOOD (constant-time):
return bcrypt.checkpw(api_key.encode(), stored_hash.encode())
```

### 4. Scope-Based Permissions

Granular access control via scopes:

```json
{
  "scopes": [
    "read:validators",
    "write:validators",
    "read:providers",
    "admin:settings"
  ]
}
```

Scopes are checked during validation:

```python
# Endpoint requires specific scope
@router.get("/validators")
async def list_validators(
    api_key: APIKey = Depends(require_scopes(["read:validators"]))
):
    ...
```

### 5. Automatic Expiration

Keys can have expiration dates:

```python
# Create key that expires in 90 days
expires_in_days = 90
expires_at = datetime.utcnow() + timedelta(days=expires_in_days)

# Background job automatically marks expired keys
APIKeyService.cleanup_expired_keys(db)
```

### 6. Audit Trail

Every key operation is logged:

```python
audit = AuditLog(
    user_id=str(created_by),
    action=AuditAction.GENERATE_API_KEY,
    resource_type='api_key',
    resource_id=str(api_key.id),
    details={
        'key_prefix': key_prefix,
        'name': name,
        'scopes': scopes,
    },
    ip_address=ip_address,
)
```

---

## API Endpoints

### POST `/api/v1/auth/api-key/generate`

Generate a new API key.

**Headers:**
- `X-API-Key`: Master API key (required)
- `Content-Type`: application/json

**Request Body:**
```json
{
  "name": "CI/CD Pipeline Key",
  "scopes": ["read:validators", "write:validators"],
  "expires_in_days": 90
}
```

**Response:**
```json
{
  "api_key": "ak_a1b2c3d4e5f6g7h8i9j0k1l2m3n4o5p6",
  "message": "Store this key securely. It will not be shown again."
}
```

⚠️ **CRITICAL**: The plaintext key is only returned once. Store it securely!

---

### GET `/api/v1/auth/api-key/list`

List all API keys (without plaintext values).

**Headers:**
- `X-API-Key`: Master API key (required)

**Response:**
```json
[
  {
    "id": "123e4567-e89b-12d3-a456-426614174000",
    "key_prefix": "ak_a1b2c",
    "name": "CI/CD Pipeline Key",
    "status": "active",
    "scopes": ["read:validators", "write:validators"],
    "expires_at": "2026-05-02T00:00:00Z",
    "last_used_at": "2026-02-02T14:30:00Z",
    "usage_count": 1247,
    "created_at": "2026-02-02T10:00:00Z"
  }
]
```

---

### POST `/api/v1/auth/api-key/rotate`

Rotate an API key with zero-downtime overlap.

**Headers:**
- `X-API-Key`: Master API key (required)
- `Content-Type`: application/json

**Request Body:**
```json
{
  "key_id": "123e4567-e89b-12d3-a456-426614174000",
  "overlap_days": 7
}
```

**Response:**
```json
{
  "api_key": "ak_new456xyz789...",
  "message": "New key created. Old key valid for 7 more days."
}
```

**What Happens:**
1. New key created with same scopes
2. Old key marked as `ROTATING`
3. Old key expires after `overlap_days`
4. Both keys work during overlap period
5. Audit log created

---

### POST `/api/v1/auth/api-key/revoke`

Immediately revoke an API key.

**Headers:**
- `X-API-Key`: Master API key (required)
- `Content-Type`: application/json

**Request Body:**
```json
{
  "key_id": "123e4567-e89b-12d3-a456-426614174000",
  "reason": "Key compromised in security incident"
}
```

**Response:**
```json
{
  "message": "API key revoked successfully"
}
```

**What Happens:**
1. Key status set to `REVOKED`
2. Key immediately invalid (next request fails)
3. Revocation reason stored
4. Audit log created

---

## Usage Examples

### Example 1: Generate Key for CI/CD

```bash
#!/bin/bash
# generate_ci_key.sh

# Generate new key
RESPONSE=$(curl -s -X POST "http://localhost:8000/api/v1/auth/api-key/generate" \
  -H "X-API-Key: $MASTER_API_KEY" \
  -H "Content-Type: application/json" \
  -d '{
    "name": "GitHub Actions CI",
    "scopes": ["read:validators", "write:providers"],
    "expires_in_days": 90
  }')

# Extract key
NEW_KEY=$(echo $RESPONSE | jq -r '.api_key')

# Store in GitHub Secrets
gh secret set OMNIPHI_API_KEY --body "$NEW_KEY"

echo "✅ New CI key generated and stored in GitHub Secrets"
```

### Example 2: Rotate Key Before Expiration

```bash
#!/bin/bash
# rotate_key.sh

KEY_ID="123e4567-e89b-12d3-a456-426614174000"

# Rotate with 7-day overlap
RESPONSE=$(curl -s -X POST "http://localhost:8000/api/v1/auth/api-key/rotate" \
  -H "X-API-Key: $MASTER_API_KEY" \
  -H "Content-Type: application/json" \
  -d "{
    \"key_id\": \"$KEY_ID\",
    \"overlap_days\": 7
  }")

NEW_KEY=$(echo $RESPONSE | jq -r '.api_key')

echo "✅ Key rotated. You have 7 days to update all services."
echo "New key: $NEW_KEY"
```

### Example 3: Emergency Revocation

```bash
#!/bin/bash
# emergency_revoke.sh

KEY_ID="123e4567-e89b-12d3-a456-426614174000"

curl -X POST "http://localhost:8000/api/v1/auth/api-key/revoke" \
  -H "X-API-Key: $MASTER_API_KEY" \
  -H "Content-Type: application/json" \
  -d "{
    \"key_id\": \"$KEY_ID\",
    \"reason\": \"EMERGENCY: Key exposed in public repository\"
  }"

echo "🚨 Key immediately revoked"
```

---

## Automated Rotation

### Scheduled Rotation (Cron Job)

```python
# orchestrator/backend/app/tasks/credential_rotation.py

from apscheduler.schedulers.background import BackgroundScheduler

def rotate_expiring_keys():
    """Rotate keys expiring in next 7 days."""
    from app.services.api_key_service import APIKeyService
    from app.db.session import SessionLocal

    db = SessionLocal()
    try:
        # Find keys expiring soon
        expiring_soon = db.query(APIKey).filter(
            and_(
                APIKey.status == APIKeyStatus.ACTIVE.value,
                APIKey.expires_at >= datetime.utcnow(),
                APIKey.expires_at <= datetime.utcnow() + timedelta(days=7)
            )
        ).all()

        for key in expiring_soon:
            # Auto-rotate each key
            APIKeyService.rotate_api_key(
                db=db,
                old_key_id=key.id,
                rotated_by=uuid.UUID('00000000-0000-0000-0000-000000000000'),  # system
                overlap_days=7
            )
            logger.info(f"Auto-rotated key {key.key_prefix} (expiring soon)")
    finally:
        db.close()

# Schedule rotation check daily at 2 AM
scheduler = BackgroundScheduler()
scheduler.add_job(rotate_expiring_keys, 'cron', hour=2, minute=0)
scheduler.start()
```

---

## Emergency Procedures

### Scenario 1: Key Compromised

**Immediate Actions:**

1. **Revoke compromised key**:
```bash
curl -X POST "http://localhost:8000/api/v1/auth/api-key/revoke" \
  -H "X-API-Key: $MASTER_API_KEY" \
  -d '{"key_id": "$COMPROMISED_KEY_ID", "reason": "Key exposed in logs"}'
```

2. **Generate replacement immediately**:
```bash
curl -X POST "http://localhost:8000/api/v1/auth/api-key/generate" \
  -H "X-API-Key: $MASTER_API_KEY" \
  -d '{"name": "Emergency Replacement", "scopes": [...], "expires_in_days": 30}'
```

3. **Update all services** with new key
4. **Review audit logs** for unauthorized usage
5. **Investigate compromise** source

---

### Scenario 2: Mass Compromise (Security Incident)

**Emergency Mass Revocation:**

```python
from app.services.credential_rotation_service import CredentialRotationService
from app.db.models.enums import CredentialType

# Revoke ALL API keys
count = CredentialRotationService.emergency_revoke_all_keys(
    db=db,
    credential_type=CredentialType.API_KEY,
    reason="Security incident: Database backup leaked",
    revoked_by=admin_user_id
)

logger.critical(f"EMERGENCY: Revoked {count} API keys")
```

**Recovery Steps:**

1. Revoke all keys (above)
2. Generate new keys for critical services
3. Notify all API key holders
4. Investigate breach thoroughly
5. Implement additional security controls

---

## Monitoring & Alerts

### Key Metrics to Track

1. **Active Keys**: Number of active API keys
2. **Expiring Soon**: Keys expiring in next 7 days
3. **Rotation Success Rate**: % of successful rotations
4. **Failed Validation Attempts**: Potential attacks
5. **Average Key Age**: Time since key creation

### Recommended Alerts

```yaml
# Prometheus alerts

- alert: APIKeyExpiringNoRotation
  expr: api_keys_expiring_7days > 0
  for: 24h
  annotations:
    summary: "API keys expiring soon without rotation scheduled"

- alert: HighFailedKeyValidations
  expr: rate(api_key_validation_failures[5m]) > 10
  for: 5m
  annotations:
    summary: "High rate of failed API key validations (possible attack)"

- alert: NoKeyRotationIn90Days
  expr: max(api_key_age_days) > 90
  for: 1h
  annotations:
    summary: "Some API keys haven't been rotated in 90+ days"
```

---

## Migration Guide

### Step 1: Run Database Migration

```bash
cd orchestrator/backend
alembic upgrade head
```

This creates the `api_keys` and `credential_rotations` tables.

### Step 2: Generate Initial API Keys

```bash
# Generate key for each integration
curl -X POST "http://localhost:8000/api/v1/auth/api-key/generate" \
  -H "X-API-Key: $MASTER_API_KEY" \
  -H "Content-Type: application/json" \
  -d '{
    "name": "CI/CD Pipeline",
    "scopes": ["read:validators", "write:providers"],
    "expires_in_days": 90
  }'

# Store the returned key securely!
```

### Step 3: Update Applications

Replace hardcoded keys with new generated keys:

```python
# Before (hardcoded)
API_KEY = "some-hardcoded-key"

# After (from secure storage)
API_KEY = os.environ["OMNIPHI_API_KEY"]  # From secrets manager
```

### Step 4: Set Up Automated Rotation

Enable the rotation scheduler:

```python
# orchestrator/backend/app/main.py

from app.tasks.credential_rotation import scheduler

# Start scheduler on application startup
scheduler.start()
```

### Step 5: Configure Monitoring

Set up alerts for key expiration and failed validations.

---

## Best Practices

### DO ✅

- **Rotate keys every 90 days** (minimum)
- **Use shortest practical expiration** (30-90 days)
- **Limit scopes** to only what's needed
- **Monitor failed validations** for attacks
- **Store keys in secrets manager** (Vault, AWS Secrets Manager)
- **Use separate keys per environment** (dev, staging, prod)

### DON'T ❌

- **Never commit keys to git** (even .env files)
- **Never log plaintext keys** (only first 8 chars)
- **Never share keys between services** (1 key per service)
- **Never skip rotation** (automate it!)
- **Never use keys without expiration** (always set expires_in_days)

---

## Implementation Status

| Component | Status | File |
|-----------|--------|------|
| Database Models | ✅ Complete | [api_key.py](../app/db/models/api_key.py) |
| Database Models | ✅ Complete | [credential_rotation.py](../app/db/models/credential_rotation.py) |
| Database Migration | ✅ Complete | [h8i9j0k1l2m3_add_api_key_rotation.py](../alembic/versions/h8i9j0k1l2m3_add_api_key_rotation.py) |
| API Key Service | ✅ Complete | [api_key_service.py](../app/services/api_key_service.py) |
| Rotation Service | ✅ Complete | [credential_rotation_service.py](../app/services/credential_rotation_service.py) |
| API Endpoints | ✅ Complete | [auth.py](../app/api/v1/auth.py) |
| Audit Log Integration | ✅ Complete | [audit_log.py](../app/models/audit_log.py) |
| Scheduler (Cron) | ⏳ Pending | To be implemented |
| Monitoring Alerts | ⏳ Pending | To be configured |

---

## Next Steps (Week 2-3)

1. **Implement automated scheduler** for key rotation
2. **Set up Prometheus metrics** for monitoring
3. **Configure alerts** for expiring/compromised keys
4. **Extend to cloud provider credentials** (AWS, DigitalOcean)
5. **Third-party audit** of implementation

---

## References

- [Audit Report](../../AUDIT_REPORT_2025.md) - HIGH-1 Finding
- [Remediation Plan](../../AUDIT_REMEDIATION_PLAN.md) - Implementation timeline
- [OWASP API Security](https://owasp.org/www-project-api-security/) - API key best practices
- [NIST Digital Identity Guidelines](https://pages.nist.gov/800-63-3/) - Credential management

---

**Document Version**: 1.0
**Last Updated**: February 2, 2026
**Maintained By**: Senior Blockchain Engineer
**Security Review**: Required before production deployment
