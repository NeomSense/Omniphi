"""Application configuration."""

from typing import Optional
from pydantic_settings import BaseSettings


class Settings(BaseSettings):
    """Application settings loaded from environment variables."""

    # API Settings
    API_V1_STR: str = "/api/v1"
    PROJECT_NAME: str = "Omniphi Validator Orchestrator"
    VERSION: str = "1.0.0"
    DEBUG: bool = False

    # Database
    DATABASE_URL: Optional[str] = None  # If set, overrides PostgreSQL settings
    POSTGRES_USER: str = "omniphi"
    POSTGRES_PASSWORD: str = "omniphi_password"
    POSTGRES_SERVER: str = "localhost"
    POSTGRES_PORT: str = "5432"
    POSTGRES_DB: str = "validator_orchestrator"

    @property
    def SQLALCHEMY_DATABASE_URI(self) -> str:
        # If DATABASE_URL is set (e.g., for SQLite), use it
        if self.DATABASE_URL:
            return self.DATABASE_URL
        # Otherwise use PostgreSQL
        return f"postgresql://{self.POSTGRES_USER}:{self.POSTGRES_PASSWORD}@{self.POSTGRES_SERVER}:{self.POSTGRES_PORT}/{self.POSTGRES_DB}"

    # Omniphi Chain
    OMNIPHI_CHAIN_ID: str = "omniphi-mainnet-1"
    OMNIPHI_RPC_URL: str = "http://localhost:26657"
    OMNIPHI_REST_URL: str = "http://localhost:1317"
    OMNIPHI_GRPC_URL: str = "localhost:9090"

    # Omniphi Binary
    OMNIPHI_BINARY_URL: str = "https://github.com/omniphi/releases/download/v1.0.0/posd"
    OMNIPHI_GENESIS_URL: str = "https://raw.githubusercontent.com/omniphi/networks/main/mainnet/genesis.json"

    # Docker
    DOCKER_NETWORK: str = "omniphi-validator-network"
    DOCKER_IMAGE: str = "omniphi/validator-node:latest"

    # Cloud Providers
    AWS_REGION: Optional[str] = None
    AWS_ACCESS_KEY_ID: Optional[str] = None
    AWS_SECRET_ACCESS_KEY: Optional[str] = None

    GCP_PROJECT_ID: Optional[str] = None
    GCP_CREDENTIALS_PATH: Optional[str] = None

    # Security
    SECRET_KEY: str = "your-secret-key-change-in-production"
    JWT_ALGORITHM: str = "HS256"
    ACCESS_TOKEN_EXPIRE_MINUTES: int = 30
    MASTER_API_KEY: str = "your-master-api-key-change-in-production"

    # Rate Limiting
    RATE_LIMIT_ENABLED: bool = True
    RATE_LIMIT_PER_MINUTE: int = 60
    RATE_LIMIT_PER_HOUR: int = 1000

    # CORS
    BACKEND_CORS_ORIGINS: list = [
        "http://localhost:3000",
        "http://localhost:5173",
        "http://localhost:8080",  # Added for Vite dev server
        "https://validators.omniphi.xyz"
    ]

    class Config:
        env_file = ".env"
        case_sensitive = True


settings = Settings()
