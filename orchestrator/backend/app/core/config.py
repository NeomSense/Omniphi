"""Application configuration with mandatory security settings."""

import os
import sys
from typing import Optional, List
from pydantic_settings import BaseSettings
from pydantic import field_validator


class Settings(BaseSettings):
    """
    Application settings loaded from environment variables.

    SECURITY: SECRET_KEY and MASTER_API_KEY have no defaults and MUST be
    set via environment variables. The application will refuse to start
    without them configured properly.
    """

    # API Settings
    API_V1_STR: str = "/api/v1"
    PROJECT_NAME: str = "Omniphi Validator Orchestrator"
    VERSION: str = "1.0.0"
    DEBUG: bool = False

    # Database
    DATABASE_URL: Optional[str] = None  # If set, overrides PostgreSQL settings
    POSTGRES_USER: str = "omniphi"
    POSTGRES_PASSWORD: Optional[str] = None  # MUST be set in production
    POSTGRES_SERVER: str = "localhost"
    POSTGRES_PORT: str = "5432"
    POSTGRES_DB: str = "validator_orchestrator"

    @property
    def SQLALCHEMY_DATABASE_URI(self) -> str:
        # If DATABASE_URL is set (e.g., for SQLite), use it
        if self.DATABASE_URL:
            return self.DATABASE_URL
        # Otherwise use PostgreSQL
        if not self.POSTGRES_PASSWORD:
            raise ValueError("POSTGRES_PASSWORD must be set for PostgreSQL connection")
        return f"postgresql://{self.POSTGRES_USER}:{self.POSTGRES_PASSWORD}@{self.POSTGRES_SERVER}:{self.POSTGRES_PORT}/{self.POSTGRES_DB}"

    # Omniphi Chain
    OMNIPHI_CHAIN_ID: str = "omniphi-mainnet-1"
    OMNIPHI_RPC_URL: str = "http://localhost:26657"
    OMNIPHI_REST_URL: str = "http://localhost:1317"
    OMNIPHI_GRPC_URL: str = "localhost:9090"

    # Omniphi Binary - MUST include checksums in production
    OMNIPHI_BINARY_URL: str = "https://github.com/omniphi/releases/download/v1.0.0/posd"
    OMNIPHI_BINARY_SHA256: Optional[str] = None  # Required for production
    OMNIPHI_GENESIS_URL: str = "https://raw.githubusercontent.com/omniphi/networks/main/mainnet/genesis.json"
    OMNIPHI_GENESIS_SHA256: Optional[str] = None  # Required for production

    # Docker
    DOCKER_NETWORK: str = "omniphi-validator-network"
    DOCKER_IMAGE: str = "omniphi/validator-node:latest"
    KEYRING_BACKEND: str = "file"  # Default to secure 'file' backend, not 'test'

    # Cloud Providers
    AWS_REGION: Optional[str] = None
    AWS_ACCESS_KEY_ID: Optional[str] = None
    AWS_SECRET_ACCESS_KEY: Optional[str] = None

    # AWS Infrastructure Security - MUST be configured when deploying to AWS
    # These map to Terraform's admin_cidr_blocks and monitoring_cidr_blocks.
    # Empty list = no SSH/monitoring access (secure default, but causes operational lockout).
    # Set to your office/VPN IPs before deploying, e.g.: ["203.0.113.10/32", "198.51.100.0/24"]
    AWS_ADMIN_CIDR_BLOCKS: List[str] = []
    AWS_MONITORING_CIDR_BLOCKS: List[str] = []

    GCP_PROJECT_ID: Optional[str] = None
    GCP_CREDENTIALS_PATH: Optional[str] = None

    # Security - NO DEFAULTS for critical secrets
    SECRET_KEY: Optional[str] = None  # MUST be set via environment
    JWT_ALGORITHM: str = "HS256"
    ACCESS_TOKEN_EXPIRE_MINUTES: int = 30
    MASTER_API_KEY: Optional[str] = None  # MUST be set via environment

    # Rate Limiting
    RATE_LIMIT_ENABLED: bool = True
    RATE_LIMIT_PER_MINUTE: int = 60
    RATE_LIMIT_PER_HOUR: int = 1000

    # Redis - Required for multi-instance deployments (nonce storage, caching)
    REDIS_URL: str = "redis://localhost:6379/0"
    REDIS_PASSWORD: Optional[str] = None  # Set in production if Redis auth is enabled
    NONCE_EXPIRY_SECONDS: int = 300  # 5 minutes
    # SECURITY: Set to true in production to prevent startup without Redis.
    # Without Redis, nonce replay protection is memory-local and fails across instances.
    REQUIRE_REDIS: bool = False

    # Production mode - enables strict validation of all production requirements
    # When true: requires Redis, binary checksums, and other production hardening
    PRODUCTION_MODE: bool = False

    # CORS - restrictive by default
    BACKEND_CORS_ORIGINS: List[str] = [
        "https://validators.omniphi.xyz"
    ]

    @field_validator('SECRET_KEY')
    @classmethod
    def validate_secret_key(cls, v):
        """Validate SECRET_KEY is set and secure."""
        if v is None:
            raise ValueError(
                "SECURITY ERROR: SECRET_KEY environment variable is not set. "
                "Generate a secure key with: python -c \"import secrets; print(secrets.token_hex(32))\""
            )
        if v in ["your-secret-key-change-in-production", "changeme", "secret", ""]:
            raise ValueError(
                "SECURITY ERROR: SECRET_KEY is set to an insecure default value. "
                "Generate a secure key with: python -c \"import secrets; print(secrets.token_hex(32))\""
            )
        if len(v) < 32:
            raise ValueError(
                "SECURITY ERROR: SECRET_KEY must be at least 32 characters. "
                "Generate a secure key with: python -c \"import secrets; print(secrets.token_hex(32))\""
            )
        return v

    @field_validator('MASTER_API_KEY')
    @classmethod
    def validate_master_api_key(cls, v):
        """Validate MASTER_API_KEY is set and secure."""
        if v is None:
            raise ValueError(
                "SECURITY ERROR: MASTER_API_KEY environment variable is not set. "
                "Generate a secure key with: python -c \"import secrets; print(secrets.token_hex(32))\""
            )
        if v in ["your-master-api-key-change-in-production", "changeme", "master", ""]:
            raise ValueError(
                "SECURITY ERROR: MASTER_API_KEY is set to an insecure default value. "
                "Generate a secure key with: python -c \"import secrets; print(secrets.token_hex(32))\""
            )
        if len(v) < 32:
            raise ValueError(
                "SECURITY ERROR: MASTER_API_KEY must be at least 32 characters. "
                "Generate a secure key with: python -c \"import secrets; print(secrets.token_hex(32))\""
            )
        return v

    @field_validator('AWS_ADMIN_CIDR_BLOCKS')
    @classmethod
    def validate_admin_cidr(cls, v):
        """Reject wildcard CIDR blocks for admin/SSH access."""
        wildcard = {"0.0.0.0/0", "::/0"}
        for cidr in v:
            if cidr in wildcard:
                raise ValueError(
                    "SECURITY ERROR: AWS_ADMIN_CIDR_BLOCKS cannot contain 0.0.0.0/0 or ::/0. "
                    "Specify exact admin/VPN IPs, e.g. ['203.0.113.10/32']."
                )
        return v

    @field_validator('AWS_MONITORING_CIDR_BLOCKS')
    @classmethod
    def validate_monitoring_cidr(cls, v):
        """Reject wildcard CIDR blocks for monitoring access."""
        wildcard = {"0.0.0.0/0", "::/0"}
        for cidr in v:
            if cidr in wildcard:
                raise ValueError(
                    "SECURITY ERROR: AWS_MONITORING_CIDR_BLOCKS cannot contain 0.0.0.0/0 or ::/0. "
                    "Specify exact monitoring server IPs."
                )
        return v

    @field_validator('DEBUG')
    @classmethod
    def warn_debug_mode(cls, v):
        """Warn if DEBUG is enabled."""
        if v:
            import logging
            logging.warning(
                "SECURITY WARNING: DEBUG mode is enabled. "
                "This exposes sensitive information and must be disabled in production."
            )
        return v

    class Config:
        env_file = ".env"
        case_sensitive = True


