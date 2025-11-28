/**
 * Omniphi Cloud Internal Dashboard
 * Main Application with Routing
 */

import { BrowserRouter, Routes, Route } from 'react-router-dom';
import { Layout } from './components/layout/Layout';
import { FleetOverviewPage } from './pages/FleetOverviewPage';
import { RegionalCapacityPage } from './pages/RegionalCapacityPage';
import { NodeExplorerPage } from './pages/NodeExplorerPage';
import { NodeDetailPage } from './pages/NodeDetailPage';
import { UpgradeManagementPage } from './pages/UpgradeManagementPage';
import { IncidentDashboardPage } from './pages/IncidentDashboardPage';
import { CostDashboardPage } from './pages/CostDashboardPage';

function App() {
  return (
    <BrowserRouter>
      <Routes>
        <Route path="/" element={<Layout />}>
          <Route index element={<FleetOverviewPage />} />
          <Route path="regions" element={<RegionalCapacityPage />} />
          <Route path="nodes" element={<NodeExplorerPage />} />
          <Route path="nodes/:nodeId" element={<NodeDetailPage />} />
          <Route path="upgrades" element={<UpgradeManagementPage />} />
          <Route path="incidents" element={<IncidentDashboardPage />} />
          <Route path="costs" element={<CostDashboardPage />} />
        </Route>
      </Routes>
    </BrowserRouter>
  );
}

export default App;
