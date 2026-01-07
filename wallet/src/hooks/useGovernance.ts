/**
 * Governance Hooks
 * React Query hooks for governance operations
 */

import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query';
import { DeliverTxResponse } from '@cosmjs/stargate';
import { MsgVote } from 'cosmjs-types/cosmos/gov/v1beta1/tx';
import toast from 'react-hot-toast';
import {
  getProposals,
  getProposal,
  getVote,
  getProposalTally,
  Proposal,
  Vote,
} from '@/lib/api';
import { useWalletStore } from '@/stores/wallet';
import { CHAIN_ID, DENOM, VOTE_OPTIONS, VOTE_OPTION_LABELS } from '@/lib/constants';

// Query keys
const QUERY_KEYS = {
  proposals: ['proposals'] as const,
  proposal: (id: string) => ['proposal', id] as const,
  vote: (proposalId: string, voter: string) => ['vote', proposalId, voter] as const,
  tally: (proposalId: string) => ['tally', proposalId] as const,
};

/**
 * Fetch all proposals
 */
export function useProposals(status?: string) {
  return useQuery({
    queryKey: [...QUERY_KEYS.proposals, status],
    queryFn: () => getProposals(status),
    staleTime: 30_000, // 30 seconds
  });
}

/**
 * Fetch single proposal
 */
export function useProposal(proposalId: string) {
  return useQuery({
    queryKey: QUERY_KEYS.proposal(proposalId),
    queryFn: () => getProposal(proposalId),
    enabled: !!proposalId,
    staleTime: 30_000,
  });
}

/**
 * Fetch user's vote on a proposal
 */
export function useVote(proposalId: string) {
  const wallet = useWalletStore((state) => state.wallet);

  return useQuery({
    queryKey: QUERY_KEYS.vote(proposalId, wallet?.cosmos.address || ''),
    queryFn: () => getVote(proposalId, wallet!.cosmos.address),
    enabled: !!proposalId && !!wallet,
    staleTime: 30_000,
  });
}

/**
 * Fetch proposal tally
 */
export function useProposalTally(proposalId: string) {
  return useQuery({
    queryKey: QUERY_KEYS.tally(proposalId),
    queryFn: () => getProposalTally(proposalId),
    enabled: !!proposalId,
    staleTime: 10_000, // 10 seconds for live voting
  });
}

/**
 * Vote on a proposal
 */
export function useVoteOnProposal() {
  const queryClient = useQueryClient();
  const getSigningClient = useWalletStore((state) => state.getSigningClient);
  const wallet = useWalletStore((state) => state.wallet);

  return useMutation({
    mutationFn: async ({
      proposalId,
      option,
    }: {
      proposalId: string;
      option: number;
    }): Promise<DeliverTxResponse> => {
      if (!wallet) {
        throw new Error('Wallet not connected');
      }

      const client = await getSigningClient();
      const voterAddress = wallet.cosmos.address;

      // Build vote message
      const voteMsg = {
        typeUrl: '/cosmos.gov.v1beta1.MsgVote',
        value: MsgVote.fromPartial({
          proposalId: BigInt(proposalId),
          voter: voterAddress,
          option: option,
        }),
      };

      // Calculate fee
      const fee = {
        amount: [{ denom: DENOM, amount: '50000' }],
        gas: '200000',
      };

      // Broadcast transaction
      const result = await client.signAndBroadcast(
        voterAddress,
        [voteMsg],
        fee,
        `Vote ${VOTE_OPTION_LABELS[option]} on proposal #${proposalId}`
      );

      if (result.code !== 0) {
        throw new Error(`Transaction failed: ${result.rawLog}`);
      }

      return result;
    },
    onSuccess: (_, variables) => {
      // Invalidate related queries
      queryClient.invalidateQueries({
        queryKey: QUERY_KEYS.vote(variables.proposalId, wallet?.cosmos.address || ''),
      });
      queryClient.invalidateQueries({
        queryKey: QUERY_KEYS.tally(variables.proposalId),
      });
      queryClient.invalidateQueries({
        queryKey: QUERY_KEYS.proposal(variables.proposalId),
      });

      toast.success(
        `Voted ${VOTE_OPTION_LABELS[variables.option]} on proposal #${variables.proposalId}`
      );
    },
    onError: (error) => {
      const message = error instanceof Error ? error.message : 'Vote failed';
      toast.error(message);
    },
  });
}

