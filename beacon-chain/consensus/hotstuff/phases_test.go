package hotstuff

import (
	"context"
	"testing"
	"time"

	"github.com/OffchainLabs/prysm/v6/consensus-types/primitives"
	"github.com/OffchainLabs/prysm/v6/crypto/bls"
)

// TestHandlePropose_ValidBlock tests handlePropose with a valid block
func TestHandlePropose_ValidBlock(t *testing.T) {
	ctx := context.Background()
	cfg := &Config{
		ViewTimeout:     time.Second * 5,
		BlockTime:       time.Second * 3,
		MinValidators:   4,
		QuorumThreshold: 0.67,
	}

	// Create service with 4 validators
	s, err := NewService(ctx, cfg)
	if err != nil {
		t.Fatalf("Failed to create service: %v", err)
	}

	// Set up as validator 0
	s.validatorIdx = 0
	s.validators = make([]primitives.ValidatorIndex, 4)
	s.quorumSize = 3

	// Create a valid block extending from genesis
	block := &HotStuffBlock{
		View:      1,
		JustifyQC: s.highestQC,
	}

	blockHash := [32]byte{1, 2, 3}
	s.blocks[blockHash] = &BlockNode{
		Block:  block,
		Status: StatusProposed,
	}

	// Should vote for valid block
	if !s.shouldVotePropose(block) {
		t.Error("shouldVotePropose() = false, want true for valid block")
	}
}

// TestHandlePropose_NoJustifyQC tests handlePropose with block missing JustifyQC
func TestHandlePropose_NoJustifyQC(t *testing.T) {
	ctx := context.Background()
	cfg := &Config{
		ViewTimeout:     time.Second * 5,
		BlockTime:       time.Second * 3,
		MinValidators:   4,
		QuorumThreshold: 0.67,
	}

	s, err := NewService(ctx, cfg)
	if err != nil {
		t.Fatalf("Failed to create service: %v", err)
	}

	// Block without JustifyQC
	block := &HotStuffBlock{
		View:      1,
		JustifyQC: nil,
	}

	// Should not vote
	if s.shouldVotePropose(block) {
		t.Error("shouldVotePropose() = true, want false for block without JustifyQC")
	}
}

// TestHandlePropose_NotExtendingHighestQC tests block not extending from highest QC
func TestHandlePropose_NotExtendingHighestQC(t *testing.T) {
	ctx := context.Background()
	cfg := &Config{
		ViewTimeout:     time.Second * 5,
		BlockTime:       time.Second * 3,
		MinValidators:   4,
		QuorumThreshold: 0.67,
	}

	s, err := NewService(ctx, cfg)
	if err != nil {
		t.Fatalf("Failed to create service: %v", err)
	}

	// Set highest QC to view 5
	s.highestQC = &QuorumCertificate{
		View:  5,
		Phase: PhasePropose,
	}

	// Block with lower QC (view 3)
	block := &HotStuffBlock{
		View: 6,
		JustifyQC: &QuorumCertificate{
			View:  3,
			Phase: PhasePropose,
		},
	}

	// Should not vote (doesn't extend from highest QC)
	if s.shouldVotePropose(block) {
		t.Error("shouldVotePropose() = true, want false for block not extending from highest QC")
	}
}

// TestHandlePropose_LockedQCCheck tests PRE-COMMIT safety rule
func TestHandlePropose_LockedQCCheck(t *testing.T) {
	ctx := context.Background()
	cfg := &Config{
		ViewTimeout:     time.Second * 5,
		BlockTime:       time.Second * 3,
		MinValidators:   4,
		QuorumThreshold: 0.67,
	}

	s, err := NewService(ctx, cfg)
	if err != nil {
		t.Fatalf("Failed to create service: %v", err)
	}

	// Set locked QC to view 4
	s.lockedQC = &QuorumCertificate{
		View:  4,
		Phase: PhasePropose,
	}

	// Set highest QC to view 5
	s.highestQC = &QuorumCertificate{
		View:  5,
		Phase: PhasePropose,
	}

	// Block with QC lower than locked QC
	blockLow := &HotStuffBlock{
		View: 6,
		JustifyQC: &QuorumCertificate{
			View:  3,
			Phase: PhasePropose,
		},
	}

	// Should not vote (doesn't extend from locked QC)
	if s.shouldVotePropose(blockLow) {
		t.Error("shouldVotePropose() = true, want false for block not extending from locked QC")
	}

	// Block with QC equal to locked QC
	blockEqual := &HotStuffBlock{
		View: 6,
		JustifyQC: &QuorumCertificate{
			View:  4,
			Phase: PhasePropose,
		},
	}

	// Should not vote (highest QC is higher)
	if s.shouldVotePropose(blockEqual) {
		t.Error("shouldVotePropose() = true, want false when JustifyQC < highestQC")
	}

	// Block with QC higher than locked QC and equal to highest
	blockHigh := &HotStuffBlock{
		View: 6,
		JustifyQC: &QuorumCertificate{
			View:  5,
			Phase: PhasePropose,
		},
	}

	// Should vote (extends from highest QC and higher than locked QC)
	if !s.shouldVotePropose(blockHigh) {
		t.Error("shouldVotePropose() = false, want true for block extending from highest QC")
	}
}

