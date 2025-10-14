package hotstuff

import (
	"testing"

	"github.com/OffchainLabs/prysm/v6/consensus-types/primitives"
	"github.com/OffchainLabs/prysm/v6/testing/assert"
	"github.com/OffchainLabs/prysm/v6/testing/require"
)

func TestRoundRobinLeaderElection_GetLeader(t *testing.T) {
	validators := []primitives.ValidatorIndex{0, 1, 2, 3, 4}
	election, err := NewRoundRobinLeaderElection(validators)
	require.NoError(t, err)

	tests := []struct {
		view           uint64
		expectedLeader primitives.ValidatorIndex
	}{
		{0, 0},  // view 0 -> validator 0
		{1, 1},  // view 1 -> validator 1
		{2, 2},  // view 2 -> validator 2
		{3, 3},  // view 3 -> validator 3
		{4, 4},  // view 4 -> validator 4
		{5, 0},  // view 5 -> validator 0 (wraps around)
		{6, 1},  // view 6 -> validator 1
		{10, 0}, // view 10 -> validator 0
	}

	for _, tt := range tests {
		leader, err := election.GetLeader(tt.view)
		require.NoError(t, err)
		assert.Equal(t, tt.expectedLeader, leader, "Leader mismatch for view %d", tt.view)
	}
}

func TestRoundRobinLeaderElection_IsLeader(t *testing.T) {
	validators := []primitives.ValidatorIndex{0, 1, 2, 3, 4}
	election, err := NewRoundRobinLeaderElection(validators)
	require.NoError(t, err)

	// View 0: validator 0 is leader
	assert.Equal(t, true, election.IsLeader(0, 0))
	assert.Equal(t, false, election.IsLeader(0, 1))
	assert.Equal(t, false, election.IsLeader(0, 2))

	// View 1: validator 1 is leader
	assert.Equal(t, false, election.IsLeader(1, 0))
	assert.Equal(t, true, election.IsLeader(1, 1))
	assert.Equal(t, false, election.IsLeader(1, 2))

	// View 5: validator 0 is leader (wraps around)
	assert.Equal(t, true, election.IsLeader(5, 0))
	assert.Equal(t, false, election.IsLeader(5, 1))
}

func TestRoundRobinLeaderElection_UpdateValidators(t *testing.T) {
	validators := []primitives.ValidatorIndex{0, 1, 2}
	election, err := NewRoundRobinLeaderElection(validators)
	require.NoError(t, err)

	// Initial leader for view 0
	leader, err := election.GetLeader(0)
	require.NoError(t, err)
	assert.Equal(t, primitives.ValidatorIndex(0), leader)

	// Update validators
	newValidators := []primitives.ValidatorIndex{5, 6, 7, 8}
	err = election.UpdateValidators(newValidators, nil)
	require.NoError(t, err)

	// Leader for view 0 should now be from new set
	leader, err = election.GetLeader(0)
	require.NoError(t, err)
	assert.Equal(t, primitives.ValidatorIndex(5), leader)

	// Leader for view 1
	leader, err = election.GetLeader(1)
	require.NoError(t, err)
	assert.Equal(t, primitives.ValidatorIndex(6), leader)
}

func TestRoundRobinLeaderElection_EmptyValidators(t *testing.T) {
	_, err := NewRoundRobinLeaderElection([]primitives.ValidatorIndex{})
	assert.ErrorContains(t, "cannot be empty", err)
}

