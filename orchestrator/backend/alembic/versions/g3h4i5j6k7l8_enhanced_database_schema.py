"""Enhanced database schema for Omniphi Cloud

Revision ID: g3h4i5j6k7l8
Revises: None
Create Date: 2024-11-24

This migration creates the enhanced database schema including:
- Enhanced Region infrastructure (regions, region_servers, server_pools)
- Enhanced Validator management (validator_setup_requests, validator_nodes, local_validator_heartbeats)
- Provider marketplace (providers, pricing_tiers, metrics, applications, verifications, slas, reviews)
- Billing system (accounts, plans, subscriptions, invoices, payments, usage)
- Snapshots and Upgrades (snapshots, upgrades, upgrade_rollouts)
- Monitoring/SRE (node_metrics, incidents)

Note: Uses SQLite-compatible types (JSON instead of JSONB, String for UUIDs)
"""
from alembic import op
import sqlalchemy as sa
from sqlalchemy.engine import reflection

# revision identifiers, used by Alembic.
revision = 'g3h4i5j6k7l8'
down_revision = None  # This is now the first migration
branch_labels = None
depends_on = None


def upgrade() -> None:
    """Create enhanced database schema."""

    # Note: ENUMs are stored as strings for SQLite compatibility.
    # PostgreSQL-specific ENUM types would be created here in production.

    # =========================================================================
    # TABLES - Region & Infrastructure
    # =========================================================================

    op.create_table(
        'regions',
        sa.Column('id', sa.String(36), primary_key=True),
        sa.Column('code', sa.String(50), nullable=False, unique=True, index=True),
        sa.Column('name', sa.String(100), nullable=False),
        sa.Column('display_name', sa.String(100), nullable=False),
        sa.Column('continent', sa.String(50), nullable=False),
        sa.Column('country', sa.String(100), nullable=False),
        sa.Column('country_code', sa.String(2), nullable=False),
        sa.Column('city', sa.String(100), nullable=True),
        sa.Column('latitude', sa.Float(), nullable=True),
        sa.Column('longitude', sa.Float(), nullable=True),
        sa.Column('timezone', sa.String(50), nullable=True),
        sa.Column('status', sa.String(20), nullable=False, default='active', index=True),
        sa.Column('is_default', sa.Boolean(), nullable=False, default=False),
        sa.Column('priority', sa.Integer(), nullable=False, default=100),
        sa.Column('cloud_provider', sa.String(50), nullable=False, default='omniphi'),
        sa.Column('cloud_region', sa.String(100), nullable=True),
        sa.Column('cloud_zones', sa.JSON(), nullable=False, default=[]),
        sa.Column('total_servers', sa.Integer(), nullable=False, default=0),
        sa.Column('available_servers', sa.Integer(), nullable=False, default=0),
        sa.Column('total_validators', sa.Integer(), nullable=False, default=0),
        sa.Column('active_validators', sa.Integer(), nullable=False, default=0),
        sa.Column('max_validators', sa.Integer(), nullable=False, default=1000),
        sa.Column('reserved_capacity', sa.Integer(), nullable=False, default=0),
        sa.Column('cpu_allocated', sa.Integer(), nullable=False, default=0),
        sa.Column('cpu_total', sa.Integer(), nullable=False, default=0),
        sa.Column('memory_allocated_gb', sa.Float(), nullable=False, default=0),
        sa.Column('memory_total_gb', sa.Float(), nullable=False, default=0),
        sa.Column('storage_allocated_gb', sa.Float(), nullable=False, default=0),
        sa.Column('storage_total_gb', sa.Float(), nullable=False, default=0),
        sa.Column('features', sa.JSON(), nullable=False, default={}),
        sa.Column('extra_data', sa.JSON(), nullable=False, default={}),
        sa.Column('created_at', sa.DateTime(), nullable=False, server_default=sa.func.now()),
        sa.Column('updated_at', sa.DateTime(), nullable=False, server_default=sa.func.now(), onupdate=sa.func.now()),
    )
    op.create_index('ix_regions_status_priority', 'regions', ['status', 'priority'])

    op.create_table(
        'server_pools',
        sa.Column('id', sa.String(36), primary_key=True),
        sa.Column('region_id', sa.String(36), sa.ForeignKey('regions.id', ondelete='CASCADE'), nullable=False),
        sa.Column('name', sa.String(100), nullable=False),
        sa.Column('tier_code', sa.String(50), nullable=False, index=True),
        sa.Column('machine_type', sa.String(50), nullable=False),
        sa.Column('cpu_cores', sa.Integer(), nullable=False),
        sa.Column('memory_gb', sa.Integer(), nullable=False),
        sa.Column('storage_gb', sa.Integer(), nullable=False),
        sa.Column('storage_type', sa.String(50), nullable=False, default='nvme'),
        sa.Column('network_gbps', sa.Float(), nullable=False, default=1.0),
        sa.Column('gpu_type', sa.String(100), nullable=True),
        sa.Column('gpu_count', sa.Integer(), nullable=True),
        sa.Column('total_servers', sa.Integer(), nullable=False, default=0),
        sa.Column('available_servers', sa.Integer(), nullable=False, default=0),
        sa.Column('reserved_servers', sa.Integer(), nullable=False, default=0),
        sa.Column('min_servers', sa.Integer(), nullable=False, default=0),
        sa.Column('max_servers', sa.Integer(), nullable=False, default=100),
        sa.Column('validators_per_server', sa.Integer(), nullable=False, default=1),
        sa.Column('hourly_price_usd', sa.Float(), nullable=False),
        sa.Column('monthly_price_usd', sa.Float(), nullable=False),
        sa.Column('spot_price_usd', sa.Float(), nullable=True),
        sa.Column('is_spot_available', sa.Boolean(), nullable=False, default=False),
        sa.Column('is_available', sa.Boolean(), nullable=False, default=True),
        sa.Column('features', sa.JSON(), nullable=False, default={}),
        sa.Column('created_at', sa.DateTime(), nullable=False, server_default=sa.func.now()),
        sa.Column('updated_at', sa.DateTime(), nullable=False, server_default=sa.func.now(), onupdate=sa.func.now()),
    )
    op.create_index('ix_server_pools_region_tier', 'server_pools', ['region_id', 'tier_code'])

    op.create_table(
        'region_servers',
        sa.Column('id', sa.String(36), primary_key=True),
        sa.Column('region_id', sa.String(36), sa.ForeignKey('regions.id', ondelete='CASCADE'), nullable=False),
        sa.Column('pool_id', sa.String(36), sa.ForeignKey('server_pools.id', ondelete='SET NULL'), nullable=True),
        sa.Column('hostname', sa.String(255), nullable=False, unique=True),
        sa.Column('ip_address', sa.String(45), nullable=False, index=True),
        sa.Column('internal_ip', sa.String(45), nullable=True),
        sa.Column('cloud_instance_id', sa.String(255), nullable=True, index=True),
        sa.Column('cloud_instance_type', sa.String(100), nullable=True),
        sa.Column('zone', sa.String(50), nullable=True),
        sa.Column('status', sa.String(20), nullable=False, default='available', index=True),
        sa.Column('machine_type', sa.String(50), nullable=False),
        sa.Column('cpu_cores', sa.Integer(), nullable=False),
        sa.Column('memory_gb', sa.Integer(), nullable=False),
        sa.Column('storage_gb', sa.Integer(), nullable=False),
        sa.Column('cpu_used', sa.Float(), nullable=False, default=0),
        sa.Column('memory_used_gb', sa.Float(), nullable=False, default=0),
        sa.Column('storage_used_gb', sa.Float(), nullable=False, default=0),
        sa.Column('validator_count', sa.Integer(), nullable=False, default=0),
        sa.Column('max_validators', sa.Integer(), nullable=False, default=1),
        sa.Column('health_score', sa.Float(), nullable=False, default=100.0),
        sa.Column('last_health_check', sa.DateTime(), nullable=True),
        sa.Column('provisioned_at', sa.DateTime(), nullable=True),
        sa.Column('tags', sa.JSON(), nullable=False, default={}),
        sa.Column('extra_data', sa.JSON(), nullable=False, default={}),
        sa.Column('created_at', sa.DateTime(), nullable=False, server_default=sa.func.now()),
        sa.Column('updated_at', sa.DateTime(), nullable=False, server_default=sa.func.now(), onupdate=sa.func.now()),
    )
    op.create_index('ix_region_servers_region_status', 'region_servers', ['region_id', 'status'])

    # =========================================================================
    # TABLES - Validator Management
    # =========================================================================

    op.create_table(
        'validator_setup_requests',
        sa.Column('id', sa.String(36), primary_key=True),
        sa.Column('wallet_address', sa.String(100), nullable=False, index=True),
        sa.Column('run_mode', sa.String(20), nullable=False, default='cloud'),
        sa.Column('status', sa.String(50), nullable=False, default='pending', index=True),
        sa.Column('status_message', sa.Text(), nullable=True),
        sa.Column('error_message', sa.Text(), nullable=True),
        sa.Column('provider_id', sa.String(36), nullable=True),
        sa.Column('region_id', sa.String(36), sa.ForeignKey('regions.id', ondelete='SET NULL'), nullable=True),
        sa.Column('region_code', sa.String(50), nullable=True, index=True),
        sa.Column('machine_type', sa.String(50), nullable=True),
        sa.Column('moniker', sa.String(100), nullable=True),
        sa.Column('validator_name', sa.String(100), nullable=True),
        sa.Column('commission_rate', sa.String(20), nullable=True, default='0.10'),
        sa.Column('commission_max_rate', sa.String(20), nullable=True, default='0.20'),
        sa.Column('commission_max_change_rate', sa.String(20), nullable=True, default='0.01'),
        sa.Column('min_self_delegation', sa.String(50), nullable=True, default='1'),
        sa.Column('initial_stake_amount', sa.String(50), nullable=True),
        sa.Column('stake_denom', sa.String(20), nullable=True, default='uomni'),
        sa.Column('consensus_pubkey', sa.String(255), nullable=True, unique=True),
        sa.Column('node_key', sa.Text(), nullable=True),
        sa.Column('priv_validator_key', sa.Text(), nullable=True),
        sa.Column('progress_percent', sa.Float(), nullable=False, default=0.0),
        sa.Column('current_step', sa.String(100), nullable=True),
        sa.Column('retry_count', sa.Integer(), nullable=False, default=0),
        sa.Column('max_retries', sa.Integer(), nullable=False, default=3),
        sa.Column('last_retry_at', sa.DateTime(), nullable=True),
        sa.Column('priority', sa.Integer(), nullable=False, default=0),
        sa.Column('extra_data', sa.JSON(), nullable=False, default={}),
        sa.Column('created_at', sa.DateTime(), nullable=False, server_default=sa.func.now()),
        sa.Column('updated_at', sa.DateTime(), nullable=False, server_default=sa.func.now(), onupdate=sa.func.now()),
    )
    op.create_index('ix_validator_setup_requests_wallet_status', 'validator_setup_requests', ['wallet_address', 'status'])

    op.create_table(
        'validator_nodes',
        sa.Column('id', sa.String(36), primary_key=True),
        sa.Column('setup_request_id', sa.String(36), sa.ForeignKey('validator_setup_requests.id', ondelete='CASCADE'), nullable=False),
        sa.Column('region_id', sa.String(36), sa.ForeignKey('regions.id', ondelete='SET NULL'), nullable=True),
        sa.Column('server_id', sa.String(36), sa.ForeignKey('region_servers.id', ondelete='SET NULL'), nullable=True),
        sa.Column('container_id', sa.String(100), nullable=True, index=True),
        sa.Column('container_name', sa.String(255), nullable=True),
        sa.Column('ip_address', sa.String(45), nullable=True),
        sa.Column('p2p_port', sa.Integer(), nullable=True, default=26656),
        sa.Column('rpc_port', sa.Integer(), nullable=True, default=26657),
        sa.Column('grpc_port', sa.Integer(), nullable=True, default=9090),
        sa.Column('api_port', sa.Integer(), nullable=True, default=1317),
        sa.Column('metrics_port', sa.Integer(), nullable=True, default=26660),
        sa.Column('status', sa.String(50), nullable=False, default='pending', index=True),
        sa.Column('is_active', sa.Boolean(), nullable=False, default=True),
        sa.Column('block_height', sa.BigInteger(), nullable=True),
        sa.Column('peer_count', sa.Integer(), nullable=True),
        sa.Column('catching_up', sa.Boolean(), nullable=True),
        sa.Column('is_synced', sa.Boolean(), nullable=False, default=False),
        sa.Column('chain_id', sa.String(100), nullable=True),
        sa.Column('chain_version', sa.String(50), nullable=True),
        sa.Column('is_jailed', sa.Boolean(), nullable=False, default=False),
        sa.Column('jailed_until', sa.DateTime(), nullable=True),
        sa.Column('voting_power', sa.String(50), nullable=True),
        sa.Column('delegator_shares', sa.String(100), nullable=True),
        sa.Column('missed_blocks', sa.Integer(), nullable=False, default=0),
        sa.Column('uptime_percent', sa.Float(), nullable=True),
        sa.Column('cpu_allocated', sa.Float(), nullable=True),
        sa.Column('memory_allocated_gb', sa.Float(), nullable=True),
        sa.Column('storage_allocated_gb', sa.Float(), nullable=True),
        sa.Column('health_score', sa.Float(), nullable=False, default=100.0),
        sa.Column('last_heartbeat', sa.DateTime(), nullable=True),
        sa.Column('started_at', sa.DateTime(), nullable=True),
        sa.Column('stopped_at', sa.DateTime(), nullable=True),
        sa.Column('extra_data', sa.JSON(), nullable=False, default={}),
        sa.Column('created_at', sa.DateTime(), nullable=False, server_default=sa.func.now()),
        sa.Column('updated_at', sa.DateTime(), nullable=False, server_default=sa.func.now(), onupdate=sa.func.now()),
    )
    op.create_index('ix_validator_nodes_setup_request', 'validator_nodes', ['setup_request_id'])
    op.create_index('ix_validator_nodes_status_active', 'validator_nodes', ['status', 'is_active'])

    op.create_table(
        'local_validator_heartbeats',
        sa.Column('id', sa.String(36), primary_key=True),
        sa.Column('wallet_address', sa.String(100), nullable=False, index=True),
        sa.Column('consensus_pubkey', sa.String(255), nullable=False, unique=True, index=True),
        sa.Column('moniker', sa.String(100), nullable=True),
        sa.Column('chain_id', sa.String(100), nullable=True),
        sa.Column('node_id', sa.String(100), nullable=True),
        sa.Column('block_height', sa.BigInteger(), nullable=True),
        sa.Column('peer_count', sa.Integer(), nullable=True),
        sa.Column('catching_up', sa.Boolean(), nullable=True),
        sa.Column('is_synced', sa.Boolean(), nullable=False, default=False),
        sa.Column('is_active_validator', sa.Boolean(), nullable=False, default=False),
        sa.Column('is_jailed', sa.Boolean(), nullable=False, default=False),
        sa.Column('voting_power', sa.String(50), nullable=True),
        sa.Column('missed_blocks', sa.Integer(), nullable=False, default=0),
        sa.Column('cpu_percent', sa.Float(), nullable=True),
        sa.Column('memory_percent', sa.Float(), nullable=True),
        sa.Column('disk_percent', sa.Float(), nullable=True),
        sa.Column('uptime_seconds', sa.BigInteger(), nullable=True),
        sa.Column('client_version', sa.String(50), nullable=True),
        sa.Column('client_os', sa.String(100), nullable=True),
        sa.Column('client_ip', sa.String(45), nullable=True),
        sa.Column('first_seen', sa.DateTime(), nullable=False, server_default=sa.func.now()),
        sa.Column('last_seen', sa.DateTime(), nullable=False, server_default=sa.func.now()),
        sa.Column('extra_data', sa.JSON(), nullable=False, default={}),
    )
    op.create_index('ix_local_validator_heartbeats_last_seen', 'local_validator_heartbeats', ['last_seen'])

    # =========================================================================
    # TABLES - Provider Marketplace
    # =========================================================================

    op.create_table(
        'providers',
        sa.Column('id', sa.String(36), primary_key=True),
        sa.Column('code', sa.String(50), nullable=False, unique=True, index=True),
        sa.Column('name', sa.String(100), nullable=False),
        sa.Column('display_name', sa.String(100), nullable=False),
        sa.Column('description', sa.Text(), nullable=True),
        sa.Column('provider_type', sa.String(20), nullable=False, default='community'),
        sa.Column('logo_url', sa.String(500), nullable=True),
        sa.Column('website_url', sa.String(500), nullable=True),
        sa.Column('documentation_url', sa.String(500), nullable=True),
        sa.Column('support_email', sa.String(255), nullable=True),
        sa.Column('support_url', sa.String(500), nullable=True),
        sa.Column('api_endpoint', sa.String(500), nullable=True),
        sa.Column('api_version', sa.String(20), nullable=True),
        sa.Column('status', sa.String(20), nullable=False, default='active', index=True),
        sa.Column('supported_regions', sa.JSON(), nullable=False, default=[]),
        sa.Column('supported_chains', sa.JSON(), nullable=False, default=[]),
        sa.Column('features', sa.JSON(), nullable=False, default={}),
        sa.Column('rating', sa.Float(), nullable=True, default=0.0),
        sa.Column('review_count', sa.Integer(), nullable=False, default=0),
        sa.Column('total_validators', sa.Integer(), nullable=False, default=0),
        sa.Column('active_validators', sa.Integer(), nullable=False, default=0),
        sa.Column('uptime_percent', sa.Float(), nullable=True, default=99.9),
        sa.Column('avg_provision_time_seconds', sa.Float(), nullable=True),
        sa.Column('is_verified', sa.Boolean(), nullable=False, default=False),
        sa.Column('verified_at', sa.DateTime(), nullable=True),
        sa.Column('extra_data', sa.JSON(), nullable=False, default={}),
        sa.Column('created_at', sa.DateTime(), nullable=False, server_default=sa.func.now()),
        sa.Column('updated_at', sa.DateTime(), nullable=False, server_default=sa.func.now(), onupdate=sa.func.now()),
    )

    op.create_table(
        'provider_pricing_tiers',
        sa.Column('id', sa.String(36), primary_key=True),
        sa.Column('provider_id', sa.String(36), sa.ForeignKey('providers.id', ondelete='CASCADE'), nullable=False),
        sa.Column('tier_code', sa.String(50), nullable=False),
        sa.Column('name', sa.String(100), nullable=False),
        sa.Column('description', sa.Text(), nullable=True),
        sa.Column('cpu_cores', sa.Integer(), nullable=False),
        sa.Column('memory_gb', sa.Integer(), nullable=False),
        sa.Column('storage_gb', sa.Integer(), nullable=False),
        sa.Column('storage_type', sa.String(50), nullable=False, default='ssd'),
        sa.Column('bandwidth_gbps', sa.Float(), nullable=True, default=1.0),
        sa.Column('hourly_price_usd', sa.Float(), nullable=False),
        sa.Column('monthly_price_usd', sa.Float(), nullable=False),
        sa.Column('setup_fee_usd', sa.Float(), nullable=False, default=0.0),
        sa.Column('promo_price_usd', sa.Float(), nullable=True),
        sa.Column('promo_ends_at', sa.DateTime(), nullable=True),
        sa.Column('is_available', sa.Boolean(), nullable=False, default=True),
        sa.Column('is_recommended', sa.Boolean(), nullable=False, default=False),
        sa.Column('available_regions', sa.JSON(), nullable=False, default=[]),
        sa.Column('available_capacity', sa.Integer(), nullable=False, default=0),
        sa.Column('sort_order', sa.Integer(), nullable=False, default=0),
        sa.Column('features', sa.JSON(), nullable=False, default={}),
        sa.Column('created_at', sa.DateTime(), nullable=False, server_default=sa.func.now()),
        sa.Column('updated_at', sa.DateTime(), nullable=False, server_default=sa.func.now(), onupdate=sa.func.now()),
    )
    op.create_index('ix_provider_pricing_tiers_provider_tier', 'provider_pricing_tiers', ['provider_id', 'tier_code'])

    op.create_table(
        'provider_metrics',
        sa.Column('id', sa.String(36), primary_key=True),
        sa.Column('provider_id', sa.String(36), sa.ForeignKey('providers.id', ondelete='CASCADE'), nullable=False),
        sa.Column('region_code', sa.String(50), nullable=True, index=True),
        sa.Column('recorded_at', sa.DateTime(), nullable=False, index=True),
        sa.Column('period_type', sa.String(20), nullable=False, default='hour'),
        sa.Column('uptime_percent', sa.Float(), nullable=False, default=100.0),
        sa.Column('latency_avg_ms', sa.Float(), nullable=True),
        sa.Column('latency_p95_ms', sa.Float(), nullable=True),
        sa.Column('latency_p99_ms', sa.Float(), nullable=True),
        sa.Column('provision_success_rate', sa.Float(), nullable=False, default=100.0),
        sa.Column('provision_avg_time_seconds', sa.Float(), nullable=True),
        sa.Column('total_provisions', sa.Integer(), nullable=False, default=0),
        sa.Column('failed_provisions', sa.Integer(), nullable=False, default=0),
        sa.Column('active_validators', sa.Integer(), nullable=False, default=0),
        sa.Column('total_requests', sa.Integer(), nullable=False, default=0),
        sa.Column('failed_requests', sa.Integer(), nullable=False, default=0),
        sa.Column('error_rate', sa.Float(), nullable=False, default=0.0),
        sa.Column('health_score', sa.Float(), nullable=False, default=100.0),
        sa.Column('extra_metrics', sa.JSON(), nullable=False, default={}),
    )
    op.create_index('ix_provider_metrics_provider_time', 'provider_metrics', ['provider_id', 'recorded_at'])

    op.create_table(
        'provider_applications',
        sa.Column('id', sa.String(36), primary_key=True),
        sa.Column('company_name', sa.String(200), nullable=False),
        sa.Column('company_website', sa.String(500), nullable=True),
        sa.Column('company_description', sa.Text(), nullable=True),
        sa.Column('contact_name', sa.String(100), nullable=False),
        sa.Column('contact_email', sa.String(255), nullable=False, index=True),
        sa.Column('contact_phone', sa.String(50), nullable=True),
        sa.Column('proposed_code', sa.String(50), nullable=False),
        sa.Column('proposed_regions', sa.JSON(), nullable=False, default=[]),
        sa.Column('proposed_pricing', sa.JSON(), nullable=False, default=[]),
        sa.Column('api_endpoint', sa.String(500), nullable=True),
        sa.Column('api_documentation_url', sa.String(500), nullable=True),
        sa.Column('infrastructure_details', sa.Text(), nullable=True),
        sa.Column('uptime_guarantee', sa.Float(), nullable=False, default=99.9),
        sa.Column('support_level', sa.String(50), nullable=True),
        sa.Column('status', sa.String(20), nullable=False, default='pending', index=True),
        sa.Column('status_reason', sa.Text(), nullable=True),
        sa.Column('submitted_at', sa.DateTime(), nullable=False, server_default=sa.func.now()),
        sa.Column('reviewed_by', sa.String(100), nullable=True),
        sa.Column('reviewed_at', sa.DateTime(), nullable=True),
        sa.Column('approved_at', sa.DateTime(), nullable=True),
        sa.Column('provider_id', sa.String(36), sa.ForeignKey('providers.id', ondelete='SET NULL'), nullable=True),
        sa.Column('verification_results', sa.JSON(), nullable=False, default={}),
        sa.Column('notes', sa.Text(), nullable=True),
        sa.Column('created_at', sa.DateTime(), nullable=False, server_default=sa.func.now()),
        sa.Column('updated_at', sa.DateTime(), nullable=False, server_default=sa.func.now(), onupdate=sa.func.now()),
    )

    op.create_table(
        'provider_verifications',
        sa.Column('id', sa.String(36), primary_key=True),
        sa.Column('application_id', sa.String(36), sa.ForeignKey('provider_applications.id', ondelete='CASCADE'), nullable=False, index=True),
        sa.Column('check_type', sa.String(50), nullable=False),
        sa.Column('check_name', sa.String(100), nullable=False),
        sa.Column('passed', sa.Boolean(), nullable=False),
        sa.Column('result_message', sa.Text(), nullable=True),
        sa.Column('result_data', sa.JSON(), nullable=True),
        sa.Column('duration_ms', sa.Float(), nullable=True),
        sa.Column('executed_at', sa.DateTime(), nullable=False, server_default=sa.func.now()),
    )

    op.create_table(
        'provider_slas',
        sa.Column('id', sa.String(36), primary_key=True),
        sa.Column('provider_id', sa.String(36), sa.ForeignKey('providers.id', ondelete='CASCADE'), nullable=False),
        sa.Column('name', sa.String(100), nullable=False),
        sa.Column('uptime_guarantee', sa.Float(), nullable=False, default=99.9),
        sa.Column('response_time_hours', sa.Integer(), nullable=False, default=4),
        sa.Column('resolution_time_hours', sa.Integer(), nullable=False, default=24),
        sa.Column('credit_tiers', sa.JSON(), nullable=False, default=[]),
        sa.Column('penalty_rate', sa.Float(), nullable=False, default=0.0),
        sa.Column('max_monthly_credit_percent', sa.Float(), nullable=False, default=100.0),
        sa.Column('exclusions', sa.JSON(), nullable=False, default=[]),
        sa.Column('is_active', sa.Boolean(), nullable=False, default=True),
        sa.Column('effective_from', sa.DateTime(), nullable=False, server_default=sa.func.now()),
        sa.Column('effective_until', sa.DateTime(), nullable=True),
        sa.Column('created_at', sa.DateTime(), nullable=False, server_default=sa.func.now()),
        sa.Column('updated_at', sa.DateTime(), nullable=False, server_default=sa.func.now(), onupdate=sa.func.now()),
    )
    op.create_index('ix_provider_slas_provider_active', 'provider_slas', ['provider_id', 'is_active'])

    op.create_table(
        'provider_reviews',
        sa.Column('id', sa.String(36), primary_key=True),
        sa.Column('provider_id', sa.String(36), sa.ForeignKey('providers.id', ondelete='CASCADE'), nullable=False, index=True),
        sa.Column('user_id', sa.String(100), nullable=False, index=True),
        sa.Column('wallet_address', sa.String(100), nullable=True),
        sa.Column('overall_rating', sa.Float(), nullable=False),
        sa.Column('reliability_rating', sa.Float(), nullable=True),
        sa.Column('performance_rating', sa.Float(), nullable=True),
        sa.Column('support_rating', sa.Float(), nullable=True),
        sa.Column('value_rating', sa.Float(), nullable=True),
        sa.Column('title', sa.String(200), nullable=True),
        sa.Column('content', sa.Text(), nullable=True),
        sa.Column('pros', sa.Text(), nullable=True),
        sa.Column('cons', sa.Text(), nullable=True),
        sa.Column('validators_count', sa.Integer(), nullable=True),
        sa.Column('months_used', sa.Integer(), nullable=True),
        sa.Column('is_verified', sa.Boolean(), nullable=False, default=False),
        sa.Column('is_visible', sa.Boolean(), nullable=False, default=True),
        sa.Column('helpful_votes', sa.Integer(), nullable=False, default=0),
        sa.Column('unhelpful_votes', sa.Integer(), nullable=False, default=0),
        sa.Column('response', sa.Text(), nullable=True),
        sa.Column('response_at', sa.DateTime(), nullable=True),
        sa.Column('created_at', sa.DateTime(), nullable=False, server_default=sa.func.now()),
        sa.Column('updated_at', sa.DateTime(), nullable=False, server_default=sa.func.now(), onupdate=sa.func.now()),
    )

    # =========================================================================
    # TABLES - Billing System
    # =========================================================================

    op.create_table(
        'billing_accounts',
        sa.Column('id', sa.String(36), primary_key=True),
        sa.Column('wallet_address', sa.String(100), nullable=False, unique=True, index=True),
        sa.Column('billing_email', sa.String(255), nullable=True, index=True),
        sa.Column('company_name', sa.String(200), nullable=True),
        sa.Column('billing_name', sa.String(200), nullable=True),
        sa.Column('billing_address', sa.JSON(), nullable=True),
        sa.Column('tax_id', sa.String(100), nullable=True),
        sa.Column('stripe_customer_id', sa.String(255), nullable=True, unique=True, index=True),
        sa.Column('default_payment_method', sa.String(20), nullable=True),
        sa.Column('crypto_payment_address', sa.String(100), nullable=True),
        sa.Column('balance', sa.Float(), nullable=False, default=0.0),
        sa.Column('credits_balance', sa.Float(), nullable=False, default=0.0),
        sa.Column('currency', sa.String(3), nullable=False, default='USD'),
        sa.Column('is_suspended', sa.Boolean(), nullable=False, default=False),
        sa.Column('suspended_at', sa.DateTime(), nullable=True),
        sa.Column('suspended_reason', sa.Text(), nullable=True),
        sa.Column('auto_pay_enabled', sa.Boolean(), nullable=False, default=True),
        sa.Column('invoice_settings', sa.JSON(), nullable=False, default={}),
        sa.Column('extra_data', sa.JSON(), nullable=False, default={}),
        sa.Column('created_at', sa.DateTime(), nullable=False, server_default=sa.func.now()),
        sa.Column('updated_at', sa.DateTime(), nullable=False, server_default=sa.func.now(), onupdate=sa.func.now()),
    )

    op.create_table(
        'billing_plans',
        sa.Column('id', sa.String(36), primary_key=True),
        sa.Column('code', sa.String(50), nullable=False, unique=True, index=True),
        sa.Column('name', sa.String(100), nullable=False),
        sa.Column('display_name', sa.String(100), nullable=False),
        sa.Column('description', sa.Text(), nullable=True),
        sa.Column('plan_type', sa.String(20), nullable=False, default='starter'),
        sa.Column('monthly_price_usd', sa.Float(), nullable=False, default=0.0),
        sa.Column('annual_price_usd', sa.Float(), nullable=True),
        sa.Column('annual_discount_percent', sa.Float(), nullable=True, default=0.0),
        sa.Column('setup_fee_usd', sa.Float(), nullable=False, default=0.0),
        sa.Column('validators_included', sa.Integer(), nullable=False, default=0),
        sa.Column('max_validators', sa.Integer(), nullable=True),
        sa.Column('storage_gb_included', sa.Integer(), nullable=False, default=0),
        sa.Column('bandwidth_gb_included', sa.Integer(), nullable=False, default=0),
        sa.Column('support_level', sa.String(50), nullable=False, default='community'),
        sa.Column('features', sa.JSON(), nullable=False, default={}),
        sa.Column('limits', sa.JSON(), nullable=False, default={}),
        sa.Column('overage_rates', sa.JSON(), nullable=False, default={}),
        sa.Column('trial_days', sa.Integer(), nullable=False, default=0),
        sa.Column('is_active', sa.Boolean(), nullable=False, default=True),
        sa.Column('is_public', sa.Boolean(), nullable=False, default=True),
        sa.Column('sort_order', sa.Integer(), nullable=False, default=0),
        sa.Column('stripe_price_id', sa.String(255), nullable=True),
        sa.Column('stripe_price_id_annual', sa.String(255), nullable=True),
        sa.Column('extra_data', sa.JSON(), nullable=False, default={}),
        sa.Column('created_at', sa.DateTime(), nullable=False, server_default=sa.func.now()),
        sa.Column('updated_at', sa.DateTime(), nullable=False, server_default=sa.func.now(), onupdate=sa.func.now()),
    )

    op.create_table(
        'billing_subscriptions',
        sa.Column('id', sa.String(36), primary_key=True),
        sa.Column('billing_account_id', sa.String(36), sa.ForeignKey('billing_accounts.id', ondelete='CASCADE'), nullable=False, index=True),
        sa.Column('plan_id', sa.String(36), sa.ForeignKey('billing_plans.id', ondelete='RESTRICT'), nullable=False),
        sa.Column('status', sa.String(20), nullable=False, default='active', index=True),
        sa.Column('billing_cycle', sa.String(20), nullable=False, default='monthly'),
        sa.Column('current_period_start', sa.DateTime(), nullable=False),
        sa.Column('current_period_end', sa.DateTime(), nullable=False),
        sa.Column('trial_start', sa.DateTime(), nullable=True),
        sa.Column('trial_end', sa.DateTime(), nullable=True),
        sa.Column('cancel_at_period_end', sa.Boolean(), nullable=False, default=False),
        sa.Column('cancelled_at', sa.DateTime(), nullable=True),
        sa.Column('cancellation_reason', sa.Text(), nullable=True),
        sa.Column('stripe_subscription_id', sa.String(255), nullable=True, unique=True, index=True),
        sa.Column('quantity', sa.Integer(), nullable=False, default=1),
        sa.Column('discount_percent', sa.Float(), nullable=True),
        sa.Column('discount_ends_at', sa.DateTime(), nullable=True),
        sa.Column('promo_code', sa.String(50), nullable=True),
        sa.Column('activated_at', sa.DateTime(), nullable=True),
        sa.Column('extra_data', sa.JSON(), nullable=False, default={}),
        sa.Column('created_at', sa.DateTime(), nullable=False, server_default=sa.func.now()),
        sa.Column('updated_at', sa.DateTime(), nullable=False, server_default=sa.func.now(), onupdate=sa.func.now()),
    )

    op.create_table(
        'billing_invoices',
        sa.Column('id', sa.String(36), primary_key=True),
        sa.Column('billing_account_id', sa.String(36), sa.ForeignKey('billing_accounts.id', ondelete='CASCADE'), nullable=False, index=True),
        sa.Column('subscription_id', sa.String(36), sa.ForeignKey('billing_subscriptions.id', ondelete='SET NULL'), nullable=True),
        sa.Column('invoice_number', sa.String(50), nullable=False, unique=True, index=True),
        sa.Column('status', sa.String(20), nullable=False, default='draft', index=True),
        sa.Column('subtotal', sa.Float(), nullable=False, default=0.0),
        sa.Column('tax_amount', sa.Float(), nullable=False, default=0.0),
        sa.Column('tax_rate', sa.Float(), nullable=True),
        sa.Column('discount_amount', sa.Float(), nullable=False, default=0.0),
        sa.Column('credits_applied', sa.Float(), nullable=False, default=0.0),
        sa.Column('total_amount', sa.Float(), nullable=False, default=0.0),
        sa.Column('amount_paid', sa.Float(), nullable=False, default=0.0),
        sa.Column('amount_due', sa.Float(), nullable=False, default=0.0),
        sa.Column('currency', sa.String(3), nullable=False, default='USD'),
        sa.Column('due_date', sa.DateTime(), nullable=True),
        sa.Column('paid_at', sa.DateTime(), nullable=True),
        sa.Column('voided_at', sa.DateTime(), nullable=True),
        sa.Column('period_start', sa.DateTime(), nullable=True),
        sa.Column('period_end', sa.DateTime(), nullable=True),
        sa.Column('line_items', sa.JSON(), nullable=False, default=[]),
        sa.Column('stripe_invoice_id', sa.String(255), nullable=True, unique=True, index=True),
        sa.Column('stripe_payment_intent_id', sa.String(255), nullable=True),
        sa.Column('pdf_url', sa.String(500), nullable=True),
        sa.Column('notes', sa.Text(), nullable=True),
        sa.Column('extra_data', sa.JSON(), nullable=False, default={}),
        sa.Column('created_at', sa.DateTime(), nullable=False, server_default=sa.func.now()),
        sa.Column('updated_at', sa.DateTime(), nullable=False, server_default=sa.func.now(), onupdate=sa.func.now()),
    )

    op.create_table(
        'billing_payments',
        sa.Column('id', sa.String(36), primary_key=True),
        sa.Column('billing_account_id', sa.String(36), sa.ForeignKey('billing_accounts.id', ondelete='CASCADE'), nullable=False, index=True),
        sa.Column('invoice_id', sa.String(36), sa.ForeignKey('billing_invoices.id', ondelete='SET NULL'), nullable=True),
        sa.Column('amount', sa.Float(), nullable=False),
        sa.Column('currency', sa.String(3), nullable=False, default='USD'),
        sa.Column('payment_method', sa.String(20), nullable=False),
        sa.Column('status', sa.String(20), nullable=False, default='pending', index=True),
        sa.Column('stripe_payment_intent_id', sa.String(255), nullable=True, index=True),
        sa.Column('stripe_charge_id', sa.String(255), nullable=True),
        sa.Column('crypto_tx_hash', sa.String(255), nullable=True, index=True),
        sa.Column('crypto_network', sa.String(50), nullable=True),
        sa.Column('card_last4', sa.String(4), nullable=True),
        sa.Column('card_brand', sa.String(20), nullable=True),
        sa.Column('failure_code', sa.String(100), nullable=True),
        sa.Column('failure_message', sa.Text(), nullable=True),
        sa.Column('refunded_amount', sa.Float(), nullable=False, default=0.0),
        sa.Column('refund_reason', sa.Text(), nullable=True),
        sa.Column('refunded_at', sa.DateTime(), nullable=True),
        sa.Column('paid_at', sa.DateTime(), nullable=True),
        sa.Column('failed_at', sa.DateTime(), nullable=True),
        sa.Column('extra_data', sa.JSON(), nullable=False, default={}),
        sa.Column('created_at', sa.DateTime(), nullable=False, server_default=sa.func.now()),
        sa.Column('updated_at', sa.DateTime(), nullable=False, server_default=sa.func.now(), onupdate=sa.func.now()),
    )

    op.create_table(
        'billing_usage',
        sa.Column('id', sa.String(36), primary_key=True),
        sa.Column('billing_account_id', sa.String(36), sa.ForeignKey('billing_accounts.id', ondelete='CASCADE'), nullable=False, index=True),
        sa.Column('subscription_id', sa.String(36), sa.ForeignKey('billing_subscriptions.id', ondelete='SET NULL'), nullable=True),
        sa.Column('metric_name', sa.String(100), nullable=False, index=True),
        sa.Column('quantity', sa.Float(), nullable=False, default=0.0),
        sa.Column('unit', sa.String(50), nullable=False),
        sa.Column('unit_price', sa.Float(), nullable=True),
        sa.Column('included_quantity', sa.Float(), nullable=False, default=0.0),
        sa.Column('overage_quantity', sa.Float(), nullable=False, default=0.0),
        sa.Column('overage_rate', sa.Float(), nullable=True),
        sa.Column('period_start', sa.DateTime(), nullable=False),
        sa.Column('period_end', sa.DateTime(), nullable=False),
        sa.Column('extra_data', sa.JSON(), nullable=False, default={}),
        sa.Column('created_at', sa.DateTime(), nullable=False, server_default=sa.func.now()),
        sa.Column('updated_at', sa.DateTime(), nullable=False, server_default=sa.func.now(), onupdate=sa.func.now()),
    )
    op.create_index('ix_billing_usage_account_period', 'billing_usage', ['billing_account_id', 'period_start'])

    # =========================================================================
    # TABLES - Snapshots & Upgrades
    # =========================================================================

    op.create_table(
        'snapshots',
        sa.Column('id', sa.String(36), primary_key=True),
        sa.Column('chain_id', sa.String(100), nullable=False, index=True),
        sa.Column('chain_version', sa.String(50), nullable=True),
        sa.Column('block_height', sa.BigInteger(), nullable=False, index=True),
        sa.Column('block_time', sa.DateTime(), nullable=True),
        sa.Column('snapshot_type', sa.String(50), nullable=False, default='full'),
        sa.Column('file_name', sa.String(255), nullable=False),
        sa.Column('file_path', sa.String(500), nullable=False),
        sa.Column('file_size_bytes', sa.BigInteger(), nullable=False),
        sa.Column('compressed_size_bytes', sa.BigInteger(), nullable=True),
        sa.Column('compression', sa.String(20), nullable=True, default='lz4'),
        sa.Column('checksum_sha256', sa.String(64), nullable=True),
        sa.Column('checksum_md5', sa.String(32), nullable=True),
        sa.Column('download_url', sa.String(500), nullable=True),
        sa.Column('mirror_urls', sa.JSON(), nullable=False, default=[]),
        sa.Column('available_regions', sa.JSON(), nullable=False, default=[]),
        sa.Column('is_available', sa.Boolean(), nullable=False, default=True),
        sa.Column('is_verified', sa.Boolean(), nullable=False, default=False),
        sa.Column('verified_at', sa.DateTime(), nullable=True),
        sa.Column('verified_by', sa.String(100), nullable=True),
        sa.Column('is_pruned', sa.Boolean(), nullable=False, default=False),
        sa.Column('pruning_settings', sa.JSON(), nullable=True),
        sa.Column('download_count', sa.Integer(), nullable=False, default=0),
        sa.Column('last_downloaded_at', sa.DateTime(), nullable=True),
        sa.Column('extra_data', sa.JSON(), nullable=False, default={}),
        sa.Column('created_at', sa.DateTime(), nullable=False, server_default=sa.func.now()),
        sa.Column('updated_at', sa.DateTime(), nullable=False, server_default=sa.func.now(), onupdate=sa.func.now()),
    )
    op.create_index('ix_snapshots_chain_height', 'snapshots', ['chain_id', 'block_height'])

    op.create_table(
        'upgrades',
        sa.Column('id', sa.String(36), primary_key=True),
        sa.Column('chain_id', sa.String(100), nullable=False, index=True),
        sa.Column('name', sa.String(100), nullable=False),
        sa.Column('description', sa.Text(), nullable=True),
        sa.Column('upgrade_height', sa.BigInteger(), nullable=False, index=True),
        sa.Column('estimated_time', sa.DateTime(), nullable=True),
        sa.Column('from_version', sa.String(50), nullable=True),
        sa.Column('to_version', sa.String(50), nullable=False),
        sa.Column('binary_url', sa.String(500), nullable=True),
        sa.Column('binary_checksum', sa.String(64), nullable=True),
        sa.Column('upgrade_info', sa.JSON(), nullable=False, default={}),
        sa.Column('status', sa.String(20), nullable=False, default='pending', index=True),
        sa.Column('scheduled_time', sa.DateTime(), nullable=True),
        sa.Column('scheduled_by', sa.String(100), nullable=True),
        sa.Column('started_at', sa.DateTime(), nullable=True),
        sa.Column('completed_at', sa.DateTime(), nullable=True),
        sa.Column('failed_at', sa.DateTime(), nullable=True),
        sa.Column('rolled_back_at', sa.DateTime(), nullable=True),
        sa.Column('cancelled_at', sa.DateTime(), nullable=True),
        sa.Column('cancelled_by', sa.String(100), nullable=True),
        sa.Column('cancellation_reason', sa.Text(), nullable=True),
        sa.Column('error_message', sa.Text(), nullable=True),
        sa.Column('total_nodes', sa.Integer(), nullable=False, default=0),
        sa.Column('nodes_upgraded', sa.Integer(), nullable=False, default=0),
        sa.Column('nodes_failed', sa.Integer(), nullable=False, default=0),
        sa.Column('progress_percent', sa.Float(), nullable=False, default=0.0),
        sa.Column('rollout_strategy', sa.String(50), nullable=False, default='rolling'),
        sa.Column('rollout_config', sa.JSON(), nullable=False, default={}),
        sa.Column('pre_upgrade_snapshot_id', sa.String(36), sa.ForeignKey('snapshots.id', ondelete='SET NULL'), nullable=True),
        sa.Column('extra_data', sa.JSON(), nullable=False, default={}),
        sa.Column('created_at', sa.DateTime(), nullable=False, server_default=sa.func.now()),
        sa.Column('updated_at', sa.DateTime(), nullable=False, server_default=sa.func.now(), onupdate=sa.func.now()),
    )

    op.create_table(
        'upgrade_rollouts',
        sa.Column('id', sa.String(36), primary_key=True),
        sa.Column('upgrade_id', sa.String(36), sa.ForeignKey('upgrades.id', ondelete='CASCADE'), nullable=False, index=True),
        sa.Column('region_id', sa.String(36), sa.ForeignKey('regions.id', ondelete='SET NULL'), nullable=True),
        sa.Column('region_code', sa.String(50), nullable=False, index=True),
        sa.Column('batch_number', sa.Integer(), nullable=False, default=1),
        sa.Column('status', sa.String(20), nullable=False, default='pending', index=True),
        sa.Column('total_nodes', sa.Integer(), nullable=False, default=0),
        sa.Column('nodes_upgraded', sa.Integer(), nullable=False, default=0),
        sa.Column('nodes_failed', sa.Integer(), nullable=False, default=0),
        sa.Column('progress_percent', sa.Float(), nullable=False, default=0.0),
        sa.Column('started_at', sa.DateTime(), nullable=True),
        sa.Column('completed_at', sa.DateTime(), nullable=True),
        sa.Column('failed_at', sa.DateTime(), nullable=True),
        sa.Column('rolled_back_at', sa.DateTime(), nullable=True),
        sa.Column('error_message', sa.Text(), nullable=True),
        sa.Column('health_check_passed', sa.Boolean(), nullable=True),
        sa.Column('health_check_at', sa.DateTime(), nullable=True),
        sa.Column('health_check_details', sa.JSON(), nullable=True),
        sa.Column('node_results', sa.JSON(), nullable=False, default=[]),
        sa.Column('extra_data', sa.JSON(), nullable=False, default={}),
        sa.Column('created_at', sa.DateTime(), nullable=False, server_default=sa.func.now()),
        sa.Column('updated_at', sa.DateTime(), nullable=False, server_default=sa.func.now(), onupdate=sa.func.now()),
    )
    op.create_index('ix_upgrade_rollouts_upgrade_batch', 'upgrade_rollouts', ['upgrade_id', 'batch_number'])

    # =========================================================================
    # TABLES - Monitoring/SRE
    # =========================================================================

    op.create_table(
        'node_metrics',
        sa.Column('id', sa.String(36), primary_key=True),
        sa.Column('validator_node_id', sa.String(36), sa.ForeignKey('validator_nodes.id', ondelete='CASCADE'), nullable=False, index=True),
        sa.Column('recorded_at', sa.DateTime(), nullable=False, index=True),
        sa.Column('period_type', sa.String(20), nullable=False, default='minute'),
        sa.Column('cpu_percent', sa.Float(), nullable=True),
        sa.Column('cpu_cores_used', sa.Float(), nullable=True),
        sa.Column('load_average_1m', sa.Float(), nullable=True),
        sa.Column('load_average_5m', sa.Float(), nullable=True),
        sa.Column('load_average_15m', sa.Float(), nullable=True),
        sa.Column('memory_percent', sa.Float(), nullable=True),
        sa.Column('memory_used_gb', sa.Float(), nullable=True),
        sa.Column('memory_available_gb', sa.Float(), nullable=True),
        sa.Column('swap_percent', sa.Float(), nullable=True),
        sa.Column('disk_percent', sa.Float(), nullable=True),
        sa.Column('disk_used_gb', sa.Float(), nullable=True),
        sa.Column('disk_available_gb', sa.Float(), nullable=True),
        sa.Column('disk_read_mb_s', sa.Float(), nullable=True),
        sa.Column('disk_write_mb_s', sa.Float(), nullable=True),
        sa.Column('disk_iops', sa.Integer(), nullable=True),
        sa.Column('network_rx_mb_s', sa.Float(), nullable=True),
        sa.Column('network_tx_mb_s', sa.Float(), nullable=True),
        sa.Column('network_connections', sa.Integer(), nullable=True),
        sa.Column('block_height', sa.BigInteger(), nullable=True),
        sa.Column('blocks_behind', sa.Integer(), nullable=True),
        sa.Column('is_syncing', sa.Boolean(), nullable=True),
        sa.Column('sync_speed_blocks_per_sec', sa.Float(), nullable=True),
        sa.Column('peer_count', sa.Integer(), nullable=True),
        sa.Column('inbound_peers', sa.Integer(), nullable=True),
        sa.Column('outbound_peers', sa.Integer(), nullable=True),
        sa.Column('peer_latency_avg_ms', sa.Float(), nullable=True),
        sa.Column('voting_power', sa.String(50), nullable=True),
        sa.Column('missed_blocks', sa.Integer(), nullable=True),
        sa.Column('missed_blocks_window', sa.Integer(), nullable=True),
        sa.Column('uptime_percent', sa.Float(), nullable=True),
        sa.Column('is_jailed', sa.Boolean(), nullable=True),
        sa.Column('rpc_requests_per_sec', sa.Float(), nullable=True),
        sa.Column('rpc_latency_avg_ms', sa.Float(), nullable=True),
        sa.Column('rpc_error_rate', sa.Float(), nullable=True),
        sa.Column('process_cpu_percent', sa.Float(), nullable=True),
        sa.Column('process_memory_mb', sa.Float(), nullable=True),
        sa.Column('goroutines', sa.Integer(), nullable=True),
        sa.Column('open_files', sa.Integer(), nullable=True),
        sa.Column('health_score', sa.Float(), nullable=True),
        sa.Column('health_status', sa.String(20), nullable=True),
        sa.Column('extra_metrics', sa.JSON(), nullable=False, default={}),
    )
    op.create_index('ix_node_metrics_node_time', 'node_metrics', ['validator_node_id', 'recorded_at'])
    op.create_index('ix_node_metrics_period', 'node_metrics', ['period_type', 'recorded_at'])

    op.create_table(
        'incidents',
        sa.Column('id', sa.String(36), primary_key=True),
        sa.Column('validator_node_id', sa.String(36), sa.ForeignKey('validator_nodes.id', ondelete='SET NULL'), nullable=True, index=True),
        sa.Column('region_id', sa.String(36), sa.ForeignKey('regions.id', ondelete='SET NULL'), nullable=True, index=True),
        sa.Column('region_code', sa.String(50), nullable=True, index=True),
        sa.Column('incident_number', sa.String(50), nullable=False, unique=True, index=True),
        sa.Column('title', sa.String(255), nullable=False),
        sa.Column('severity', sa.String(20), nullable=False, default='medium', index=True),
        sa.Column('status', sa.String(50), nullable=False, default='open', index=True),
        sa.Column('alert_type', sa.String(50), nullable=True, index=True),
        sa.Column('category', sa.String(50), nullable=True),
        sa.Column('description', sa.Text(), nullable=True),
        sa.Column('impact', sa.Text(), nullable=True),
        sa.Column('affected_validators', sa.Integer(), nullable=False, default=1),
        sa.Column('affected_customers', sa.Integer(), nullable=False, default=0),
        sa.Column('detected_by', sa.String(100), nullable=True),
        sa.Column('detected_at', sa.DateTime(), nullable=False, index=True),
        sa.Column('alert_id', sa.String(255), nullable=True),
        sa.Column('acknowledged_by', sa.String(100), nullable=True),
        sa.Column('acknowledged_at', sa.DateTime(), nullable=True),
        sa.Column('assigned_to', sa.String(100), nullable=True),
        sa.Column('escalated', sa.Boolean(), nullable=False, default=False),
        sa.Column('escalated_at', sa.DateTime(), nullable=True),
        sa.Column('root_cause', sa.Text(), nullable=True),
        sa.Column('root_cause_category', sa.String(50), nullable=True),
        sa.Column('contributing_factors', sa.JSON(), nullable=False, default=[]),
        sa.Column('resolution', sa.Text(), nullable=True),
        sa.Column('resolution_type', sa.String(50), nullable=True),
        sa.Column('resolved_by', sa.String(100), nullable=True),
        sa.Column('resolved_at', sa.DateTime(), nullable=True),
        sa.Column('time_to_acknowledge_minutes', sa.Float(), nullable=True),
        sa.Column('time_to_resolve_minutes', sa.Float(), nullable=True),
        sa.Column('downtime_minutes', sa.Float(), nullable=True),
        sa.Column('post_mortem_completed', sa.Boolean(), nullable=False, default=False),
        sa.Column('post_mortem_url', sa.String(500), nullable=True),
        sa.Column('lessons_learned', sa.Text(), nullable=True),
        sa.Column('action_items', sa.JSON(), nullable=False, default=[]),
        sa.Column('public_message', sa.Text(), nullable=True),
        sa.Column('status_page_updated', sa.Boolean(), nullable=False, default=False),
        sa.Column('customers_notified', sa.Boolean(), nullable=False, default=False),
        sa.Column('related_incidents', sa.JSON(), nullable=False, default=[]),
        sa.Column('timeline', sa.JSON(), nullable=False, default=[]),
        sa.Column('attachments', sa.JSON(), nullable=False, default=[]),
        sa.Column('tags', sa.JSON(), nullable=False, default=[]),
        sa.Column('extra_data', sa.JSON(), nullable=False, default={}),
        sa.Column('created_at', sa.DateTime(), nullable=False, server_default=sa.func.now()),
        sa.Column('updated_at', sa.DateTime(), nullable=False, server_default=sa.func.now(), onupdate=sa.func.now()),
        sa.Column('closed_at', sa.DateTime(), nullable=True),
    )
    op.create_index('ix_incidents_severity_status', 'incidents', ['severity', 'status'])
    op.create_index('ix_incidents_open', 'incidents', ['status', 'severity', 'detected_at'])


