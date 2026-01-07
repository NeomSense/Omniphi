import { Link } from 'react-router-dom';

export default function Transactions() {
  return (
    <div className="space-y-6">
      <div className="flex items-center justify-between">
        <h1 className="text-2xl font-bold">Transactions</h1>
      </div>

      <div className="card p-8 text-center">
        <div className="w-16 h-16 bg-dark-700 rounded-full flex items-center justify-center mx-auto mb-4">
          <svg className="w-8 h-8 text-dark-400" fill="none" stroke="currentColor" viewBox="0 0 24 24">
            <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M9 5H7a2 2 0 00-2 2v12a2 2 0 002 2h10a2 2 0 002-2V7a2 2 0 00-2-2h-2M9 5a2 2 0 002 2h2a2 2 0 002-2M9 5a2 2 0 012-2h2a2 2 0 012 2" />
          </svg>
        </div>
        <h2 className="text-xl font-semibold text-dark-200 mb-2">Transaction Search</h2>
        <p className="text-dark-400 mb-4">
          Search for a specific transaction by entering its hash in the search bar above.
        </p>
        <p className="text-sm text-dark-500">
          Transactions are also shown within each block's detail page.
        </p>
        <Link to="/blocks" className="btn-primary inline-block mt-6">
          Browse Blocks
        </Link>
      </div>
    </div>
  );
}