def _check_production_requirements():
    """
    Check production security requirements at startup.
    Called when the module is imported.
    """
    # Check if we're in a genuine testing context
    # SECURITY: Requires pytest to be loaded - env var alone is not sufficient
    if "pytest" in sys.modules and os.environ.get("TESTING") == "true":
        return

    # Check for required environment variables
    required_vars = ["SECRET_KEY", "MASTER_API_KEY"]
    missing = [var for var in required_vars if not os.environ.get(var)]

    if missing:
        print("\n" + "=" * 70, file=sys.stderr)
        print("SECURITY ERROR: Required environment variables not set", file=sys.stderr)
        print("=" * 70, file=sys.stderr)
        print(f"\nMissing: {', '.join(missing)}", file=sys.stderr)
        print("\nTo generate secure keys, run:", file=sys.stderr)
        print('  python -c "import secrets; print(secrets.token_hex(32))"', file=sys.stderr)
        print("\nThen set in your .env file or environment:", file=sys.stderr)
        for var in missing:
            print(f"  export {var}=<generated_key>", file=sys.stderr)
        print("=" * 70 + "\n", file=sys.stderr)

        # In production, refuse to start
        if not os.environ.get("ALLOW_INSECURE_STARTUP"):
            sys.exit(1)

    # Production mode enforcement
    is_production = os.environ.get("PRODUCTION_MODE", "").lower() in ("true", "1", "yes")
    require_redis = os.environ.get("REQUIRE_REDIS", "").lower() in ("true", "1", "yes")

    if is_production or require_redis:
        prod_errors = []

        # Verify Redis connectivity is possible (package installed)
        if require_redis or is_production:
            try:
                import redis as _redis_check
                # Verify connection at startup
                _redis_url = os.environ.get("REDIS_URL", "redis://localhost:6379/0")
                _redis_pw = os.environ.get("REDIS_PASSWORD")
                _client = _redis_check.from_url(
                    _redis_url, password=_redis_pw,
                    socket_connect_timeout=5, socket_timeout=5
                )
                _client.ping()
                _client.close()
            except ImportError:
                prod_errors.append(
                    "REQUIRE_REDIS is set but the redis package is not installed. "
                    "Install it: pip install redis"
                )
            except Exception as e:
                prod_errors.append(
                    f"REQUIRE_REDIS is set but Redis connection failed: {e}. "
                    "Nonce replay protection requires Redis in multi-instance deployments."
                )

        # Verify binary checksums are set in production
        if is_production:
            if not os.environ.get("OMNIPHI_BINARY_SHA256"):
                prod_errors.append(
                    "PRODUCTION_MODE requires OMNIPHI_BINARY_SHA256 to be set. "
                    "Binary integrity cannot be verified without checksums."
                )
            if not os.environ.get("OMNIPHI_GENESIS_SHA256"):
                prod_errors.append(
                    "PRODUCTION_MODE requires OMNIPHI_GENESIS_SHA256 to be set. "
                    "Genesis integrity cannot be verified without checksums."
                )

        if prod_errors:
            print("\n" + "=" * 70, file=sys.stderr)
            print("PRODUCTION READINESS ERROR", file=sys.stderr)
            print("=" * 70, file=sys.stderr)
            for err in prod_errors:
                print(f"  - {err}", file=sys.stderr)
            print("=" * 70 + "\n", file=sys.stderr)
            sys.exit(1)

    # Warn about AWS operational lockout risk
    if os.environ.get("AWS_REGION"):
        warnings = []
        if not os.environ.get("AWS_ADMIN_CIDR_BLOCKS"):
            warnings.append(
                "AWS_ADMIN_CIDR_BLOCKS is empty — SSH access to instances will be disabled. "
                "Set to your admin/VPN IPs to enable SSH."
            )
        if not os.environ.get("AWS_MONITORING_CIDR_BLOCKS"):
            warnings.append(
                "AWS_MONITORING_CIDR_BLOCKS is empty — Prometheus/metrics endpoints will be unreachable. "
                "Set to your monitoring server IPs."
            )
        if warnings:
            print("\n" + "=" * 70, file=sys.stderr)
            print("AWS CONFIGURATION WARNING", file=sys.stderr)
            print("=" * 70, file=sys.stderr)
            for w in warnings:
                print(f"  - {w}", file=sys.stderr)
            print("=" * 70 + "\n", file=sys.stderr)


# Run production check on import
_check_production_requirements()

# Create settings instance (will validate on creation)
def _is_test_environment() -> bool:
    """
    Check if running in a genuine test environment.

    SECURITY: Multiple conditions must be met to use test defaults.
    Just setting TESTING=true is NOT sufficient - pytest must be loaded.
    This prevents production bypasses via env var manipulation.
    """
    return (
        "pytest" in sys.modules
        and os.environ.get("TESTING") == "true"
    )


try:
    settings = Settings()
except Exception as e:
    print(f"\nConfiguration Error: {e}\n", file=sys.stderr)
    if not _is_test_environment():
        # SECURITY: In non-test environments, refuse to start without proper config
        sys.exit(1)
    # For testing ONLY (requires pytest to be loaded), create with defaults
    os.environ.setdefault("SECRET_KEY", "test-secret-key-only-for-testing-minimum-32-chars")
    os.environ.setdefault("MASTER_API_KEY", "test-master-api-key-only-for-testing-min-32")
    settings = Settings()
