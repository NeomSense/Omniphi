# 🚀 Omniphi Local Validator App - Phase Implementation Guide

## ✅ Completed: Package Setup

**Dependencies Added:**
- `react-router-dom` - Routing
- `recharts` - Charts for rewards/metrics
- `clsx` - Conditional CSS classes
- `date-fns` - Date formatting
- `systeminformation` - System metrics
- `tailwindcss` + plugins - Styling

**Config Files Created:**
- ✅ `tailwind.config.js` - Omniphi purple/blue gradient theme
- ✅ `postcss.config.js` - PostCSS configuration
- ✅ `src/types/validator.ts` - Complete type definitions

## 📦 Installation Commands

```bash
cd validator-orchestrator/local-validator-app

# Install new dependencies
npm install

# This will install:
# - react-router-dom@^6.20.0
# - recharts@^2.10.3
# - clsx@^2.0.0
# - date-fns@^3.0.0
# - systeminformation@^5.21.0
# - tailwindcss + autoprefixer + postcss
```

---

## 🎯 PHASE 1: Core Dashboard (Priority 1)

### Step 1: Update index.css with Tailwind

**File:** `src/index.css`

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

### Step 2: Create Reusable UI Components

**File:** `src/components/ui/Badge.tsx`

```typescript
import { clsx } from 'clsx';

interface BadgeProps {
  variant: 'success' | 'warning' | 'error' | 'info';
  children: React.ReactNode;
  className?: string;
}

export function Badge({ variant, children, className }: BadgeProps) {
  return (
    <span
      className={clsx(
        'badge',
        {
          'badge-success': variant === 'success',
          'badge-warning': variant === 'warning',
          'badge-error': variant === 'error',
          'badge-info': variant === 'info',
        },
        className
      )}
    >
      {children}
    </span>
  );
}
```

**File:** `src/components/ui/Card.tsx`

```typescript
import { clsx } from 'clsx';

interface CardProps {
  children: React.ReactNode;
  title?: string;
  subtitle?: string;
  className?: string;
  dark?: boolean;
  actions?: React.ReactNode;
}

export function Card({ children, title, subtitle, className, dark, actions }: CardProps) {
  return (
    <div className={clsx(dark ? 'card-dark' : 'card', className)}>
      {(title || actions) && (
        <div className="flex items-center justify-between mb-4">
          <div>
            {title && (
              <h3 className={clsx(
                'text-lg font-semibold',
                dark ? 'text-white' : 'text-gray-900'
              )}>
                {title}
              </h3>
            )}
            {subtitle && (
              <p className={clsx(
                'text-sm mt-1',
                dark ? 'text-gray-300' : 'text-gray-500'
              )}>
                {subtitle}
              </p>
            )}
          </div>
          {actions && <div>{actions}</div>}
        </div>
      )}
      {children}
    </div>
  );
}
```

**File:** `src/components/ui/StatCard.tsx`

```typescript
import { Card } from './Card';
import { clsx } from 'clsx';

interface StatCardProps {
  label: string;
  value: string | number;
  subValue?: string;
  trend?: 'up' | 'down' | 'neutral';
  trendValue?: string;
  icon?: React.ReactNode;
  variant?: 'default' | 'gradient';
}

export function StatCard({
  label,
  value,
  subValue,
  trend,
  trendValue,
  icon,
  variant = 'default'
}: StatCardProps) {
  const isGradient = variant === 'gradient';

  return (
    <Card dark={isGradient} className="relative overflow-hidden">
      {icon && (
        <div className={clsx(
          'absolute top-4 right-4 opacity-20',
          isGradient ? 'text-white' : 'text-omniphi-500'
        )}>
          {icon}
        </div>
      )}

      <div className="relative">
        <p className={clsx(
          'text-sm font-medium mb-2',
          isGradient ? 'text-gray-200' : 'text-gray-500'
        )}>
          {label}
        </p>

        <p className={clsx(
          'text-3xl font-bold mb-1',
          isGradient ? 'text-white' : 'text-gray-900'
        )}>
          {value}
        </p>

        {subValue && (
          <p className={clsx(
            'text-sm',
            isGradient ? 'text-gray-300' : 'text-gray-600'
          )}>
            {subValue}
          </p>
        )}

        {trend && trendValue && (
          <div className="flex items-center mt-2 space-x-1">
            <span className={clsx(
              'text-sm font-medium',
              trend === 'up' && 'text-green-500',
              trend === 'down' && 'text-red-500',
              trend === 'neutral' && 'text-gray-500'
            )}>
              {trend === 'up' && '↑'}
              {trend === 'down' && '↓'}
              {trend === 'neutral' && '→'}
              {trendValue}
            </span>
          </div>
        )}
      </div>
    </Card>
  );
}
```

