package hotstuff

import (
	"testing"
)

func TestPhase_String(t *testing.T) {
	tests := []struct {
		phase    Phase
		expected string
	}{
		{PhasePrepare, "PREPARE"},
		{PhasePreCommit, "PRE-COMMIT"},
		{PhaseCommit, "COMMIT"},
		{PhaseDecide, "DECIDE"},
		{Phase(99), "UNKNOWN"},
	}

	for _, tt := range tests {
		t.Run(tt.expected, func(t *testing.T) {
			result := tt.phase.String()
			if result != tt.expected {
				t.Errorf("Phase.String() = %v, want %v", result, tt.expected)
			}
		})
	}
}

func TestBlockStatus_String(t *testing.T) {
	tests := []struct {
		status   BlockStatus
		expected string
	}{
		{StatusProposed, "PROPOSED"},
		{StatusPrepared, "PREPARED"},
		{StatusPreCommitted, "PRE-COMMITTED"},
		{StatusCommitted, "COMMITTED"},
		{StatusDecided, "DECIDED"},
		{BlockStatus(99), "UNKNOWN"},
	}

	for _, tt := range tests {
		t.Run(tt.expected, func(t *testing.T) {
			result := tt.status.String()
			if result != tt.expected {
				t.Errorf("BlockStatus.String() = %v, want %v", result, tt.expected)
			}
		})
	}
}

func TestHotStuffBlock_Hash(t *testing.T) {
	block := &HotStuffBlock{
		View: 1,
	}

	hash, err := block.Hash()
	if err != nil {
		t.Fatalf("Hash() error = %v", err)
	}

	// Hash should be deterministic
	hash2, err := block.Hash()
	if err != nil {
		t.Fatalf("Hash() error = %v", err)
	}

	if hash != hash2 {
		t.Errorf("Hash() not deterministic: %v != %v", hash, hash2)
	}
}

func TestQuorumCertificate_Basic(t *testing.T) {
	qc := &QuorumCertificate{
		View:      1,
		Phase:     PhasePrepare,
		BlockHash: [32]byte{1, 2, 3},
	}

	// Basic QC creation should work
	if qc.View != 1 {
		t.Errorf("QC.View = %d, want 1", qc.View)
	}
	if qc.Phase != PhasePrepare {
		t.Errorf("QC.Phase = %v, want PhasePrepare", qc.Phase)
	}
}
