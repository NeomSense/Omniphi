// SPDX-License-Identifier: MIT
pragma solidity ^0.8.24;

import "./interfaces/IERC20.sol";

// ============================================================================
//  OmniphiBridge — Lock-and-mint bridge between Ethereum and Omniphi
// ============================================================================
//
//  Deposit flow  (Ethereum -> Omniphi):
//    1. User calls deposit() with ETH or an ERC-20 token.
//    2. Contract locks the funds and emits a Deposit event.
//    3. Off-chain relayers observe the event and mint wrapped tokens on Omniphi.
//
//  Withdrawal flow (Omniphi -> Ethereum):
//    1. User burns wrapped tokens on Omniphi.
//    2. Relayers co-sign a withdrawal attestation.
//    3. Anyone submits withdraw() with M-of-N relayer signatures.
//    4. Contract releases the locked funds to the recipient.
//
//  Security model:
//    - M-of-N multisig: configurable threshold of registered relayers.
//    - Nonce tracking prevents replay of both deposits and withdrawals.
//    - ReentrancyGuard on all state-changing external functions.
//    - Pausable for emergency halts.
//    - Owner-only relayer management (addRelayer / removeRelayer / setThreshold).
// ============================================================================

/// @title OmniphiBridge
/// @author Omniphi Core Team
/// @notice Lock-and-mint bridge between Ethereum and the Omniphi blockchain.
contract OmniphiBridge {
    // ── Constants ───────────────────────────────────────────────────────

    /// @notice Sentinel address representing native ETH (not an ERC-20).
    address public constant ETH_ADDRESS = address(0);

    // ── Ownable state ───────────────────────────────────────────────────

    address public owner;
    address public pendingOwner;

    // ── Pausable state ──────────────────────────────────────────────────

    bool public paused;

    // ── Reentrancy guard state ──────────────────────────────────────────

    uint256 private constant _NOT_ENTERED = 1;
    uint256 private constant _ENTERED     = 2;
    uint256 private _status;

    // ── Bridge state ────────────────────────────────────────────────────

    /// @notice Monotonically increasing deposit nonce.
    uint256 public depositNonce;

    /// @notice Required number of relayer signatures for a withdrawal.
    uint256 public threshold;

    /// @notice Set of registered relayers.
    mapping(address => bool) public isRelayer;

    /// @notice Number of active relayers (kept in sync with isRelayer).
    uint256 public relayerCount;

    /// @notice Tracks which withdrawal nonces have already been processed.
    mapping(uint256 => bool) public processedWithdrawals;

    /// @notice Per-token locked balance (for accounting / auditing).
    mapping(address => uint256) public lockedBalance;

    // ── Events ──────────────────────────────────────────────────────────

    /// @notice Emitted when a user locks funds on Ethereum for bridging.
    event Deposit(
        uint256 indexed nonce,
        address indexed token,
        uint256 amount,
        address indexed sender,
        string omniphiRecipient
    );

    /// @notice Emitted when locked funds are released back to Ethereum.
    event Withdrawal(
        uint256 indexed nonce,
        address indexed token,
        uint256 amount,
        address indexed recipient
    );

    /// @notice Emitted when a relayer is added.
    event RelayerAdded(address indexed relayer);

    /// @notice Emitted when a relayer is removed.
    event RelayerRemoved(address indexed relayer);

    /// @notice Emitted when the signature threshold is changed.
    event ThresholdChanged(uint256 oldThreshold, uint256 newThreshold);

    /// @notice Emitted when the contract is paused.
    event Paused(address indexed account);

    /// @notice Emitted when the contract is unpaused.
    event Unpaused(address indexed account);

    /// @notice Emitted when ownership transfer is initiated.
    event OwnershipTransferStarted(address indexed previousOwner, address indexed newOwner);

    /// @notice Emitted when ownership transfer is completed.
    event OwnershipTransferred(address indexed previousOwner, address indexed newOwner);

    // ── Errors ──────────────────────────────────────────────────────────

    error NotOwner();
    error NotPendingOwner();
    error ContractPaused();
    error ContractNotPaused();
    error ReentrantCall();
    error ZeroAmount();
    error EmptyRecipient();
    error InvalidSignatureCount();
    error DuplicateSignature();
    error InvalidSignature();
    error SignerNotRelayer();
    error WithdrawalAlreadyProcessed();
    error RelayerAlreadyRegistered();
    error RelayerNotRegistered();
    error ThresholdTooHigh();
    error ThresholdZero();
    error InsufficientLockedBalance();
    error EthTransferFailed();
    error Erc20TransferFailed();
    error Erc20TransferFromFailed();
    error ZeroAddress();

    // ── Modifiers ───────────────────────────────────────────────────────

    modifier onlyOwner() {
        if (msg.sender != owner) revert NotOwner();
        _;
    }

    modifier whenNotPaused() {
        if (paused) revert ContractPaused();
        _;
    }

    modifier whenPaused() {
        if (!paused) revert ContractNotPaused();
        _;
    }

    modifier nonReentrant() {
        if (_status == _ENTERED) revert ReentrantCall();
        _status = _ENTERED;
        _;
        _status = _NOT_ENTERED;
    }

    // ── Constructor ─────────────────────────────────────────────────────

    /// @param initialRelayers  Initial set of relayer addresses.
    /// @param initialThreshold Number of signatures required for withdrawals.
    constructor(address[] memory initialRelayers, uint256 initialThreshold) {
        if (initialThreshold == 0) revert ThresholdZero();
        if (initialThreshold > initialRelayers.length) revert ThresholdTooHigh();

        owner   = msg.sender;
        _status = _NOT_ENTERED;

        for (uint256 i = 0; i < initialRelayers.length; i++) {
            address r = initialRelayers[i];
            if (r == address(0)) revert ZeroAddress();
            if (isRelayer[r]) revert RelayerAlreadyRegistered();
            isRelayer[r] = true;
            emit RelayerAdded(r);
        }

        relayerCount = initialRelayers.length;
        threshold    = initialThreshold;

        emit ThresholdChanged(0, initialThreshold);
        emit OwnershipTransferred(address(0), msg.sender);
    }

    // ── Deposit (Ethereum -> Omniphi) ───────────────────────────────────

    /// @notice Lock ETH or ERC-20 tokens for bridging to Omniphi.
    /// @param token            The ERC-20 token address, or address(0) for ETH.
    /// @param amount           The amount to lock (ignored for ETH; msg.value used).
    /// @param omniphiRecipient The bech32 address on Omniphi that will receive
    ///                         the wrapped tokens.
    function deposit(
        address token,
        uint256 amount,
        string calldata omniphiRecipient
    ) external payable whenNotPaused nonReentrant {
        if (bytes(omniphiRecipient).length == 0) revert EmptyRecipient();

        uint256 bridgedAmount;

        if (token == ETH_ADDRESS) {
            // Native ETH deposit — amount comes from msg.value.
            if (msg.value == 0) revert ZeroAmount();
            bridgedAmount = msg.value;
        } else {
            // ERC-20 deposit — pull tokens from the sender.
            if (amount == 0) revert ZeroAmount();
            if (msg.value != 0) revert ZeroAmount(); // no ETH should accompany ERC-20 deposits

            uint256 balanceBefore = IERC20(token).balanceOf(address(this));
            // Use transferFrom; caller must have approved this contract.
            (bool ok, bytes memory ret) = token.call(
                abi.encodeWithSelector(IERC20.transferFrom.selector, msg.sender, address(this), amount)
            );
            if (!ok || (ret.length > 0 && !abi.decode(ret, (bool)))) {
                revert Erc20TransferFromFailed();
            }
            uint256 balanceAfter = IERC20(token).balanceOf(address(this));
            // Use the actual received amount to handle fee-on-transfer tokens.
            bridgedAmount = balanceAfter - balanceBefore;
            if (bridgedAmount == 0) revert ZeroAmount();
        }

        lockedBalance[token] += bridgedAmount;

        uint256 nonce = depositNonce;
        depositNonce  = nonce + 1;

        emit Deposit(nonce, token, bridgedAmount, msg.sender, omniphiRecipient);
    }

    // ── Withdrawal (Omniphi -> Ethereum) ────────────────────────────────

    /// @notice Release locked funds after M-of-N relayer attestation.
    /// @param token      The ERC-20 token address, or address(0) for ETH.
    /// @param amount     The amount to release.
    /// @param recipient  The Ethereum address that receives the tokens.
    /// @param nonce      The unique withdrawal nonce (from Omniphi burn event).
    /// @param signatures Concatenated ECDSA signatures (each 65 bytes: r‖s‖v).
    function withdraw(
        address token,
        uint256 amount,
        address recipient,
        uint256 nonce,
        bytes calldata signatures
    ) external whenNotPaused nonReentrant {
        if (amount == 0) revert ZeroAmount();
        if (recipient == address(0)) revert ZeroAddress();
        if (processedWithdrawals[nonce]) revert WithdrawalAlreadyProcessed();

        // ── Verify signatures ───────────────────────────────────────────
        _verifySignatures(token, amount, recipient, nonce, signatures);

        // ── Mark processed before external calls (CEI pattern) ──────────
        processedWithdrawals[nonce] = true;

        if (lockedBalance[token] < amount) revert InsufficientLockedBalance();
        lockedBalance[token] -= amount;

        // ── Transfer funds ──────────────────────────────────────────────
        if (token == ETH_ADDRESS) {
            (bool sent, ) = recipient.call{value: amount}("");
            if (!sent) revert EthTransferFailed();
        } else {
            (bool ok, bytes memory ret) = token.call(
                abi.encodeWithSelector(IERC20.transfer.selector, recipient, amount)
            );
            if (!ok || (ret.length > 0 && !abi.decode(ret, (bool)))) {
                revert Erc20TransferFailed();
            }
        }

        emit Withdrawal(nonce, token, amount, recipient);
    }

    // ── Signature verification ──────────────────────────────────────────

    /// @dev Verifies that at least `threshold` unique relayer signatures
    ///      attest to the given withdrawal parameters.
    function _verifySignatures(
        address token,
        uint256 amount,
        address recipient,
        uint256 nonce,
        bytes calldata signatures
    ) internal view {
        uint256 sigCount = signatures.length / 65;
        if (sigCount < threshold)    revert InvalidSignatureCount();
        if (signatures.length % 65 != 0) revert InvalidSignatureCount();

        // EIP-191 prefixed hash of the withdrawal message.
        bytes32 messageHash = keccak256(
            abi.encodePacked(
                "\x19Ethereum Signed Message:\n32",
                keccak256(
                    abi.encodePacked(
                        block.chainid,
                        address(this),
                        token,
                        amount,
                        recipient,
                        nonce
                    )
                )
            )
        );

        address lastSigner = address(0);

        for (uint256 i = 0; i < sigCount; i++) {
            uint256 offset = i * 65;
            bytes32 r;
            bytes32 s;
            uint8   v;

            // solhint-disable-next-line no-inline-assembly
            assembly {
                // signatures is calldata; calldataload reads 32 bytes from the
                // given calldata offset.  signatures.offset is the absolute
                // calldata offset of the first byte of `signatures`.
                r := calldataload(add(signatures.offset, offset))
                s := calldataload(add(signatures.offset, add(offset, 32)))
                v := byte(0, calldataload(add(signatures.offset, add(offset, 64))))
            }

            // Normalize v (some signers return 0/1 instead of 27/28).
            if (v < 27) v += 27;

            // Reject malleable signatures (EIP-2).
            if (uint256(s) > 0x7FFFFFFFFFFFFFFFFFFFFFFFFFFFFFFF5D576E7357A4501DDFE92F46681B20A0) {
                revert InvalidSignature();
            }

            address signer = ecrecover(messageHash, v, r, s);
            if (signer == address(0))  revert InvalidSignature();
            if (!isRelayer[signer])    revert SignerNotRelayer();

            // Signers must be in strictly ascending order to prevent duplicates
            // without requiring an O(n^2) uniqueness check.
            if (signer <= lastSigner) revert DuplicateSignature();
            lastSigner = signer;
        }
    }

    // ── Relayer management (owner only) ─────────────────────────────────

    /// @notice Register a new relayer address.
    function addRelayer(address relayer) external onlyOwner {
        if (relayer == address(0)) revert ZeroAddress();
        if (isRelayer[relayer])    revert RelayerAlreadyRegistered();

        isRelayer[relayer] = true;
        relayerCount++;

        emit RelayerAdded(relayer);
    }

    /// @notice Remove a relayer address.  Reverts if the removal would make
    ///         the threshold unreachable.
    function removeRelayer(address relayer) external onlyOwner {
        if (!isRelayer[relayer])   revert RelayerNotRegistered();
        if (relayerCount - 1 < threshold) revert ThresholdTooHigh();

        isRelayer[relayer] = false;
        relayerCount--;

        emit RelayerRemoved(relayer);
    }

    /// @notice Update the signature threshold.
    function setThreshold(uint256 newThreshold) external onlyOwner {
        if (newThreshold == 0)             revert ThresholdZero();
        if (newThreshold > relayerCount)   revert ThresholdTooHigh();

        uint256 old = threshold;
        threshold   = newThreshold;

        emit ThresholdChanged(old, newThreshold);
    }

    // ── Emergency pause ─────────────────────────────────────────────────

    /// @notice Halt all deposits and withdrawals.
    function pause() external onlyOwner whenNotPaused {
        paused = true;
        emit Paused(msg.sender);
    }

    /// @notice Resume normal operation.
    function unpause() external onlyOwner whenPaused {
        paused = false;
        emit Unpaused(msg.sender);
    }

    // ── Ownership (two-step transfer) ───────────────────────────────────

    /// @notice Initiate ownership transfer.  The new owner must call
    ///         `acceptOwnership()` to complete the transfer.
    function transferOwnership(address newOwner) external onlyOwner {
        if (newOwner == address(0)) revert ZeroAddress();
        pendingOwner = newOwner;
        emit OwnershipTransferStarted(owner, newOwner);
    }

    /// @notice Complete ownership transfer.
    function acceptOwnership() external {
        if (msg.sender != pendingOwner) revert NotPendingOwner();
        emit OwnershipTransferred(owner, msg.sender);
        owner        = msg.sender;
        pendingOwner = address(0);
    }

    // ── View helpers ────────────────────────────────────────────────────

    /// @notice Convenience: hash a withdrawal message the same way
    ///         `_verifySignatures` does, so relayers can sign it off-chain.
    function withdrawalHash(
        address token,
        uint256 amount,
        address recipient,
        uint256 nonce
    ) external view returns (bytes32) {
        return keccak256(
            abi.encodePacked(
                "\x19Ethereum Signed Message:\n32",
                keccak256(
                    abi.encodePacked(
                        block.chainid,
                        address(this),
                        token,
                        amount,
                        recipient,
                        nonce
                    )
                )
            )
        );
    }

    /// @notice Allows the contract to receive plain ETH transfers (e.g. from
    ///         WETH unwrapping).
    receive() external payable {}
}
