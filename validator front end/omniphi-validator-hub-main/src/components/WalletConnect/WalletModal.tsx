import { Dialog, DialogContent, DialogHeader, DialogTitle, DialogDescription } from '@/components/ui/dialog';
import { Button } from '@/components/ui/button';
import { Wallet } from 'lucide-react';
import { useWallet } from '@/hooks/useWallet';
import { WalletType } from '@/services/wallet';

interface WalletModalProps {
  isOpen: boolean;
  onClose: () => void;
}

export const WalletModal = ({ isOpen, onClose }: WalletModalProps) => {
  const { connect, isConnecting } = useWallet();

  const handleConnect = async (type: WalletType) => {
    try {
      await connect(type);
      onClose();
    } catch (error) {
      console.error('Wallet connection failed:', error);
    }
  };

  const wallets = [
    {
      type: 'mock' as WalletType,
      name: 'Mock Wallet',
      description: 'Testing wallet with pre-filled data',
      icon: '🧪',
    },
    {
      type: 'keplr' as WalletType,
      name: 'Keplr',
      description: 'Connect with Keplr wallet',
      icon: '🔮',
    },
    {
      type: 'leap' as WalletType,
      name: 'Leap',
      description: 'Connect with Leap wallet',
      icon: '🐸',
    },
    {
      type: 'metamask-snap' as WalletType,
      name: 'MetaMask Snap',
      description: 'Connect with MetaMask (Coming Soon)',
      icon: '🦊',
      disabled: true,
    },
  ];

  return (
    <Dialog open={isOpen} onOpenChange={onClose}>
      <DialogContent className="sm:max-w-md">
        <DialogHeader>
          <DialogTitle className="flex items-center gap-2">
            <Wallet className="h-5 w-5" />
            Connect Wallet
          </DialogTitle>
          <DialogDescription>
            Choose your preferred wallet to connect to Omniphi
          </DialogDescription>
        </DialogHeader>

        <div className="space-y-3 py-4">
          {wallets.map((wallet) => (
            <Button
              key={wallet.type}
              variant="outline"
              className="w-full justify-start h-auto p-4 hover:bg-accent/50 transition-colors"
              onClick={() => handleConnect(wallet.type)}
              disabled={isConnecting || wallet.disabled}
            >
              <span className="text-2xl mr-3">{wallet.icon}</span>
              <div className="text-left flex-1">
                <div className="font-semibold">{wallet.name}</div>
                <div className="text-xs text-muted-foreground">{wallet.description}</div>
              </div>
            </Button>
          ))}
        </div>

        <div className="text-xs text-muted-foreground text-center pt-2 border-t">
          New to Cosmos wallets?{' '}
          <a
            href="https://keplr.app"
            target="_blank"
            rel="noopener noreferrer"
            className="text-primary hover:underline"
          >
            Learn more
          </a>
        </div>
      </DialogContent>
    </Dialog>
  );
};
