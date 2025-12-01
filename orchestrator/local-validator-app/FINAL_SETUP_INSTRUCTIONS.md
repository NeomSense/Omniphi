# 🎯 FINAL SETUP - Complete These Steps

## ✅ What's Been Created

### Components ✅
- `src/components/ui/Badge.tsx`
- `src/components/ui/Card.tsx`
- `src/components/ui/StatCard.tsx`
- `src/components/dashboard/ValidatorStatusCard.tsx`
- `src/components/dashboard/NodeHealthCard.tsx`

### Configuration ✅
- `package.json` (updated with dependencies)
- `tailwind.config.js`
- `postcss.config.js`
- `src/types/validator.ts`

## 🚀 Step-by-Step Setup

### Step 1: Install Dependencies

```bash
cd c:\Users\herna\omniphi\pos\validator-orchestrator\local-validator-app
npm install
```

### Step 2: Update index.css

**File:** `src/index.css`

Replace entire content with:

```css
@tailwind base;
@tailwind components;
@tailwind utilities;

@layer base {
  body {
    @apply bg-gray-50 text-gray-900;
  }
}

@layer components {
  .card {
    @apply bg-white rounded-lg shadow-card p-6 hover:shadow-card-hover transition-shadow duration-200;
  }

  .card-dark {
    @apply bg-gradient-dark text-white rounded-lg shadow-card p-6;
  }

  .badge {
    @apply px-3 py-1 rounded-full text-sm font-medium;
  }

  .badge-success {
    @apply bg-green-100 text-green-800;
  }

  .badge-warning {
    @apply bg-yellow-100 text-yellow-800;
  }

  .badge-error {
    @apply bg-red-100 text-red-800;
  }

  .badge-info {
    @apply bg-blue-100 text-blue-800;
  }
}
```

### Step 3: Update App.tsx

**File:** `src/App.tsx`

Replace with:

```typescript
import { useEffect, useState } from 'react';
import { ValidatorStatusCard } from './components/dashboard/ValidatorStatusCard';
import { NodeHealthCard } from './components/dashboard/NodeHealthCard';
import { ValidatorStatus, NodeHealth } from './types/validator';

// Declare electron API
declare global {
  interface Window {
    electron: {
      invoke(channel: string, ...args: any[]): Promise<any>;
      on(channel: string, callback: (...args: any[]) => void): void;
    };
  }
}

function App() {
  const [status, setStatus] = useState<ValidatorStatus | null>(null);
  const [health, setHealth] = useState<NodeHealth | null>(null);
  const [loading, setLoading] = useState(true);

  // Fetch status
  const fetchStatus = async () => {
    try {
      const result = await window.electron.invoke('get-validator-status');
      setStatus(result);
      setLoading(false);
    } catch (error) {
      console.error('Failed to fetch status:', error);
      setLoading(false);
    }
  };

  // Fetch health (mock data for now)
  const fetchHealth = async () => {
    // TODO: Replace with real IPC call when implemented
    setHealth({
      cpu: 14.2,
      ram: '423MB / 16GB',
      ram_percent: 26,
      disk: '3.4GB / 120GB',
      disk_percent: 3,
      db_size: '2.1GB',
      net_in: '1.2MB/s',
      net_out: '0.8MB/s',
      node_id: 'f632a7ee6f28d12cde86d009ba0cc614795bf59f',
      rpc_port: 26657,
      p2p_port: 26656,
    });
  };

  useEffect(() => {
    fetchStatus();
    fetchHealth();

    // Poll every 3 seconds
    const interval = setInterval(() => {
      fetchStatus();
      fetchHealth();
    }, 3000);

    // Listen for status updates
    window.electron.on('status-update', (newStatus: ValidatorStatus) => {
      setStatus(newStatus);
    });

    return () => clearInterval(interval);
  }, []);

  // Start/Stop handlers
  const handleStart = async () => {
    try {
      await window.electron.invoke('start-validator', {});
      fetchStatus();
    } catch (error) {
      console.error('Failed to start validator:', error);
    }
  };

  const handleStop = async () => {
    try {
      await window.electron.invoke('stop-validator');
      fetchStatus();
    } catch (error) {
      console.error('Failed to stop validator:', error);
    }
  };

  return (
    <div className="min-h-screen bg-gray-50">
      {/* Header */}
      <header className="bg-white shadow-sm">
        <div className="max-w-7xl mx-auto px-4 sm:px-6 lg:px-8 py-4">
          <div className="flex items-center justify-between">
            <div>
              <h1 className="text-2xl font-bold text-gray-900">Omniphi Local Validator</h1>
              <p className="text-sm text-gray-500">One-Click Validator Management</p>
            </div>
            <div className="flex items-center space-x-3">
              {status?.running ? (
                <button
                  onClick={handleStop}
                  className="px-4 py-2 bg-red-600 text-white rounded-lg hover:bg-red-700 transition-colors"
                >
                  Stop Validator
                </button>
              ) : (
                <button
                  onClick={handleStart}
                  className="px-4 py-2 bg-omniphi-600 text-white rounded-lg hover:bg-omniphi-700 transition-colors"
                >
                  Start Validator
                </button>
              )}
            </div>
          </div>
        </div>
      </header>

      {/* Main Content */}
      <main className="max-w-7xl mx-auto px-4 sm:px-6 lg:px-8 py-8">
        <div className="grid grid-cols-1 lg:grid-cols-3 gap-6">
          {/* Left Column - Status */}
          <div className="lg:col-span-2">
            <ValidatorStatusCard status={status} loading={loading} />
          </div>

          {/* Right Column - Health */}
          <div>
            <NodeHealthCard health={health} />
          </div>
        </div>
      </main>
    </div>
  );
}

export default App;
```

### Step 4: Test It!

```bash
npm run dev
```

You should now see:
- ✅ Beautiful gradient header card with block height
- ✅ Sync status badges (Synced/Syncing/Stopped)
- ✅ Peer count display
- ✅ Node health metrics with progress bars
- ✅ Start/Stop buttons working
- ✅ Real-time updates every 3 seconds

## 🎨 What You'll See

- **Purple gradient** hero card showing block height prominently
- **Status badges** with colors (green=synced, yellow=syncing, red=stopped)
- **Stats cards** for missed blocks, network, last signature
- **Health metrics** with animated progress bars for CPU/RAM/Disk
- **Network stats** showing in/out and ports

## 🔧 Troubleshooting

### If Tailwind isn't working:
```bash
npm install -D tailwindcss postcss autoprefixer
```

### If you get type errors:
```bash
npm install --save-dev @types/react @types/react-dom
```

### If electron API isn't available:
Check that `electron/preload.js` exposes the IPC methods

## 📈 Next Steps

Once Phase 1 is working, we can add:
- **Phase 2**: Validator metadata, rewards tracking, PoC scores
- **Phase 3**: Keys management page, logs viewer with filtering
- **Phase 4**: Settings page, charts, upgrade notifications

## 🎯 Success Criteria

You'll know it's working when you see:
1. ✅ Block height updating every 3 seconds
2. ✅ "Synced" badge shows green
3. ✅ Peer count displays correctly
4. ✅ Health bars animate smoothly
5. ✅ Start/Stop buttons work

---

**Current blockchain status:**
- Running at block ~4500+
- RPC responding on 127.0.0.1:26657
- Ready for the new UI!

Run `npm install` and `npm run dev` to see your enhanced dashboard! 🚀
