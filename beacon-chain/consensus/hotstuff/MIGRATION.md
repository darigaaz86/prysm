# Migration Guide: 4-Phase to 2-Phase HotStuff

This guide explains the changes from the original 4-phase HotStuff to the optimized 2-phase version.

## Overview of Changes

The 2-phase optimization reduces consensus from 4 phases to 2 phases:
- **PROPOSE** (combines PREPARE + PRE-COMMIT)
- **COMMIT** (combines COMMIT + DECIDE)

This achieves 50% faster block times while maintaining all safety guarantees.

## Breaking Changes

### 1. Phase Enumeration

**Before (4-phase):**
```go
const (
    PhasePrepare Phase = iota
    PhasePreCommit
    PhaseCommit
    PhaseDecide
)
```

**After (2-phase):**
```go
const (
    PhasePropose Phase = iota
    PhaseCommit
)
```

### 2. BlockNode Structure

**Before (4-phase):**
```go
type BlockNode struct {
    Block        *HotStuffBlock
    PrepareQC    *QuorumCertificate
    PreCommitQC  *QuorumCertificate
    CommitQC     *QuorumCertificate
    Status       BlockStatus
}
```

**After (2-phase):**
```go
type BlockNode struct {
    Block      *HotStuffBlock
    ProposeQC  *QuorumCertificate  // Replaces PrepareQC + PreCommitQC
    CommitQC   *QuorumCertificate
    Status     BlockStatus
}
```

### 3. BlockStatus Values

**Before (4-phase):**
```go
const (
    StatusUnknown BlockStatus = iota
    StatusProposed
    StatusPrepared
    StatusPreCommitted
    StatusCommitted
    StatusDecided
)
```

**After (2-phase):**
```go
const (
    StatusUnknown BlockStatus = iota
    StatusProposed
    StatusCommitted
    StatusExecuted
)
```

## Configuration Updates

### Block Time Configuration

Update your configuration to use 3-second blocks:

**Before:**
```yaml
block_time: 6s
```

**After:**
```yaml
block_time: 3s
```

### Phase Timing

With 2 phases, each phase should complete in <1.5 seconds:
- PROPOSE phase: ~1.5s
- COMMIT phase: ~1.5s
- Total: ~3s per block

## Code Migration

### Updating Phase Handlers

**Before (4-phase):**
```go
switch phase {
case PhasePrepare:
    // Handle PREPARE
case PhasePreCommit:
    // Handle PRE-COMMIT
case PhaseCommit:
    // Handle COMMIT
case PhaseDecide:
    // Handle DECIDE
}
```

**After (2-phase):**
```go
switch phase {
case PhasePropose:
    // Handle PROPOSE (combines PREPARE + PRE-COMMIT logic)
case PhaseCommit:
    // Handle COMMIT (combines COMMIT + DECIDE logic)
}
```

### Updating Safety Checks

**PROPOSE Phase** combines PREPARE and PRE-COMMIT safety rules:
```go
func shouldVotePropose(block *HotStuffBlock) bool {
    // PREPARE safety: extends from highest QC
    if CompareQC(block.JustifyQC, s.highestQC) < 0 {
        return false
    }
    
    // PRE-COMMIT safety: extends from locked QC or higher
    if s.lockedQC != nil && CompareQC(block.JustifyQC, s.lockedQC) < 0 {
        return false
    }
    
    return true
}
```

**COMMIT Phase** verifies ProposeQC and executes block:
```go
func shouldVoteCommit(block *HotStuffBlock, node *BlockNode) bool {
    // Must have valid ProposeQC
    if node.ProposeQC == nil {
        return false
    }
    
    if err := VerifyQC(node.ProposeQC, publicKeys, quorumSize); err != nil {
        return false
    }
    
    return true
}
```

### Updating QC References

Replace references to PrepareQC and PreCommitQC with ProposeQC:

**Before:**
```go
if node.PrepareQC != nil && node.PreCommitQC != nil {
    // Both QCs exist
}
```

**After:**
```go
if node.ProposeQC != nil {
    // ProposeQC exists (equivalent to having both PrepareQC and PreCommitQC)
}
```

## Testing Updates

### Update Test Cases

1. Remove tests for PhasePrepare, PhasePreCommit, PhaseDecide
2. Add tests for PhasePropose and PhaseCommit
3. Update expected phase transitions
4. Update expected block status progression

### Performance Testing

With 2-phase consensus, verify:
- Each phase completes in <1.5s
- Blocks are produced every 3 seconds
- Throughput is 2x the 4-phase implementation
- Safety is maintained under Byzantine faults

## Troubleshooting

### Issue: Blocks not finalizing

**Cause**: ProposeQC not being built correctly

**Solution**: Verify that PROPOSE phase safety rules are passing and votes are being collected

### Issue: Phase transitions failing

**Cause**: Invalid phase transition (e.g., PROPOSE → PROPOSE)

**Solution**: Ensure only PROPOSE → COMMIT transitions occur, and COMMIT advances to next view

### Issue: Locked QC not updating

**Cause**: Locked QC update logic not in COMMIT phase

**Solution**: Verify that locked QC is updated in handleCommit before voting

## Performance Expectations

### 4-Phase (Before)
- Block time: 6 seconds
- Phase duration: ~1.5s per phase × 4 = 6s
- TPS: Limited by 6s block time

### 2-Phase (After)
- Block time: 3 seconds
- Phase duration: ~1.5s per phase × 2 = 3s
- TPS: 2x improvement (same transactions per block, 2x more blocks)

## Rollback Procedure

If you need to rollback to 4-phase:

1. Revert type definitions in `types.go`
2. Restore 4 phase handlers in `phases.go`
3. Update configuration back to 6s blocks
4. Restore PrepareQC and PreCommitQC in BlockNode
5. Update all phase references in code

## Support

For issues or questions:
- Check the updated README.md for 2-phase documentation
- Review the design document in `.kiro/specs/hotstuff-2phase-optimization/design.md`
- Run tests: `go test ./beacon-chain/consensus/hotstuff/...`
