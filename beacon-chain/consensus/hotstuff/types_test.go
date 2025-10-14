package hotstuff

import (
	"testing"

	"github.com/OffchainLabs/prysm/v6/consensus-types/primitives"
	"github.com/OffchainLabs/prysm/v6/testing/assert"
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
		{Phase(99), "UNKNOWN(99)"},
	}

	for _, tt := range tests {
		result := tt.phase.String()
		assert.Equal(t, tt.expected, result, "Phase.String() mismatch")
	}
}

func TestBlockStatus_String(t *testing.T) {
	tests := []struct {
		status   BlockStatus
		expected string
	}{
		{StatusUnknown, "UNKNOWN"},
		{StatusProposed, "PROPOSED"},
		{StatusPrepared, "PREPARED"},
		{StatusPreCommitted, "PRE-COMMITTED"},
		{StatusCommitted, "COMMITTED"},
		{StatusDecided, "DECIDED"},
		{BlockStatus(99), "UNKNOWN(99)"},
	}

	for _, tt := range tests {
		result := tt.status.String()
		assert.Equal(t, tt.expected, result, "BlockStatus.String() mismatch")
	}
}

func TestQuorumCertificate_IsValid(t *testing.T) {
	tests := []struct {
		name          string
		qc            *QuorumCertificate
		totalVals     uint64
		quorumSize    uint64
		expectedValid bool
	}{
		{
			name:          "nil QC",
			qc:            nil,
			totalVals:     10,
			quorumSize:    7,
			expectedValid: false,
		},
		{
			name: "valid QC with exact quorum",
			qc: &QuorumCertificate{
				SignerIndices: []primitives.ValidatorIndex{0, 1, 2, 3, 4, 5, 6},
			},
			totalVals:     10,
			quorumSize:    7,
			expectedValid: true,
		},
		{
			name: "valid QC with more than quorum",
			qc: &QuorumCertificate{
				SignerIndices: []primitives.ValidatorIndex{0, 1, 2, 3, 4, 5, 6, 7, 8},
			},
			totalVals:     10,
			quorumSize:    7,
			expectedValid: true,
		},
		{
			name: "invalid QC with less than quorum",
			qc: &QuorumCertificate{
				SignerIndices: []primitives.ValidatorIndex{0, 1, 2, 3, 4, 5},
			},
			totalVals:     10,
			quorumSize:    7,
			expectedValid: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.qc.IsValid(tt.totalVals, tt.quorumSize)
			assert.Equal(t, tt.expectedValid, result, "IsValid() mismatch")
		})
	}
}

func TestBlockNode_IsCommitted(t *testing.T) {
	tests := []struct {
		name     string
		status   BlockStatus
		expected bool
	}{
		{"unknown", StatusUnknown, false},
		{"proposed", StatusProposed, false},
		{"prepared", StatusPrepared, false},
		{"pre-committed", StatusPreCommitted, false},
		{"committed", StatusCommitted, true},
		{"decided", StatusDecided, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			node := &BlockNode{Status: tt.status}
			result := node.IsCommitted()
			assert.Equal(t, tt.expected, result, "IsCommitted() mismatch")
		})
	}
}

func TestBlockNode_IsDecided(t *testing.T) {
	tests := []struct {
		name     string
		status   BlockStatus
		expected bool
	}{
		{"unknown", StatusUnknown, false},
		{"proposed", StatusProposed, false},
		{"prepared", StatusPrepared, false},
		{"pre-committed", StatusPreCommitted, false},
		{"committed", StatusCommitted, false},
		{"decided", StatusDecided, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			node := &BlockNode{Status: tt.status}
			result := node.IsDecided()
			assert.Equal(t, tt.expected, result, "IsDecided() mismatch")
		})
	}
}
