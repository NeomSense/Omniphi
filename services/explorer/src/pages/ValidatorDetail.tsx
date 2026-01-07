import { useQuery } from '@tanstack/react-query';
import { useParams, Link } from 'react-router-dom';
import { getValidator, formatAmount } from '@/lib/api';

export default function ValidatorDetail() {
  const { address } = useParams<{ address: string }>();

  const { data: validator, isLoading, error } = useQuery({
    queryKey: ['validator', address],
    queryFn: () => getValidator(address!),
    enabled: !!address,
  });

  if (isLoading) {
    return (
      <div className="flex items-center justify-center min-h-[400px]">
        <div className="w-8 h-8 border-4 border-omniphi-500 border-t-transparent rounded-full animate-spin" />
      </div>
    );
  }

  if (error || !validator) {
    return (
      <div className="card p-8 text-center">
        <h2 className="text-xl font-semibold text-red-400 mb-2">Validator Not Found</h2>
        <p className="text-dark-400">The validator could not be found.</p>
        <Link to="/validators" className="btn-primary inline-block mt-4">
          Back to Validators
        </Link>
      </div>
    );
  }

  const commission = parseFloat(validator.commission.commission_rates.rate) * 100;
  const maxCommission = parseFloat(validator.commission.commission_rates.max_rate) * 100;
  const maxChange = parseFloat(validator.commission.commission_rates.max_change_rate) * 100;

  return (
    <div className="space-y-6">
      <div className="flex items-center gap-4">
        <Link to="/validators" className="text-dark-400 hover:text-dark-200">
          ← Validators
        </Link>
      </div>

      {/* Header */}
      <div className="card p-6">
        <div className="flex items-center gap-4">
          <div className="w-16 h-16 bg-gradient-to-br from-omniphi-400 to-omniphi-600 rounded-2xl flex items-center justify-center">
            <span className="text-2xl font-bold text-white">
              {validator.description.moniker.charAt(0).toUpperCase()}
            </span>
          </div>
          <div>
            <h1 className="text-2xl font-bold text-dark-100">
              {validator.description.moniker}
            </h1>
            <p className="font-mono text-sm text-dark-400 mt-1">
              {validator.operator_address}
            </p>
            <div className="flex items-center gap-2 mt-2">
              {validator.jailed ? (
                <span className="px-2 py-1 status-error rounded text-xs">Jailed</span>
              ) : (
                <span className="px-2 py-1 status-success rounded text-xs">Active</span>
              )}
            </div>
          </div>
        </div>
      </div>

      {/* Stats */}
      <div className="grid grid-cols-2 md:grid-cols-4 gap-4">
        <div className="card p-4">
          <p className="text-sm text-dark-400">Voting Power</p>
          <p className="text-xl font-bold text-dark-100 mt-1">
            {formatAmount(validator.tokens)} OMNI
          </p>
        </div>
        <div className="card p-4">
          <p className="text-sm text-dark-400">Commission</p>
          <p className="text-xl font-bold text-dark-100 mt-1">{commission.toFixed(1)}%</p>
        </div>
        <div className="card p-4">
          <p className="text-sm text-dark-400">Max Commission</p>
          <p className="text-xl font-bold text-dark-100 mt-1">{maxCommission.toFixed(1)}%</p>
        </div>
        <div className="card p-4">
          <p className="text-sm text-dark-400">Max Change Rate</p>
          <p className="text-xl font-bold text-dark-100 mt-1">{maxChange.toFixed(1)}%</p>
        </div>
      </div>

      {/* Details */}
      <div className="card">
        <div className="p-4 border-b border-dark-700">
          <h2 className="font-semibold">Validator Details</h2>
        </div>
        <div className="p-4 space-y-4">
          <DetailRow label="Operator Address" value={validator.operator_address} mono />
          <DetailRow label="Delegator Shares" value={formatAmount(validator.delegator_shares.split('.')[0])} />
          <DetailRow label="Min Self Delegation" value={formatAmount(validator.min_self_delegation)} />
          {validator.description.website && (
            <DetailRow
              label="Website"
              value={
                <a
                  href={validator.description.website}
                  target="_blank"
                  rel="noopener noreferrer"
                  className="text-omniphi-400 hover:text-omniphi-300"
                >
                  {validator.description.website}
                </a>
              }
            />
          )}
          {validator.description.details && (
            <DetailRow label="Details" value={validator.description.details} />
          )}
          {validator.description.security_contact && (
            <DetailRow label="Security Contact" value={validator.description.security_contact} />
          )}
        </div>
      </div>
    </div>
  );
}

function DetailRow({
  label,
  value,
  mono = false,
}: {
  label: string;
  value: React.ReactNode;
  mono?: boolean;
}) {
  return (
    <div className="flex flex-col sm:flex-row sm:items-start gap-1 sm:gap-4">
      <span className="text-dark-400 sm:w-40 flex-shrink-0">{label}</span>
      <span className={`${mono ? 'font-mono text-sm' : ''} text-dark-100 break-all`}>
        {value}
      </span>
    </div>
  );
}
