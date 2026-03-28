"""Tests for the OmniphiClient class."""

from __future__ import annotations

import pytest

from omniphi.client import OmniphiClient
from omniphi.constants import DEFAULT_REST_URL, DEFAULT_RPC_URL, MODULE_PARAM_PATHS
from omniphi.errors import (
    OmniphiBroadcastError,
    OmniphiConnectionError,
    OmniphiError,
    OmniphiTxError,
)


class TestClientInit:
    """Tests for OmniphiClient initialization."""

    def test_default_urls(self) -> None:
        client = OmniphiClient()
        assert client._rpc_url == DEFAULT_RPC_URL
        assert client._rest_url == DEFAULT_REST_URL
        client.close()

    def test_custom_urls(self) -> None:
        client = OmniphiClient(
            rpc_url="http://mynode:26657",
            rest_url="http://mynode:1317",
        )
        assert client._rpc_url == "http://mynode:26657"
        assert client._rest_url == "http://mynode:1317"
        client.close()

    def test_trailing_slash_stripped(self) -> None:
        client = OmniphiClient(
            rpc_url="http://mynode:26657/",
            rest_url="http://mynode:1317/",
        )
        assert not client._rpc_url.endswith("/")
        assert not client._rest_url.endswith("/")
        client.close()

    def test_context_manager(self) -> None:
        with OmniphiClient() as client:
            assert isinstance(client, OmniphiClient)
        # Should not raise after close

    def test_repr(self) -> None:
        client = OmniphiClient()
        r = repr(client)
        assert "OmniphiClient" in r
        assert "26657" in r
        assert "1317" in r
        client.close()


class TestErrorClasses:
    """Tests for the error hierarchy."""

    def test_omniphi_error_is_exception(self) -> None:
        assert issubclass(OmniphiError, Exception)

    def test_connection_error_inherits(self) -> None:
        assert issubclass(OmniphiConnectionError, OmniphiError)

    def test_tx_error_inherits(self) -> None:
        assert issubclass(OmniphiTxError, OmniphiError)

    def test_broadcast_error_inherits(self) -> None:
        assert issubclass(OmniphiBroadcastError, OmniphiError)

    def test_error_message(self) -> None:
        err = OmniphiError("something broke")
        assert str(err) == "something broke"
        assert err.message == "something broke"

    def test_error_details(self) -> None:
        err = OmniphiError("test", details={"key": "value"})
        assert err.details == {"key": "value"}

    def test_connection_error_url(self) -> None:
        err = OmniphiConnectionError("fail", url="http://localhost:1317")
        assert err.url == "http://localhost:1317"

    def test_tx_error_code(self) -> None:
        err = OmniphiTxError("bad tx", code=5)
        assert err.code == 5

    def test_broadcast_error_fields(self) -> None:
        err = OmniphiBroadcastError(
            "broadcast failed",
            tx_hash="ABCDEF123456",
            code=11,
            codespace="sdk",
            raw_log="insufficient funds",
        )
        assert err.tx_hash == "ABCDEF123456"
        assert err.code == 11
        assert err.codespace == "sdk"
        assert err.raw_log == "insufficient funds"

    def test_broadcast_error_repr(self) -> None:
        err = OmniphiBroadcastError(
            "fail",
            tx_hash="ABC",
            code=5,
            codespace="sdk",
            raw_log="oops",
        )
        r = repr(err)
        assert "code=5" in r
        assert "sdk" in r

    def test_error_defaults(self) -> None:
        err = OmniphiError("test")
        assert err.details == {}

        conn_err = OmniphiConnectionError()
        assert conn_err.url == ""

        tx_err = OmniphiTxError()
        assert tx_err.code == 0

        bc_err = OmniphiBroadcastError()
        assert bc_err.tx_hash == ""
        assert bc_err.code == 0
        assert bc_err.codespace == ""
        assert bc_err.raw_log == ""


class TestClientModuleParams:
    """Tests for module_params validation logic."""

    def test_unknown_module_raises(self) -> None:
        client = OmniphiClient()
        with pytest.raises(OmniphiError, match="Unknown module"):
            client.query_module_params("nonexistent_module")
        client.close()

    def test_known_module_names(self) -> None:
        """All modules in MODULE_PARAM_PATHS should be recognized."""
        client = OmniphiClient()
        for module_name in MODULE_PARAM_PATHS:
            # Should not raise ValueError -- the HTTP call will fail
            # (no server), but the module lookup itself succeeds
            try:
                client.query_module_params(module_name)
            except OmniphiConnectionError:
                pass  # Expected: no server running
            except OmniphiError as e:
                if "Unknown module" in str(e):
                    pytest.fail(f"Module {module_name!r} was not recognized")
        client.close()

    def test_send_tokens_requires_wallet(self) -> None:
        client = OmniphiClient()
        with pytest.raises(OmniphiTxError, match="Wallet required"):
            client.send_tokens("omni1abc", "omni1def", 100)
        client.close()

    def test_delegate_requires_wallet(self) -> None:
        client = OmniphiClient()
        with pytest.raises(OmniphiTxError, match="Wallet required"):
            client.delegate("omni1abc", "omnivaloper1def", 100)
        client.close()

    def test_submit_contribution_requires_wallet(self) -> None:
        client = OmniphiClient()
        with pytest.raises(OmniphiTxError, match="Wallet required"):
            client.submit_contribution("omni1abc", "code", "ipfs://hash", b"\x00" * 32)
        client.close()
