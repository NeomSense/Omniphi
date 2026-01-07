import { useQuery } from '@tanstack/react-query';
import { useParams, Link } from 'react-router-dom';
import { getAccountBalance, getAccountDelegations, formatAmount } from '@/lib/api';

export default function Account() {
  const { address } = useParams<{ address: string }>();

  const { data: balances = [], isLoading: balancesLoading } = useQuery({
    queryKey: ['accountBalance', address],
    queryFn: () => getAccountBalance(address!),
    enabled: !!address,
  });

  const { data: delegations = [], isLoading: delegationsLoading } = useQuery({
    queryKey: ['accountDelegations', address],
    queryFn: () => getAccountDelegations(address!),
    enabled: !!address,
  });

  const isLoading = balancesLoading || delegationsLoading;
  const omniBalance = balances.find((b) => b.denom === 'omniphi')?.amount || '0';

  return (
    <div className="space-y-6">
      <div className="flex items-center gap-4">
        <Link to="/" className="text-dark-400 hover:text-dark-200">
          ← Dashboard
        </Link>
        <h1 className="text-2xl font-bold">Account</h1>
      </div>

      {/* Address Card */}
      <div className="card p-6">
        <div className="flex items-center gap-4">
          <div className="w-14 h-14 bg-gradient-to-br from-omniphi-400 to-omniphi-600 rounded-2xl flex items-center justify-center">
            <svg className="w-7 h-7 text-white" fill="none" stroke="currentColor" viewBox="0 0 24 24">
              <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M16 7a4 4 0 11-8 0 4 4 0 018 0zM12 14a7 7 0 00-7 7h14a7 7 0 00-7-7z" />
            </svg>
          </div>
          <div className="overflow-hidden">
            <p className="text-sm text-dark-400">Address</p>
            <p className="font-mono text-dark-100 break-all">{address}</p>
          </div>
        </div>
      </div>

      {/* Balances */}
      <div className="card">
        <div className="p-4 border-b border-dark-700">
          <h2 className="font-semibold">Balances</h2>
        </div>
        {isLoading ? (
          <div className="p-8 text-center text-dark-400">Loading...</div>
        ) : balances.length === 0 ? (
          <div className="p-8 text-center text-dark-400">No balances found</div>
        ) : (
          <div className="p-4">
            <div className="grid grid-cols-2 md:grid-cols-3 gap-4">
              {balances.map((balance) => (
                <div key={balance.denom} className="bg-dark-800 rounded-lg p-4">
                  <p className="text-sm text-dark-400">
                    {balance.denom === 'uomni' ? 'OMNI' : balance.denom}
                  </p>
                  <p className="text-xl font-bold text-dark-100 mt-1">
                    {formatAmount(balance.amount)}
                  </p>
                </div>
              ))}
            </div>
          </div>
        )}
      </div>

      {/* Staking */}
      <div className="card">
        <div className="p-4 border-b border-dark-700">
          <h2 className="font-semibold">Staking</h2>
        </div>
        {isLoading ? (
          <div className="p-8 text-center text-dark-400">Loading...</div>
        ) : delegations.length === 0 ? (
          <div className="p-8 text-center text-dark-400">No active delegations</div>
        ) : (
          <div className="divide-y divide-dark-700">
            {delegations.map((delegation: any, index: number) => (
              <div key={index} className="p-4 table-row">
                <div className="flex items-center justify-between">
                  <div>
                    <Link
                      to={`/validator/${delegation.delegation.validator_address}`}
                      className="text-omniphi-400 hover:text-omniphi-300"
                    >
                      {delegation.delegation.validator_address.slice(0, 20)}...
                    </Link>
                  </div>
                  <div className="text-right">
                    <p className="font-mono text-dark-200">
                      {formatAmount(delegation.balance.amount)} OMNI
                    </p>
                    <p className="text-sm text-dark-500">Delegated</p>
                  </div>
                </div>
              </div>
            ))}
          </div>
        )}
      </div>

      {/* Summary Stats */}
      <div className="grid grid-cols-2 md:grid-cols-4 gap-4">
        <div className="card p-4">
          <p className="text-sm text-dark-400">Available</p>
          <p className="text-xl font-bold text-dark-100 mt-1">
            {formatAmount(omniBalance)} OMNI
          </p>
        </div>
        <div className="card p-4">
          <p className="text-sm text-dark-400">Delegated</p>
          <p className="text-xl font-bold text-green-400 mt-1">
            {formatAmount(
              delegations.reduce(
                (sum: bigint, d: any) => sum + BigInt(d.balance?.amount || 0),
                0n
              ).toString()
            )} OMNI
          </p>
        </div>
        <div className="card p-4">
          <p className="text-sm text-dark-400">Delegations</p>
          <p className="text-xl font-bold text-dark-100 mt-1">{delegations.length}</p>
        </div>
        <div className="card p-4">
          <p className="text-sm text-dark-400">Tokens</p>
          <p className="text-xl font-bold text-dark-100 mt-1">{balances.length}</p>
        </div>
      </div>
    </div>
  );
}
