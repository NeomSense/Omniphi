import { Link, useLocation } from 'react-router-dom';
import { useState } from 'react';

interface LayoutProps {
  children: React.ReactNode;
}

export default function Layout({ children }: LayoutProps) {
  const location = useLocation();
  const [searchQuery, setSearchQuery] = useState('');

  const handleSearch = (e: React.FormEvent) => {
    e.preventDefault();
    const query = searchQuery.trim();

    if (!query) return;

    // Determine search type
    if (query.startsWith('omni1')) {
      window.location.href = `/account/${query}`;
    } else if (query.startsWith('omnivaloper')) {
      window.location.href = `/validator/${query}`;
    } else if (/^[0-9]+$/.test(query)) {
      window.location.href = `/block/${query}`;
    } else if (query.length === 64) {
      window.location.href = `/tx/${query}`;
    } else {
      // Default to tx search
      window.location.href = `/tx/${query}`;
    }
  };

  const navItems = [
    { path: '/', label: 'Dashboard' },
    { path: '/blocks', label: 'Blocks' },
    { path: '/txs', label: 'Transactions' },
    { path: '/validators', label: 'Validators' },
  ];

  return (
    <div className="min-h-screen flex flex-col">
      {/* Header */}
      <header className="bg-dark-900/80 backdrop-blur-md border-b border-dark-700 sticky top-0 z-50">
        <div className="max-w-7xl mx-auto px-4 py-4">
          <div className="flex items-center justify-between gap-8">
            {/* Logo */}
            <Link to="/" className="flex items-center gap-3">
              <div className="w-10 h-10 bg-gradient-to-br from-omniphi-400 to-omniphi-600 rounded-xl flex items-center justify-center">
                <span className="text-white font-bold text-lg">O</span>
              </div>
              <div>
                <h1 className="text-xl font-bold bg-gradient-to-r from-omniphi-400 to-omniphi-600 bg-clip-text text-transparent">
                  Omniphi Explorer
                </h1>
                <p className="text-xs text-dark-400">Testnet</p>
              </div>
            </Link>

            {/* Search */}
            <form onSubmit={handleSearch} className="flex-1 max-w-xl">
              <div className="relative">
                <input
                  type="text"
                  value={searchQuery}
                  onChange={(e) => setSearchQuery(e.target.value)}
                  placeholder="Search by Address / Tx Hash / Block Height"
                  className="w-full input pr-12"
                />
                <button
                  type="submit"
                  className="absolute right-2 top-1/2 -translate-y-1/2 p-2 text-dark-400 hover:text-omniphi-400"
                >
                  <svg className="w-5 h-5" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                    <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M21 21l-6-6m2-5a7 7 0 11-14 0 7 7 0 0114 0z" />
                  </svg>
                </button>
              </div>
            </form>

            {/* Nav */}
            <nav className="hidden md:flex items-center gap-1">
              {navItems.map((item) => (
                <Link
                  key={item.path}
                  to={item.path}
                  className={`px-4 py-2 rounded-lg text-sm font-medium transition-colors ${
                    location.pathname === item.path
                      ? 'bg-omniphi-500/20 text-omniphi-400'
                      : 'text-dark-300 hover:text-dark-100 hover:bg-dark-700'
                  }`}
                >
                  {item.label}
                </Link>
              ))}
            </nav>
          </div>
        </div>
      </header>

      {/* Main Content */}
      <main className="flex-1">
        <div className="max-w-7xl mx-auto px-4 py-8">{children}</div>
      </main>

      {/* Footer */}
      <footer className="bg-dark-900/50 border-t border-dark-700 py-6">
        <div className="max-w-7xl mx-auto px-4">
          <div className="flex items-center justify-between text-sm text-dark-400">
            <p>Omniphi Blockchain Explorer</p>
            <div className="flex items-center gap-4">
              <a href="https://github.com/omniphi" target="_blank" rel="noopener" className="hover:text-omniphi-400">
                GitHub
              </a>
              <a href="https://omniphi.network" target="_blank" rel="noopener" className="hover:text-omniphi-400">
                Website
              </a>
            </div>
          </div>
        </div>
      </footer>
    </div>
  );
}
