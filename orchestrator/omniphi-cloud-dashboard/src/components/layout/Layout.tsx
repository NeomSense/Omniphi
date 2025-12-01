/**
 * Main Layout Component
 */

import { Outlet } from 'react-router-dom';
import { Sidebar } from './Sidebar';

export function Layout() {
  return (
    <div className="min-h-screen">
      <Sidebar />
      <main className="ml-64 p-6">
        <Outlet />
      </main>
    </div>
  );
}

export default Layout;
