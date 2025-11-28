/**
 * Sidebar Navigation Component
 */

import { NavLink } from 'react-router-dom';
import {
  LayoutDashboard,
  FileText,
  Server,
  ScrollText,
  Settings,
  ClipboardList,
  AlertTriangle,
  LogOut,
} from 'lucide-react';
import { useAuthStore } from '../../store/authStore';

const navItems = [
  { to: '/', icon: LayoutDashboard, label: 'Overview' },
  { to: '/requests', icon: FileText, label: 'Setup Requests' },
  { to: '/nodes', icon: Server, label: 'Validator Nodes' },
  { to: '/logs', icon: ScrollText, label: 'Logs' },
  { to: '/settings', icon: Settings, label: 'Settings' },
  { to: '/audit', icon: ClipboardList, label: 'Audit Log' },
  { to: '/alerts', icon: AlertTriangle, label: 'Alerts' },
];

export function Sidebar() {
  const { user, logout } = useAuthStore();

  return (
    <aside className="sidebar">
      {/* Logo */}
      <div className="sidebar-header">
        <div className="flex items-center space-x-3">
          <div className="w-8 h-8 bg-gradient-to-br from-omniphi-500 to-omniphi-700 rounded-lg flex items-center justify-center">
            <span className="text-white font-bold text-lg">O</span>
          </div>
          <div>
            <h1 className="text-white font-semibold text-sm">Omniphi</h1>
            <p className="text-dark-400 text-xs">Admin Panel</p>
          </div>
        </div>
      </div>

      {/* Navigation */}
      <nav className="sidebar-nav">
        <div className="space-y-1">
          {navItems.map((item) => (
            <NavLink
              key={item.to}
              to={item.to}
              className={({ isActive }) =>
                `sidebar-link ${isActive ? 'active' : ''}`
              }
              end={item.to === '/'}
            >
              <item.icon className="sidebar-link-icon" />
              <span>{item.label}</span>
            </NavLink>
          ))}
        </div>
      </nav>

      {/* User Section */}
      <div className="p-4 border-t border-dark-700">
        <div className="flex items-center justify-between">
          <div className="flex items-center space-x-3">
            <div className="w-8 h-8 bg-dark-700 rounded-full flex items-center justify-center">
              <span className="text-dark-300 text-sm font-medium">
                {user?.username?.[0]?.toUpperCase() || 'A'}
              </span>
            </div>
            <div>
              <p className="text-sm font-medium text-dark-200">{user?.username}</p>
              <p className="text-xs text-dark-500 capitalize">{user?.role}</p>
            </div>
          </div>
          <button
            onClick={logout}
            className="btn btn-ghost btn-icon"
            title="Logout"
          >
            <LogOut className="w-4 h-4" />
          </button>
        </div>
      </div>
    </aside>
  );
}

export default Sidebar;