### Step 3: Create Enhanced Status Display

**File:** `src/components/dashboard/ValidatorStatusCard.tsx`

```typescript
import { Card } from '../ui/Card';
import { Badge } from '../ui/Badge';
import { StatCard } from '../ui/StatCard';
import { ValidatorStatus } from '../../types/validator';
import { formatDistance } from 'date-fns';

interface ValidatorStatusCardProps {
  status: ValidatorStatus | null;
  loading?: boolean;
}

export function ValidatorStatusCard({ status, loading }: ValidatorStatusCardProps) {
  if (loading || !status) {
    return (
      <Card title="Validator Status">
        <div className="animate-pulse space-y-4">
          <div className="h-24 bg-gray-200 rounded"></div>
          <div className="h-24 bg-gray-200 rounded"></div>
        </div>
      </Card>
    );
  }

  const getSyncBadge = () => {
    if (!status.running) return <Badge variant="error">Stopped</Badge>;
    if (status.syncing) return <Badge variant="warning">Syncing</Badge>;
    return <Badge variant="success">Synced</Badge>;
  };

  const getJailBadge = () => {
    if (status.jailed) return <Badge variant="error">Jailed</Badge>;
    if (status.is_active) return <Badge variant="success">Active</Badge>;
    return <Badge variant="warning">Inactive</Badge>;
  };

  return (
    <div className="space-y-4">
      {/* Hero Card */}
      <Card dark className="bg-gradient-omniphi">
        <div className="flex items-center justify-between">
          <div>
            <h2 className="text-2xl font-bold text-white mb-1">
              {status.moniker}
            </h2>
            <p className="text-omniphi-200">
              {status.chain_id}
            </p>
          </div>
          <div className="flex flex-col items-end space-y-2">
            {getSyncBadge()}
            {getJailBadge()}
          </div>
        </div>

        <div className="mt-6 grid grid-cols-2 gap-4">
          <div>
            <p className="text-omniphi-200 text-sm">Block Height</p>
            <p className="text-4xl font-bold text-white">
              {status.block_height.toLocaleString()}
            </p>
          </div>
          <div>
            <p className="text-omniphi-200 text-sm">Peers</p>
            <p className="text-4xl font-bold text-white">
              {status.peers}
            </p>
          </div>
        </div>

        {status.uptime > 0 && (
          <div className="mt-4 pt-4 border-t border-omniphi-400">
            <p className="text-omniphi-200 text-sm">
              Uptime: {formatDistance(0, status.uptime * 1000)}
            </p>
          </div>
        )}
      </Card>

      {/* Stats Grid */}
      <div className="grid grid-cols-3 gap-4">
        <StatCard
          label="Missed Blocks"
          value={status.missed_blocks}
          variant={status.missed_blocks > 10 ? 'default' : 'gradient'}
        />

        <StatCard
          label="Network"
          value={status.network_id}
          subValue="Network ID"
        />

        <StatCard
          label="Last Signature"
          value={status.last_signature
            ? formatDistance(new Date(status.last_signature), new Date(), { addSuffix: true })
            : 'Never'
          }
        />
      </div>
    </div>
  );
}
```

### Step 4: Create Node Health Card

**File:** `src/components/dashboard/NodeHealthCard.tsx`

