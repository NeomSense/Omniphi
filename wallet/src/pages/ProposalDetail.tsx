/**
 * Proposal Detail Page
 * Full proposal view with voting UI
 */

import React from 'react';
import { useParams, Link } from 'react-router-dom';
import { ProposalDetail } from '@/components/governance/ProposalDetail';
import { Button } from '@/components/ui/Button';

const ProposalDetailPage: React.FC = () => {
  const { proposalId } = useParams<{ proposalId: string }>();

  if (!proposalId) {
    return (
      <div className="text-center py-12">
        <p className="text-dark-400">Proposal not found</p>
        <Link to="/governance">
          <Button variant="secondary" className="mt-4">
            Back to Governance
          </Button>
        </Link>
      </div>
    );
  }

  return (
    <div className="space-y-6">
      {/* Back button */}
      <Link
        to="/governance"
        className="inline-flex items-center gap-2 text-dark-400 hover:text-dark-200 transition-colors"
      >
        <svg className="w-5 h-5" fill="none" viewBox="0 0 24 24" stroke="currentColor">
          <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M15 19l-7-7 7-7" />
        </svg>
        Back to Governance
      </Link>

      {/* Proposal Detail Component */}
      <ProposalDetail proposalId={proposalId} />
    </div>
  );
};

export default ProposalDetailPage;