func TestStakeWeightedLeaderElection_GetLeader(t *testing.T) {
	validators := []primitives.ValidatorIndex{0, 1, 2}
	stakes := []uint64{10, 20, 30} // Total: 60
	// Cumulative: [10, 30, 60]
	// Validator 0: positions 0-9
	// Validator 1: positions 10-29
	// Validator 2: positions 30-59

	election, err := NewStakeWeightedLeaderElection(validators, stakes)
	require.NoError(t, err)

	tests := []struct {
		view           uint64
		expectedLeader primitives.ValidatorIndex
	}{
		{0, 0},  // position 0 -> validator 0
		{5, 0},  // position 5 -> validator 0
		{10, 1}, // position 10 -> validator 1
		{20, 1}, // position 20 -> validator 1
		{30, 2}, // position 30 -> validator 2
		{50, 2}, // position 50 -> validator 2
		{60, 0}, // position 0 (wraps) -> validator 0
		{70, 1}, // position 10 (wraps) -> validator 1
	}

	for _, tt := range tests {
		leader, err := election.GetLeader(tt.view)
		require.NoError(t, err)
		assert.Equal(t, tt.expectedLeader, leader, "Leader mismatch for view %d", tt.view)
	}
}

func TestStakeWeightedLeaderElection_EqualStakes(t *testing.T) {
	// With equal stakes, should behave similar to round-robin
	validators := []primitives.ValidatorIndex{0, 1, 2, 3}
	stakes := []uint64{10, 10, 10, 10} // Equal stakes
	// Total: 40, each validator gets 10 positions

	election, err := NewStakeWeightedLeaderElection(validators, stakes)
	require.NoError(t, err)

	// Check distribution
	leaderCounts := make(map[primitives.ValidatorIndex]int)
	for view := uint64(0); view < 40; view++ {
		leader, err := election.GetLeader(view)
		require.NoError(t, err)
		leaderCounts[leader]++
	}

	// Each validator should be leader exactly 10 times
	for _, count := range leaderCounts {
		assert.Equal(t, 10, count, "Each validator should be leader 10 times with equal stakes")
	}
}

func TestStakeWeightedLeaderElection_HighStakeValidator(t *testing.T) {
	validators := []primitives.ValidatorIndex{0, 1, 2}
	stakes := []uint64{1, 1, 98} // Validator 2 has 98% of stake
	// Total: 100

	election, err := NewStakeWeightedLeaderElection(validators, stakes)
	require.NoError(t, err)

	// Count leaders over 100 views
	leaderCounts := make(map[primitives.ValidatorIndex]int)
	for view := uint64(0); view < 100; view++ {
		leader, err := election.GetLeader(view)
		require.NoError(t, err)
		leaderCounts[leader]++
	}

	// Validator 2 should be leader ~98 times
	assert.Equal(t, 1, leaderCounts[0], "Validator 0 should be leader 1 time")
	assert.Equal(t, 1, leaderCounts[1], "Validator 1 should be leader 1 time")
	assert.Equal(t, 98, leaderCounts[2], "Validator 2 should be leader 98 times")
}

func TestStakeWeightedLeaderElection_UpdateValidators(t *testing.T) {
	validators := []primitives.ValidatorIndex{0, 1, 2}
	stakes := []uint64{10, 20, 30}

	election, err := NewStakeWeightedLeaderElection(validators, stakes)
	require.NoError(t, err)

	// Update with new validators and stakes
	newValidators := []primitives.ValidatorIndex{5, 6}
	newStakes := []uint64{50, 50}
	err = election.UpdateValidators(newValidators, newStakes)
	require.NoError(t, err)

	// Check new leaders
	leader, err := election.GetLeader(0)
	require.NoError(t, err)
	assert.Equal(t, primitives.ValidatorIndex(5), leader)

	leader, err = election.GetLeader(50)
	require.NoError(t, err)
	assert.Equal(t, primitives.ValidatorIndex(6), leader)
}

func TestStakeWeightedLeaderElection_InvalidInputs(t *testing.T) {
	tests := []struct {
		name       string
		validators []primitives.ValidatorIndex
		stakes     []uint64
		errMsg     string
	}{
		{
			name:       "empty validators",
			validators: []primitives.ValidatorIndex{},
			stakes:     []uint64{},
			errMsg:     "cannot be empty",
		},
		{
			name:       "mismatched lengths",
			validators: []primitives.ValidatorIndex{0, 1},
			stakes:     []uint64{10},
			errMsg:     "must have same length",
		},
		{
			name:       "zero stake",
			validators: []primitives.ValidatorIndex{0, 1},
			stakes:     []uint64{10, 0},
			errMsg:     "zero stake",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := NewStakeWeightedLeaderElection(tt.validators, tt.stakes)
			assert.ErrorContains(t, tt.errMsg, err)
		})
	}
}

