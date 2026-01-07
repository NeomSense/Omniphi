/**
 * Wallet Store
 * Global state management for wallet using Zustand
 */

import { create } from 'zustand';
import { persist } from 'zustand/middleware';
import { SigningStargateClient } from '@cosmjs/stargate';
import {
  OmniphiWallet,
  createWalletFromMnemonic,
  createNewWallet,
  createSigningClient,
  encryptMnemonic,
  decryptMnemonic,
  EncryptedWallet,
} from '@/lib/wallet';
import { getBalance, getDelegations, getStakingRewards, Delegation, RewardsResponse } from '@/lib/api';
import { STORAGE_KEYS, DENOM } from '@/lib/constants';

export interface WalletState {
  // Wallet data
  wallet: OmniphiWallet | null;
  encryptedWallet: EncryptedWallet | null;
  isUnlocked: boolean;

  // Balances
  balance: string;
  delegations: Delegation[];
  rewards: RewardsResponse | null;

  // Signing client
  signingClient: SigningStargateClient | null;

  // UI state
  isLoading: boolean;
  error: string | null;

  // Actions
  createWallet: (password: string) => Promise<string>;
  importWallet: (mnemonic: string, password: string) => Promise<void>;
  unlockWallet: (password: string) => Promise<void>;
  lockWallet: () => void;
  clearWallet: () => void;

  // Data fetching
  refreshBalance: () => Promise<void>;
  refreshDelegations: () => Promise<void>;
  refreshRewards: () => Promise<void>;
  refreshAll: () => Promise<void>;

  // Signing client
  getSigningClient: () => Promise<SigningStargateClient>;
}

export const useWalletStore = create<WalletState>()(
  persist(
    (set, get) => ({
      // Initial state
      wallet: null,
      encryptedWallet: null,
      isUnlocked: false,
      balance: '0',
      delegations: [],
      rewards: null,
      signingClient: null,
      isLoading: false,
      error: null,

      /**
       * Create a new wallet with fresh mnemonic
       */
      createWallet: async (password: string) => {
        set({ isLoading: true, error: null });

        try {
          // Generate new wallet
          const wallet = await createNewWallet();

          // Encrypt mnemonic
          const encryptedMnemonic = await encryptMnemonic(wallet.mnemonic, password);

          const encryptedWallet: EncryptedWallet = {
            encryptedMnemonic,
            address: wallet.cosmos.address,
            evmAddress: wallet.evm.address,
            createdAt: Date.now(),
          };

          // Don't create signing client here - it will be created lazily when needed
          set({
            wallet,
            encryptedWallet,
            isUnlocked: true,
            signingClient: null,
            isLoading: false,
          });

          // Fetch initial balance (don't await - let it happen in background)
          get().refreshAll().catch(console.error);

          // Return mnemonic for user to save
          return wallet.mnemonic;
        } catch (error) {
          const message = error instanceof Error ? error.message : 'Failed to create wallet';
          set({ error: message, isLoading: false });
          throw error;
        }
      },

      /**
       * Import wallet from mnemonic
       */
      importWallet: async (mnemonic: string, password: string) => {
        set({ isLoading: true, error: null });

        try {
          // Create wallet from mnemonic
          const wallet = await createWalletFromMnemonic(mnemonic);

          // Encrypt mnemonic
          const encryptedMnemonic = await encryptMnemonic(mnemonic, password);

          const encryptedWallet: EncryptedWallet = {
            encryptedMnemonic,
            address: wallet.cosmos.address,
            evmAddress: wallet.evm.address,
            createdAt: Date.now(),
          };

          // Don't create signing client here - it will be created lazily when needed
          set({
            wallet,
            encryptedWallet,
            isUnlocked: true,
            signingClient: null,
            isLoading: false,
          });

          // Fetch initial data (don't await - let it happen in background)
          get().refreshAll().catch(console.error);
        } catch (error) {
          const message = error instanceof Error ? error.message : 'Failed to import wallet';
          set({ error: message, isLoading: false });
          throw error;
        }
      },

      /**
       * Unlock wallet with password
       */
      unlockWallet: async (password: string) => {
        const { encryptedWallet } = get();

        if (!encryptedWallet) {
          throw new Error('No wallet found');
        }

        set({ isLoading: true, error: null });

        try {
          // Decrypt mnemonic
          const mnemonic = await decryptMnemonic(
            encryptedWallet.encryptedMnemonic,
            password
          );

          // Recreate wallet
          const wallet = await createWalletFromMnemonic(mnemonic);

          // Don't create signing client here - it will be created lazily when needed
          set({
            wallet,
            isUnlocked: true,
            signingClient: null,
            isLoading: false,
          });

          // Refresh data (don't await - let it happen in background)
          get().refreshAll().catch(console.error);
        } catch (error) {
          const message = error instanceof Error ? error.message : 'Invalid password';
          set({ error: message, isLoading: false });
          throw error;
        }
      },

      /**
       * Lock wallet (clear sensitive data from memory)
       */
      lockWallet: () => {
        set({
          wallet: null,
          isUnlocked: false,
          signingClient: null,
        });
      },

      /**
       * Clear wallet completely
       */
      clearWallet: () => {
        set({
          wallet: null,
          encryptedWallet: null,
          isUnlocked: false,
          balance: '0',
          delegations: [],
          rewards: null,
          signingClient: null,
          error: null,
        });
      },

      /**
       * Refresh balance
       */
      refreshBalance: async () => {
        const { wallet } = get();
        if (!wallet) return;

        try {
          const balance = await getBalance(wallet.cosmos.address);
          set({ balance });
        } catch (error) {
          console.error('Failed to fetch balance:', error);
        }
      },

      /**
       * Refresh delegations
       */
      refreshDelegations: async () => {
        const { wallet } = get();
        if (!wallet) return;

        try {
          const delegations = await getDelegations(wallet.cosmos.address);
          set({ delegations });
        } catch (error) {
          console.error('Failed to fetch delegations:', error);
        }
      },

      /**
       * Refresh staking rewards
       */
      refreshRewards: async () => {
        const { wallet } = get();
        if (!wallet) return;

        try {
          const rewards = await getStakingRewards(wallet.cosmos.address);
          set({ rewards });
        } catch (error) {
          console.error('Failed to fetch rewards:', error);
        }
      },

      /**
       * Refresh all data
       */
      refreshAll: async () => {
        const state = get();
        await Promise.all([
          state.refreshBalance(),
          state.refreshDelegations(),
          state.refreshRewards(),
        ]);
      },

      /**
       * Get or create signing client
       */
      getSigningClient: async () => {
        const { signingClient, wallet } = get();

        if (signingClient) {
          return signingClient;
        }

        if (!wallet) {
          throw new Error('Wallet not unlocked');
        }

        const client = await createSigningClient(wallet.cosmos.signer);
        set({ signingClient: client });
        return client;
      },
    }),
    {
      name: STORAGE_KEYS.WALLET,
      partialize: (state) => ({
        encryptedWallet: state.encryptedWallet,
      }),
    }
  )
);

// Selectors for computed values
export const selectTotalStaked = (state: WalletState): string => {
  return state.delegations
    .reduce((sum, d) => sum + BigInt(d.balance.amount), 0n)
    .toString();
};

export const selectTotalRewards = (state: WalletState): string => {
  if (!state.rewards?.total) return '0';
  const omniReward = state.rewards.total.find((r) => r.denom === DENOM);
  return omniReward?.amount || '0';
};

export const selectTotalBalance = (state: WalletState): string => {
  const available = BigInt(state.balance);
  const staked = BigInt(selectTotalStaked(state));
  return (available + staked).toString();
};
