/**
 * Marketplace Header Component
 */

import { Link, useLocation } from 'react-router-dom';
import { Store, Server, GitCompare, UserPlus, Wallet } from 'lucide-react';

const navItems = [
  { path: '/', label: 'Marketplace', icon: Store },
  { path: '/my-validators', label: 'My Validators', icon: Server },
  { path: '/compare', label: 'Compare', icon: GitCompare },
  { path: '/become-provider', label: 'Become a Provider', icon: UserPlus },
];

export function Header() {
  const location = useLocation();

  return (
    <header className="sticky top-0 z-50 bg-dark-950/90 backdrop-blur-md border-b border-dark-800">
      <div className="max-w-7xl mx-auto px-4 sm:px-6 lg:px-8">
        <div className="flex items-center justify-between h-16">
          {/* Logo */}
          <Link to="/" className="flex items-center space-x-3">
            <div className="w-10 h-10 bg-gradient-to-br from-omniphi-500 to-omniphi-700 rounded-xl flex items-center justify-center">
              <Store className="w-6 h-6 text-white" />
            </div>
            <div>
              <h1 className="text-lg font-bold text-white">Validator Marketplace</h1>
              <p className="text-xs text-dark-400">by Omniphi</p>
            </div>
          </Link>

          {/* Navigation */}
          <nav className="hidden md:flex items-center space-x-1">
            {navItems.map((item) => {
              const isActive = location.pathname === item.path;
              return (
                <Link
                  key={item.path}
                  to={item.path}
                  className={`flex items-center px-4 py-2 rounded-lg text-sm font-medium transition-colors ${
                    isActive
                      ? 'bg-omniphi-600/20 text-omniphi-400'
                      : 'text-dark-300 hover:text-white hover:bg-dark-800'
                  }`}
                >
                  <item.icon className="w-4 h-4 mr-2" />
                  {item.label}
                </Link>
              );
            })}
          </nav>

          {/* Wallet Button */}
          <button className="btn btn-primary">
            <Wallet className="w-4 h-4 mr-2" />
            Connect Wallet
          </button>
        </div>
      </div>
    </header>
  );
}

export default Header;
