package hotstuff

import (
	"testing"
)

// Note: Some QC tests are limited due to import cycle constraints with primitives package
// Full integration tests should be done at a higher level

func TestQCBuilder_Basic(t *testing.T) {
	blockHash := [32]byte{1, 2, 3}
	builder := NewQCBuilder(1, PhasePrepare, blockHash, 7, 10)

	// Test basic properties
	if builder.VoteCount() != 0 {
		t.Errorf("VoteCount() = %d, want 0", builder.VoteCount())
	}
	if builder.HasQuorum() {
		t.Error("HasQuorum() = true, want false initially")
	}

	// Builder should be created successfully
	if builder == nil {
		t.Fatal("NewQCBuilder() returned nil")
	}
}

func TestCompareQC(t *testing.T) {
	tests := []struct {
		name     string
		qc1      *QuorumCertificate
		qc2      *QuorumCertificate
		expected int
	}{
		{
			name:     "both nil",
			qc1:      nil,
			qc2:      nil,
			expected: 0,
		},
		{
			name:     "qc1 nil",
			qc1:      nil,
			qc2:      &QuorumCertificate{View: 1},
			expected: -1,
		},
		{
			name:     "qc2 nil",
			qc1:      &QuorumCertificate{View: 1},
			qc2:      nil,
			expected: 1,
		},
		{
			name:     "qc1 higher view",
			qc1:      &QuorumCertificate{View: 2},
			qc2:      &QuorumCertificate{View: 1},
			expected: 1,
		},
		{
			name:     "qc2 higher view",
			qc1:      &QuorumCertificate{View: 1},
			qc2:      &QuorumCertificate{View: 2},
			expected: -1,
		},
		{
			name:     "same view, qc1 higher phase",
			qc1:      &QuorumCertificate{View: 1, Phase: PhaseCommit},
			qc2:      &QuorumCertificate{View: 1, Phase: PhasePrepare},
			expected: 1,
		},
		{
			name:     "same view, qc2 higher phase",
			qc1:      &QuorumCertificate{View: 1, Phase: PhasePrepare},
			qc2:      &QuorumCertificate{View: 1, Phase: PhaseCommit},
			expected: -1,
		},
		{
			name:     "equal",
			qc1:      &QuorumCertificate{View: 1, Phase: PhasePrepare},
			qc2:      &QuorumCertificate{View: 1, Phase: PhasePrepare},
			expected: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := CompareQC(tt.qc1, tt.qc2)
			if result != tt.expected {
				t.Errorf("CompareQC() = %d, want %d", result, tt.expected)
			}
		})
	}
}

func TestHighestQC(t *testing.T) {
	tests := []struct {
		name     string
		qcs      []*QuorumCertificate
		expected *QuorumCertificate
	}{
		{
			name:     "empty list",
			qcs:      []*QuorumCertificate{},
			expected: nil,
		},
		{
			name: "single QC",
			qcs: []*QuorumCertificate{
				{View: 1, Phase: PhasePrepare},
			},
			expected: &QuorumCertificate{View: 1, Phase: PhasePrepare},
		},
		{
			name: "multiple QCs, highest by view",
			qcs: []*QuorumCertificate{
				{View: 1, Phase: PhasePrepare},
				{View: 3, Phase: PhasePrepare},
				{View: 2, Phase: PhasePrepare},
			},
			expected: &QuorumCertificate{View: 3, Phase: PhasePrepare},
		},
		{
			name: "multiple QCs, same view, highest by phase",
			qcs: []*QuorumCertificate{
				{View: 1, Phase: PhasePrepare},
				{View: 1, Phase: PhaseCommit},
				{View: 1, Phase: PhasePreCommit},
			},
			expected: &QuorumCertificate{View: 1, Phase: PhaseCommit},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := HighestQC(tt.qcs)
			if tt.expected == nil {
				if result != nil {
					t.Errorf("HighestQC() = %v, want nil", result)
				}
			} else {
				if result == nil {
					t.Fatal("HighestQC() = nil, want non-nil")
				}
				if result.View != tt.expected.View {
					t.Errorf("HighestQC().View = %d, want %d", result.View, tt.expected.View)
				}
				if result.Phase != tt.expected.Phase {
					t.Errorf("HighestQC().Phase = %v, want %v", result.Phase, tt.expected.Phase)
				}
			}
		})
	}
}
