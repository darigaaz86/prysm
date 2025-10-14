# Consensus Abstraction Layer

This package provides an abstraction layer for different consensus mechanisms in Prysm.

## Overview

The consensus package allows Prysm to support multiple consensus algorithms:
- **PoS**: Ethereum's Proof of Stake (existing implementation)
- **HotStuff**: BFT consensus algorithm (new implementation)

## Architecture

```
┌─────────────────────────────────────┐
│     Consensus Interface             │
│  (interface.go)                     │
└──────────────┬──────────────────────┘
               │
       ┌───────┴────────┐
       │                │
       ▼                ▼
┌─────────────┐  ┌─────────────┐
│  PoS Mode   │  │  HotStuff   │
│  (pos/)     │  │  (hotstuff/)│
└─────────────┘  └─────────────┘
       │
       ▼
┌─────────────────────────────────────┐
│  Existing Blockchain Service        │
│  (blockchain.Service)               │
└─────────────────────────────────────┘
```

## Usage

### Configuration

Create a consensus configuration:

```go
config := &consensus.Config{
    Mode: consensus.ModePoS, // or consensus.ModeHotStuff
    PoS: &consensus.PoSConfig{
        SlotDuration:  12 * time.Second,
        SlotsPerEpoch: 32,
    },
}
```

### Creating a Consensus Instance

```go
// Create factory
factory, err := consensus.NewFactory(config)
if err != nil {
    return err
}

// Create consensus instance
consensusService, err := factory.Create(ctx)
if err != nil {
    return err
}

// Start consensus
if err := consensusService.Start(); err != nil {
    return err
}
```

### Using the Consensus Interface

```go
// Receive a block
err := consensusService.ReceiveBlock(ctx, block, blockRoot)

// Process an attestation
err := consensusService.ProcessAttestation(ctx, attestation)

// Get current head
headRoot, err := consensusService.Head(ctx)

// Get finalized checkpoint
checkpoint := consensusService.FinalizedCheckpoint()
```

## Modes

### PoS Mode

The PoS mode wraps Prysm's existing blockchain service. It provides:
- Full Ethereum PoS consensus
- LMD-GHOST fork choice
- Casper FFG finality
- Attestation processing

### HotStuff Mode

The HotStuff mode implements a BFT consensus algorithm. It provides:
- Three-phase commit protocol
- Linear communication complexity
- Optimistic responsiveness
- Byzantine fault tolerance

## Configuration File

You can configure consensus via YAML:

```yaml
consensus:
  mode: "pos"  # or "hotstuff"
  
  pos:
    slot_duration: 12s
    slots_per_epoch: 32
    epochs_per_sync_committee: 256
  
  hotstuff:
    view_timeout: 10s
    block_time: 6s
    min_validators: 4
    quorum_threshold: 0.67
    leader_rotation: "round-robin"
    enable_view_change: true
```

## Implementation Status

- ✅ Consensus interface defined
- ✅ Configuration system
- ✅ Factory pattern
- ✅ PoS wrapper (adapts existing blockchain service)
- ⏳ HotStuff implementation (in progress)

## Files

- `interface.go` - Consensus interface definition
- `config.go` - Configuration structures
- `factory.go` - Factory for creating consensus instances
- `pos/` - PoS consensus wrapper
- `hotstuff/` - HotStuff consensus implementation

## Testing

```bash
# Run consensus package tests
go test ./beacon-chain/consensus/...

# Run with coverage
go test -cover ./beacon-chain/consensus/...
```

## Next Steps

1. Complete HotStuff implementation
2. Add integration tests
3. Add performance benchmarks
4. Update beacon node to use consensus abstraction
