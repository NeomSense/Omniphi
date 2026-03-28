# Omniphi Network Architecture

This document describes the network topology, port assignments, firewall rules, and deployment patterns for the Omniphi blockchain. It covers single-validator development setups through production sentry-node architectures with geographic distribution.

---

## Table of Contents

1. [Architecture Overview](#architecture-overview)
2. [Single Validator Setup (Development)](#single-validator-setup-development)
3. [Sentry Node Architecture (Production)](#sentry-node-architecture-production)
4. [Node Types](#node-types)
5. [Peer Discovery: Seeds vs Persistent Peers](#peer-discovery-seeds-vs-persistent-peers)
6. [Network Diagrams](#network-diagrams)
7. [Port Reference](#port-reference)
8. [Firewall Rules by Node Type](#firewall-rules-by-node-type)
9. [DNS and Load Balancer Setup](#dns-and-load-balancer-setup)
10. [Geographic Distribution](#geographic-distribution)

---

## Architecture Overview

Omniphi runs a dual-layer architecture:

- **Go Consensus Layer** (`posd`) -- CometBFT-based PoS consensus, custom modules (PoC, PoR, RewardMult, Guard, Timelock, Tokenomics, Feemarket, RepGov, Royalty, UCI), IBC interoperability.
- **Rust Sequencer Layer** (`poseq-node`) -- Proof of Sequencing for fair transaction ordering, anti-MEV, and batch attestation. Communicates with the Go layer via a file-based bridge (ExportBatch JSON) and the `x/poseq` on-chain module.

Both layers have their own P2P networks, metrics endpoints, and storage.

```
User / dApp / Wallet
       |
       |  REST (1317) / gRPC (9090) / RPC (26657)
       v
+----------------------------------------------+
|           Go Consensus Layer (posd)           |
|  CometBFT P2P (:26656)                       |
|  Modules: staking, gov, poc, por, rewardmult, |
|           guard, timelock, feemarket,          |
|           tokenomics, repgov, royalty, uci,    |
|           poseq, contracts                     |
|  Prometheus (:26660)                          |
+----------------------------------------------+
       ^
       |  ExportBatch JSON / ChainCommitteeSnapshot
       |  (file bridge or relayer)
       v
+----------------------------------------------+
|         Rust Sequencer Layer (poseq-node)     |
|  PoSeq TCP P2P (:7001)                       |
|  Fair ordering, anti-MEV, batch attestation   |
|  Prometheus / Health / Status (:9191)         |
|  sled durable storage                         |
+----------------------------------------------+
```

---

## Single Validator Setup (Development)

For local development and testing, run everything on a single machine. This is the simplest topology and is suitable for:
- Local testnet development
- CI/CD pipeline testing
- Learning the system

```
+------------------------------------------------------------------+
|  Single Machine (localhost)                                       |
|                                                                   |
|  +--------------------------+  +----------------------------+    |
|  |  posd (Go chain)         |  |  poseq-node (Rust PoSeq)   |    |
|  |  P2P:     127.0.0.1:26656|  |  P2P:     127.0.0.1:7001   |    |
|  |  RPC:     127.0.0.1:26657|  |  Metrics: 127.0.0.1:9191   |    |
|  |  API:     127.0.0.1:1317 |  |  Health:  127.0.0.1:9191   |    |
|  |  gRPC:    127.0.0.1:9090 |  |  Data:    ~/.pos/poseq/data|    |
|  |  Metrics: 127.0.0.1:26660|  +----------------------------+    |
|  |  Home:    ~/.pos/        |                                     |
|  +--------------------------+                                     |
+------------------------------------------------------------------+
```

### Quick Start (Single Validator)

```bash
# Initialize
posd init "dev-validator" --chain-id omniphi-testnet-dev

# Configure genesis (add validator account, set feemarket treasury_address)
posd keys add validator --keyring-backend test
VALIDATOR_ADDR=$(posd keys show validator --keyring-backend test -a)
posd genesis add-genesis-account $VALIDATOR_ADDR 1000000000000omniphi
posd genesis gentx validator 500000000000omniphi \
    --chain-id omniphi-testnet-dev \
    --keyring-backend test
posd genesis collect-gentxs

# Fix guard genesis if needed
go run scripts/fix_guard_genesis.go ~/.pos/config/genesis.json $VALIDATOR_ADDR

# Start
posd start
```

---

## Sentry Node Architecture (Production)

Production validators must never be directly exposed to the public P2P network. The sentry node architecture places the validator behind a layer of full nodes (sentries) that relay blocks and transactions.

### Why Sentries?

- **DDoS protection:** Attackers cannot target the validator directly because its IP is never advertised.
- **Privacy:** The validator's network identity (`node_id`) is marked as private on sentry nodes.
- **Redundancy:** If one sentry goes down, the validator stays connected through others.
- **Geographic flexibility:** Sentries can be placed in multiple regions to reduce latency.

### Minimum Production Topology

At least 2 sentry nodes in different datacenters, connected to the validator via a private network (VPN, VPC peering, or private IP range).

```
                        Public Internet
                             |
                +------------+------------+
                |                         |
         +-----------+            +-----------+
         | Sentry A  |            | Sentry B  |
         | Region: US|            | Region: EU|
         | Public IP |            | Public IP |
         |           |            |           |
         | CometBFT  |            | CometBFT  |
         | P2P :26656|            | P2P :26656|
         | RPC :26657|            | RPC :26657|  (optional, for API)
         | API :1317 |            | API :1317 |  (optional, for API)
         |           |            |           |
         | PoSeq     |            | PoSeq     |
         | P2P :7001 |            | P2P :7001 |  (optional)
         +-----------+            +-----------+
                |    Private       |
                |    Network       |
                |  (10.0.0.0/24)  |
                +--------+--------+
                         |
                  +-------------+
                  |  Validator  |
                  |  NO public  |
                  |  IP address |
                  |             |
                  | CometBFT    |
                  | P2P :26656  |  (private only)
                  | RPC :26657  |  (localhost only)
                  |             |
                  | PoSeq       |
                  | P2P :7001   |  (private only)
                  +-------------+
```

### Sentry Node Configuration

On each sentry node, `~/.pos/config/config.toml`:

```toml
[p2p]
# Public listen address — accept connections from the internet
laddr = "tcp://0.0.0.0:26656"

# Connect to the validator via the private network
persistent_peers = "<validator-node-id>@10.0.0.10:26656"

# CRITICAL: Hide the validator's node ID from the public network
# This prevents other nodes from trying to connect to the validator directly
private_peer_ids = "<validator-node-id>"

# Enable peer exchange for public peer discovery
pex = true

# Seed nodes for initial peer discovery
seeds = "seed1-id@seed1.omniphi.io:26656,seed2-id@seed2.omniphi.io:26656"

# Accept many public peers
max_num_inbound_peers = 100
max_num_outbound_peers = 20
```

### Validator Node Configuration (Behind Sentries)

On the validator, `~/.pos/config/config.toml`:

```toml
[p2p]
# Listen on private network interface only
laddr = "tcp://10.0.0.10:26656"

# ONLY connect to your own sentry nodes — never public peers
persistent_peers = "<sentry-a-id>@10.0.0.11:26656,<sentry-b-id>@10.0.0.12:26656"

# Disable peer exchange — the validator must not discover or advertise to public peers
pex = false

# Do not enforce strict address book rules (private network)
addr_book_strict = false

# No seeds — the validator only talks to sentries
seeds = ""
```

---

## Node Types

### Full Node

A full node participates in the P2P network, validates all transactions and blocks, and maintains the current state. It prunes old blocks.

| Setting | Value |
|---------|-------|
| Pruning | `default` or `custom` (keep-recent=100) |
| State sync | Enabled for fast catch-up |
| Disk usage | ~50-100 GB (after pruning) |
| Use case | API endpoints, sentry nodes, general participation |

**app.toml pruning settings:**
```toml
pruning = "custom"
pruning-keep-recent = "362880"  # ~25 days at 6s blocks
pruning-interval = "100"
```

### Archive Node

An archive node retains all historical state at every block height. Required for block explorers, indexers, and historical queries.

| Setting | Value |
|---------|-------|
| Pruning | `nothing` |
| State sync | Disabled (must replay from genesis or restore from snapshot) |
| Disk usage | 500 GB+ (grows continuously) |
| Use case | Block explorers, analytics, historical API queries |

**app.toml pruning settings:**
```toml
pruning = "nothing"
```

**config.toml indexer settings:**
```toml
[tx_index]
indexer = "kv"
```

### Pruned Node

A pruned node aggressively removes old state to minimize disk usage. Suitable for validators that do not serve API queries.

| Setting | Value |
|---------|-------|
| Pruning | `everything` or aggressive custom |
| Disk usage | ~20-50 GB |
| Use case | Validators (behind sentries), lightweight participation |

**app.toml pruning settings:**
```toml
pruning = "custom"
pruning-keep-recent = "100"
pruning-interval = "10"
```

### Comparison Table

| Property | Full Node | Archive Node | Pruned Node |
|----------|-----------|-------------|-------------|
| Validates blocks | Yes | Yes | Yes |
| Serves current queries | Yes | Yes | Yes |
| Serves historical queries | Limited | All heights | No |
| Disk growth | ~1 GB/day | ~5 GB/day | ~0.2 GB/day |
| Minimum disk | 100 GB | 1 TB+ | 50 GB |
| State sync | Yes | No | Yes |
| API/RPC serving | Recommended | Required | Not recommended |

---

## Peer Discovery: Seeds vs Persistent Peers

### Seed Nodes

Seed nodes exist solely to bootstrap peer discovery. When your node connects to a seed, it receives a list of active peers, then disconnects. Seeds do not maintain long-lived connections.

**Behavior:**
1. Node connects to seed on startup.
2. Seed returns a list of known peers.
3. Node disconnects from seed.
4. Node connects to the discovered peers.

**config.toml:**
```toml
[p2p]
seeds = "seed1-id@seed1.omniphi.io:26656,seed2-id@seed2.omniphi.io:26656"
```

### Persistent Peers

Persistent peers are nodes your node will always try to stay connected to, reconnecting automatically if the connection drops. Use these for:
- Validator-to-sentry connections (critical)
- Connections to trusted operators
- Ensuring minimum peer diversity

**config.toml:**
```toml
[p2p]
persistent_peers = "peer1-id@peer1.omniphi.io:26656,peer2-id@peer2.omniphi.io:26656"
```

### Unconditional Peers

Unconditional peers bypass the peer count limits. Even if `max_num_inbound_peers` is reached, connections from unconditional peers are accepted. Use for sentry-to-validator links.

**config.toml:**
```toml
[p2p]
unconditional_peer_ids = "<validator-node-id>"
```

### Peer Configuration Summary

| Node Type | seeds | persistent_peers | pex | private_peer_ids |
|-----------|-------|-------------------|-----|------------------|
| Validator (behind sentries) | empty | sentry IDs only | false | empty |
| Sentry node | seed list | validator ID | true | validator ID |
| Full node (public) | seed list | trusted peers | true | empty |
| Seed node | other seeds | empty | true | empty |

---

## Network Diagrams

### Full Production Network Topology

```
                              Internet Users / dApps
                                      |
                                      |  HTTPS
                                      v
                            +-------------------+
                            |  Load Balancer /  |
                            |  Cloudflare / DNS |
                            |  rpc.omniphi.io   |
                            |  api.omniphi.io   |
                            +-------------------+
                                |           |
                   +------------+           +------------+
                   |                                     |
            +-------------+                       +-------------+
            | RPC Node A  |                       | RPC Node B  |
            | (Full Node) |                       | (Full Node) |
            | US-East     |                       | EU-West     |
            |             |                       |             |
            | :26657 RPC  |                       | :26657 RPC  |
            | :1317  API  |                       | :1317  API  |
            | :9090  gRPC |                       | :9090  gRPC |
            | :26656 P2P  |                       | :26656 P2P  |
            +------+------+                       +------+------+
                   |                                     |
                   |        Public P2P Network           |
                   |     (CometBFT gossip :26656)        |
                   |                                     |
    +--------------+----------+-----------+--------------+
    |              |          |           |              |
+---+---+    +----+----+ +---+---+  +----+----+   +----+----+
|Sentry |    |Sentry   | |Sentry |  |Sentry   |   |Other    |
|  A-1  |    |  A-2    | |  B-1  |  |  B-2    |   |Full     |
|US-East|    |US-West  | |EU-West|  |EU-East  |   |Nodes    |
+---+---+    +----+----+ +---+---+  +----+----+   +---------+
    |              |          |           |
    +--------------+          +-----------+
           |                        |
     Private Net A            Private Net B
     (10.0.1.0/24)           (10.0.2.0/24)
           |                        |
    +------+------+          +------+------+
    | Validator A |          | Validator B |
    | US-East     |          | EU-West     |
    | (no public  |          | (no public  |
    |  IP)        |          |  IP)        |
    +------+------+          +------+------+
           |                        |
    +------+------+          +------+------+
    | PoSeq Node  |          | PoSeq Node  |
    | (co-located)|          | (co-located)|
    +-------------+          +-------------+
```

### PoSeq Layer Topology

The PoSeq nodes form their own gossip network, separate from the CometBFT P2P layer:

```
+---------------------------------------------------------------------+
|  PoSeq P2P Network (TCP gossip, port 7001)                          |
|                                                                      |
|  PoSeq-1 :7001  <-------->  PoSeq-2 :7001                          |
|      ^                           ^                                   |
|      |        Full Mesh          |                                   |
|      v                           v                                   |
|  PoSeq-3 :7001  <-------->  PoSeq-4 :7001  <----->  PoSeq-5 :7001  |
|                                                                      |
|  Quorum: 3/5                                                         |
|                                                                      |
|  Export path:   /data/poseq/exports/*.json                           |
|  Snapshot path: /data/poseq/snapshots/*.json                         |
+---------------------------------------------------------------------+
         |
         |  ExportBatch JSON (file bridge or relayer binary)
         v
+---------------------------------------------------------------------+
|  Go Chain — x/poseq module: IngestExportBatch                        |
|  Validates batch commitments, updates on-chain PoSeq state           |
+---------------------------------------------------------------------+
```

---

## Port Reference

### Go Consensus Node (posd)

| Port | Protocol | Service | Default Bind | Description |
|------|----------|---------|--------------|-------------|
| 26656 | TCP | CometBFT P2P | `0.0.0.0:26656` | Block and transaction gossip between nodes |
| 26657 | TCP | CometBFT RPC | `127.0.0.1:26657` | JSON-RPC for querying blocks, txs, consensus state |
| 26660 | TCP | Prometheus | `127.0.0.1:26660` | CometBFT and application metrics |
| 1317 | TCP | REST API | `127.0.0.1:1317` | Cosmos SDK REST / LCD endpoints |
| 9090 | TCP | gRPC | `127.0.0.1:9090` | Cosmos SDK gRPC query and tx endpoints |
| 9091 | TCP | gRPC-Web | `127.0.0.1:9091` | Browser-compatible gRPC (disabled by default) |

### Rust PoSeq Node (poseq-node)

| Port | Protocol | Service | Default Bind | Description |
|------|----------|---------|--------------|-------------|
| 7001 | TCP | PoSeq P2P | `0.0.0.0:7001` | Batch proposals, attestation votes, peer gossip |
| 9191 | TCP | Prometheus / HTTP | `127.0.0.1:9191` | `/metrics`, `/healthz`, `/status` endpoints |

### Monitoring (Infrastructure)

| Port | Protocol | Service | Default Bind | Description |
|------|----------|---------|--------------|-------------|
| 9100 | TCP | Node Exporter | `127.0.0.1:9100` | System metrics (CPU, memory, disk, network) |
| 9090 | TCP | Prometheus Server | N/A (monitoring host) | Time-series metrics database |
| 3000 | TCP | Grafana | N/A (monitoring host) | Dashboard and visualization |
| 9093 | TCP | AlertManager | N/A (monitoring host) | Alert routing and notification |

### Port Assignment for Multi-Node Devnet

For running multiple nodes on a single machine (devnet/testnet):

| Node | CometBFT P2P | CometBFT RPC | API | gRPC | Prometheus | PoSeq P2P | PoSeq Metrics |
|------|-------------|-------------|-----|------|-----------|----------|--------------|
| Node 1 | 26656 | 26657 | 1317 | 9090 | 26660 | 7001 | 9191 |
| Node 2 | 26666 | 26667 | 1318 | 9092 | 26670 | 7002 | 9192 |
| Node 3 | 26676 | 26677 | 1319 | 9094 | 26680 | 7003 | 9193 |

---

## Firewall Rules by Node Type

### Validator Node (Behind Sentries)

The validator should have NO public-facing ports. All traffic goes through the private network.

```bash
# UFW rules for validator (private network only)
sudo ufw default deny incoming
sudo ufw default allow outgoing

# SSH from admin network only
sudo ufw allow from 10.0.0.0/24 to any port 22 proto tcp comment 'SSH from private net'

# CometBFT P2P from sentries only
sudo ufw allow from 10.0.0.11 to any port 26656 proto tcp comment 'Sentry A CometBFT'
sudo ufw allow from 10.0.0.12 to any port 26656 proto tcp comment 'Sentry B CometBFT'

# PoSeq P2P from sentries only (if sentries relay PoSeq traffic)
sudo ufw allow from 10.0.0.11 to any port 7001 proto tcp comment 'Sentry A PoSeq'
sudo ufw allow from 10.0.0.12 to any port 7001 proto tcp comment 'Sentry B PoSeq'

# Prometheus scraping from monitoring server
sudo ufw allow from 10.0.0.100 to any port 26660 proto tcp comment 'Prometheus CometBFT'
sudo ufw allow from 10.0.0.100 to any port 9191 proto tcp comment 'Prometheus PoSeq'
sudo ufw allow from 10.0.0.100 to any port 9100 proto tcp comment 'Prometheus Node Exporter'

sudo ufw enable
```

### Sentry Node

Sentries expose P2P ports to the public internet and relay to the validator on the private network.

```bash
# UFW rules for sentry node
sudo ufw default deny incoming
sudo ufw default allow outgoing

# SSH
sudo ufw allow 22/tcp comment 'SSH'

# CometBFT P2P — public
sudo ufw allow 26656/tcp comment 'CometBFT P2P (public)'

# PoSeq P2P — public (if this sentry also relays PoSeq)
sudo ufw allow 7001/tcp comment 'PoSeq P2P (public)'

# DO NOT expose these on a sentry unless serving public API:
# 26657 (RPC), 1317 (API), 9090 (gRPC)

# Prometheus scraping from monitoring server
sudo ufw allow from 10.0.0.100 to any port 26660 proto tcp comment 'Prometheus'
sudo ufw allow from 10.0.0.100 to any port 9100 proto tcp comment 'Node Exporter'

sudo ufw enable
```

### Public RPC / API Node

Full nodes that serve public queries need additional ports open.

```bash
# UFW rules for public RPC/API node
sudo ufw default deny incoming
sudo ufw default allow outgoing

# SSH
sudo ufw allow 22/tcp comment 'SSH'

# CometBFT P2P — public
sudo ufw allow 26656/tcp comment 'CometBFT P2P'

# Public query endpoints (behind reverse proxy / load balancer recommended)
sudo ufw allow 26657/tcp comment 'CometBFT RPC'
sudo ufw allow 1317/tcp comment 'REST API'
sudo ufw allow 9090/tcp comment 'gRPC'

# PoSeq P2P (if participating)
sudo ufw allow 7001/tcp comment 'PoSeq P2P'

# Prometheus (internal only)
sudo ufw allow from 10.0.0.100 to any port 26660 proto tcp comment 'Prometheus'
sudo ufw allow from 10.0.0.100 to any port 9100 proto tcp comment 'Node Exporter'

sudo ufw enable
```

### Firewall Summary Table

| Port | Validator | Sentry | RPC Node | Archive Node | Seed Node |
|------|-----------|--------|----------|-------------|-----------|
| 22 (SSH) | Private only | Public | Public | Public | Public |
| 26656 (P2P) | Private only | Public | Public | Public | Public |
| 26657 (RPC) | Localhost | Blocked | Public | Public | Blocked |
| 1317 (API) | Localhost | Blocked | Public | Public | Blocked |
| 9090 (gRPC) | Localhost | Blocked | Public | Public | Blocked |
| 26660 (Metrics) | Private only | Private only | Private only | Private only | Private only |
| 7001 (PoSeq) | Private only | Public | Optional | Blocked | Blocked |
| 9191 (PoSeq Metrics) | Localhost | Localhost | Localhost | N/A | N/A |
| 9100 (Node Exporter) | Private only | Private only | Private only | Private only | Private only |

---

## DNS and Load Balancer Setup

### DNS Records

Set up the following DNS records for your public-facing infrastructure:

```
# A records pointing to your RPC/API nodes or load balancer
rpc.omniphi.io      A    <lb-ip-address>
api.omniphi.io      A    <lb-ip-address>
grpc.omniphi.io     A    <lb-ip-address>

# Seed nodes (stable, long-lived)
seed1.omniphi.io    A    <seed1-ip>
seed2.omniphi.io    A    <seed2-ip>

# Per-region endpoints (optional, for latency-aware routing)
rpc-us.omniphi.io   A    <us-rpc-ip>
rpc-eu.omniphi.io   A    <eu-rpc-ip>
rpc-ap.omniphi.io   A    <ap-rpc-ip>
```

### Nginx Reverse Proxy for RPC

Deploy Nginx on each RPC node to add TLS, rate limiting, and request filtering.

Create `/etc/nginx/sites-available/omniphi-rpc`:

```nginx
# Rate limiting zones
limit_req_zone $binary_remote_addr zone=rpc_limit:10m rate=30r/s;
limit_req_zone $binary_remote_addr zone=api_limit:10m rate=20r/s;

# CometBFT RPC (HTTPS)
server {
    listen 443 ssl http2;
    server_name rpc.omniphi.io;

    ssl_certificate /etc/letsencrypt/live/rpc.omniphi.io/fullchain.pem;
    ssl_certificate_key /etc/letsencrypt/live/rpc.omniphi.io/privkey.pem;
    ssl_protocols TLSv1.2 TLSv1.3;
    ssl_ciphers HIGH:!aNULL:!MD5;

    # Rate limiting
    limit_req zone=rpc_limit burst=50 nodelay;

    # Block dangerous RPC methods
    location / {
        # Deny unsafe methods that could affect node state
        if ($request_body ~* "(unsafe_flush_mempool|dial_seeds|dial_peers)") {
            return 403;
        }

        proxy_pass http://127.0.0.1:26657;
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto $scheme;

        # WebSocket support (for /websocket endpoint)
        proxy_http_version 1.1;
        proxy_set_header Upgrade $http_upgrade;
        proxy_set_header Connection "upgrade";
        proxy_read_timeout 86400;
    }
}

# REST API (HTTPS)
server {
    listen 443 ssl http2;
    server_name api.omniphi.io;

    ssl_certificate /etc/letsencrypt/live/api.omniphi.io/fullchain.pem;
    ssl_certificate_key /etc/letsencrypt/live/api.omniphi.io/privkey.pem;
    ssl_protocols TLSv1.2 TLSv1.3;
    ssl_ciphers HIGH:!aNULL:!MD5;

    # CORS headers for browser clients
    add_header Access-Control-Allow-Origin "*" always;
    add_header Access-Control-Allow-Methods "GET, POST, OPTIONS" always;
    add_header Access-Control-Allow-Headers "Content-Type, Authorization" always;

    if ($request_method = 'OPTIONS') {
        return 204;
    }

    limit_req zone=api_limit burst=30 nodelay;

    location / {
        proxy_pass http://127.0.0.1:1317;
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto $scheme;
    }
}

# Redirect HTTP to HTTPS
server {
    listen 80;
    server_name rpc.omniphi.io api.omniphi.io;
    return 301 https://$host$request_uri;
}
```

```bash
# Enable the site and get TLS certificates
sudo ln -s /etc/nginx/sites-available/omniphi-rpc /etc/nginx/sites-enabled/
sudo certbot --nginx -d rpc.omniphi.io -d api.omniphi.io
sudo nginx -t && sudo systemctl reload nginx
```

### AWS Application Load Balancer

For AWS deployments, use an ALB to distribute traffic across multiple RPC nodes:

```
ALB Configuration:
  Listener:  HTTPS :443 (ACM certificate for *.omniphi.io)
  Target Group 1 (RPC):
    Protocol: HTTP
    Port: 26657
    Health Check: GET /health  (expect 200)
    Targets: rpc-node-1, rpc-node-2
  Target Group 2 (API):
    Protocol: HTTP
    Port: 1317
    Health Check: GET /cosmos/base/tendermint/v1beta1/node_info  (expect 200)
    Targets: rpc-node-1, rpc-node-2

  Routing Rules:
    Host: rpc.omniphi.io  ->  Target Group 1
    Host: api.omniphi.io  ->  Target Group 2
```

### gRPC Load Balancing

For gRPC, use an NLB (Network Load Balancer) or Envoy proxy, because ALBs do not natively support HTTP/2 gRPC:

```
NLB Configuration:
  Listener: TCP :9090
  Target Group:
    Protocol: TCP
    Port: 9090
    Health Check: TCP :9090
    Targets: rpc-node-1, rpc-node-2
```

---

## Geographic Distribution

### Recommended Regions

For a global validator operation, distribute your infrastructure across at least 3 geographic regions to minimize latency and maximize resilience:

| Region | Purpose | Provider Examples |
|--------|---------|-------------------|
| US East (Virginia / N. Virginia) | Primary sentry + RPC | AWS us-east-1, GCP us-east4 |
| EU West (Frankfurt / Amsterdam) | Secondary sentry + RPC | AWS eu-central-1, GCP europe-west1 |
| Asia Pacific (Tokyo / Singapore) | Tertiary sentry (optional) | AWS ap-northeast-1, GCP asia-east1 |

### Region Selection Criteria

1. **Proximity to other validators:** Check where the majority of the active set runs. Being close reduces block propagation latency.
2. **Network peering:** Choose providers with good peering to other cloud providers (most validators run on AWS, GCP, or Hetzner).
3. **Regulatory environment:** Consider data residency requirements if applicable.
4. **Cost optimization:** EU and US regions are typically cheaper than Asia-Pacific.

### Multi-Region Topology Example

```
          US-East-1                    EU-Central-1               AP-Northeast-1
    +------------------+         +------------------+         +------------------+
    | Sentry A (pub)   |<------->| Sentry B (pub)   |<------->| Sentry C (pub)   |
    | RPC Node A (pub) |         | RPC Node B (pub) |         |                  |
    | :26656, :26657   |         | :26656, :26657   |         | :26656           |
    +--------+---------+         +--------+---------+         +------------------+
             |                            |
        VPC Peering                  VPC Peering
             |                            |
    +--------+---------+         +--------+---------+
    | Validator (priv)  |         | Hot Standby       |
    | PoSeq Node        |         | (sync'd, offline) |
    | US-East-1 VPC     |         | EU-Central-1 VPC  |
    +------------------+         +------------------+
```

### Latency Budget

CometBFT consensus is latency-sensitive. Target these round-trip times between your nodes:

| Connection | Target RTT | Maximum RTT |
|------------|-----------|-------------|
| Validator to Sentry | < 5 ms | 20 ms |
| Sentry to Sentry (same region) | < 10 ms | 50 ms |
| Sentry to Sentry (cross-region) | < 100 ms | 200 ms |
| Node to majority of active set | < 150 ms | 300 ms |

### Network Bandwidth Requirements

| Node Type | Inbound | Outbound | Total |
|-----------|---------|----------|-------|
| Validator | 10 Mbps | 10 Mbps | 20 Mbps |
| Sentry | 50 Mbps | 50 Mbps | 100 Mbps |
| RPC Node | 20 Mbps | 100 Mbps | 120 Mbps |
| Archive Node | 20 Mbps | 200 Mbps | 220 Mbps |

These are steady-state estimates. During state sync or snapshot serving, bandwidth usage can spike significantly.

---

## Appendix: Node ID Discovery

To find a node's ID (needed for persistent_peers configuration):

```bash
# On the node itself
posd tendermint show-node-id
# Output: a1b2c3d4e5f6...  (40 hex character node ID)

# Full peer address format:
# <node-id>@<ip-address>:<port>
# Example: a1b2c3d4e5f6@10.0.0.11:26656
```

For PoSeq nodes, the node ID is the `id` field in `poseq.toml` (64 hex characters).

---

## Appendix: Private Network Setup

### WireGuard VPN (Recommended for Cross-Cloud)

If your validator and sentries are on different cloud providers, use WireGuard to create a private mesh network:

```bash
# Install WireGuard
sudo apt install -y wireguard

# Generate keys on each node
wg genkey | tee /etc/wireguard/private.key | wg pubkey > /etc/wireguard/public.key
chmod 600 /etc/wireguard/private.key
```

**Validator `/etc/wireguard/wg0.conf`:**

```ini
[Interface]
Address = 10.0.0.10/24
ListenPort = 51820
PrivateKey = <validator-private-key>

[Peer]
# Sentry A
PublicKey = <sentry-a-public-key>
AllowedIPs = 10.0.0.11/32
Endpoint = <sentry-a-public-ip>:51820
PersistentKeepalive = 25

[Peer]
# Sentry B
PublicKey = <sentry-b-public-key>
AllowedIPs = 10.0.0.12/32
Endpoint = <sentry-b-public-ip>:51820
PersistentKeepalive = 25
```

**Sentry A `/etc/wireguard/wg0.conf`:**

```ini
[Interface]
Address = 10.0.0.11/24
ListenPort = 51820
PrivateKey = <sentry-a-private-key>

[Peer]
# Validator
PublicKey = <validator-public-key>
AllowedIPs = 10.0.0.10/32
Endpoint = <validator-public-ip>:51820
PersistentKeepalive = 25

[Peer]
# Sentry B
PublicKey = <sentry-b-public-key>
AllowedIPs = 10.0.0.12/32
Endpoint = <sentry-b-public-ip>:51820
PersistentKeepalive = 25
```

```bash
# Enable and start WireGuard on all nodes
sudo systemctl enable wg-quick@wg0
sudo systemctl start wg-quick@wg0

# Verify connectivity
ping 10.0.0.10  # from sentry, ping validator
ping 10.0.0.11  # from validator, ping sentry A

# Allow WireGuard through firewall
sudo ufw allow 51820/udp comment 'WireGuard'
```

### AWS VPC Peering (Same Provider)

If all nodes are on AWS, use VPC peering instead of WireGuard for lower latency:

1. Create VPCs in each region (e.g., `10.0.1.0/24` in us-east-1, `10.0.2.0/24` in eu-central-1).
2. Create VPC peering connections between regions.
3. Update route tables in each VPC to route to the peered CIDR.
4. Update security groups to allow CometBFT P2P (26656) and PoSeq P2P (7001) from the peered CIDR.
