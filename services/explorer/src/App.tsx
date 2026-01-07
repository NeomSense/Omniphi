import { Routes, Route } from 'react-router-dom';
import { Suspense, lazy } from 'react';
import Layout from './components/Layout';

const Home = lazy(() => import('./pages/Home'));
const Blocks = lazy(() => import('./pages/Blocks'));
const BlockDetail = lazy(() => import('./pages/BlockDetail'));
const Transactions = lazy(() => import('./pages/Transactions'));
const TxDetail = lazy(() => import('./pages/TxDetail'));
const Validators = lazy(() => import('./pages/Validators'));
const ValidatorDetail = lazy(() => import('./pages/ValidatorDetail'));
const Account = lazy(() => import('./pages/Account'));

function LoadingSpinner() {
  return (
    <div className="flex items-center justify-center min-h-[400px]">
      <div className="w-8 h-8 border-4 border-omniphi-500 border-t-transparent rounded-full animate-spin" />
    </div>
  );
}

function App() {
  return (
    <Layout>
      <Suspense fallback={<LoadingSpinner />}>
        <Routes>
          <Route path="/" element={<Home />} />
          <Route path="/blocks" element={<Blocks />} />
          <Route path="/block/:height" element={<BlockDetail />} />
          <Route path="/txs" element={<Transactions />} />
          <Route path="/tx/:hash" element={<TxDetail />} />
          <Route path="/validators" element={<Validators />} />
          <Route path="/validator/:address" element={<ValidatorDetail />} />
          <Route path="/account/:address" element={<Account />} />
        </Routes>
      </Suspense>
    </Layout>
  );
}

export default App;