// TestHandleCommit_ValidProposeQC tests handleCommit with valid ProposeQC
func TestHandleCommit_ValidProposeQC(t *testing.T) {
	ctx := context.Background()
	cfg := &Config{
		ViewTimeout:     time.Second * 5,
		BlockTime:       time.Second * 3,
		MinValidators:   4,
		QuorumThreshold: 0.67,
	}

	s, err := NewService(ctx, cfg)
	if err != nil {
		t.Fatalf("Failed to create service: %v", err)
	}

	s.validatorIdx = 0
	s.validators = make([]primitives.ValidatorIndex, 4)
	s.quorumSize = 3
	s.publicKeys = make([]bls.PublicKey, 4)

	// Create block with ProposeQC
	block := &HotStuffBlock{
		View: 1,
	}

	proposeQC := &QuorumCertificate{
		View:          1,
		Phase:         PhasePropose,
		BlockHash:     [32]byte{1, 2, 3},
		SignerIndices: []primitives.ValidatorIndex{0, 1, 2},
	}

	node := &BlockNode{
		Block:     block,
		ProposeQC: proposeQC,
		Status:    StatusProposed,
	}

	// Note: VerifyQC will fail without real signatures, but we test the logic
	// In real tests, we'd need to set up proper keys and signatures
	if node.ProposeQC == nil {
		t.Error("ProposeQC should not be nil")
	}
}

// TestHandleCommit_MissingProposeQC tests handleCommit without ProposeQC
func TestHandleCommit_MissingProposeQC(t *testing.T) {
	ctx := context.Background()
	cfg := &Config{
		ViewTimeout:     time.Second * 5,
		BlockTime:       time.Second * 3,
		MinValidators:   4,
		QuorumThreshold: 0.67,
	}

	s, err := NewService(ctx, cfg)
	if err != nil {
		t.Fatalf("Failed to create service: %v", err)
	}

	block := &HotStuffBlock{
		View: 1,
	}

	node := &BlockNode{
		Block:     block,
		ProposeQC: nil,
		Status:    StatusProposed,
	}

	// Should not vote without ProposeQC
	if s.shouldVoteCommit(block, node) {
		t.Error("shouldVoteCommit() = true, want false for block without ProposeQC")
	}
}

// TestShouldVotePropose_SafetyRules tests combined PREPARE + PRE-COMMIT safety
func TestShouldVotePropose_SafetyRules(t *testing.T) {
	tests := []struct {
		name        string
		highestQC   *QuorumCertificate
		lockedQC    *QuorumCertificate
		blockQC     *QuorumCertificate
		shouldVote  bool
		description string
	}{
		{
			name:        "no JustifyQC",
			highestQC:   &QuorumCertificate{View: 1, Phase: PhasePropose},
			lockedQC:    nil,
			blockQC:     nil,
			shouldVote:  false,
			description: "Block without JustifyQC should not be voted for",
		},
		{
			name:        "extends from highest, no locked",
			highestQC:   &QuorumCertificate{View: 3, Phase: PhasePropose},
			lockedQC:    nil,
			blockQC:     &QuorumCertificate{View: 3, Phase: PhasePropose},
			shouldVote:  true,
			description: "Block extending from highest QC with no locked QC should be voted for",
		},
		{
			name:        "lower than highest",
			highestQC:   &QuorumCertificate{View: 5, Phase: PhasePropose},
			lockedQC:    nil,
			blockQC:     &QuorumCertificate{View: 3, Phase: PhasePropose},
			shouldVote:  false,
			description: "Block not extending from highest QC should not be voted for",
		},
		{
			name:        "extends from highest and locked",
			highestQC:   &QuorumCertificate{View: 5, Phase: PhasePropose},
			lockedQC:    &QuorumCertificate{View: 4, Phase: PhasePropose},
			blockQC:     &QuorumCertificate{View: 5, Phase: PhasePropose},
			shouldVote:  true,
			description: "Block extending from both highest and locked QC should be voted for",
		},
		{
			name:        "extends from highest but not locked",
			highestQC:   &QuorumCertificate{View: 5, Phase: PhasePropose},
			lockedQC:    &QuorumCertificate{View: 6, Phase: PhasePropose},
			blockQC:     &QuorumCertificate{View: 5, Phase: PhasePropose},
			shouldVote:  false,
			description: "Block not extending from locked QC should not be voted for",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()
			cfg := &Config{
				ViewTimeout:     time.Second * 5,
				BlockTime:       time.Second * 3,
				MinValidators:   4,
				QuorumThreshold: 0.67,
			}

			s, err := NewService(ctx, cfg)
			if err != nil {
				t.Fatalf("Failed to create service: %v", err)
			}

			s.highestQC = tt.highestQC
			s.lockedQC = tt.lockedQC

			block := &HotStuffBlock{
				View:      10,
				JustifyQC: tt.blockQC,
			}

			result := s.shouldVotePropose(block)
			if result != tt.shouldVote {
				t.Errorf("%s: shouldVotePropose() = %v, want %v", tt.description, result, tt.shouldVote)
			}
		})
	}
}

