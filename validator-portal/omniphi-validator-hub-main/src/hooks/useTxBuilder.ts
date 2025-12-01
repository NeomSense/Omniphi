import { useCallback } from 'react';
import { ValidatorConfig } from '@/types/validator';

export interface MsgCreateValidator {
  description: {
    moniker: string;
    identity: string;
    website: string;
    security_contact: string;
    details: string;
  };
  commission: {
    rate: string;
    max_rate: string;
    max_change_rate: string;
  };
  min_self_delegation: string;
  delegator_address: string;
  validator_address: string;
  pubkey: {
    '@type': string;
    key: string;
  };
  value: {
    denom: string;
    amount: string;
  };
}

export interface Transaction {
  body: {
    messages: any[];
    memo: string;
  };
  auth_info: {
    signer_infos: any[];
    fee: {
      amount: { denom: string; amount: string }[];
      gas_limit: string;
    };
  };
  signatures: string[];
}

export const useTxBuilder = () => {
  const buildMsgCreateValidator = useCallback(
    (
      config: ValidatorConfig,
      consensusPubkey: string,
      delegatorAddress: string,
      validatorAddress: string,
      selfDelegationAmount: string = '1000000' // 1 OMNI in uomni
    ): MsgCreateValidator => {
      return {
        description: {
          moniker: config.moniker,
          identity: '',
          website: config.website || '',
          security_contact: config.securityContact || '',
          details: config.details || '',
        },
        commission: {
          rate: config.commission.rate,
          max_rate: config.commission.maxRate,
          max_change_rate: config.commission.maxChangeRate,
        },
        min_self_delegation: config.minSelfDelegation,
        delegator_address: delegatorAddress,
        validator_address: validatorAddress,
        pubkey: {
          '@type': '/cosmos.crypto.ed25519.PubKey',
          key: consensusPubkey,
        },
        value: {
          denom: 'uomni',
          amount: selfDelegationAmount,
        },
      };
    },
    []
  );

  const buildMsgEditValidator = useCallback(
    (
      validatorAddress: string,
      updates: Partial<ValidatorConfig>
    ) => {
      return {
        '@type': '/cosmos.staking.v1beta1.MsgEditValidator',
        description: {
          moniker: updates.moniker || '[do-not-modify]',
          identity: '[do-not-modify]',
          website: updates.website || '[do-not-modify]',
          security_contact: updates.securityContact || '[do-not-modify]',
          details: updates.details || '[do-not-modify]',
        },
        validator_address: validatorAddress,
        commission_rate: updates.commission?.rate || null,
        min_self_delegation: updates.minSelfDelegation || null,
      };
    },
    []
  );

  const buildMsgDelegate = useCallback(
    (
      delegatorAddress: string,
      validatorAddress: string,
      amount: string,
      denom: string = 'uomni'
    ) => {
      return {
        '@type': '/cosmos.staking.v1beta1.MsgDelegate',
        delegator_address: delegatorAddress,
        validator_address: validatorAddress,
        amount: {
          denom,
          amount,
        },
      };
    },
    []
  );

  const buildMsgWithdrawRewards = useCallback(
    (delegatorAddress: string, validatorAddress: string) => {
      return {
        '@type': '/cosmos.distribution.v1beta1.MsgWithdrawDelegatorReward',
        delegator_address: delegatorAddress,
        validator_address: validatorAddress,
      };
    },
    []
  );

  const buildTransaction = useCallback(
    (
      messages: any[],
      memo: string = '',
      gasLimit: string = '200000',
      feeAmount: string = '5000'
    ): Transaction => {
      return {
        body: {
          messages,
          memo,
        },
        auth_info: {
          signer_infos: [],
          fee: {
            amount: [
              {
                denom: 'uomni',
                amount: feeAmount,
              },
            ],
            gas_limit: gasLimit,
          },
        },
        signatures: [],
      };
    },
    []
  );

  const estimateGas = useCallback((messageCount: number): string => {
    // Simple gas estimation based on message count
    const baseGas = 100000;
    const perMessageGas = 50000;
    return (baseGas + messageCount * perMessageGas).toString();
  }, []);

  return {
    buildMsgCreateValidator,
    buildMsgEditValidator,
    buildMsgDelegate,
    buildMsgWithdrawRewards,
    buildTransaction,
    estimateGas,
  };
};
