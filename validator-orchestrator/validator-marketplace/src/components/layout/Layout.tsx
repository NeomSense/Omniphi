/**
 * Main Layout Component
 */

import { Outlet } from 'react-router-dom';
import { Header } from './Header';

export function Layout() {
  return (
    <div className="min-h-screen bg-dark-950">
      <Header />
      <main>
        <Outlet />
      </main>
      <footer className="border-t border-dark-800 py-8 mt-16">
        <div className="max-w-7xl mx-auto px-4 sm:px-6 lg:px-8">
          <div className="flex items-center justify-between">
            <p className="text-sm text-dark-500">
              &copy; 2024 Omniphi Network. All rights reserved.
            </p>
            <div className="flex items-center space-x-6">
              <a href="https://omniphi.network" className="text-sm text-dark-400 hover:text-white">
                Website
              </a>
              <a href="https://docs.omniphi.network" className="text-sm text-dark-400 hover:text-white">
                Documentation
              </a>
              <a href="https://discord.omniphi.network" className="text-sm text-dark-400 hover:text-white">
                Discord
              </a>
            </div>
          </div>
        </div>
      </footer>
    </div>
  );
}

export default Layout;
