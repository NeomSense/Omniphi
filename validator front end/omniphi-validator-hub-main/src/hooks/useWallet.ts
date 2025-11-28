import { useState, useCallback } from 'react';
import { walletService, WalletType, WalletAccount } from '@/services/wallet';
import { useValidatorStore } from '@/store/validatorStore';

export const useWallet = () => {
  const [isConnecting, setIsConnecting] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const { connectWallet, disconnectWallet, walletAddress, isWalletConnected } = useValidatorStore();

  const connect = useCallback(async (type: WalletType) => {
    setIsConnecting(true);
    setError(null);

    try {
      const account: WalletAccount = await walletService.connect(type);
      connectWallet(account.address);
      return account;
    } catch (err: any) {
      const errorMessage = err.message || 'Failed to connect wallet';
      setError(errorMessage);
      throw err;
    } finally {
      setIsConnecting(false);
    }
  }, [connectWallet]);

  const disconnect = useCallback(() => {
    walletService.disconnect();
    disconnectWallet();
    setError(null);
  }, [disconnectWallet]);

  const signTransaction = useCallback(async (tx: any) => {
    try {
      return await walletService.signTransaction(tx);
    } catch (err: any) {
      const errorMessage = err.message || 'Failed to sign transaction';
      setError(errorMessage);
      throw err;
    }
  }, []);

  const getAccountInfo = useCallback(async (address: string) => {
    try {
      return await walletService.getAccountInfo(address);
    } catch (err: any) {
      const errorMessage = err.message || 'Failed to fetch account info';
      setError(errorMessage);
      throw err;
    }
  }, []);

  return {
    connect,
    disconnect,
    signTransaction,
    getAccountInfo,
    isConnecting,
    isConnected: isWalletConnected,
    address: walletAddress,
    error,
    walletType: walletService.getWalletType(),
  };
};
