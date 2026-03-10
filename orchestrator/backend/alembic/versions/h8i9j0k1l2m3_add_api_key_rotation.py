"""Add API key rotation system

Revision ID: h8i9j0k1l2m3
Revises: g3h4i5j6k7l8
Create Date: 2026-02-02

This migration implements HIGH-1 security remediation: Automated API key rotation
with zero-downtime credential management, audit trails, and emergency revocation.

Adds:
- api_keys table: Hashed API key storage with lifecycle management
- credential_rotations table: Tracks rotation operations across all credential types
- Comprehensive indexes for performance
- Foreign key relationships for rotation chains

Security features:
- bcrypt hashed keys (never plaintext)
- Key prefix for display/logging only
- Last-used tracking
- Rotation chains for zero-downtime
- Emergency revocation support
"""
from alembic import op
import sqlalchemy as sa
from sqlalchemy.dialects import postgresql

# revision identifiers, used by Alembic.
revision = 'h8i9j0k1l2m3'
down_revision = 'g3h4i5j6k7l8'
branch_labels = None
depends_on = None


def upgrade() -> None:
    """Create API key rotation tables."""

    # =========================================================================
    # CREDENTIAL_ROTATIONS TABLE
    # =========================================================================
    # Create first since api_keys references it

    op.create_table(
        'credential_rotations',
        # Primary key and audit fields (from AuditableModel)
        sa.Column('id', postgresql.UUID(as_uuid=True), primary_key=True),
        sa.Column('created_at', sa.DateTime(), nullable=False),
        sa.Column('updated_at', sa.DateTime(), nullable=False),
        sa.Column('created_by', postgresql.UUID(as_uuid=True), nullable=True),
        sa.Column('updated_by', postgresql.UUID(as_uuid=True), nullable=True),
        sa.Column('is_deleted', sa.Boolean(), nullable=False, default=False),
        sa.Column('deleted_at', sa.DateTime(), nullable=True),

        # Rotation identification
        sa.Column('rotation_name', sa.String(255), nullable=False),

        # Credential type and target
        sa.Column('credential_type', sa.String(50), nullable=False, index=True),
        sa.Column('resource_type', sa.String(100), nullable=True),
        sa.Column('resource_id', sa.String(255), nullable=True, index=True),

        # Rotation status and lifecycle
        sa.Column('status', sa.String(50), nullable=False, index=True),

        # Credential references
        sa.Column('old_credential_id', postgresql.UUID(as_uuid=True), nullable=True),
        sa.Column('new_credential_id', postgresql.UUID(as_uuid=True), nullable=True),

        # Timing
        sa.Column('scheduled_at', sa.DateTime(), nullable=True, index=True),
        sa.Column('started_at', sa.DateTime(), nullable=True),
        sa.Column('completed_at', sa.DateTime(), nullable=True),
        sa.Column('overlap_duration', sa.Interval(), nullable=True),

        # Rotation trigger
        sa.Column('rotation_reason', sa.String(500), nullable=False),
        sa.Column('triggered_by', postgresql.UUID(as_uuid=True), nullable=True),

        # Error tracking
        sa.Column('error_message', sa.String(2000), nullable=True),
        sa.Column('error_stage', sa.String(100), nullable=True),
        sa.Column('retry_count', sa.Integer(), nullable=False, default=0),
        sa.Column('max_retries', sa.Integer(), nullable=False, default=3),

        # Rollback support
        sa.Column('can_rollback', sa.Boolean(), nullable=False, default=True),
        sa.Column('rolled_back_at', sa.DateTime(), nullable=True),
        sa.Column('rollback_reason', sa.String(500), nullable=True),

        # Validation and testing
        sa.Column('validation_tests', postgresql.JSONB(), nullable=False, default=list),

        # Metadata
        sa.Column('metadata', postgresql.JSONB(), nullable=False, default=dict),
    )

    # Indexes for credential_rotations
    op.create_index(
        'ix_rotations_status_scheduled',
        'credential_rotations',
        ['status', 'scheduled_at']
    )
    op.create_index(
        'ix_rotations_type_resource',
        'credential_rotations',
        ['credential_type', 'resource_id']
    )
    op.create_index(
        'ix_rotations_created_status',
        'credential_rotations',
        ['created_at', 'status']
    )

    # =========================================================================
    # API_KEYS TABLE
    # =========================================================================

    op.create_table(
        'api_keys',
        # Primary key and audit fields (from AuditableModel)
        sa.Column('id', postgresql.UUID(as_uuid=True), primary_key=True),
        sa.Column('created_at', sa.DateTime(), nullable=False),
        sa.Column('updated_at', sa.DateTime(), nullable=False),
        sa.Column('created_by', postgresql.UUID(as_uuid=True), nullable=True),
        sa.Column('updated_by', postgresql.UUID(as_uuid=True), nullable=True),
        sa.Column('is_deleted', sa.Boolean(), nullable=False, default=False),
        sa.Column('deleted_at', sa.DateTime(), nullable=True),

        # Key identification and storage
        sa.Column('key_hash', sa.String(255), nullable=False, index=True),
        sa.Column('key_prefix', sa.String(8), nullable=False, index=True),

        # Key metadata
        sa.Column('name', sa.String(255), nullable=False),
        sa.Column('status', sa.String(50), nullable=False, index=True),

        # Lifecycle timestamps
        sa.Column('expires_at', sa.DateTime(), nullable=True, index=True),
        sa.Column('last_used_at', sa.DateTime(), nullable=True),
        sa.Column('revoked_at', sa.DateTime(), nullable=True),
        sa.Column('revoked_reason', sa.String(500), nullable=True),

        # Permissions and scopes
        sa.Column('scopes', postgresql.JSONB(), nullable=False, default=list),

        # Usage tracking
        sa.Column('usage_count', sa.Integer(), nullable=False, default=0),
        sa.Column('last_used_ip', sa.String(45), nullable=True),

        # Rotation chain
        sa.Column('rotation_id', postgresql.UUID(as_uuid=True), nullable=True, index=True),
        sa.Column('replaces_key_id', postgresql.UUID(as_uuid=True), nullable=True),

        # Metadata
        sa.Column('metadata', postgresql.JSONB(), nullable=False, default=dict),

        # Foreign keys
        sa.ForeignKeyConstraint(['rotation_id'], ['credential_rotations.id'], ondelete='SET NULL'),
        sa.ForeignKeyConstraint(['replaces_key_id'], ['api_keys.id'], ondelete='SET NULL'),
    )

    # Indexes for api_keys
    op.create_index(
        'ix_api_keys_status_expires',
        'api_keys',
        ['status', 'expires_at']
    )
    op.create_index(
        'ix_api_keys_created_by_status',
        'api_keys',
        ['created_by', 'status']
    )


def downgrade() -> None:
    """Drop API key rotation tables."""

    # Drop tables in reverse order (respecting foreign keys)
    op.drop_table('api_keys')
    op.drop_table('credential_rotations')
