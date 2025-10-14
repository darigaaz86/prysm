package hotstuff

import (
	"fmt"

	"github.com/OffchainLabs/prysm/v6/consensus-types/primitives"
	"github.com/pkg/errors"
)

// LeaderElection defines the interface for leader election strategies.
type LeaderElection interface {
	// GetLeader returns the leader for a given view.
	GetLeader(view uint64) (primitives.ValidatorIndex, error)
	// IsLeader checks if a validator is the leader for a given view.
	IsLeader(view uint64, validatorIndex primitives.ValidatorIndex) bool
	// UpdateValidators updates the validator set.
	UpdateValidators(validators []primitives.ValidatorIndex, stakes []uint64) error
}

// RoundRobinLeaderElection implements simple round-robin leader selection.
// Each validator takes turns being the leader in order.
type RoundRobinLeaderElection struct {
	validators []primitives.ValidatorIndex
}

// NewRoundRobinLeaderElection creates a new round-robin leader election.
func NewRoundRobinLeaderElection(validators []primitives.ValidatorIndex) (*RoundRobinLeaderElection, error) {
	if len(validators) == 0 {
		return nil, errors.New("validator list cannot be empty")
	}

	return &RoundRobinLeaderElection{
		validators: validators,
	}, nil
}

// GetLeader returns the leader for a given view using round-robin.
func (r *RoundRobinLeaderElection) GetLeader(view uint64) (primitives.ValidatorIndex, error) {
	if len(r.validators) == 0 {
		return 0, errors.New("no validators available")
	}

	// Simple round-robin: view % num_validators
	leaderIdx := view % uint64(len(r.validators))
	return r.validators[leaderIdx], nil
}

// IsLeader checks if a validator is the leader for a given view.
func (r *RoundRobinLeaderElection) IsLeader(view uint64, validatorIndex primitives.ValidatorIndex) bool {
	leader, err := r.GetLeader(view)
	if err != nil {
		return false
	}
	return leader == validatorIndex
}

// UpdateValidators updates the validator set.
func (r *RoundRobinLeaderElection) UpdateValidators(validators []primitives.ValidatorIndex, stakes []uint64) error {
	if len(validators) == 0 {
		return errors.New("validator list cannot be empty")
	}
	r.validators = validators
	return nil
}

// StakeWeightedLeaderElection implements stake-weighted leader selection.
// Validators with higher stake have proportionally higher chance of being leader.
type StakeWeightedLeaderElection struct {
	validators       []primitives.ValidatorIndex
	stakes           []uint64
	totalStake       uint64
	cumulativeStakes []uint64 // Cumulative stakes for binary search
}

// NewStakeWeightedLeaderElection creates a new stake-weighted leader election.
func NewStakeWeightedLeaderElection(validators []primitives.ValidatorIndex, stakes []uint64) (*StakeWeightedLeaderElection, error) {
	if len(validators) == 0 {
		return nil, errors.New("validator list cannot be empty")
	}
	if len(validators) != len(stakes) {
		return nil, errors.New("validators and stakes must have same length")
	}

	s := &StakeWeightedLeaderElection{
		validators: validators,
		stakes:     stakes,
	}

	if err := s.computeCumulativeStakes(); err != nil {
		return nil, err
	}

	return s, nil
}

// computeCumulativeStakes computes cumulative stakes for efficient lookup.
func (s *StakeWeightedLeaderElection) computeCumulativeStakes() error {
	s.totalStake = 0
	s.cumulativeStakes = make([]uint64, len(s.stakes))

	for i, stake := range s.stakes {
		if stake == 0 {
			return fmt.Errorf("validator %d has zero stake", i)
		}
		s.totalStake += stake
		s.cumulativeStakes[i] = s.totalStake
	}

	if s.totalStake == 0 {
		return errors.New("total stake is zero")
	}

	return nil
}

// GetLeader returns the leader for a given view using stake-weighted selection.
// Uses the view number as a deterministic seed for selection.
func (s *StakeWeightedLeaderElection) GetLeader(view uint64) (primitives.ValidatorIndex, error) {
	if len(s.validators) == 0 {
		return 0, errors.New("no validators available")
	}
	if s.totalStake == 0 {
		return 0, errors.New("total stake is zero")
	}

	// Use view as deterministic seed
	// Map view to a position in the total stake range
	position := view % s.totalStake

	// Binary search to find the validator at this position
	idx := s.findValidatorByStake(position)
	return s.validators[idx], nil
}

