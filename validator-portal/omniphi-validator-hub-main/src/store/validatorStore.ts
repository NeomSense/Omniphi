import { create } from 'zustand';
import { persist } from 'zustand/middleware';
import { ValidatorMode, ValidatorStatus, ValidatorConfig, ValidatorInfo } from '@/types/validator';

interface ValidatorStore {
  // Wallet
  walletAddress: string | null;
  isWalletConnected: boolean;
  connectWallet: (address: string) => void;
  disconnectWallet: () => void;

  // Validator Mode
  validatorMode: ValidatorMode;
  setValidatorMode: (mode: ValidatorMode) => void;

  // Validator Config
  validatorConfig: ValidatorConfig | null;
  setValidatorConfig: (config: ValidatorConfig) => void;

  // Validator Status
  validatorStatus: ValidatorStatus;
  setValidatorStatus: (status: ValidatorStatus) => void;

  // Validator Info
  validatorInfo: ValidatorInfo | null;
  setValidatorInfo: (info: ValidatorInfo) => void;

  // Setup Request ID (from backend)
  setupRequestId: string | null;
  setSetupRequestId: (id: string) => void;

  // Wizard
  currentStep: number;
  setCurrentStep: (step: number) => void;

  // Reset
  reset: () => void;
}

const initialState = {
  walletAddress: null,
  isWalletConnected: false,
  validatorMode: null,
  validatorConfig: null,
  validatorStatus: 'not_started' as ValidatorStatus,
  validatorInfo: null,
  setupRequestId: null,
  currentStep: 0,
};

export const useValidatorStore = create<ValidatorStore>()(
  persist(
    (set) => ({
      ...initialState,

      connectWallet: (address) =>
        set({ walletAddress: address, isWalletConnected: true }),

      disconnectWallet: () =>
        set({ walletAddress: null, isWalletConnected: false }),

      setValidatorMode: (mode) => set({ validatorMode: mode }),

      setValidatorConfig: (config) => set({ validatorConfig: config }),

      setValidatorStatus: (status) => set({ validatorStatus: status }),

      setValidatorInfo: (info) => set({ validatorInfo: info }),

      setSetupRequestId: (id) => set({ setupRequestId: id }),

      setCurrentStep: (step) => set({ currentStep: step }),

      reset: () => set(initialState),
    }),
    {
      name: 'validator-storage',
    }
  )
);
