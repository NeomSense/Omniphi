/**
 * Governance Page
 * List all proposals and allow voting
 */

import React from 'react';
import { ProposalList } from '@/components/governance/ProposalList';
import { Card, CardHeader, CardTitle, CardContent } from '@/components/ui/Card';
import { GOV_QUORUM, GOV_THRESHOLD, GOV_VETO_THRESHOLD, GOV_VOTING_PERIOD_HOURS, GOV_DEPOSIT_PERIOD_HOURS, GOV_MIN_DEPOSIT } from '@/lib/constants';
import { formatAmount } from '@/lib/utils';

const Governance: React.FC = () => {
  return (
    <div className="space-y-6">
      {/* Header */}
      <div>
        <h1 className="text-2xl font-bold text-dark-100">Governance</h1>
        <p className="text-dark-400 mt-1">Vote on proposals to shape the future of Omniphi</p>
      </div>

      {/* Governance Parameters */}
      <Card>
        <CardHeader>
          <CardTitle>Governance Parameters</CardTitle>
        </CardHeader>
        <CardContent>
          <div className="grid grid-cols-2 md:grid-cols-5 gap-4">
            <div className="p-4 bg-dark-800 rounded-lg">
              <p className="text-sm text-dark-500">Min Deposit</p>
              <p className="font-mono font-medium text-dark-100">{formatAmount(GOV_MIN_DEPOSIT)} OMNI</p>
            </div>
            <div className="p-4 bg-dark-800 rounded-lg">
              <p className="text-sm text-dark-500">Deposit Period</p>
              <p className="font-mono font-medium text-dark-100">{GOV_DEPOSIT_PERIOD_HOURS}h</p>
            </div>
            <div className="p-4 bg-dark-800 rounded-lg">
              <p className="text-sm text-dark-500">Voting Period</p>
              <p className="font-mono font-medium text-dark-100">{GOV_VOTING_PERIOD_HOURS}h</p>
            </div>
            <div className="p-4 bg-dark-800 rounded-lg">
              <p className="text-sm text-dark-500">Quorum</p>
              <p className="font-mono font-medium text-dark-100">{GOV_QUORUM * 100}%</p>
            </div>
            <div className="p-4 bg-dark-800 rounded-lg">
              <p className="text-sm text-dark-500">Threshold</p>
              <p className="font-mono font-medium text-dark-100">{GOV_THRESHOLD * 100}%</p>
            </div>
          </div>
        </CardContent>
      </Card>

      {/* Proposals List */}
      <ProposalList />
    </div>
  );
};

export default Governance;
