/**
 * Omniphi Validator Marketplace - Main App
 */

import { BrowserRouter, Routes, Route } from 'react-router-dom';
import { Layout } from './components/layout/Layout';
import { MarketplacePage } from './pages/MarketplacePage';
import { ProviderDetailPage } from './pages/ProviderDetailPage';
import { BecomeProviderPage } from './pages/BecomeProviderPage';
import { MyValidatorsPage } from './pages/MyValidatorsPage';
import { ComparePage } from './pages/ComparePage';

function App() {
  return (
    <BrowserRouter>
      <Routes>
        <Route path="/" element={<Layout />}>
          <Route index element={<MarketplacePage />} />
          <Route path="provider/:id" element={<ProviderDetailPage />} />
          <Route path="become-provider" element={<BecomeProviderPage />} />
          <Route path="my-validators" element={<MyValidatorsPage />} />
          <Route path="compare" element={<ComparePage />} />
        </Route>
      </Routes>
    </BrowserRouter>
  );
}

export default App;
