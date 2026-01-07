# Omniphi Block Explorer

A modern, responsive block explorer for the Omniphi blockchain.

## Features

- Real-time block updates
- Transaction search and details
- Validator information and statistics
- Account balance and staking overview
- Network statistics dashboard
- Mobile-responsive design

## Quick Start

### Development

```bash
# Install dependencies
npm install

# Start dev server
npm run dev
```

The explorer will be available at http://localhost:3000

### Production Build

```bash
# Build for production
npm run build

# Preview production build
npm run preview
```

### Docker Deployment

```bash
# Build and run
docker build -t omniphi-explorer .
docker run -p 3000:80 omniphi-explorer
```

## Configuration

Create a `.env` file for environment-specific settings:

```bash
VITE_API_URL=http://your-node-ip:1318
VITE_RPC_URL=http://your-node-ip:26657
```

## Pages

- **Dashboard** (`/`) - Network overview with recent blocks and validators
- **Blocks** (`/blocks`) - List of recent blocks
- **Block Detail** (`/block/:height`) - Block information and transactions
- **Validators** (`/validators`) - Active validator list
- **Validator Detail** (`/validator/:address`) - Validator information
- **Account** (`/account/:address`) - Account balances and delegations
- **Transaction** (`/tx/:hash`) - Transaction details

## Tech Stack

- React 18 with TypeScript
- Vite for fast development
- TailwindCSS for styling
- React Query for data fetching
- React Router for navigation

## License

MIT License - Omniphi Network