```typescript
import { Card } from '../ui/Card';
import { NodeHealth } from '../../types/validator';

interface NodeHealthCardProps {
  health: NodeHealth | null;
}

export function NodeHealthCard({ health }: NodeHealthCardProps) {
  if (!health) {
    return <Card title="Node Health"><p className="text-gray-500">No data available</p></Card>;
  }

  const getHealthColor = (percent: number) => {
    if (percent > 90) return 'text-red-600';
    if (percent > 75) return 'text-yellow-600';
    return 'text-green-600';
  };

  return (
    <Card title="Node Health Metrics">
      <div className="space-y-4">
        {/* CPU */}
        <div>
          <div className="flex justify-between mb-2">
            <span className="text-sm font-medium text-gray-700">CPU Usage</span>
            <span className={`text-sm font-bold ${getHealthColor(health.cpu)}`}>
              {health.cpu.toFixed(1)}%
            </span>
          </div>
          <div className="w-full bg-gray-200 rounded-full h-2">
            <div
              className="bg-omniphi-600 h-2 rounded-full transition-all duration-300"
              style={{ width: `${Math.min(health.cpu, 100)}%` }}
            />
          </div>
        </div>

        {/* RAM */}
        <div>
          <div className="flex justify-between mb-2">
            <span className="text-sm font-medium text-gray-700">RAM Usage</span>
            <span className={`text-sm font-bold ${getHealthColor(health.ram_percent)}`}>
              {health.ram}
            </span>
          </div>
          <div className="w-full bg-gray-200 rounded-full h-2">
            <div
              className="bg-omniphi-600 h-2 rounded-full transition-all duration-300"
              style={{ width: `${Math.min(health.ram_percent, 100)}%` }}
            />
          </div>
        </div>

        {/* Disk */}
        <div>
          <div className="flex justify-between mb-2">
            <span className="text-sm font-medium text-gray-700">Disk Usage</span>
            <span className={`text-sm font-bold ${getHealthColor(health.disk_percent)}`}>
              {health.disk}
            </span>
          </div>
          <div className="w-full bg-gray-200 rounded-full h-2">
            <div
              className="bg-omniphi-600 h-2 rounded-full transition-all duration-300"
              style={{ width: `${Math.min(health.disk_percent, 100)}%` }}
            />
          </div>
        </div>

        {/* Network & Ports */}
        <div className="grid grid-cols-2 gap-4 pt-4 border-t">
          <div>
            <p className="text-xs text-gray-500">Network In</p>
            <p className="text-sm font-semibold text-gray-900">{health.net_in}</p>
          </div>
          <div>
            <p className="text-xs text-gray-500">Network Out</p>
            <p className="text-sm font-semibold text-gray-900">{health.net_out}</p>
          </div>
          <div>
            <p className="text-xs text-gray-500">RPC Port</p>
            <p className="text-sm font-semibold text-gray-900">{health.rpc_port}</p>
          </div>
          <div>
            <p className="text-xs text-gray-500">P2P Port</p>
            <p className="text-sm font-semibold text-gray-900">{health.p2p_port}</p>
          </div>
        </div>

        {/* Node ID */}
        <div className="pt-2">
          <p className="text-xs text-gray-500">Node ID</p>
          <p className="text-xs font-mono text-gray-700 break-all">{health.node_id}</p>
        </div>
      </div>
    </Card>
  );
}
```

---

## 📁 File Structure

```
local-validator-app/
├── src/
│   ├── components/
│   │   ├── ui/                      # Reusable UI components
│   │   │   ├── Badge.tsx
│   │   │   ├── Card.tsx
│   │   │   ├── StatCard.tsx
│   │   │   └── Button.tsx
│   │   ├── dashboard/               # Dashboard-specific components
│   │   │   ├── ValidatorStatusCard.tsx
│   │   │   ├── NodeHealthCard.tsx
│   │   │   ├── ValidatorMetadataCard.tsx
│   │   │   ├── RewardsPanel.tsx
│   │   │   ├── PoCScorePanel.tsx
│   │   │   └── UpgradeNotification.tsx
│   │   ├── keys/                    # Keys management components
│   │   │   ├── ConsensusKeyDisplay.tsx
│   │   │   └── KeyExport.tsx
│   │   ├── logs/                    # Log viewer components
│   │   │   ├── LogViewer.tsx
│   │   │   └── LogFilter.tsx
│   │   └── settings/                # Settings components
│   │       ├── GeneralSettings.tsx
│   │       ├── NetworkSettings.tsx
│   │       └── HeartbeatSettings.tsx
│   ├── pages/
│   │   ├── Dashboard.tsx
│   │   ├── Keys.tsx
│   │   ├── Logs.tsx
│   │   └── Settings.tsx
│   ├── hooks/
│   │   ├── useValidatorStatus.ts
│   │   ├── useNodeHealth.ts
│   │   └── useDashboardData.ts
│   ├── services/
│   │   └── api.ts                   # IPC communication
│   └── types/
│       └── validator.ts              # ✅ Created
├── electron/
│   ├── main.js
│   ├── preload.js
│   ├── ipc-handlers.js              # Enhanced with new endpoints
│   └── http-bridge.js
├── tailwind.config.js                # ✅ Created
├── postcss.config.js                 # ✅ Created
└── package.json                      # ✅ Updated
```

---

## 🔄 Next Steps

**To continue implementation, run:**

```bash
npm install
```

Then I'll create:
1. Dashboard layout with routing
2. API service for IPC communication
3. Custom hooks for data fetching
4. Enhanced IPC handlers in Electron

**Continue to Phase 1 Part 2?** Reply "continue" and I'll build the routing, API layer, and complete Dashboard page.
