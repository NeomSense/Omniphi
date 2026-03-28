// SPDX-License-Identifier: MIT
pragma solidity ^0.8.24;

import "forge-std/Vm.sol";

/// @title SigUtils — Signature generation helpers for OmniphiBridge tests
/// @notice Builds EIP-191 message hashes matching the contract's
///         _verifySignatures logic and produces sorted, concatenated sigs.
library SigUtils {
    /// @dev Replicates the exact hashing scheme from OmniphiBridge._verifySignatures.
    function withdrawalMessageHash(
        uint256 chainId,
        address bridge,
        address token,
        uint256 amount,
        address recipient,
        uint256 nonce
    ) internal pure returns (bytes32) {
        return keccak256(
            abi.encodePacked(
                "\x19Ethereum Signed Message:\n32",
                keccak256(
                    abi.encodePacked(
                        chainId,
                        bridge,
                        token,
                        amount,
                        recipient,
                        nonce
                    )
                )
            )
        );
    }

    /// @dev Sign a withdrawal message with a single private key and return
    ///      the 65-byte compact signature (r || s || v).
    function signWithdrawal(
        Vm vm,
        uint256 privateKey,
        uint256 chainId,
        address bridge,
        address token,
        uint256 amount,
        address recipient,
        uint256 nonce
    ) internal pure returns (bytes memory sig, address signer) {
        bytes32 digest = withdrawalMessageHash(chainId, bridge, token, amount, recipient, nonce);
        (uint8 v, bytes32 r, bytes32 s) = vm.sign(privateKey, digest);
        sig = abi.encodePacked(r, s, v);
        signer = vm.addr(privateKey);
    }

    /// @dev Build concatenated signatures from an array of private keys.
    ///      The keys are sorted by their derived address (ascending) before
    ///      signing, which is required by the contract.
    ///      Returns the concatenated bytes and the sorted signer addresses.
    function buildSortedSignatures(
        Vm vm,
        uint256[] memory privateKeys,
        uint256 chainId,
        address bridge,
        address token,
        uint256 amount,
        address recipient,
        uint256 nonce
    ) internal pure returns (bytes memory concatenated, address[] memory signers) {
        uint256 n = privateKeys.length;
        signers = new address[](n);

        // Derive addresses.
        for (uint256 i = 0; i < n; i++) {
            signers[i] = vm.addr(privateKeys[i]);
        }

        // Simple insertion sort by address (ascending).
        for (uint256 i = 1; i < n; i++) {
            address keyAddr = signers[i];
            uint256 keyPk   = privateKeys[i];
            uint256 j = i;
            while (j > 0 && signers[j - 1] > keyAddr) {
                signers[j]     = signers[j - 1];
                privateKeys[j] = privateKeys[j - 1];
                j--;
            }
            signers[j]     = keyAddr;
            privateKeys[j] = keyPk;
        }

        // Sign in sorted order and concatenate.
        bytes32 digest = withdrawalMessageHash(chainId, bridge, token, amount, recipient, nonce);

        concatenated = new bytes(0);
        for (uint256 i = 0; i < n; i++) {
            (uint8 v, bytes32 r, bytes32 s) = vm.sign(privateKeys[i], digest);
            concatenated = abi.encodePacked(concatenated, r, s, v);
        }
    }
}
