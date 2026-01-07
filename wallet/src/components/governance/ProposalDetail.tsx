import React, { useState } from 'react';
import { useNavigate } from 'react-router-dom';
import {
  ArrowLeftIcon,
  ClockIcon,
  CheckCircleIcon,
  XCircleIcon,
  HandThumbUpIcon,
  HandThumbDownIcon,
  MinusCircleIcon,
  ExclamationTriangleIcon,
} from '@heroicons/react/24/outline';
import { Card, CardHeader, CardTitle, CardContent, Button, Badge, Modal } from '@/components/ui';
import {
  useProposal,
  useProposalTally,
  useVote,
  useVoteOnProposal,
  getProposalStatusLabel,
  formatTimeRemaining,
  calculateVotePercentages,
  getUserVoteOption,
} from '@/hooks/useGovernance';
import { useWalletStore } from '@/stores/wallet';
import { formatAmount, formatAddress } from '@/lib/wallet';
import { VOTE_OPTIONS, VOTE_OPTION_LABELS, DENOM } from '@/lib/constants';

const VoteButton: React.FC<{
  option: number;
  label: string;
  icon: React.ReactNode;
  selected: boolean;
  disabled: boolean;
  onClick: () => void;
  variant: 'yes' | 'no' | 'abstain' | 'veto';
}> = ({ option, label, icon, selected, disabled, onClick, variant }) => {
  const variants = {
    yes: selected
      ? 'bg-green-600 border-green-500 text-white'
      : 'border-green-500/30 text-green-400 hover:bg-green-500/10',
    no: selected
      ? 'bg-red-600 border-red-500 text-white'
      : 'border-red-500/30 text-red-400 hover:bg-red-500/10',
    abstain: selected
      ? 'bg-dark-600 border-dark-500 text-white'
      : 'border-dark-500/30 text-dark-400 hover:bg-dark-500/10',
    veto: selected
      ? 'bg-red-800 border-red-700 text-white'
      : 'border-red-700/30 text-red-500 hover:bg-red-700/10',
  };

  return (
    <button
      onClick={onClick}
      disabled={disabled}
      className={`flex-1 p-4 rounded-xl border-2 transition-all duration-200 disabled:opacity-50 disabled:cursor-not-allowed ${variants[variant]}`}
    >
      <div className="flex flex-col items-center gap-2">
        {icon}
        <span className="font-medium">{label}</span>
      </div>
    </button>
  );
};

interface ProposalDetailProps {
  proposalId: string;
}