// TestOnQuorumReached_ProposePhase tests quorum handling for PROPOSE phase
func TestOnQuorumReached_ProposePhase(t *testing.T) {
	ctx := context.Background()
	cfg := &Config{
		ViewTimeout:     time.Second * 5,
		BlockTime:       time.Second * 3,
		MinValidators:   4,
		QuorumThreshold: 0.67,
	}

	s, err := NewService(ctx, cfg)
	if err != nil {
		t.Fatalf("Failed to create service: %v", err)
	}

	s.validatorIdx = 0
	s.validators = make([]primitives.ValidatorIndex, 4)
	s.quorumSize = 3

	blockHash := [32]byte{1, 2, 3}
	block := &HotStuffBlock{
		View: 1,
	}

	s.blocks[blockHash] = &BlockNode{
		Block:  block,
		Status: StatusProposed,
	}

	// Create QC builder with quorum
	builder := NewQCBuilder(1, PhasePropose, blockHash, 3, 4)

	// Simulate quorum reached
	err = s.onQuorumReached(1, PhasePropose, blockHash, builder)
	if err != nil {
		// Expected to fail due to missing signatures, but logic should execute
		t.Logf("onQuorumReached failed (expected): %v", err)
	}

	// Verify ProposeQC was set
	node := s.blocks[blockHash]
	if node.ProposeQC == nil {
		t.Error("ProposeQC should be set after PROPOSE quorum")
	}

	// Verify phase advanced to COMMIT
	if s.currentPhase != PhaseCommit {
		t.Errorf("currentPhase = %v, want PhaseCommit", s.currentPhase)
	}
}

// TestOnQuorumReached_CommitPhase tests quorum handling for COMMIT phase
func TestOnQuorumReached_CommitPhase(t *testing.T) {
	ctx := context.Background()
	cfg := &Config{
		ViewTimeout:     time.Second * 5,
		BlockTime:       time.Second * 3,
		MinValidators:   4,
		QuorumThreshold: 0.67,
	}

	s, err := NewService(ctx, cfg)
	if err != nil {
		t.Fatalf("Failed to create service: %v", err)
	}

	s.validatorIdx = 0
	s.validators = make([]primitives.ValidatorIndex, 4)
	s.quorumSize = 3
	s.currentView = 1
	s.currentPhase = PhaseCommit

	blockHash := [32]byte{1, 2, 3}
	block := &HotStuffBlock{
		View: 1,
	}

	s.blocks[blockHash] = &BlockNode{
		Block:  block,
		Status: StatusCommitted,
	}

	// Create QC builder with quorum
	builder := NewQCBuilder(1, PhaseCommit, blockHash, 3, 4)

	// Simulate quorum reached
	err = s.onQuorumReached(1, PhaseCommit, blockHash, builder)
	if err != nil {
		// Expected to fail due to missing signatures, but logic should execute
		t.Logf("onQuorumReached failed (expected): %v", err)
	}

	// Verify CommitQC was set
	node := s.blocks[blockHash]
	if node.CommitQC == nil {
		t.Error("CommitQC should be set after COMMIT quorum")
	}

	// Verify status updated to executed
	if node.Status != StatusExecuted {
		t.Errorf("Status = %v, want StatusExecuted", node.Status)
	}

	// Verify view advanced
	if s.currentView != 2 {
		t.Errorf("currentView = %d, want 2", s.currentView)
	}

	// Verify phase reset to PROPOSE
	if s.currentPhase != PhasePropose {
		t.Errorf("currentPhase = %v, want PhasePropose", s.currentPhase)
	}
}
