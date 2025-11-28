/**
 * Sidebar Navigation Component
 */

import { NavLink } from 'react-router-dom';
import {
  LayoutDashboard,
  Globe,
  Server,
  ArrowUpCircle,
  AlertTriangle,
  DollarSign,
  Cloud,
} from 'lucide-react';

const navItems = [
  { path: '/', label: 'Fleet Overview', icon: LayoutDashboard },
  { path: '/regions', label: 'Regional Capacity', icon: Globe },
  { path: '/nodes', label: 'Node Explorer', icon: Server },
  { path: '/upgrades', label: 'Upgrade Management', icon: ArrowUpCircle },
  { path: '/incidents', label: 'Incidents & Alerts', icon: AlertTriangle },
  { path: '/costs', label: 'Cost & Billing', icon: DollarSign },
];

export function Sidebar() {
  return (
    <div className="sidebar">
      {/* Logo */}
      <div className="p-4 border-b border-dark-800">
        <div className="flex items-center space-x-3">
          <div className="w-10 h-10 bg-omniphi-600 rounded-xl flex items-center justify-center">
            <Cloud className="w-6 h-6 text-white" />
          </div>
          <div>
            <h1 className="text-white font-semibold text-sm">Omniphi Cloud</h1>
            <p className="text-dark-500 text-xs">Infrastructure Dashboard</p>
          </div>
        </div>
      </div>

      {/* Navigation */}
      <nav className="flex-1 py-4 overflow-y-auto">
        {navItems.map((item) => (
          <NavLink
            key={item.path}
            to={item.path}
            className={({ isActive }) =>
              `sidebar-link ${isActive ? 'active' : ''}`
            }
            end={item.path === '/'}
          >
            <item.icon className="w-5 h-5 mr-3" />
            {item.label}
          </NavLink>
        ))}
      </nav>

      {/* Footer */}
      <div className="p-4 border-t border-dark-800">
        <div className="flex items-center space-x-2">
          <div className="status-dot status-dot-healthy" />
          <span className="text-xs text-dark-400">All Systems Operational</span>
        </div>
      </div>
    </div>
  );
}

export default Sidebar;
