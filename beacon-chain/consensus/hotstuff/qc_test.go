package hotstuff

import (
	"testing"

	"github.com/OffchainLabs/prysm/v6/consensus-types/primitives"
	"github.com/OffchainLabs/prysm/v6/testing/assert"
	"github.com/OffchainLabs/prysm/v6/testing/require"
)

func TestQCBuilder_AddVote(t *testing.T) {
	blockHash := [32]byte{1, 2, 3}
	builder := NewQCBuilder(1, PhasePrepare, blockHash, 7, 10)

	// Create a test vote
	vote := &Vote{
		View:           1,
		Phase:          PhasePrepare,
		BlockHash:      blockHash,
		ValidatorIndex: 0,
	}

	// Add vote should succeed
	added, err := builder.AddVote(vote)
	require.NoError(t, err)
	assert.Equal(t, true, added, "First vote should be added")
	assert.Equal(t, uint64(1), builder.VoteCount(), "Vote count should be 1")

	// Adding same vote again should return false (duplicate)
	added, err = builder.AddVote(vote)
	require.NoError(t, err)
	assert.Equal(t, false, added, "Duplicate vote should not be added")
	assert.Equal(t, uint64(1), builder.VoteCount(), "Vote count should still be 1")
}

func TestQCBuilder_AddVote_WrongView(t *testing.T) {
	blockHash := [32]byte{1, 2, 3}
	builder := NewQCBuilder(1, PhasePrepare, blockHash, 7, 10)

	vote := &Vote{
		View:           2, // Wrong view
		Phase:          PhasePrepare,
		BlockHash:      blockHash,
		ValidatorIndex: 0,
	}

	added, err := builder.AddVote(vote)
	assert.ErrorContains(t, "does not match builder view", err)
	assert.Equal(t, false, added)
}

func TestQCBuilder_AddVote_WrongPhase(t *testing.T) {
	blockHash := [32]byte{1, 2, 3}
	builder := NewQCBuilder(1, PhasePrepare, blockHash, 7, 10)

	vote := &Vote{
		View:           1,
		Phase:          PhaseCommit, // Wrong phase
		BlockHash:      blockHash,
		ValidatorIndex: 0,
	}

	added, err := builder.AddVote(vote)
	assert.ErrorContains(t, "does not match builder phase", err)
	assert.Equal(t, false, added)
}

func TestQCBuilder_AddVote_WrongBlockHash(t *testing.T) {
	blockHash := [32]byte{1, 2, 3}
	builder := NewQCBuilder(1, PhasePrepare, blockHash, 7, 10)

	vote := &Vote{
		View:           1,
		Phase:          PhasePrepare,
		BlockHash:      [32]byte{4, 5, 6}, // Wrong block hash
		ValidatorIndex: 0,
	}

	added, err := builder.AddVote(vote)
	assert.ErrorContains(t, "does not match builder block hash", err)
	assert.Equal(t, false, added)
}

func TestQCBuilder_HasQuorum(t *testing.T) {
	blockHash := [32]byte{1, 2, 3}
	builder := NewQCBuilder(1, PhasePrepare, blockHash, 7, 10)

	// Initially no quorum
	assert.Equal(t, false, builder.HasQuorum(), "Should not have quorum initially")

	// Add votes until we have quorum
	for i := uint64(0); i < 7; i++ {
		vote := &Vote{
			View:           1,
			Phase:          PhasePrepare,
			BlockHash:      blockHash,
			ValidatorIndex: primitives.ValidatorIndex(i),
		}
		_, err := builder.AddVote(vote)
		require.NoError(t, err)
	}

	// Now should have quorum
	assert.Equal(t, true, builder.HasQuorum(), "Should have quorum after 7 votes")
}

func TestCreateSignerBitmap(t *testing.T) {
	tests := []struct {
		name          string
		signerIndices []primitives.ValidatorIndex
		totalVals     uint64
		expectedBits  []int // Bit positions that should be set
	}{
		{
			name:          "single signer",
			signerIndices: []primitives.ValidatorIndex{0},
			totalVals:     10,
			expectedBits:  []int{0},
		},
		{
			name:          "multiple signers",
			signerIndices: []primitives.ValidatorIndex{0, 2, 5, 7},
			totalVals:     10,
			expectedBits:  []int{0, 2, 5, 7},
		},
		{
			name:          "all signers",
			signerIndices: []primitives.ValidatorIndex{0, 1, 2, 3, 4, 5, 6, 7},
			totalVals:     8,
			expectedBits:  []int{0, 1, 2, 3, 4, 5, 6, 7},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			bitmap := createSignerBitmap(tt.signerIndices, tt.totalVals)

			// Check that expected bits are set
			for _, bitPos := range tt.expectedBits {
				byteIdx := bitPos / 8
				bitIdx := bitPos % 8
				assert.Equal(t, true, (bitmap[byteIdx]&(1<<bitIdx)) != 0,
					"Bit %d should be set", bitPos)
			}

			// Verify we can extract the same indices back
			extracted := GetSignerIndices(bitmap, tt.totalVals)
			assert.DeepEqual(t, tt.signerIndices, extracted, "Extracted indices should match")
		})
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
			assert.Equal(t, tt.expected, result, "CompareQC result mismatch")
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
				assert.Equal(t, true, result == nil, "Expected nil result")
			} else {
				require.NotNil(t, result)
				assert.Equal(t, tt.expected.View, result.View, "View mismatch")
				assert.Equal(t, tt.expected.Phase, result.Phase, "Phase mismatch")
			}
		})
	}
}
