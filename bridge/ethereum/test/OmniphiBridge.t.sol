// SPDX-License-Identifier: MIT
pragma solidity ^0.8.24;

import "forge-std/Test.sol";
import "../contracts/OmniphiBridge.sol";
import "./MockERC20.sol";
import "./helpers/SigUtils.sol";

/// @title OmniphiBridgeTest — Comprehensive Foundry test suite
/// @notice 30+ test functions covering deployment, deposits, withdrawals,
///         relayer management, pause/unpause, and edge cases.
contract OmniphiBridgeTest is Test {
    using SigUtils for *;

    // ── Constants / test accounts ────────────────────────────────────────

    // Private keys for deterministic relayer addresses.
    uint256 constant RELAYER_PK_1 = 0xA001;
    uint256 constant RELAYER_PK_2 = 0xA002;
    uint256 constant RELAYER_PK_3 = 0xA003;
    uint256 constant RELAYER_PK_4 = 0xA004;
    uint256 constant NON_RELAYER_PK = 0xB001;

    address relayer1;
    address relayer2;
    address relayer3;
    address relayer4;
    address nonRelayer;

    address deployer = address(0xD00D);
    address alice    = address(0xA11CE);
    address bob      = address(0xB0B);

    OmniphiBridge bridge;
    MockERC20     token;

    string constant OMNI_ADDR = "omni1abc123def456";

    // ── setUp ────────────────────────────────────────────────────────────

    function setUp() public {
        // Derive addresses from private keys.
        relayer1   = vm.addr(RELAYER_PK_1);
        relayer2   = vm.addr(RELAYER_PK_2);
        relayer3   = vm.addr(RELAYER_PK_3);
        relayer4   = vm.addr(RELAYER_PK_4);
        nonRelayer = vm.addr(NON_RELAYER_PK);

        // Deploy bridge with 3 relayers, threshold 2.
        address[] memory relayers = new address[](3);
        relayers[0] = relayer1;
        relayers[1] = relayer2;
        relayers[2] = relayer3;

        vm.prank(deployer);
        bridge = new OmniphiBridge(relayers, 2);

        // Deploy a mock ERC-20.
        token = new MockERC20("Test Token", "TT", 18);

        // Fund accounts.
        vm.deal(alice, 100 ether);
        vm.deal(bob, 100 ether);
        vm.deal(address(bridge), 50 ether); // pre-fund bridge for ETH withdrawals

        // Mint tokens to alice and pre-fund bridge for ERC-20 withdrawals.
        token.mint(alice, 1_000_000e18);
        token.mint(address(bridge), 500_000e18);

        // Track locked balance for pre-funded amounts by depositing properly.
        // We directly set locked balances via storage manipulation for test convenience.
        // For ETH locked balance:
        // lockedBalance[ETH_ADDRESS] is at slot keccak256(abi.encode(address(0), 6))
        // But it's simpler to do real deposits. Let's use a simpler approach:
        // We'll make deposits to build up locked balance when needed in specific tests.
    }

    // ====================================================================
    //  DEPLOYMENT TESTS
    // ====================================================================

    function test_constructor_setsOwner() public view {
        assertEq(bridge.owner(), deployer);
    }

    function test_constructor_setsThreshold() public view {
        assertEq(bridge.threshold(), 2);
    }

    function test_constructor_registersRelayers() public view {
        assertTrue(bridge.isRelayer(relayer1));
        assertTrue(bridge.isRelayer(relayer2));
        assertTrue(bridge.isRelayer(relayer3));
        assertFalse(bridge.isRelayer(nonRelayer));
        assertEq(bridge.relayerCount(), 3);
    }

    function test_constructor_revertsThresholdZero() public {
        address[] memory relayers = new address[](1);
        relayers[0] = relayer1;
        vm.expectRevert(OmniphiBridge.ThresholdZero.selector);
        new OmniphiBridge(relayers, 0);
    }

    function test_constructor_revertsThresholdTooHigh() public {
        address[] memory relayers = new address[](1);
        relayers[0] = relayer1;
        vm.expectRevert(OmniphiBridge.ThresholdTooHigh.selector);
        new OmniphiBridge(relayers, 5);
    }

    function test_constructor_revertsZeroAddressRelayer() public {
        address[] memory relayers = new address[](2);
        relayers[0] = relayer1;
        relayers[1] = address(0);
        vm.expectRevert(OmniphiBridge.ZeroAddress.selector);
        new OmniphiBridge(relayers, 1);
    }

    function test_constructor_revertsDuplicateRelayer() public {
        address[] memory relayers = new address[](2);
        relayers[0] = relayer1;
        relayers[1] = relayer1;
        vm.expectRevert(OmniphiBridge.RelayerAlreadyRegistered.selector);
        new OmniphiBridge(relayers, 1);
    }

    function test_constructor_depositNonceStartsAtZero() public view {
        assertEq(bridge.depositNonce(), 0);
    }

    // ====================================================================
    //  DEPOSIT TESTS
    // ====================================================================

    function test_depositETH_emitsEvent() public {
        vm.prank(alice);
        vm.expectEmit(true, true, true, true);
        emit OmniphiBridge.Deposit(0, address(0), 1 ether, alice, OMNI_ADDR);
        bridge.deposit{value: 1 ether}(address(0), 0, OMNI_ADDR);
    }

    function test_depositETH_incrementsNonce() public {
        vm.startPrank(alice);
        bridge.deposit{value: 1 ether}(address(0), 0, OMNI_ADDR);
        assertEq(bridge.depositNonce(), 1);
        bridge.deposit{value: 2 ether}(address(0), 0, OMNI_ADDR);
        assertEq(bridge.depositNonce(), 2);
        vm.stopPrank();
    }

    function test_depositETH_updatesLockedBalance() public {
        vm.prank(alice);
        bridge.deposit{value: 5 ether}(address(0), 0, OMNI_ADDR);
        assertEq(bridge.lockedBalance(address(0)), 5 ether);
    }

    function test_depositETH_revertsZeroAmount() public {
        vm.prank(alice);
        vm.expectRevert(OmniphiBridge.ZeroAmount.selector);
        bridge.deposit{value: 0}(address(0), 0, OMNI_ADDR);
    }

    function test_depositETH_revertsEmptyRecipient() public {
        vm.prank(alice);
        vm.expectRevert(OmniphiBridge.EmptyRecipient.selector);
        bridge.deposit{value: 1 ether}(address(0), 0, "");
    }

    function test_depositERC20_transfersTokensAndEmits() public {
        vm.startPrank(alice);
        token.approve(address(bridge), 500e18);

        vm.expectEmit(true, true, true, true);
        emit OmniphiBridge.Deposit(0, address(token), 500e18, alice, OMNI_ADDR);
        bridge.deposit(address(token), 500e18, OMNI_ADDR);
        vm.stopPrank();

        assertEq(token.balanceOf(alice), 1_000_000e18 - 500e18);
        assertEq(bridge.lockedBalance(address(token)), 500e18);
    }

    function test_depositERC20_revertsZeroAmount() public {
        vm.startPrank(alice);
        token.approve(address(bridge), 1e18);
        vm.expectRevert(OmniphiBridge.ZeroAmount.selector);
        bridge.deposit(address(token), 0, OMNI_ADDR);
        vm.stopPrank();
    }

    function test_depositERC20_revertsIfETHSentWithERC20() public {
        vm.startPrank(alice);
        token.approve(address(bridge), 1e18);
        vm.expectRevert(OmniphiBridge.ZeroAmount.selector);
        bridge.deposit{value: 1 ether}(address(token), 1e18, OMNI_ADDR);
        vm.stopPrank();
    }

    function test_deposit_revertsWhenPaused() public {
        vm.prank(deployer);
        bridge.pause();

        vm.prank(alice);
        vm.expectRevert(OmniphiBridge.ContractPaused.selector);
        bridge.deposit{value: 1 ether}(address(0), 0, OMNI_ADDR);
    }

    function test_depositNonce_incrementsSequentially() public {
        vm.startPrank(alice);
        for (uint256 i = 0; i < 5; i++) {
            bridge.deposit{value: 0.1 ether}(address(0), 0, OMNI_ADDR);
            assertEq(bridge.depositNonce(), i + 1);
        }
        vm.stopPrank();
    }

    // ====================================================================
    //  WITHDRAWAL TESTS
    // ====================================================================

    /// @dev Helper: do an ETH deposit so the bridge has locked ETH balance,
    ///      then build sorted signatures for a withdrawal.
    function _depositAndSignETH(
        uint256 depositAmt,
        uint256 withdrawAmt,
        address recipient,
        uint256 wdNonce,
        uint256[] memory pks
    ) internal returns (bytes memory sigs) {
        // Deposit ETH to build locked balance.
        vm.prank(alice);
        bridge.deposit{value: depositAmt}(address(0), 0, OMNI_ADDR);

        (sigs, ) = SigUtils.buildSortedSignatures(
            vm, pks, block.chainid, address(bridge),
            address(0), withdrawAmt, recipient, wdNonce
        );
    }

    /// @dev Helper: do an ERC-20 deposit so the bridge has locked token balance,
    ///      then build sorted signatures for a withdrawal.
    function _depositAndSignERC20(
        uint256 depositAmt,
        uint256 withdrawAmt,
        address recipient,
        uint256 wdNonce,
        uint256[] memory pks
    ) internal returns (bytes memory sigs) {
        vm.startPrank(alice);
        token.approve(address(bridge), depositAmt);
        bridge.deposit(address(token), depositAmt, OMNI_ADDR);
        vm.stopPrank();

        (sigs, ) = SigUtils.buildSortedSignatures(
            vm, pks, block.chainid, address(bridge),
            address(token), withdrawAmt, recipient, wdNonce
        );
    }

    function _twoRelayerPKs() internal pure returns (uint256[] memory pks) {
        pks = new uint256[](2);
        pks[0] = RELAYER_PK_1;
        pks[1] = RELAYER_PK_2;
    }

    function _threeRelayerPKs() internal pure returns (uint256[] memory pks) {
        pks = new uint256[](3);
        pks[0] = RELAYER_PK_1;
        pks[1] = RELAYER_PK_2;
        pks[2] = RELAYER_PK_3;
    }

    function test_withdrawETH_success() public {
        uint256 bobBalBefore = bob.balance;
        bytes memory sigs = _depositAndSignETH(5 ether, 2 ether, bob, 0, _twoRelayerPKs());

        vm.expectEmit(true, true, true, true);
        emit OmniphiBridge.Withdrawal(0, address(0), 2 ether, bob);
        bridge.withdraw(address(0), 2 ether, bob, 0, sigs);

        assertEq(bob.balance, bobBalBefore + 2 ether);
        assertEq(bridge.lockedBalance(address(0)), 3 ether);
        assertTrue(bridge.processedWithdrawals(0));
    }

    function test_withdrawERC20_success() public {
        uint256 bobBalBefore = token.balanceOf(bob);
        bytes memory sigs = _depositAndSignERC20(1000e18, 400e18, bob, 0, _twoRelayerPKs());

        bridge.withdraw(address(token), 400e18, bob, 0, sigs);

        assertEq(token.balanceOf(bob), bobBalBefore + 400e18);
        assertEq(bridge.lockedBalance(address(token)), 600e18);
        assertTrue(bridge.processedWithdrawals(0));
    }

    function test_withdraw_revertsInsufficientSignatures() public {
        vm.prank(alice);
        bridge.deposit{value: 5 ether}(address(0), 0, OMNI_ADDR);

        // Only 1 signature when threshold is 2.
        uint256[] memory pks = new uint256[](1);
        pks[0] = RELAYER_PK_1;
        (bytes memory sigs, ) = SigUtils.buildSortedSignatures(
            vm, pks, block.chainid, address(bridge),
            address(0), 2 ether, bob, 0
        );

        vm.expectRevert(OmniphiBridge.InvalidSignatureCount.selector);
        bridge.withdraw(address(0), 2 ether, bob, 0, sigs);
    }

    function test_withdraw_revertsDuplicateSigner() public {
        vm.prank(alice);
        bridge.deposit{value: 5 ether}(address(0), 0, OMNI_ADDR);

        // Build two identical signatures from the same key (will fail ascending check).
        bytes32 digest = SigUtils.withdrawalMessageHash(
            block.chainid, address(bridge), address(0), 2 ether, bob, 0
        );
        (uint8 v, bytes32 r, bytes32 s) = vm.sign(RELAYER_PK_1, digest);
        bytes memory sigs = abi.encodePacked(r, s, v, r, s, v);

        vm.expectRevert(OmniphiBridge.DuplicateSignature.selector);
        bridge.withdraw(address(0), 2 ether, bob, 0, sigs);
    }

    function test_withdraw_revertsNonRelayerSigner() public {
        vm.prank(alice);
        bridge.deposit{value: 5 ether}(address(0), 0, OMNI_ADDR);

        // One valid relayer + one non-relayer.
        uint256[] memory pks = new uint256[](2);
        pks[0] = RELAYER_PK_1;
        pks[1] = NON_RELAYER_PK;
        (bytes memory sigs, ) = SigUtils.buildSortedSignatures(
            vm, pks, block.chainid, address(bridge),
            address(0), 2 ether, bob, 0
        );

        vm.expectRevert(OmniphiBridge.SignerNotRelayer.selector);
        bridge.withdraw(address(0), 2 ether, bob, 0, sigs);
    }

    function test_withdraw_revertsReplayedNonce() public {
        bytes memory sigs = _depositAndSignETH(5 ether, 1 ether, bob, 0, _twoRelayerPKs());
        bridge.withdraw(address(0), 1 ether, bob, 0, sigs);

        vm.expectRevert(OmniphiBridge.WithdrawalAlreadyProcessed.selector);
        bridge.withdraw(address(0), 1 ether, bob, 0, sigs);
    }

    function test_withdraw_revertsWhenPaused() public {
        bytes memory sigs = _depositAndSignETH(5 ether, 1 ether, bob, 0, _twoRelayerPKs());

        vm.prank(deployer);
        bridge.pause();

        vm.expectRevert(OmniphiBridge.ContractPaused.selector);
        bridge.withdraw(address(0), 1 ether, bob, 0, sigs);
    }

    function test_withdraw_revertsZeroAmount() public {
        uint256[] memory pks = _twoRelayerPKs();
        (bytes memory sigs, ) = SigUtils.buildSortedSignatures(
            vm, pks, block.chainid, address(bridge),
            address(0), 0, bob, 0
        );

        vm.expectRevert(OmniphiBridge.ZeroAmount.selector);
        bridge.withdraw(address(0), 0, bob, 0, sigs);
    }

    function test_withdraw_revertsZeroRecipient() public {
        vm.prank(alice);
        bridge.deposit{value: 5 ether}(address(0), 0, OMNI_ADDR);

        uint256[] memory pks = _twoRelayerPKs();
        (bytes memory sigs, ) = SigUtils.buildSortedSignatures(
            vm, pks, block.chainid, address(bridge),
            address(0), 1 ether, address(0), 0
        );

        vm.expectRevert(OmniphiBridge.ZeroAddress.selector);
        bridge.withdraw(address(0), 1 ether, address(0), 0, sigs);
    }

    function test_withdraw_revertsInsufficientLockedBalance() public {
        // Deposit only 1 ETH but try to withdraw 10 ETH.
        vm.prank(alice);
        bridge.deposit{value: 1 ether}(address(0), 0, OMNI_ADDR);

        uint256[] memory pks = _twoRelayerPKs();
        (bytes memory sigs, ) = SigUtils.buildSortedSignatures(
            vm, pks, block.chainid, address(bridge),
            address(0), 10 ether, bob, 0
        );

        vm.expectRevert(OmniphiBridge.InsufficientLockedBalance.selector);
        bridge.withdraw(address(0), 10 ether, bob, 0, sigs);
    }

    function test_withdraw_signaturesMustBeAscendingOrder() public {
        vm.prank(alice);
        bridge.deposit{value: 5 ether}(address(0), 0, OMNI_ADDR);

        // Build signatures in DESCENDING order (wrong).
        bytes32 digest = SigUtils.withdrawalMessageHash(
            block.chainid, address(bridge), address(0), 1 ether, bob, 0
        );

        address addr1 = vm.addr(RELAYER_PK_1);
        address addr2 = vm.addr(RELAYER_PK_2);

        // Determine which key has the higher address so we can force descending.
        uint256 firstPk;
        uint256 secondPk;
        if (addr1 > addr2) {
            firstPk  = RELAYER_PK_1; // higher address first = descending = wrong
            secondPk = RELAYER_PK_2;
        } else {
            firstPk  = RELAYER_PK_2;
            secondPk = RELAYER_PK_1;
        }

        (uint8 v1, bytes32 r1, bytes32 s1) = vm.sign(firstPk, digest);
        (uint8 v2, bytes32 r2, bytes32 s2) = vm.sign(secondPk, digest);
        bytes memory sigs = abi.encodePacked(r1, s1, v1, r2, s2, v2);

        vm.expectRevert(OmniphiBridge.DuplicateSignature.selector);
        bridge.withdraw(address(0), 1 ether, bob, 0, sigs);
    }

    function test_withdraw_exactThresholdSignaturesSucceeds() public {
        // Threshold is 2, provide exactly 2 signatures.
        bytes memory sigs = _depositAndSignETH(10 ether, 3 ether, bob, 0, _twoRelayerPKs());
        bridge.withdraw(address(0), 3 ether, bob, 0, sigs);
        assertTrue(bridge.processedWithdrawals(0));
    }

    function test_withdraw_moreThanThresholdSignaturesSucceeds() public {
        // Threshold is 2, provide 3 signatures — should still pass.
        bytes memory sigs = _depositAndSignETH(10 ether, 3 ether, bob, 0, _threeRelayerPKs());
        bridge.withdraw(address(0), 3 ether, bob, 0, sigs);
        assertTrue(bridge.processedWithdrawals(0));
    }

    function test_withdraw_malformedSignatureLengthReverts() public {
        vm.prank(alice);
        bridge.deposit{value: 5 ether}(address(0), 0, OMNI_ADDR);

        // Send 100 bytes (not a multiple of 65).
        bytes memory badSigs = new bytes(100);
        vm.expectRevert(OmniphiBridge.InvalidSignatureCount.selector);
        bridge.withdraw(address(0), 1 ether, bob, 0, badSigs);
    }

    // ====================================================================
    //  RELAYER MANAGEMENT TESTS
    // ====================================================================

    function test_addRelayer_success() public {
        vm.prank(deployer);
        vm.expectEmit(true, false, false, false);
        emit OmniphiBridge.RelayerAdded(relayer4);
        bridge.addRelayer(relayer4);

        assertTrue(bridge.isRelayer(relayer4));
        assertEq(bridge.relayerCount(), 4);
    }

    function test_addRelayer_revertsNotOwner() public {
        vm.prank(alice);
        vm.expectRevert(OmniphiBridge.NotOwner.selector);
        bridge.addRelayer(relayer4);
    }

    function test_addRelayer_revertsZeroAddress() public {
        vm.prank(deployer);
        vm.expectRevert(OmniphiBridge.ZeroAddress.selector);
        bridge.addRelayer(address(0));
    }

    function test_addRelayer_revertsDuplicate() public {
        vm.prank(deployer);
        vm.expectRevert(OmniphiBridge.RelayerAlreadyRegistered.selector);
        bridge.addRelayer(relayer1);
    }

    function test_removeRelayer_success() public {
        // Add a 4th relayer first so removal doesn't violate threshold.
        vm.startPrank(deployer);
        bridge.addRelayer(relayer4);
        bridge.removeRelayer(relayer3);
        vm.stopPrank();

        assertFalse(bridge.isRelayer(relayer3));
        assertEq(bridge.relayerCount(), 3);
    }

    function test_removeRelayer_revertsNotOwner() public {
        vm.prank(alice);
        vm.expectRevert(OmniphiBridge.NotOwner.selector);
        bridge.removeRelayer(relayer1);
    }

    function test_removeRelayer_revertsNotRegistered() public {
        vm.prank(deployer);
        vm.expectRevert(OmniphiBridge.RelayerNotRegistered.selector);
        bridge.removeRelayer(relayer4);
    }

    function test_removeRelayer_revertsIfThresholdWouldBeUnreachable() public {
        // 3 relayers, threshold 2: removing one leaves 2 == threshold, OK.
        // But if threshold were 3, removing one would leave 2 < 3, revert.
        vm.startPrank(deployer);
        bridge.setThreshold(3);
        vm.expectRevert(OmniphiBridge.ThresholdTooHigh.selector);
        bridge.removeRelayer(relayer1);
        vm.stopPrank();
    }

    function test_setThreshold_success() public {
        vm.prank(deployer);
        vm.expectEmit(false, false, false, true);
        emit OmniphiBridge.ThresholdChanged(2, 3);
        bridge.setThreshold(3);
        assertEq(bridge.threshold(), 3);
    }

    function test_setThreshold_revertsNotOwner() public {
        vm.prank(alice);
        vm.expectRevert(OmniphiBridge.NotOwner.selector);
        bridge.setThreshold(1);
    }

    function test_setThreshold_revertsZero() public {
        vm.prank(deployer);
        vm.expectRevert(OmniphiBridge.ThresholdZero.selector);
        bridge.setThreshold(0);
    }

    function test_setThreshold_revertsTooHigh() public {
        vm.prank(deployer);
        vm.expectRevert(OmniphiBridge.ThresholdTooHigh.selector);
        bridge.setThreshold(10);
    }

    // ====================================================================
    //  PAUSE / UNPAUSE TESTS
    // ====================================================================

    function test_pause_onlyOwner() public {
        vm.prank(alice);
        vm.expectRevert(OmniphiBridge.NotOwner.selector);
        bridge.pause();
    }

    function test_pause_success() public {
        vm.prank(deployer);
        vm.expectEmit(true, false, false, false);
        emit OmniphiBridge.Paused(deployer);
        bridge.pause();
        assertTrue(bridge.paused());
    }

    function test_pause_revertsIfAlreadyPaused() public {
        vm.startPrank(deployer);
        bridge.pause();
        vm.expectRevert(OmniphiBridge.ContractPaused.selector);
        bridge.pause();
        vm.stopPrank();
    }

    function test_unpause_onlyOwner() public {
        vm.prank(deployer);
        bridge.pause();

        vm.prank(alice);
        vm.expectRevert(OmniphiBridge.NotOwner.selector);
        bridge.unpause();
    }

    function test_unpause_success() public {
        vm.startPrank(deployer);
        bridge.pause();
        vm.expectEmit(true, false, false, false);
        emit OmniphiBridge.Unpaused(deployer);
        bridge.unpause();
        vm.stopPrank();

        assertFalse(bridge.paused());
    }

    function test_unpause_revertsIfNotPaused() public {
        vm.prank(deployer);
        vm.expectRevert(OmniphiBridge.ContractNotPaused.selector);
        bridge.unpause();
    }

    function test_operationsResumeAfterUnpause() public {
        vm.prank(deployer);
        bridge.pause();

        // Deposit should revert while paused.
        vm.prank(alice);
        vm.expectRevert(OmniphiBridge.ContractPaused.selector);
        bridge.deposit{value: 1 ether}(address(0), 0, OMNI_ADDR);

        // Unpause.
        vm.prank(deployer);
        bridge.unpause();

        // Deposit should work now.
        vm.prank(alice);
        bridge.deposit{value: 1 ether}(address(0), 0, OMNI_ADDR);
        assertEq(bridge.depositNonce(), 1);
    }

    // ====================================================================
    //  OWNERSHIP TESTS
    // ====================================================================

    function test_transferOwnership_twoStep() public {
        vm.prank(deployer);
        bridge.transferOwnership(alice);
        assertEq(bridge.pendingOwner(), alice);
        assertEq(bridge.owner(), deployer); // still deployer

        vm.prank(alice);
        bridge.acceptOwnership();
        assertEq(bridge.owner(), alice);
        assertEq(bridge.pendingOwner(), address(0));
    }

    function test_transferOwnership_revertsNotOwner() public {
        vm.prank(alice);
        vm.expectRevert(OmniphiBridge.NotOwner.selector);
        bridge.transferOwnership(bob);
    }

    function test_transferOwnership_revertsZeroAddress() public {
        vm.prank(deployer);
        vm.expectRevert(OmniphiBridge.ZeroAddress.selector);
        bridge.transferOwnership(address(0));
    }

    function test_acceptOwnership_revertsNotPendingOwner() public {
        vm.prank(deployer);
        bridge.transferOwnership(alice);

        vm.prank(bob);
        vm.expectRevert(OmniphiBridge.NotPendingOwner.selector);
        bridge.acceptOwnership();
    }

    // ====================================================================
    //  EDGE CASES
    // ====================================================================

    function test_multipleSequentialDepositsAndWithdrawals() public {
        // Deposit 3 times.
        vm.startPrank(alice);
        bridge.deposit{value: 1 ether}(address(0), 0, OMNI_ADDR);
        bridge.deposit{value: 2 ether}(address(0), 0, OMNI_ADDR);
        bridge.deposit{value: 3 ether}(address(0), 0, OMNI_ADDR);
        vm.stopPrank();

        assertEq(bridge.depositNonce(), 3);
        assertEq(bridge.lockedBalance(address(0)), 6 ether);

        // Withdraw twice with different nonces.
        uint256[] memory pks = _twoRelayerPKs();

        (bytes memory sigs0, ) = SigUtils.buildSortedSignatures(
            vm, pks, block.chainid, address(bridge),
            address(0), 2 ether, bob, 100
        );
        bridge.withdraw(address(0), 2 ether, bob, 100, sigs0);

        // Need fresh pks array since buildSortedSignatures mutates it.
        pks = _twoRelayerPKs();
        (bytes memory sigs1, ) = SigUtils.buildSortedSignatures(
            vm, pks, block.chainid, address(bridge),
            address(0), 1 ether, bob, 101
        );
        bridge.withdraw(address(0), 1 ether, bob, 101, sigs1);

        assertEq(bridge.lockedBalance(address(0)), 3 ether);
        assertTrue(bridge.processedWithdrawals(100));
        assertTrue(bridge.processedWithdrawals(101));
    }

    function test_largeAmountDeposit() public {
        // Test with a large (but not overflowing) amount.
        uint256 largeAmt = 1e36; // 1e36 tokens
        token.mint(alice, largeAmt);

        vm.startPrank(alice);
        token.approve(address(bridge), largeAmt);
        bridge.deposit(address(token), largeAmt, OMNI_ADDR);
        vm.stopPrank();

        assertEq(bridge.lockedBalance(address(token)), largeAmt);
    }

    function test_withdrawalHash_matchesInternalHash() public view {
        // The public withdrawalHash view should match what the contract
        // uses internally to verify signatures.
        bytes32 h = bridge.withdrawalHash(address(token), 100e18, bob, 42);
        bytes32 expected = SigUtils.withdrawalMessageHash(
            block.chainid, address(bridge), address(token), 100e18, bob, 42
        );
        assertEq(h, expected);
    }

    function test_receiveETH_fallback() public {
        // The contract has a receive() function and should accept plain ETH.
        uint256 balBefore = address(bridge).balance;
        vm.prank(alice);
        (bool ok, ) = address(bridge).call{value: 1 ether}("");
        assertTrue(ok);
        assertEq(address(bridge).balance, balBefore + 1 ether);
    }

    function test_withdraw_differentNoncesAreIndependent() public {
        // Two withdrawals with different nonces should both succeed.
        vm.prank(alice);
        bridge.deposit{value: 10 ether}(address(0), 0, OMNI_ADDR);

        uint256[] memory pks = _twoRelayerPKs();
        (bytes memory sigs0, ) = SigUtils.buildSortedSignatures(
            vm, pks, block.chainid, address(bridge),
            address(0), 1 ether, bob, 0
        );
        bridge.withdraw(address(0), 1 ether, bob, 0, sigs0);

        pks = _twoRelayerPKs();
        (bytes memory sigs1, ) = SigUtils.buildSortedSignatures(
            vm, pks, block.chainid, address(bridge),
            address(0), 1 ether, bob, 1
        );
        bridge.withdraw(address(0), 1 ether, bob, 1, sigs1);

        assertTrue(bridge.processedWithdrawals(0));
        assertTrue(bridge.processedWithdrawals(1));
        assertEq(bridge.lockedBalance(address(0)), 8 ether);
    }

    function test_addRelayerThenUseInWithdrawal() public {
        // Add relayer4, then use it to sign a withdrawal.
        vm.prank(deployer);
        bridge.addRelayer(relayer4);

        vm.prank(alice);
        bridge.deposit{value: 5 ether}(address(0), 0, OMNI_ADDR);

        // Use relayer1 + relayer4 (new relayer).
        uint256[] memory pks = new uint256[](2);
        pks[0] = RELAYER_PK_1;
        pks[1] = RELAYER_PK_4;
        (bytes memory sigs, ) = SigUtils.buildSortedSignatures(
            vm, pks, block.chainid, address(bridge),
            address(0), 2 ether, bob, 0
        );
        bridge.withdraw(address(0), 2 ether, bob, 0, sigs);
        assertTrue(bridge.processedWithdrawals(0));
    }

    function test_removeRelayerInvalidatesSignatures() public {
        // Remove relayer2, then try to use its signature.
        vm.startPrank(deployer);
        bridge.addRelayer(relayer4);   // add 4th so removal is allowed
        bridge.removeRelayer(relayer2);
        vm.stopPrank();

        vm.prank(alice);
        bridge.deposit{value: 5 ether}(address(0), 0, OMNI_ADDR);

        // Try to withdraw with relayer1 + relayer2 (removed).
        uint256[] memory pks = new uint256[](2);
        pks[0] = RELAYER_PK_1;
        pks[1] = RELAYER_PK_2;
        (bytes memory sigs, ) = SigUtils.buildSortedSignatures(
            vm, pks, block.chainid, address(bridge),
            address(0), 1 ether, bob, 0
        );

        vm.expectRevert(OmniphiBridge.SignerNotRelayer.selector);
        bridge.withdraw(address(0), 1 ether, bob, 0, sigs);
    }

    function test_withdraw_signatureFromWrongChainIdFails() public {
        vm.prank(alice);
        bridge.deposit{value: 5 ether}(address(0), 0, OMNI_ADDR);

        // Sign with wrong chain ID.
        uint256[] memory pks = _twoRelayerPKs();
        (bytes memory sigs, ) = SigUtils.buildSortedSignatures(
            vm, pks, 999999, address(bridge),
            address(0), 1 ether, bob, 0
        );

        // The recovered signer won't match any relayer.
        vm.expectRevert(); // Could be SignerNotRelayer or DuplicateSignature or InvalidSignature
        bridge.withdraw(address(0), 1 ether, bob, 0, sigs);
    }
}
