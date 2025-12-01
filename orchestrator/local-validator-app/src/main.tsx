/**
 * Omniphi Local Validator - Application Entry Point
 *
 * Choose between two routing modes:
 * - App: State-based navigation (simpler, works everywhere)
 * - AppRouter: React Router with hash-based URLs (deep linking support)
 */

import React from 'react'
import ReactDOM from 'react-dom/client'
import App from './App.tsx'
import './index.css'

// Toggle this to use React Router instead of state-based navigation
const USE_REACT_ROUTER = false;

async function renderApp() {
  if (USE_REACT_ROUTER) {
    // Dynamically import router to avoid loading if not needed
    const { AppRouter } = await import('./router.tsx');
    ReactDOM.createRoot(document.getElementById('root')!).render(
      <React.StrictMode>
        <AppRouter />
      </React.StrictMode>,
    );
  } else {
    ReactDOM.createRoot(document.getElementById('root')!).render(
      <React.StrictMode>
        <App />
      </React.StrictMode>,
    );
  }
}

renderApp();
