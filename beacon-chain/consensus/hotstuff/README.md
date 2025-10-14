# HotStuff Consensus Implementation

This package implements the HotStuff BFT consensus algorithm for Prysm.

## Overview

HotStuff is a leader-based Byzantine Fault Tolerant (BFT) consensus protocol with:
- **Linear communication complexity**: O(n) messages per view
- **Optimistic responsiveness**: Progress in network delay time
- **Simplicity**: Three-phase commit protocol
- **Safety**: Byzantine fault tolerance (tolerates f < n/3 failures)
- **Liveness**: Guaranteed progress with synchrony

## Architecture

### Core Types (`types.go`)

- **View**: Represents a consensus round with a designated leader
- **Phase**: The current phase (PREPARE, PRE-COMMIT, COMMIT, DECIDE)
- **QuorumCertificate (QC)**: Collection of 2f+1 signatures proving agreement
- **Vote**: A validator's vote in a specific phase
- **HotStuffBlock**: Beacon block extended with HotStuff metadata
- **BlockNode**: Node in the block tree with status tracking

### Quorum Certificates (`qc.go`)

- **QCBuilder**: Builds QCs from collected votes
- **VerifyQC**: Verifies QC signatures and quorum
- **CompareQC**: Compares QCs to find the highest
- **HighestQC**: Returns the highest QC from a list

## Three-Phase Protocol

```
View v (Leader: replica i)

Phase 1: PREPARE
  Leader → All: PREPARE(v, block, qc_high)
  All → Leader: VOTE-PREPARE(v, block_hash)
  Leader collects 2f+1 votes → prepare_qc

Phase 2: PRE-COMMIT
  Leader → All: PRE-COMMIT(v, prepare_qc)
  All → Leader: VOTE-PRE-COMMIT(v, block_hash)
  Leader collects 2f+1 votes → precommit_qc

Phase 3: COMMIT
  Leader → All: COMMIT(v, precommit_qc)
  All → Leader: VOTE-COMMIT(v, block_hash)
  Leader collects 2f+1 votes → commit_qc

Phase 4: DECIDE
  Leader → All: DECIDE(v, commit_qc)
  All: Execute block, update state
```

## Usage

### Creating a QC Builder

```go
// Create a QC builder for view 1, PREPARE phase
blockHash := [32]byte{1, 2, 3}
builder := NewQCBuilder(1, PhasePrepare, blockHash, 7, 10)

// Add votes
vote := &Vote{
    View:           1,
    Phase:          PhasePrepare,
    BlockHash:      blockHash,
    ValidatorIndex: 0,
    Signature:      sig,
}
added, err := builder.AddVote(vote)

// Check if we have quorum
if builder.HasQuorum() {
    qc, err := builder.Build()
    // Use the QC
}
```

### Verifying a QC

```go
// Verify a QC
err := VerifyQC(qc, publicKeys, quorumSize)
if err != nil {
    // QC is invalid
}
```

### Comparing QCs

```go
// Find the highest QC
qcs := []*QuorumCertificate{qc1, qc2, qc3}
highest := HighestQC(qcs)

// Compare two QCs
result := CompareQC(qc1, qc2)
// result > 0: qc1 is higher
// result < 0: qc2 is higher
// result == 0: equal
```

## Block Status Progression

```
UNKNOWN → PROPOSED → PREPARED → PRE-COMMITTED → COMMITTED → DECIDED
```

- **PROPOSED**: Block has been proposed by leader
- **PREPARED**: Block has a PREPARE QC (2f+1 PREPARE votes)
- **PRE-COMMITTED**: Block has a PRE-COMMIT QC (2f+1 PRE-COMMIT votes)
- **COMMITTED**: Block has a COMMIT QC (2f+1 COMMIT votes)
- **DECIDED**: Block has been executed

## Safety Rules

1. A validator votes for a block only if:
   - It extends from the highest QC it knows
   - It's valid according to the state transition rules

2. A block is committed only after three consecutive QCs:
   - PREPARE QC → PRE-COMMIT QC → COMMIT QC

3. This ensures Byzantine fault tolerance with f < n/3 failures

## Quorum Size

For n validators and f Byzantine faults:
- n = 3f + 1 (minimum)
- Quorum size = 2f + 1 (supermajority)

Example:
- 10 validators → tolerates 3 faults → quorum = 7
- 7 validators → tolerates 2 faults → quorum = 5
- 4 validators → tolerates 1 fault → quorum = 3

## Implementation Status

- ✅ Core types defined
- ✅ QC builder and verification
- ✅ Vote collection and aggregation
- ✅ Unit tests
- ⏳ Leader election (next step)
- ⏳ Three-phase protocol (next step)
- ⏳ View change mechanism (next step)
- ⏳ Integration with blockchain service (next step)

## Testing

```bash
# Run HotStuff tests
go test ./beacon-chain/consensus/hotstuff/...

# Run with coverage
go test -cover ./beacon-chain/consensus/hotstuff/...

# Run specific test
go test -run TestQCBuilder_AddVote ./beacon-chain/consensus/hotstuff/
```

## References

- **HotStuff Paper**: https://arxiv.org/abs/1803.05069
- **BFT Consensus**: https://pmg.csail.mit.edu/papers/osdi99.pdf
- **Prysm Consensus**: ../README.md
