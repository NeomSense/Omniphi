import { useState } from 'react';
import { Button } from '@/components/ui/button';
import { Wallet, LogOut } from 'lucide-react';
import { useWallet } from '@/hooks/useWallet';
import { WalletModal } from './WalletConnect/WalletModal';

export const WalletConnect = () => {
  const [isModalOpen, setIsModalOpen] = useState(false);
  const { isConnected, address, disconnect } = useWallet();

  const formatAddress = (addr: string) => {
    return `${addr.slice(0, 10)}...${addr.slice(-6)}`;
  };

  return (
    <>
      {isConnected && address ? (
        <div className="flex items-center gap-2">
          <div className="px-3 py-1.5 rounded-lg bg-primary/10 border border-primary/20 text-sm font-mono">
            {formatAddress(address)}
          </div>
          <Button
            variant="outline"
            size="sm"
            onClick={disconnect}
            className="hover:bg-destructive/10 hover:text-destructive hover:border-destructive/20"
          >
            <LogOut className="mr-2 h-3 w-3" />
            Disconnect
          </Button>
        </div>
      ) : (
        <Button onClick={() => setIsModalOpen(true)} className="glow-primary">
          <Wallet className="mr-2 h-4 w-4" />
          Connect Wallet
        </Button>
      )}

      <WalletModal isOpen={isModalOpen} onClose={() => setIsModalOpen(false)} />
    </>
  );
};
