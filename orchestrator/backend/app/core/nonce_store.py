"""
Redis-backed nonce storage for replay attack prevention.

SECURITY: This module provides distributed nonce storage that works across
multiple application instances. The in-memory fallback should ONLY be used
for development/testing.

In production, Redis is REQUIRED for:
1. Multi-instance deployments (load balancers, Kubernetes)
2. Persistence across restarts
3. Atomic operations to prevent race conditions
"""

import logging
import time
from typing import Optional

try:
    import redis
    REDIS_AVAILABLE = True
except ImportError:
    REDIS_AVAILABLE = False

from app.core.config import settings

logger = logging.getLogger(__name__)


class NonceStore:
    """
    Nonce storage for replay attack prevention.

    Uses Redis in production for distributed, persistent storage.
    Falls back to in-memory dict for development only.
    """

    def __init__(self):
        self._redis_client: Optional[redis.Redis] = None
        self._memory_cache: dict[str, float] = {}
        self._using_redis = False

        self._initialize_redis()

    def _initialize_redis(self):
        """Initialize Redis connection.

        SECURITY: If REQUIRE_REDIS or PRODUCTION_MODE is set, Redis failure
        raises RuntimeError to prevent startup with unsafe in-memory fallback.
        """
        redis_required = settings.REQUIRE_REDIS or settings.PRODUCTION_MODE

        if not REDIS_AVAILABLE:
            msg = (
                "SECURITY WARNING: redis package not installed. "
                "Using in-memory nonce storage which does NOT work with multiple instances. "
                "Install redis: pip install redis"
            )
            if redis_required:
                raise RuntimeError(
                    "FATAL: Redis is required (REQUIRE_REDIS=true or PRODUCTION_MODE=true) "
                    "but the redis package is not installed. "
                    "Install it: pip install redis"
                )
            logger.warning(msg)
            return

        try:
            # Parse Redis URL and add password if provided
            self._redis_client = redis.from_url(
                settings.REDIS_URL,
                password=settings.REDIS_PASSWORD,
                decode_responses=True,
                socket_connect_timeout=5,
                socket_timeout=5
            )

            # Test connection
            self._redis_client.ping()
            self._using_redis = True
            logger.info("Redis nonce store initialized successfully")

        except redis.ConnectionError as e:
            if redis_required:
                raise RuntimeError(
                    f"FATAL: Redis is required (REQUIRE_REDIS=true or PRODUCTION_MODE=true) "
                    f"but connection failed: {e}. "
                    "Nonce replay protection cannot operate without Redis in production."
                ) from e
            logger.warning(
                f"SECURITY WARNING: Failed to connect to Redis: {e}. "
                "Using in-memory nonce storage which does NOT work with multiple instances."
            )
            self._redis_client = None
        except Exception as e:
            if redis_required:
                raise RuntimeError(
                    f"FATAL: Redis is required but initialization failed: {e}"
                ) from e
            logger.error(f"Unexpected error initializing Redis: {e}")
            self._redis_client = None

    def verify_and_consume(self, nonce: str, wallet_address: str) -> bool:
        """
        Verify nonce hasn't been used and consume it atomically.

        SECURITY: This operation MUST be atomic to prevent race conditions
        where two requests with the same nonce could both succeed.

        Args:
            nonce: The one-time nonce to verify
            wallet_address: The wallet address associated with the nonce

        Returns:
            True if nonce is valid and was consumed, False if already used
        """
        cache_key = f"nonce:{wallet_address}:{nonce}"

        if self._using_redis and self._redis_client:
            return self._verify_redis(cache_key)
        else:
            return self._verify_memory(cache_key)

    def _verify_redis(self, cache_key: str) -> bool:
        """
        Atomically verify and consume nonce using Redis.

        Uses SET NX (set if not exists) for atomic check-and-set.
        This prevents race conditions in distributed environments.
        """
        try:
            # SETNX returns True if key was set (nonce is new)
            # Returns False if key already exists (replay attempt)
            result = self._redis_client.set(
                cache_key,
                int(time.time()),
                nx=True,  # Only set if not exists (atomic)
                ex=settings.NONCE_EXPIRY_SECONDS  # Auto-expire
            )

            if result:
                logger.debug(f"Nonce consumed: {cache_key[:50]}...")
                return True
            else:
                logger.warning(f"Nonce replay detected: {cache_key[:50]}...")
                return False

        except redis.RedisError as e:
            logger.error(f"Redis error during nonce verification: {e}")
            # SECURITY: Fail closed - reject on Redis errors
            # This prevents bypasses when Redis is temporarily unavailable
            return False

    def _verify_memory(self, cache_key: str) -> bool:
        """
        In-memory nonce verification (development only).

        WARNING: This does NOT work with multiple application instances
        and loses all nonces on restart. Use Redis in production.
        """
        current_time = time.time()

        # Clean expired nonces
        expired_keys = [
            k for k, v in self._memory_cache.items()
            if current_time - v > settings.NONCE_EXPIRY_SECONDS
        ]
        for k in expired_keys:
            del self._memory_cache[k]

        # Check if nonce was already used
        if cache_key in self._memory_cache:
            logger.warning(f"Nonce replay detected (memory): {cache_key[:50]}...")
            return False

        # Store nonce to prevent reuse
        self._memory_cache[cache_key] = current_time
        logger.debug(f"Nonce consumed (memory): {cache_key[:50]}...")
        return True

    def is_using_redis(self) -> bool:
        """Check if Redis is being used for storage."""
        return self._using_redis

    def health_check(self) -> dict:
        """Return health status of the nonce store."""
        status = {
            "backend": "redis" if self._using_redis else "memory",
            "healthy": True,
            "warning": None
        }

        if not self._using_redis:
            status["warning"] = (
                "Using in-memory storage. This does NOT work with multiple "
                "instances and loses data on restart. Configure Redis for production."
            )

        if self._using_redis and self._redis_client:
            try:
                self._redis_client.ping()
            except redis.RedisError as e:
                status["healthy"] = False
                status["warning"] = f"Redis connection failed: {e}"

        return status


# Global instance - initialized on import
nonce_store = NonceStore()
