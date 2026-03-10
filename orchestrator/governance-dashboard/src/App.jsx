import { BrowserRouter, Routes, Route } from 'react-router-dom';
import Layout from './components/Layout';
import Home from './pages/Home';
import ProposalPage from './pages/ProposalPage';

export default function App() {
  return (
    <BrowserRouter>
      <Routes>
        <Route element={<Layout />}>
          <Route index element={<Home />} />
          <Route path="proposal/:id" element={<ProposalPage />} />
        </Route>
      </Routes>
    </BrowserRouter>
  );
}
