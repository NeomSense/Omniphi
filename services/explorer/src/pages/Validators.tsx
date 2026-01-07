import { useQuery } from '@tanstack/react-query';
import { Link } from 'react-router-dom';
import { getValidators, formatAmount, truncateAddress } from '@/lib/api';

export default function Validators() {
  const { data: validators = [], isLoading } = useQuery({
    queryKey: ['validators'],
    queryFn: getValidators,
  });

  // Calculate total voting power
  const totalPower = validators.reduce((sum, v) => sum + BigInt(v.tokens), 0n);

  return (
    <div className="space-y-6">
      <div className="flex items-center justify-between">
        <h1 className="text-2xl font-bold">Validators</h1>
        <p className="text-dark-400">
          {validators.length} active validators
        </p>
      </div>

      <div className="card overflow-hidden">
        <table className="w-full">
          <thead className="bg-dark-800">
            <tr className="text-left text-sm text-dark-400">
              <th className="px-4 py-3 font-medium w-12">#</th>
              <th className="px-4 py-3 font-medium">Validator</th>
              <th className="px-4 py-3 font-medium">Voting Power</th>
              <th className="px-4 py-3 font-medium">Share</th>
              <th className="px-4 py-3 font-medium">Commission</th>
              <th className="px-4 py-3 font-medium">Status</th>
            </tr>
          </thead>
          <tbody className="divide-y divide-dark-700">
            {isLoading ? (
              <tr>
                <td colSpan={6} className="px-4 py-8 text-center text-dark-400">
                  Loading validators...
                </td>
              </tr>
            ) : validators.length === 0 ? (
              <tr>
                <td colSpan={6} className="px-4 py-8 text-center text-dark-400">
                  No validators found
                </td>
              </tr>
            ) : (
              validators.map((validator, index) => {
                const share = totalPower > 0n
                  ? Number((BigInt(validator.tokens) * 10000n) / totalPower) / 100
                  : 0;
                const commission = parseFloat(validator.commission.commission_rates.rate) * 100;

                return (
                  <tr key={validator.operator_address} className="table-row">
                    <td className="px-4 py-3 text-dark-400">{index + 1}</td>
                    <td className="px-4 py-3">
                      <Link
                        to={`/validator/${validator.operator_address}`}
                        className="block"
                      >
                        <p className="font-medium text-dark-100 hover:text-omniphi-400">
                          {validator.description.moniker}
                        </p>
                        <p className="text-xs font-mono text-dark-500">
                          {truncateAddress(validator.operator_address)}
                        </p>
                      </Link>
                    </td>
                    <td className="px-4 py-3">
                      <span className="font-mono text-dark-200">
                        {formatAmount(validator.tokens)} OMNI
                      </span>
                    </td>
                    <td className="px-4 py-3">
                      <div className="flex items-center gap-2">
                        <div className="w-16 h-2 bg-dark-700 rounded-full overflow-hidden">
                          <div
                            className="h-full bg-omniphi-500"
                            style={{ width: `${Math.min(share, 100)}%` }}
                          />
                        </div>
                        <span className="text-sm text-dark-300">{share.toFixed(1)}%</span>
                      </div>
                    </td>
                    <td className="px-4 py-3">
                      <span className="text-dark-300">{commission.toFixed(1)}%</span>
                    </td>
                    <td className="px-4 py-3">
                      {validator.jailed ? (
                        <span className="px-2 py-1 status-error rounded text-xs">Jailed</span>
                      ) : (
                        <span className="px-2 py-1 status-success rounded text-xs">Active</span>
                      )}
                    </td>
                  </tr>
                );
              })
            )}
          </tbody>
        </table>
      </div>
    </div>
  );
}
