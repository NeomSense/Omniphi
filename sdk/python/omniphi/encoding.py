"""Encoding utilities for Cosmos SDK Amino JSON signing and wire formats."""

from __future__ import annotations

import base64
import json
from typing import Any


def sort_json(obj: Any) -> Any:
    """Recursively sort JSON object keys for canonical deterministic encoding.

    Cosmos SDK Amino JSON signing requires keys to be sorted alphabetically
    at every nesting level, with no extra whitespace.

    Args:
        obj: Any JSON-serializable Python object.

    Returns:
        A new object with all dict keys sorted recursively.
    """
    if isinstance(obj, dict):
        return {k: sort_json(v) for k, v in sorted(obj.items())}
    if isinstance(obj, (list, tuple)):
        return [sort_json(item) for item in obj]
    return obj


def canonical_json(obj: Any) -> bytes:
    """Encode an object as canonical (sorted-key, compact) JSON bytes.

    This is the format required by Cosmos SDK SignDoc for Amino JSON signing.

    Args:
        obj: Any JSON-serializable Python object.

    Returns:
        UTF-8 encoded compact JSON with sorted keys.
    """
    sorted_obj = sort_json(obj)
    return json.dumps(sorted_obj, separators=(",", ":"), ensure_ascii=False).encode("utf-8")


def encode_varint(value: int) -> bytes:
    """Encode a non-negative integer as a protobuf varint.

    Args:
        value: Non-negative integer.

    Returns:
        Varint-encoded bytes.

    Raises:
        ValueError: If value is negative.
    """
    if value < 0:
        raise ValueError(f"Cannot encode negative value as varint: {value}")

    result = bytearray()
    while value > 0x7F:
        result.append((value & 0x7F) | 0x80)
        value >>= 7
    result.append(value & 0x7F)
    return bytes(result)


def encode_length_prefixed(data: bytes) -> bytes:
    """Prefix data with its varint-encoded length.

    Args:
        data: Raw bytes to prefix.

    Returns:
        Length-prefixed bytes.
    """
    return encode_varint(len(data)) + data


def bytes_to_base64(data: bytes) -> str:
    """Encode bytes to a base64 string.

    Args:
        data: Raw bytes.

    Returns:
        Base64-encoded string.
    """
    return base64.b64encode(data).decode("ascii")


def base64_to_bytes(data: str) -> bytes:
    """Decode a base64 string to bytes.

    Args:
        data: Base64-encoded string.

    Returns:
        Decoded raw bytes.
    """
    return base64.b64decode(data)


def int_to_string(value: int) -> str:
    """Convert an integer to its string representation for Cosmos SDK messages.

    Cosmos SDK uses string encoding for large integers (sdk.Int, sdk.Dec)
    in JSON messages.

    Args:
        value: Integer value.

    Returns:
        String representation of the integer.
    """
    return str(value)