func TestNewLeaderElection(t *testing.T) {
	validators := []primitives.ValidatorIndex{0, 1, 2}
	stakes := []uint64{10, 20, 30}

	tests := []struct {
		name     string
		strategy LeaderRotation
		wantErr  bool
	}{
		{
			name:     "round-robin",
			strategy: LeaderRotationRoundRobin,
			wantErr:  false,
		},
		{
			name:     "stake-weighted",
			strategy: LeaderRotationStakeWeighted,
			wantErr:  false,
		},
		{
			name:     "unknown strategy",
			strategy: "unknown",
			wantErr:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			election, err := NewLeaderElection(tt.strategy, validators, stakes)
			if tt.wantErr {
				assert.NotNil(t, err)
				assert.Equal(t, true, election == nil)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, election)
			}
		})
	}
}

func TestViewChangeLeaderElection_GetLeader(t *testing.T) {
	validators := []primitives.ValidatorIndex{0, 1, 2, 3, 4}
	baseElection, err := NewRoundRobinLeaderElection(validators)
	require.NoError(t, err)

	vcElection := NewViewChangeLeaderElection(baseElection)

	// Initially, should return normal leaders
	leader, err := vcElection.GetLeader(0)
	require.NoError(t, err)
	assert.Equal(t, primitives.ValidatorIndex(0), leader)

	leader, err = vcElection.GetLeader(1)
	require.NoError(t, err)
	assert.Equal(t, primitives.ValidatorIndex(1), leader)

	// Mark leader 1 as failed for view 1
	vcElection.MarkLeaderFailed(1, 1)

	// Now view 1 should return next leader (view 2's leader)
	leader, err = vcElection.GetLeader(1)
	require.NoError(t, err)
	assert.Equal(t, primitives.ValidatorIndex(2), leader, "Should skip failed leader")

	// Other views should be unaffected
	leader, err = vcElection.GetLeader(0)
	require.NoError(t, err)
	assert.Equal(t, primitives.ValidatorIndex(0), leader)

	leader, err = vcElection.GetLeader(2)
	require.NoError(t, err)
	assert.Equal(t, primitives.ValidatorIndex(2), leader)
}

func TestViewChangeLeaderElection_ClearFailedLeaders(t *testing.T) {
	validators := []primitives.ValidatorIndex{0, 1, 2}
	baseElection, err := NewRoundRobinLeaderElection(validators)
	require.NoError(t, err)

	vcElection := NewViewChangeLeaderElection(baseElection)

	// Mark leader as failed
	vcElection.MarkLeaderFailed(1, 1)

	// Verify it's skipped
	leader, err := vcElection.GetLeader(1)
	require.NoError(t, err)
	assert.Equal(t, primitives.ValidatorIndex(2), leader)

	// Clear failed leaders
	vcElection.ClearFailedLeaders()

	// Now should return normal leader
	leader, err = vcElection.GetLeader(1)
	require.NoError(t, err)
	assert.Equal(t, primitives.ValidatorIndex(1), leader)
}

func TestViewChangeLeaderElection_IsLeader(t *testing.T) {
	validators := []primitives.ValidatorIndex{0, 1, 2}
	baseElection, err := NewRoundRobinLeaderElection(validators)
	require.NoError(t, err)

	vcElection := NewViewChangeLeaderElection(baseElection)

	// Mark leader 1 as failed for view 1
	vcElection.MarkLeaderFailed(1, 1)

	// Validator 1 should not be leader for view 1
	assert.Equal(t, false, vcElection.IsLeader(1, 1))

	// Validator 2 should be leader for view 1 (next in line)
	assert.Equal(t, true, vcElection.IsLeader(1, 2))
}
