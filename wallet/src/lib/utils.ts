/**
 * Utility Functions
 * Common helpers for the Omniphi wallet
 */

import { DECIMALS, DISPLAY_DENOM } from './constants';

/**
 * Format amount from base units to display units
 */
export function formatAmount(
  amount: string | number | bigint,
  decimals: number = DECIMALS,
  maxDecimals: number = 4
): string {
  // Handle decimal strings (e.g., from rewards API)
  if (typeof amount === 'string' && amount.includes('.')) {
    const [whole, frac = ''] = amount.split('.');
    const amountBigInt = BigInt(whole || 0);
    const divisor = BigInt(10 ** decimals);
    const wholePart = amountBigInt / divisor;
    const fractionalPart = amountBigInt % divisor;

    if (fractionalPart === 0n) {
      return wholePart.toLocaleString();
    }

    const fractionalStr = fractionalPart.toString().padStart(decimals, '0');
    const trimmed = fractionalStr.slice(0, maxDecimals).replace(/0+$/, '');

    if (!trimmed) {
      return wholePart.toLocaleString();
    }

    return `${wholePart.toLocaleString()}.${trimmed}`;
  }

  const amountBigInt = typeof amount === 'bigint' ? amount : BigInt(amount || 0);
  const divisor = BigInt(10 ** decimals);
  const wholePart = amountBigInt / divisor;
  const fractionalPart = amountBigInt % divisor;

  if (fractionalPart === 0n) {
    return wholePart.toLocaleString();
  }

  const fractionalStr = fractionalPart.toString().padStart(decimals, '0');
  const trimmed = fractionalStr.slice(0, maxDecimals).replace(/0+$/, '');

  if (!trimmed) {
    return wholePart.toLocaleString();
  }

  return `${wholePart.toLocaleString()}.${trimmed}`;
}

/**
 * Format amount with full precision
 */
export function formatAmountFull(
  amount: string | number | bigint,
  decimals: number = DECIMALS
): string {
  const amountBigInt = typeof amount === 'bigint' ? amount : BigInt(amount || 0);
  const divisor = BigInt(10 ** decimals);
  const wholePart = amountBigInt / divisor;
  const fractionalPart = amountBigInt % divisor;

  if (fractionalPart === 0n) {
    return wholePart.toString();
  }

  const fractionalStr = fractionalPart.toString().padStart(decimals, '0').replace(/0+$/, '');
  return `${wholePart}.${fractionalStr}`;
}

/**
 * Parse display amount to base units
 */
export function parseAmount(
  displayAmount: string,
  decimals: number = DECIMALS
): string {
  const [whole, fraction = ''] = displayAmount.split('.');
  const paddedFraction = fraction.slice(0, decimals).padEnd(decimals, '0');
  const result = BigInt(whole || 0) * BigInt(10 ** decimals) + BigInt(paddedFraction);
  return result.toString();
}

/**
 * Truncate address for display
 */
export function truncateAddress(
  address: string,
  startChars: number = 10,
  endChars: number = 6
): string {
  if (!address) return '';
  if (address.length <= startChars + endChars + 3) return address;
  return `${address.slice(0, startChars)}...${address.slice(-endChars)}`;
}

/**
 * Format date for display
 */
export function formatDate(date: Date | string | number): string {
  const d = new Date(date);
  return d.toLocaleDateString('en-US', {
    year: 'numeric',
    month: 'short',
    day: 'numeric',
  });
}

/**
 * Format date with time
 */
export function formatDateTime(date: Date | string | number): string {
  const d = new Date(date);
  return d.toLocaleDateString('en-US', {
    year: 'numeric',
    month: 'short',
    day: 'numeric',
    hour: '2-digit',
    minute: '2-digit',
  });
}

/**
 * Calculate time remaining from a date
 */
export function timeRemaining(endDate: Date | string): string {
  const end = new Date(endDate).getTime();
  const now = Date.now();
  const diff = end - now;

  if (diff <= 0) return 'Ended';

  const days = Math.floor(diff / (1000 * 60 * 60 * 24));
  const hours = Math.floor((diff % (1000 * 60 * 60 * 24)) / (1000 * 60 * 60));
  const minutes = Math.floor((diff % (1000 * 60 * 60)) / (1000 * 60));

  if (days > 0) return `${days}d ${hours}h remaining`;
  if (hours > 0) return `${hours}h ${minutes}m remaining`;
  return `${minutes}m remaining`;
}

/**
 * Validate cosmos address
 */
export function isValidAddress(address: string, prefix: string = 'omni'): boolean {
  if (!address) return false;
  return address.startsWith(prefix) && address.length === prefix.length + 39;
}

/**
 * Generate a random color from address (for avatars)
 */
export function addressToColor(address: string): string {
  let hash = 0;
  for (let i = 0; i < address.length; i++) {
    hash = address.charCodeAt(i) + ((hash << 5) - hash);
  }
  const h = hash % 360;
  return `hsl(${h}, 70%, 50%)`;
}

/**
 * Copy text to clipboard
 */
export async function copyToClipboard(text: string): Promise<boolean> {
  try {
    await navigator.clipboard.writeText(text);
    return true;
  } catch {
    return false;
  }
}

/**
 * Debounce function
 */
export function debounce<T extends (...args: unknown[]) => unknown>(
  fn: T,
  delay: number
): (...args: Parameters<T>) => void {
  let timeoutId: ReturnType<typeof setTimeout>;
  return (...args: Parameters<T>) => {
    clearTimeout(timeoutId);
    timeoutId = setTimeout(() => fn(...args), delay);
  };
}

/**
 * Class names helper (simplified clsx)
 */
export function cn(...classes: (string | boolean | undefined | null)[]): string {
  return classes.filter(Boolean).join(' ');
}
