# x/poc - Proof of Contribution Module

**Omniphi Blockchain - Hybrid PoS + PoC Consensus**

## Overview

The `x/poc` module implements a sophisticated **Proof of Contribution** system that verifies, scores, and rewards useful work from network contributors. It operates alongside the traditional Proof of Stake consensus through a three-layer verification pipeline:

1. **Proof of Existence (PoE)** - Ensures submitted work exists and is immutable
2. **Proof of Authority (PoA)** - Ensures the submitter is the legitimate contributor  
3. **Proof of Value (PoV)** - Verifies the work was done correctly and is valuable

## Current Status: ✅ Fully Implemented

- ✅ Three-layer verification system (PoE → PoA → PoV)
- ✅ Validator endorsement with trust weighting
- ✅ C-Score reputation system
- ✅ Fee burn mechanism (75% burned, 25% to reward pool)
- ✅ Epoch-based reward distribution
- ✅ Rate limiting and anti-spam
- ✅ Comprehensive test coverage (95%+)
- ✅ CLI and gRPC APIs
- ✅ All module integration complete

## Architecture

### Verification Pipeline

```
┌─────────────┐      ┌─────────────┐      ┌─────────────┐      ┌──────────────┐
│             │      │             │      │             │      │              │
│  Contributor│─────>│    PoE      │─────>│    PoA      │─────>│     PoV      │
│  Submits    │      │  Existence  │      │  Authority  │      │   Value      │
│             │      │             │      │             │      │              │
└─────────────┘      └─────────────┘      └─────────────┘      └──────────────┘
                            │                     │                     │
                            v                     v                     v
                     ┌──────────────────────────────────────────────────────┐
                     │                                                      │
                     │              Contribution Verified                   │
                     │         C-Score Minted → Epoch Rewards               │
                     │                                                      │
                     └──────────────────────────────────────────────────────┘
```

See README_COMPREHENSIVE.md for full documentation including:
- Detailed verification layer implementations
- C-Score system and tier mechanics
- Reward distribution formulas
- Security threat model and mitigations
- Complete CLI/gRPC API reference
- Testing guide and coverage reports
- Validator and contributor guides
