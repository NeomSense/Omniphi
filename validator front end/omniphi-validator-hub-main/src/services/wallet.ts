import { toast } from 'sonner';

export type WalletType = 'keplr' | 'leap' | 'metamask-snap' | 'mock';

export interface WalletAccount {
  address: string;
  pubkey: string;
  algo: string;
}

export interface ChainInfo {
  chainId: string;
  chainName: string;
  rpc: string;
  rest: string;
  bech32Prefix: string;
  currencies: {
    coinDenom: string;
    coinMinimalDenom: string;
    coinDecimals: number;
  }[];
  feeCurrencies: {
    coinDenom: string;
    coinMinimalDenom: string;
    coinDecimals: number;
    gasPriceStep: {
      low: number;
      average: number;
      high: number;
    };
  }[];
}

const OMNIPHI_CHAIN_INFO: ChainInfo = {
  chainId: import.meta.env.VITE_CHAIN_ID || 'omniphi-1',
  chainName: import.meta.env.VITE_CHAIN_NAME || 'Omniphi',
  rpc: import.meta.env.VITE_RPC_URL || 'http://localhost:26657',
  rest: import.meta.env.VITE_REST_URL || 'http://localhost:1317',
  bech32Prefix: 'omniphi',
  currencies: [
    {
      coinDenom: 'OMNI',
      coinMinimalDenom: 'uomni',
      coinDecimals: 6,
    },
  ],
  feeCurrencies: [
    {
      coinDenom: 'OMNI',
      coinMinimalDenom: 'uomni',
      coinDecimals: 6,
      gasPriceStep: {
        low: 0.01,
        average: 0.025,
        high: 0.04,
      },
    },
  ],
};

class WalletService {
  private walletType: WalletType | null = null;

  async connectKeplr(): Promise<WalletAccount> {
    if (!window.keplr) {
      throw new Error('Keplr extension not found. Please install Keplr.');
    }

    try {
      await window.keplr.experimentalSuggestChain(OMNIPHI_CHAIN_INFO);
      await window.keplr.enable(OMNIPHI_CHAIN_INFO.chainId);

      const offlineSigner = window.keplr.getOfflineSigner(OMNIPHI_CHAIN_INFO.chainId);
      const accounts = await offlineSigner.getAccounts();

      if (!accounts || accounts.length === 0) {
        throw new Error('No accounts found');
      }

      const account = accounts[0];
      const pubkeyBytes = account.pubkey;
      const pubkey = Buffer.from(pubkeyBytes).toString('base64');

      this.walletType = 'keplr';
      toast.success('Keplr wallet connected');

      return {
        address: account.address,
        pubkey,
        algo: account.algo,
      };
    } catch (error) {
      console.error('Keplr connection error:', error);
      throw error;
    }
  }

  async connectLeap(): Promise<WalletAccount> {
    if (!window.leap) {
      throw new Error('Leap extension not found. Please install Leap.');
    }

    try {
      await window.leap.experimentalSuggestChain(OMNIPHI_CHAIN_INFO);
      await window.leap.enable(OMNIPHI_CHAIN_INFO.chainId);

      const offlineSigner = window.leap.getOfflineSigner(OMNIPHI_CHAIN_INFO.chainId);
      const accounts = await offlineSigner.getAccounts();

      if (!accounts || accounts.length === 0) {
        throw new Error('No accounts found');
      }

      const account = accounts[0];
      const pubkeyBytes = account.pubkey;
      const pubkey = Buffer.from(pubkeyBytes).toString('base64');

      this.walletType = 'leap';
      toast.success('Leap wallet connected');

      return {
        address: account.address,
        pubkey,
        algo: account.algo,
      };
    } catch (error) {
      console.error('Leap connection error:', error);
      throw error;
    }
  }

  async connectMetaMaskSnap(): Promise<WalletAccount> {
    // MetaMask Snap integration (placeholder for now)
    toast.info('MetaMask Snap support coming soon');
    throw new Error('MetaMask Snap not yet implemented');
  }

  async connectMock(): Promise<WalletAccount> {
    // Mock wallet for testing
    this.walletType = 'mock';
    toast.success('Mock wallet connected');
    
    return {
      address: 'omniphi1mock7w8fk5j6l3h2g9d4f5s6a7mock8validator',
      pubkey: 'A1234567890abcdefghijklmnopqrstuvwxyzABCDEF=',
      algo: 'secp256k1',
    };
  }

  async connect(type: WalletType): Promise<WalletAccount> {
    switch (type) {
      case 'mock':
        return this.connectMock();
      case 'keplr':
        return this.connectKeplr();
      case 'leap':
        return this.connectLeap();
      case 'metamask-snap':
        return this.connectMetaMaskSnap();
      default:
        throw new Error('Unsupported wallet type');
    }
  }

  disconnect() {
    this.walletType = null;
    toast.info('Wallet disconnected');
  }

  getWalletType(): WalletType | null {
    return this.walletType;
  }

  async signTransaction(tx: any): Promise<any> {
    if (!this.walletType) {
      throw new Error('No wallet connected');
    }

    try {
      if (this.walletType === 'mock') {
        // Mock signing
        toast.success('Transaction signed (mock)');
        return {
          signed: true,
          signature: {
            pub_key: {
              type: 'tendermint/PubKeySecp256k1',
              value: 'A1234567890abcdefghijklmnopqrstuvwxyzABCDEF='
            },
            signature: 'mockSignature1234567890abcdefghijklmnopqrstuvwxyz=='
          },
          tx
        };
      } else if (this.walletType === 'keplr' && window.keplr) {
        const offlineSigner = window.keplr.getOfflineSigner(OMNIPHI_CHAIN_INFO.chainId);
        // Sign transaction logic here
        return { signed: true, tx };
      } else if (this.walletType === 'leap' && window.leap) {
        const offlineSigner = window.leap.getOfflineSigner(OMNIPHI_CHAIN_INFO.chainId);
        // Sign transaction logic here
        return { signed: true, tx };
      }
      throw new Error('Wallet not available');
    } catch (error) {
      console.error('Transaction signing error:', error);
      throw error;
    }
  }

  async getAccountInfo(address: string) {
    // Return mock data for mock wallet
    if (this.walletType === 'mock') {
      return {
        accountNumber: '12345',
        sequence: '0',
      };
    }

    // Fetch account number and sequence from LCD
    const response = await fetch(
      `${OMNIPHI_CHAIN_INFO.rest}/cosmos/auth/v1beta1/accounts/${address}`
    );
    const data = await response.json();
    return {
      accountNumber: data.account.account_number,
      sequence: data.account.sequence,
    };
  }
}

// Global singleton
export const walletService = new WalletService();

// Type declarations for window
declare global {
  interface Window {
    keplr?: any;
    leap?: any;
  }
}