export const ProposalDetail: React.FC<ProposalDetailProps> = ({ proposalId }) => {
  const navigate = useNavigate();
  const [showVoteModal, setShowVoteModal] = useState(false);
  const [selectedVote, setSelectedVote] = useState<number | null>(null);

  const wallet = useWalletStore((state) => state.wallet);
  const isUnlocked = useWalletStore((state) => state.isUnlocked);

  const { data: proposal, isLoading: proposalLoading } = useProposal(proposalId);
  const { data: tally } = useProposalTally(proposalId);
  const { data: userVote } = useVote(proposalId);
  const voteMutation = useVoteOnProposal();

  if (proposalLoading) {
    return (
      <div className="max-w-4xl mx-auto">
        <Card className="animate-pulse">
          <div className="h-8 bg-dark-700 rounded w-1/4 mb-4" />
          <div className="h-12 bg-dark-700 rounded w-3/4 mb-4" />
          <div className="h-32 bg-dark-700 rounded mb-4" />
          <div className="h-24 bg-dark-700 rounded" />
        </Card>
      </div>
    );
  }

  if (!proposal) {
    return (
      <div className="max-w-4xl mx-auto">
        <Card>
          <div className="text-center py-12">
            <ExclamationTriangleIcon className="h-16 w-16 text-dark-500 mx-auto mb-4" />
            <p className="text-dark-400 text-lg">Proposal not found</p>
            <Button variant="secondary" className="mt-4" onClick={() => navigate('/governance')}>
              Back to Proposals
            </Button>
          </div>
        </Card>
      </div>
    );
  }

  const isVoting = proposal.status === 'PROPOSAL_STATUS_VOTING_PERIOD';
  const isPassed = proposal.status === 'PROPOSAL_STATUS_PASSED';
  const isRejected = proposal.status === 'PROPOSAL_STATUS_REJECTED';
  const currentTally = tally || proposal.final_tally_result;
  const percentages = calculateVotePercentages(currentTally);
  const userVoteOption = getUserVoteOption(userVote || null);
  const hasVoted = userVoteOption !== null;

  const handleVote = async () => {
    if (!selectedVote || !proposalId) return;

    try {
      await voteMutation.mutateAsync({
        proposalId,
        option: selectedVote,
      });
      setShowVoteModal(false);
      setSelectedVote(null);
    } catch (error) {
      // Error handled by mutation
    }
  };

  const getStatusBadge = () => {
    if (isVoting) return <Badge variant="info">{getProposalStatusLabel(proposal.status)}</Badge>;
    if (isPassed) return <Badge variant="success">{getProposalStatusLabel(proposal.status)}</Badge>;
    if (isRejected) return <Badge variant="error">{getProposalStatusLabel(proposal.status)}</Badge>;
    return <Badge variant="warning">{getProposalStatusLabel(proposal.status)}</Badge>;
  };

  return (
    <div className="max-w-4xl mx-auto space-y-6">
      {/* Header */}
      <div className="flex items-center gap-4">
        <Button variant="ghost" size="sm" onClick={() => navigate('/governance')}>
          <ArrowLeftIcon className="h-4 w-4" />
          Back
        </Button>
      </div>

      {/* Proposal Info */}
      <Card>
        <CardHeader>
          <div className="flex items-center gap-3 mb-2">
            <span className="text-dark-400">#{proposal.id}</span>
            {getStatusBadge()}
            {hasVoted && (
              <Badge variant="info">
                Voted: {VOTE_OPTION_LABELS[userVoteOption!]}
              </Badge>
            )}
          </div>
          <CardTitle className="text-2xl">{proposal.title || 'Untitled Proposal'}</CardTitle>
        </CardHeader>

        <CardContent>
          {/* Time info */}
          {isVoting && (
            <div className="flex items-center gap-2 text-omniphi-400 mb-6">
              <ClockIcon className="h-5 w-5" />
              <span className="font-medium">{formatTimeRemaining(proposal.voting_end_time)}</span>
            </div>
          )}

          {/* Description */}
          <div className="prose prose-invert max-w-none mb-6">
            <p className="text-dark-300 whitespace-pre-wrap">
              {proposal.summary || 'No description available'}
            </p>
          </div>

          {/* Proposer */}
          <div className="text-sm text-dark-400">
            <span>Proposed by: </span>
            <code className="text-dark-300">{formatAddress(proposal.proposer)}</code>
          </div>
        </CardContent>
      </Card>

      {/* Voting Results */}
      <Card>
        <CardHeader>
          <CardTitle>Voting Results</CardTitle>
        </CardHeader>
        <CardContent>
          {/* Progress bars */}
          <div className="space-y-4">
            {/* Yes */}
            <div>
              <div className="flex justify-between text-sm mb-1">
                <span className="text-green-400 flex items-center gap-2">
                  <HandThumbUpIcon className="h-4 w-4" />
                  Yes
                </span>
                <span className="text-dark-300">{percentages.yes.toFixed(2)}%</span>
              </div>
              <div className="h-3 bg-dark-700 rounded-full overflow-hidden">
                <div
                  className="h-full bg-green-500 rounded-full transition-all duration-500"
                  style={{ width: `${percentages.yes}%` }}
                />
              </div>
            </div>

            {/* No */}
            <div>
              <div className="flex justify-between text-sm mb-1">
                <span className="text-red-400 flex items-center gap-2">
                  <HandThumbDownIcon className="h-4 w-4" />
                  No
                </span>
                <span className="text-dark-300">{percentages.no.toFixed(2)}%</span>
              </div>
              <div className="h-3 bg-dark-700 rounded-full overflow-hidden">
                <div
                  className="h-full bg-red-500 rounded-full transition-all duration-500"
                  style={{ width: `${percentages.no}%` }}
                />
              </div>
            </div>

            {/* Abstain */}
            <div>
              <div className="flex justify-between text-sm mb-1">
                <span className="text-dark-400 flex items-center gap-2">
                  <MinusCircleIcon className="h-4 w-4" />
                  Abstain
                </span>
                <span className="text-dark-300">{percentages.abstain.toFixed(2)}%</span>
              </div>
              <div className="h-3 bg-dark-700 rounded-full overflow-hidden">
                <div
                  className="h-full bg-dark-500 rounded-full transition-all duration-500"
                  style={{ width: `${percentages.abstain}%` }}
                />
              </div>
            </div>

            {/* No with Veto */}
            <div>
              <div className="flex justify-between text-sm mb-1">
                <span className="text-red-600 flex items-center gap-2">
                  <ExclamationTriangleIcon className="h-4 w-4" />
                  No with Veto
                </span>
                <span className="text-dark-300">{percentages.noWithVeto.toFixed(2)}%</span>
              </div>
              <div className="h-3 bg-dark-700 rounded-full overflow-hidden">
                <div
                  className="h-full bg-red-700 rounded-full transition-all duration-500"
                  style={{ width: `${percentages.noWithVeto}%` }}
                />
              </div>
            </div>
          </div>

          {/* Quorum info */}
          <div className="mt-6 pt-6 border-t border-dark-700 text-sm text-dark-400">
            <p>Quorum: 25% of staked tokens must vote</p>
            <p>Pass threshold: &gt;50% Yes votes (excluding Abstain)</p>
            <p>Veto threshold: &lt;33.4% No with Veto votes</p>
          </div>
        </CardContent>
      </Card>

      {/* Vote Action */}
      {isVoting && isUnlocked && (
        <Card>
          <CardHeader>
            <CardTitle>Cast Your Vote</CardTitle>
          </CardHeader>
          <CardContent>
            {hasVoted ? (
              <div className="text-center py-4">
                <CheckCircleIcon className="h-12 w-12 text-green-400 mx-auto mb-2" />
                <p className="text-dark-300">
                  You voted <strong>{VOTE_OPTION_LABELS[userVoteOption!]}</strong> on this proposal
                </p>
                <p className="text-dark-500 text-sm mt-1">
                  You can change your vote until voting ends
                </p>
                <Button
                  variant="secondary"
                  className="mt-4"
                  onClick={() => setShowVoteModal(true)}
                >
                  Change Vote
                </Button>
              </div>
            ) : (
              <div className="flex gap-4">
                <VoteButton
                  option={VOTE_OPTIONS.YES}
                  label="Yes"
                  icon={<HandThumbUpIcon className="h-6 w-6" />}
                  selected={selectedVote === VOTE_OPTIONS.YES}
                  disabled={voteMutation.isPending}
                  onClick={() => {
                    setSelectedVote(VOTE_OPTIONS.YES);
                    setShowVoteModal(true);
                  }}
                  variant="yes"
                />
                <VoteButton
                  option={VOTE_OPTIONS.NO}
                  label="No"
                  icon={<HandThumbDownIcon className="h-6 w-6" />}
                  selected={selectedVote === VOTE_OPTIONS.NO}
                  disabled={voteMutation.isPending}
                  onClick={() => {
                    setSelectedVote(VOTE_OPTIONS.NO);
                    setShowVoteModal(true);
                  }}
                  variant="no"
                />
                <VoteButton
                  option={VOTE_OPTIONS.ABSTAIN}
                  label="Abstain"
                  icon={<MinusCircleIcon className="h-6 w-6" />}
                  selected={selectedVote === VOTE_OPTIONS.ABSTAIN}
                  disabled={voteMutation.isPending}
                  onClick={() => {
                    setSelectedVote(VOTE_OPTIONS.ABSTAIN);
                    setShowVoteModal(true);
                  }}
                  variant="abstain"
                />
                <VoteButton
                  option={VOTE_OPTIONS.NO_WITH_VETO}
                  label="Veto"
                  icon={<ExclamationTriangleIcon className="h-6 w-6" />}
                  selected={selectedVote === VOTE_OPTIONS.NO_WITH_VETO}
                  disabled={voteMutation.isPending}
                  onClick={() => {
                    setSelectedVote(VOTE_OPTIONS.NO_WITH_VETO);
                    setShowVoteModal(true);
                  }}
                  variant="veto"
                />
              </div>
            )}
          </CardContent>
        </Card>
      )}

      {/* Not connected message */}
      {isVoting && !isUnlocked && (
        <Card>
          <div className="text-center py-8">
            <p className="text-dark-400 mb-4">Connect your wallet to vote on this proposal</p>
            <Button onClick={() => navigate('/')}>Connect Wallet</Button>
          </div>
        </Card>
      )}

      {/* Vote Confirmation Modal */}
      <Modal
        isOpen={showVoteModal}
        onClose={() => {
          setShowVoteModal(false);
          setSelectedVote(null);
        }}
        title="Confirm Vote"
        size="sm"
      >
        <div className="space-y-4">
          <p className="text-dark-300">
            You are about to vote <strong>{selectedVote ? VOTE_OPTION_LABELS[selectedVote] : ''}</strong> on
            proposal #{proposal.id}.
          </p>
          <p className="text-dark-400 text-sm">
            This action will submit a transaction to the blockchain.
          </p>

          <div className="flex gap-3 mt-6">
            <Button
              variant="secondary"
              className="flex-1"
              onClick={() => {
                setShowVoteModal(false);
                setSelectedVote(null);
              }}
            >
              Cancel
            </Button>
            <Button
              className="flex-1"
              onClick={handleVote}
              isLoading={voteMutation.isPending}
            >
              Confirm Vote
            </Button>
          </div>
        </div>
      </Modal>
    </div>
  );
};
