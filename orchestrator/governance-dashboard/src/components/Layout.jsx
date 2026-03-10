import { Link, Outlet } from 'react-router-dom';

export default function Layout() {
  return (
    <div className="layout">
      <header className="header">
        <Link to="/" className="logo">
          <span className="logo-icon">◈</span> Omniphi Governance
        </Link>
        <span className="header-subtitle">Guard Dashboard</span>
      </header>
      <main className="main">
        <Outlet />
      </main>
    </div>
  );
}
