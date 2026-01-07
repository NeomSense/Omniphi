import React from 'react';
import { Link } from 'react-router-dom';
import { ClockIcon, CheckCircleIcon, XCircleIcon, ExclamationCircleIcon } from '@heroicons/react/24/outline';
import { Card } from '@/components/ui';
import { Badge } from '@/components/ui';
import {
  useProposals,
  getProposalStatusLabel,
  formatTimeRemaining,
  calculateVotePercentages,
} from '@/hooks/useGovernance';
import { Proposal } from '@/lib/api';
import { formatAmount } from '@/lib/wallet';
import { DENOM } from '@/lib/constants';

interface ProposalCardProps {
  proposal: Proposal;
}

const ProposalCard: React.FC<ProposalCardProps> = ({ proposal }) => {
  const statusLabel = getProposalStatusLabel(proposal.status);
  const isVoting = proposal.status === 'PROPOSAL_STATUS_VOTING_PERIOD';
  const isPassed = proposal.status === 'PROPOSAL_STATUS_PASSED';
  const isRejected = proposal.status === 'PROPOSAL_STATUS_REJECTED';

  const tally = proposal.final_tally_result;
  const percentages = calculateVotePercentages(tally);

  const getStatusBadge = () => {
    if (isVoting) return <Badge variant="info">{statusLabel}</Badge>;
    if (isPassed) return <Badge variant="success">{statusLabel}</Badge>;
    if (isRejected) return <Badge variant="error">{statusLabel}</Badge>;
    return <Badge variant="warning">{statusLabel}</Badge>;
  };

  const getStatusIcon = () => {
    if (isVoting) return <ClockIcon className="h-5 w-5 text-omniphi-400" />;
    if (isPassed) return <CheckCircleIcon className="h-5 w-5 text-green-400" />;
    if (isRejected) return <XCircleIcon className="h-5 w-5 text-red-400" />;
    return <ExclamationCircleIcon className="h-5 w-5 text-yellow-400" />;
  };

  return (
    <Link to={`/governance/${proposal.id}`}>
      <Card variant="hover" className="cursor-pointer">
        <div className="flex items-start justify-between">
          <div className="flex items-start gap-3">
            {getStatusIcon()}
            <div>
              <div className="flex items-center gap-2 mb-1">
                <span className="text-dark-400 text-sm">#{proposal.id}</span>
                {getStatusBadge()}
              </div>
              <h3 className="text-dark-100 font-medium mb-1">
                {proposal.title || 'Untitled Proposal'}
              </h3>
              <p className="text-dark-400 text-sm line-clamp-2">
                {proposal.summary || 'No description available'}
              </p>
            </div>
          </div>
        </div>

        {/* Voting progress bar */}
        {(isVoting || isPassed || isRejected) && percentages.total > 0n && (
          <div className="mt-4">
            <div className="flex justify-between text-xs text-dark-400 mb-1">
              <span>Yes: {percentages.yes.toFixed(1)}%</span>
              <span>No: {percentages.no.toFixed(1)}%</span>
            </div>
            <div className="h-2 bg-dark-700 rounded-full overflow-hidden flex">
              <div
                className="bg-green-500 h-full"
                style={{ width: `${percentages.yes}%` }}
              />
              <div
                className="bg-dark-500 h-full"
                style={{ width: `${percentages.abstain}%` }}
              />
              <div
                className="bg-red-500 h-full"
                style={{ width: `${percentages.no}%` }}
              />
              <div
                className="bg-red-700 h-full"
                style={{ width: `${percentages.noWithVeto}%` }}
              />
            </div>
          </div>
        )}

        {/* Time remaining for voting proposals */}
        {isVoting && (
          <div className="mt-3 flex items-center gap-2 text-sm text-dark-400">
            <ClockIcon className="h-4 w-4" />
            <span>{formatTimeRemaining(proposal.voting_end_time)}</span>
          </div>
        )}
      </Card>
    </Link>
  );
};

interface ProposalListProps {
  filter?: 'all' | 'voting' | 'passed' | 'rejected';
}

export const ProposalList: React.FC<ProposalListProps> = ({ filter = 'all' }) => {
  const statusMap: Record<string, string | undefined> = {
    all: undefined,
    voting: 'PROPOSAL_STATUS_VOTING_PERIOD',
    passed: 'PROPOSAL_STATUS_PASSED',
    rejected: 'PROPOSAL_STATUS_REJECTED',
  };

  const { data: proposals, isLoading, error } = useProposals(statusMap[filter]);

  if (isLoading) {
    return (
      <div className="space-y-4">
        {[1, 2, 3].map((i) => (
          <Card key={i} className="animate-pulse">
            <div className="h-4 bg-dark-700 rounded w-1/4 mb-2" />
            <div className="h-6 bg-dark-700 rounded w-3/4 mb-2" />
            <div className="h-4 bg-dark-700 rounded w-1/2" />
          </Card>
        ))}
      </div>
    );
  }

  if (error) {
    return (
      <Card>
        <div className="text-center py-8">
          <ExclamationCircleIcon className="h-12 w-12 text-red-400 mx-auto mb-4" />
          <p className="text-dark-400">Failed to load proposals</p>
        </div>
      </Card>
    );
  }

  if (!proposals || proposals.length === 0) {
    return (
      <Card>
        <div className="text-center py-8">
          <ClockIcon className="h-12 w-12 text-dark-500 mx-auto mb-4" />
          <p className="text-dark-400">No proposals found</p>
        </div>
      </Card>
    );
  }

  // Sort by ID descending (newest first)
  const sortedProposals = [...proposals].sort(
    (a, b) => parseInt(b.id) - parseInt(a.id)
  );

  return (
    <div className="space-y-4">
      {sortedProposals.map((proposal) => (
        <ProposalCard key={proposal.id} proposal={proposal} />
      ))}
    </div>
  );
};
