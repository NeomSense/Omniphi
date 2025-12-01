export type ValidatorMode = 'cloud' | 'local' | null;

export type ValidatorStatus = 
  | 'not_started'
  | 'configuring'
  | 'provisioning'
  | 'awaiting_signature'
  | 'active'
  | 'error';

export interface ValidatorConfig {
  moniker: string;
  website?: string;
  securityContact?: string;
  details?: string;
  commission: {
    rate: string;
    maxRate: string;
    maxChangeRate: string;
  };
  minSelfDelegation: string;
}

export interface ValidatorInfo {
  pubkey: string;
  address: string;
  status: ValidatorStatus;
  config: ValidatorConfig;
  mode: ValidatorMode;
  nodeEndpoint?: string;
  createdAt: Date;
}

export interface WizardStep {
  id: number;
  title: string;
  description: string;
  completed: boolean;
}