/**
 * Get proposal status badge color
 */
export function getProposalStatusColor(status: string): string {
  switch (status) {
    case 'PROPOSAL_STATUS_VOTING_PERIOD':
      return 'badge-info';
    case 'PROPOSAL_STATUS_PASSED':
      return 'badge-success';
    case 'PROPOSAL_STATUS_REJECTED':
      return 'badge-error';
    case 'PROPOSAL_STATUS_DEPOSIT_PERIOD':
      return 'badge-warning';
    default:
      return 'badge';
  }
}

/**
 * Get proposal status label
 */
export function getProposalStatusLabel(status: string): string {
  switch (status) {
    case 'PROPOSAL_STATUS_UNSPECIFIED':
      return 'Unspecified';
    case 'PROPOSAL_STATUS_DEPOSIT_PERIOD':
      return 'Deposit Period';
    case 'PROPOSAL_STATUS_VOTING_PERIOD':
      return 'Voting';
    case 'PROPOSAL_STATUS_PASSED':
      return 'Passed';
    case 'PROPOSAL_STATUS_REJECTED':
      return 'Rejected';
    case 'PROPOSAL_STATUS_FAILED':
      return 'Failed';
    default:
      return status;
  }
}

/**
 * Calculate vote percentages from tally
 */
export function calculateVotePercentages(tally: {
  yes_count: string;
  abstain_count: string;
  no_count: string;
  no_with_veto_count: string;
}): {
  yes: number;
  abstain: number;
  no: number;
  noWithVeto: number;
  total: bigint;
} {
  const yes = BigInt(tally.yes_count || '0');
  const abstain = BigInt(tally.abstain_count || '0');
  const no = BigInt(tally.no_count || '0');
  const noWithVeto = BigInt(tally.no_with_veto_count || '0');
  const total = yes + abstain + no + noWithVeto;

  if (total === 0n) {
    return { yes: 0, abstain: 0, no: 0, noWithVeto: 0, total: 0n };
  }

  const toPercent = (value: bigint) =>
    Number((value * 10000n) / total) / 100;

  return {
    yes: toPercent(yes),
    abstain: toPercent(abstain),
    no: toPercent(no),
    noWithVeto: toPercent(noWithVeto),
    total,
  };
}

/**
 * Get user's vote option from vote response
 */
export function getUserVoteOption(vote: Vote | null): number | null {
  if (!vote || !vote.options || vote.options.length === 0) {
    return null;
  }

  // Find the option with highest weight (for simple votes, this is the only one)
  const primaryOption = vote.options.reduce((max, opt) =>
    parseFloat(opt.weight) > parseFloat(max.weight) ? opt : max
  );

  switch (primaryOption.option) {
    case 'VOTE_OPTION_YES':
      return VOTE_OPTIONS.YES;
    case 'VOTE_OPTION_ABSTAIN':
      return VOTE_OPTIONS.ABSTAIN;
    case 'VOTE_OPTION_NO':
      return VOTE_OPTIONS.NO;
    case 'VOTE_OPTION_NO_WITH_VETO':
      return VOTE_OPTIONS.NO_WITH_VETO;
    default:
      return null;
  }
}

/**
 * Format time remaining
 */
export function formatTimeRemaining(endTime: string): string {
  const end = new Date(endTime);
  const now = new Date();
  const diff = end.getTime() - now.getTime();

  if (diff <= 0) {
    return 'Ended';
  }

  const days = Math.floor(diff / (1000 * 60 * 60 * 24));
  const hours = Math.floor((diff % (1000 * 60 * 60 * 24)) / (1000 * 60 * 60));
  const minutes = Math.floor((diff % (1000 * 60 * 60)) / (1000 * 60));

  if (days > 0) {
    return `${days}d ${hours}h remaining`;
  }
  if (hours > 0) {
    return `${hours}h ${minutes}m remaining`;
  }
  return `${minutes}m remaining`;
}