def downgrade() -> None:
    """Drop enhanced database schema."""

    # Drop tables in reverse order of creation (respecting foreign keys)
    op.drop_table('incidents')
    op.drop_table('node_metrics')
    op.drop_table('upgrade_rollouts')
    op.drop_table('upgrades')
    op.drop_table('snapshots')
    op.drop_table('billing_usage')
    op.drop_table('billing_payments')
    op.drop_table('billing_invoices')
    op.drop_table('billing_subscriptions')
    op.drop_table('billing_plans')
    op.drop_table('billing_accounts')
    op.drop_table('provider_reviews')
    op.drop_table('provider_slas')
    op.drop_table('provider_verifications')
    op.drop_table('provider_applications')
    op.drop_table('provider_metrics')
    op.drop_table('provider_pricing_tiers')
    op.drop_table('providers')
    op.drop_table('local_validator_heartbeats')
    op.drop_table('validator_nodes')
    op.drop_table('validator_setup_requests')
    op.drop_table('region_servers')
    op.drop_table('server_pools')
    op.drop_table('regions')

    # Drop enums
    op.execute("DROP TYPE IF EXISTS incidentstatus")
    op.execute("DROP TYPE IF EXISTS incidentseverity")
    op.execute("DROP TYPE IF EXISTS rolloutstatus")
    op.execute("DROP TYPE IF EXISTS upgradestatus")
    op.execute("DROP TYPE IF EXISTS invoicestatus")
    op.execute("DROP TYPE IF EXISTS paymentstatus")
    op.execute("DROP TYPE IF EXISTS paymentmethod")
    op.execute("DROP TYPE IF EXISTS subscriptionstatus")
    op.execute("DROP TYPE IF EXISTS billingcycle")
    op.execute("DROP TYPE IF EXISTS billingplantype")
    op.execute("DROP TYPE IF EXISTS verificationchecktype")
    op.execute("DROP TYPE IF EXISTS applicationstatus")
    op.execute("DROP TYPE IF EXISTS providerstatus")
    op.execute("DROP TYPE IF EXISTS providertype")
    op.execute("DROP TYPE IF EXISTS nodestatus")
    op.execute("DROP TYPE IF EXISTS setupstatus")
    op.execute("DROP TYPE IF EXISTS runmode")
    op.execute("DROP TYPE IF EXISTS machinetype")
    op.execute("DROP TYPE IF EXISTS serverstatus")
    op.execute("DROP TYPE IF EXISTS regionstatus")
