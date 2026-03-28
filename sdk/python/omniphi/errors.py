"""Exception hierarchy for the Omniphi Python SDK."""

from __future__ import annotations


class OmniphiError(Exception):
    """Base exception for all Omniphi SDK errors."""

    def __init__(self, message: str, *, details: dict | None = None) -> None:
        super().__init__(message)
        self.message = message
        self.details = details or {}

    def __repr__(self) -> str:
        return f"{self.__class__.__name__}({self.message!r})"


class OmniphiConnectionError(OmniphiError):
    """Raised when the SDK cannot reach the node (RPC or REST)."""

    def __init__(
        self,
        message: str = "Failed to connect to Omniphi node",
        *,
        url: str = "",
        details: dict | None = None,
    ) -> None:
        super().__init__(message, details=details)
        self.url = url


class OmniphiTxError(OmniphiError):
    """Raised when transaction construction or signing fails."""

    def __init__(
        self,
        message: str = "Transaction error",
        *,
        code: int = 0,
        details: dict | None = None,
    ) -> None:
        super().__init__(message, details=details)
        self.code = code


class OmniphiBroadcastError(OmniphiError):
    """Raised when a transaction broadcast returns a non-zero code."""

    def __init__(
        self,
        message: str = "Broadcast failed",
        *,
        tx_hash: str = "",
        code: int = 0,
        codespace: str = "",
        raw_log: str = "",
        details: dict | None = None,
    ) -> None:
        super().__init__(message, details=details)
        self.tx_hash = tx_hash
        self.code = code
        self.codespace = codespace
        self.raw_log = raw_log

    def __repr__(self) -> str:
        return (
            f"{self.__class__.__name__}("
            f"code={self.code}, codespace={self.codespace!r}, "
            f"tx_hash={self.tx_hash!r}, raw_log={self.raw_log!r})"
        )
