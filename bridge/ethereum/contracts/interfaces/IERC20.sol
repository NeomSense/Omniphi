// SPDX-License-Identifier: MIT
pragma solidity ^0.8.24;

/// @title IERC20 — Standard ERC-20 token interface
/// @notice Follows the EIP-20 specification exactly.
interface IERC20 {
    /// @notice Returns the total supply of the token.
    function totalSupply() external view returns (uint256);

    /// @notice Returns the balance of `account`.
    function balanceOf(address account) external view returns (uint256);

    /// @notice Transfers `amount` tokens from the caller to `to`.
    /// @return True on success.
    function transfer(address to, uint256 amount) external returns (bool);

    /// @notice Returns the remaining allowance that `spender` can spend on
    ///         behalf of `owner`.
    function allowance(address owner, address spender) external view returns (uint256);

    /// @notice Sets the allowance of `spender` to `amount` for the caller.
    /// @return True on success.
    function approve(address spender, uint256 amount) external returns (bool);

    /// @notice Transfers `amount` tokens from `from` to `to` using the
    ///         caller's allowance.
    /// @return True on success.
    function transferFrom(address from, address to, uint256 amount) external returns (bool);

    /// @notice Emitted when `value` tokens are moved from `from` to `to`.
    event Transfer(address indexed from, address indexed to, uint256 value);

    /// @notice Emitted when `owner` grants `spender` an allowance of `value`.
    event Approval(address indexed owner, address indexed spender, uint256 value);
}
