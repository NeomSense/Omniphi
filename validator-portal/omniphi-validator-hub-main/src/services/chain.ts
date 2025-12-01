import axios from 'axios';

const REST_URL = import.meta.env.VITE_REST_URL || 'http://localhost:1317';
const RPC_URL = import.meta.env.VITE_RPC_URL || 'http://localhost:26657';

export interface ValidatorInfo {
  operatorAddress: string;
  consensusPubkey: string;
  jailed: boolean;
  status: string;
  tokens: string;
  delegatorShares: string;
  description: {
    moniker: string;
    identity: string;
    website: string;
    securityContact: string;
    details: string;
  };
  commission: {
    commissionRates: {
      rate: string;
      maxRate: string;
      maxChangeRate: string;
    };
    updateTime: string;
  };
}

export interface ValidatorStatus {
  blockHeight: string;
  syncInfo: {
    latestBlockHeight: string;
    latestBlockTime: string;
    catchingUp: boolean;
  };
  validatorInfo: {
    address: string;
    votingPower: string;
  };
}

export interface DelegationRewards {
  rewards: {
    validatorAddress: string;
    reward: { denom: string; amount: string }[];
  }[];
  total: { denom: string; amount: string }[];
}

class ChainService {
  async getValidator(operatorAddress: string): Promise<ValidatorInfo> {
    try {
      const response = await axios.get(
        `${REST_URL}/cosmos/staking/v1beta1/validators/${operatorAddress}`
      );
      return response.data.validator;
    } catch (error) {
      console.error('Failed to fetch validator:', error);
      throw error;
    }
  }

  async getAllValidators(): Promise<ValidatorInfo[]> {
    try {
      const response = await axios.get(
        `${REST_URL}/cosmos/staking/v1beta1/validators?pagination.limit=1000`
      );
      return response.data.validators || [];
    } catch (error) {
      console.error('Failed to fetch validators:', error);
      return [];
    }
  }

  async getValidatorStatus(rpcEndpoint?: string): Promise<ValidatorStatus> {
    const endpoint = rpcEndpoint || RPC_URL;
    try {
      const response = await axios.get(`${endpoint}/status`);
      return response.data.result;
    } catch (error) {
      console.error('Failed to fetch validator status:', error);
      throw error;
    }
  }

  async getDelegationRewards(delegatorAddress: string): Promise<DelegationRewards> {
    try {
      const response = await axios.get(
        `${REST_URL}/cosmos/distribution/v1beta1/delegators/${delegatorAddress}/rewards`
      );
      return response.data;
    } catch (error) {
      console.error('Failed to fetch rewards:', error);
      throw error;
    }
  }

  async getNetInfo(rpcEndpoint?: string): Promise<any> {
    const endpoint = rpcEndpoint || RPC_URL;
    try {
      const response = await axios.get(`${endpoint}/net_info`);
      return response.data.result;
    } catch (error) {
      console.error('Failed to fetch net info:', error);
      throw error;
    }
  }

  async broadcastTx(txBytes: string): Promise<any> {
    try {
      const response = await axios.post(`${REST_URL}/cosmos/tx/v1beta1/txs`, {
        tx_bytes: txBytes,
        mode: 'BROADCAST_MODE_SYNC',
      });
      return response.data;
    } catch (error) {
      console.error('Failed to broadcast transaction:', error);
      throw error;
    }
  }

  async simulateTx(txBytes: string): Promise<any> {
    try {
      const response = await axios.post(`${REST_URL}/cosmos/tx/v1beta1/simulate`, {
        tx_bytes: txBytes,
      });
      return response.data.gas_info;
    } catch (error) {
      console.error('Failed to simulate transaction:', error);
      throw error;
    }
  }
}

export const chainService = new ChainService();