// findValidatorByStake finds the validator index for a given stake position.
func (s *StakeWeightedLeaderElection) findValidatorByStake(position uint64) int {
	// Binary search in cumulative stakes
	left, right := 0, len(s.cumulativeStakes)-1

	for left < right {
		mid := (left + right) / 2
		if position < s.cumulativeStakes[mid] {
			right = mid
		} else {
			left = mid + 1
		}
	}

	return left
}

// IsLeader checks if a validator is the leader for a given view.
func (s *StakeWeightedLeaderElection) IsLeader(view uint64, validatorIndex primitives.ValidatorIndex) bool {
	leader, err := s.GetLeader(view)
	if err != nil {
		return false
	}
	return leader == validatorIndex
}

// UpdateValidators updates the validator set and their stakes.
func (s *StakeWeightedLeaderElection) UpdateValidators(validators []primitives.ValidatorIndex, stakes []uint64) error {
	if len(validators) == 0 {
		return errors.New("validator list cannot be empty")
	}
	if len(validators) != len(stakes) {
		return errors.New("validators and stakes must have same length")
	}

	s.validators = validators
	s.stakes = stakes

	return s.computeCumulativeStakes()
}

// LeaderRotation represents the leader rotation strategy.
type LeaderRotation string

const (
	// LeaderRotationRoundRobin uses simple round-robin rotation.
	LeaderRotationRoundRobin LeaderRotation = "round-robin"
	// LeaderRotationStakeWeighted uses stake-weighted rotation.
	LeaderRotationStakeWeighted LeaderRotation = "stake-weighted"
)

// NewLeaderElection creates a new leader election based on the strategy.
func NewLeaderElection(
	strategy LeaderRotation,
	validators []primitives.ValidatorIndex,
	stakes []uint64,
) (LeaderElection, error) {
	switch strategy {
	case LeaderRotationRoundRobin:
		return NewRoundRobinLeaderElection(validators)
	case LeaderRotationStakeWeighted:
		return NewStakeWeightedLeaderElection(validators, stakes)
	default:
		return nil, fmt.Errorf("unknown leader rotation strategy: %s", strategy)
	}
}

// ViewChangeLeaderElection handles leader election during view changes.
// It ensures that the new leader is different from the failed leader.
type ViewChangeLeaderElection struct {
	baseElection  LeaderElection
	failedLeaders map[uint64]primitives.ValidatorIndex // view -> failed leader
}

// NewViewChangeLeaderElection creates a new view change leader election.
func NewViewChangeLeaderElection(baseElection LeaderElection) *ViewChangeLeaderElection {
	return &ViewChangeLeaderElection{
		baseElection:  baseElection,
		failedLeaders: make(map[uint64]primitives.ValidatorIndex),
	}
}

// GetLeader returns the leader for a given view, skipping failed leaders.
func (v *ViewChangeLeaderElection) GetLeader(view uint64) (primitives.ValidatorIndex, error) {
	// Get the base leader
	leader, err := v.baseElection.GetLeader(view)
	if err != nil {
		return 0, err
	}

	// Check if this leader has failed in this view
	if failedLeader, exists := v.failedLeaders[view]; exists && failedLeader == leader {
		// Try next leader
		return v.baseElection.GetLeader(view + 1)
	}

	return leader, nil
}

// IsLeader checks if a validator is the leader for a given view.
func (v *ViewChangeLeaderElection) IsLeader(view uint64, validatorIndex primitives.ValidatorIndex) bool {
	leader, err := v.GetLeader(view)
	if err != nil {
		return false
	}
	return leader == validatorIndex
}

// UpdateValidators updates the validator set.
func (v *ViewChangeLeaderElection) UpdateValidators(validators []primitives.ValidatorIndex, stakes []uint64) error {
	return v.baseElection.UpdateValidators(validators, stakes)
}

// MarkLeaderFailed marks a leader as failed for a given view.
func (v *ViewChangeLeaderElection) MarkLeaderFailed(view uint64, leader primitives.ValidatorIndex) {
	v.failedLeaders[view] = leader
}

// ClearFailedLeaders clears the failed leaders map (e.g., after successful view).
func (v *ViewChangeLeaderElection) ClearFailedLeaders() {
	v.failedLeaders = make(map[uint64]primitives.ValidatorIndex)
}
