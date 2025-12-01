import { Card } from '../ui/Card';
import { ValidatorMetadata } from '../../types/validator';

interface ValidatorMetadataCardProps {
  metadata: ValidatorMetadata | null;
  loading?: boolean;
}

export function ValidatorMetadataCard({ metadata, loading }: ValidatorMetadataCardProps) {
  if (loading) {
    return (
      <Card title="Validator Info">
        <div className="animate-pulse space-y-3">
          <div className="h-4 bg-gray-200 rounded w-3/4"></div>
          <div className="h-4 bg-gray-200 rounded w-1/2"></div>
          <div className="h-4 bg-gray-200 rounded w-2/3"></div>
        </div>
      </Card>
    );
  }

  if (!metadata) {
    return (
      <Card title="Validator Info">
        <p className="text-gray-500 text-sm">No validator metadata available</p>
      </Card>
    );
  }

  const truncateAddress = (addr: string, start = 12, end = 8) => {
    if (addr.length <= start + end) return addr;
    return `${addr.slice(0, start)}...${addr.slice(-end)}`;
  };

  const copyToClipboard = (text: string) => {
    navigator.clipboard.writeText(text);
  };

  return (
    <Card title="Validator Info">
      <div className="space-y-4">
        {/* Operator Address */}
        <div>
          <p className="text-xs text-gray-500 mb-1">Operator Address</p>
          <div className="flex items-center justify-between bg-gray-50 rounded-lg px-3 py-2">
            <code className="text-sm text-gray-700 font-mono">
              {truncateAddress(metadata.operator_address)}
            </code>
            <button
              onClick={() => copyToClipboard(metadata.operator_address)}
              className="text-purple-600 hover:text-purple-700 text-sm"
              title="Copy to clipboard"
            >
              Copy
            </button>
          </div>
        </div>

        {/* Validator Address */}
        <div>
          <p className="text-xs text-gray-500 mb-1">Validator Address</p>
          <div className="flex items-center justify-between bg-gray-50 rounded-lg px-3 py-2">
            <code className="text-sm text-gray-700 font-mono">
              {truncateAddress(metadata.validator_address)}
            </code>
            <button
              onClick={() => copyToClipboard(metadata.validator_address)}
              className="text-purple-600 hover:text-purple-700 text-sm"
              title="Copy to clipboard"
            >
              Copy
            </button>
          </div>
        </div>

        {/* Commission */}
        <div className="grid grid-cols-3 gap-3 pt-2 border-t border-gray-100">
          <div>
            <p className="text-xs text-gray-500">Commission</p>
            <p className="text-lg font-semibold text-gray-900">
              {(parseFloat(metadata.commission_rate) * 100).toFixed(1)}%
            </p>
          </div>
          <div>
            <p className="text-xs text-gray-500">Max Rate</p>
            <p className="text-lg font-semibold text-gray-900">
              {(parseFloat(metadata.commission_max_rate) * 100).toFixed(1)}%
            </p>
          </div>
          <div>
            <p className="text-xs text-gray-500">Max Change</p>
            <p className="text-lg font-semibold text-gray-900">
              {(parseFloat(metadata.commission_max_change_rate) * 100).toFixed(1)}%
            </p>
          </div>
        </div>

        {/* Tokens & Voting Power */}
        <div className="grid grid-cols-2 gap-3 pt-2 border-t border-gray-100">
          <div>
            <p className="text-xs text-gray-500">Bonded Tokens</p>
            <p className="text-lg font-semibold text-gray-900">
              {parseFloat(metadata.tokens).toLocaleString()} OMNI
            </p>
          </div>
          <div>
            <p className="text-xs text-gray-500">Voting Power</p>
            <p className="text-lg font-semibold text-gray-900">
              {metadata.voting_power}%
            </p>
          </div>
        </div>

        {/* Self Delegation */}
        <div className="pt-2 border-t border-gray-100">
          <p className="text-xs text-gray-500">Self Delegation</p>
          <p className="text-sm font-medium text-gray-700">
            {parseFloat(metadata.self_delegation).toLocaleString()} OMNI
            <span className="text-gray-400 ml-2">
              (min: {parseFloat(metadata.min_self_delegation).toLocaleString()})
            </span>
          </p>
        </div>

        {/* Optional Website/Details */}
        {(metadata.website || metadata.details) && (
          <div className="pt-2 border-t border-gray-100">
            {metadata.website && (
              <a
                href={metadata.website}
                target="_blank"
                rel="noopener noreferrer"
                className="text-sm text-purple-600 hover:text-purple-700"
              >
                {metadata.website}
              </a>
            )}
            {metadata.details && (
              <p className="text-sm text-gray-600 mt-1">{metadata.details}</p>
            )}
          </div>
        )}
      </div>
    </Card>
  );
}
