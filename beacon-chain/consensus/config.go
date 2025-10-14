package consensus

import (
	"time"

	"github.com/pkg/errors"
)

// Config holds the configuration for the consensus mechanism.
type Config struct {
	// Mode specifies which consensus mechanism to use.
	Mode Mode `yaml:"mode"`

	// PoS configuration
	PoS *PoSConfig `yaml:"pos,omitempty"`

	// HotStuff configuration
	HotStuff *HotStuffConfig `yaml:"hotstuff,omitempty"`
}

// PoSConfig holds configuration for Ethereum PoS consensus.
type PoSConfig struct {
	// SlotDuration is the duration of each slot (default: 12 seconds).
	SlotDuration time.Duration `yaml:"slot_duration"`

	// SlotsPerEpoch is the number of slots per epoch (default: 32).
	SlotsPerEpoch uint64 `yaml:"slots_per_epoch"`

	// EpochsPerSyncCommittee is the number of epochs per sync committee period.
	EpochsPerSyncCommittee uint64 `yaml:"epochs_per_sync_committee"`
}

// HotStuffConfig holds configuration for HotStuff consensus.
type HotStuffConfig struct {
	// ViewTimeout is the timeout for each view before triggering view change.
	ViewTimeout time.Duration `yaml:"view_timeout"`

	// BlockTime is the target time between blocks.
	BlockTime time.Duration `yaml:"block_time"`

	// MinValidators is the minimum number of validators required.
	MinValidators uint64 `yaml:"min_validators"`

	// QuorumThreshold is the fraction of validators needed for quorum (e.g., 0.67 for 2f+1).
	QuorumThreshold float64 `yaml:"quorum_threshold"`

	// LeaderRotation specifies the leader rotation strategy.
	// Options: "round-robin", "stake-weighted"
	LeaderRotation string `yaml:"leader_rotation"`

	// EnableViewChange enables the view change mechanism.
	EnableViewChange bool `yaml:"enable_view_change"`
}

// DefaultConfig returns the default consensus configuration.
func DefaultConfig() *Config {
	return &Config{
		Mode: ModePoS,
		PoS: &PoSConfig{
			SlotDuration:           12 * time.Second,
			SlotsPerEpoch:          32,
			EpochsPerSyncCommittee: 256,
		},
		HotStuff: &HotStuffConfig{
			ViewTimeout:      10 * time.Second,
			BlockTime:        6 * time.Second,
			MinValidators:    4,
			QuorumThreshold:  0.67,
			LeaderRotation:   "round-robin",
			EnableViewChange: true,
		},
	}
}

// Validate validates the consensus configuration.
func (c *Config) Validate() error {
	switch c.Mode {
	case ModePoS:
		if c.PoS == nil {
			return errors.New("PoS configuration is required when mode is 'pos'")
		}
		return c.PoS.Validate()
	case ModeHotStuff:
		if c.HotStuff == nil {
			return errors.New("HotStuff configuration is required when mode is 'hotstuff'")
		}
		return c.HotStuff.Validate()
	default:
		return errors.Errorf("unknown consensus mode: %s", c.Mode)
	}
}

// Validate validates the PoS configuration.
func (c *PoSConfig) Validate() error {
	if c.SlotDuration <= 0 {
		return errors.New("slot duration must be positive")
	}
	if c.SlotsPerEpoch == 0 {
		return errors.New("slots per epoch must be positive")
	}
	if c.EpochsPerSyncCommittee == 0 {
		return errors.New("epochs per sync committee must be positive")
	}
	return nil
}

// Validate validates the HotStuff configuration.
func (c *HotStuffConfig) Validate() error {
	if c.ViewTimeout <= 0 {
		return errors.New("view timeout must be positive")
	}
	if c.BlockTime <= 0 {
		return errors.New("block time must be positive")
	}
	if c.MinValidators < 4 {
		return errors.New("minimum validators must be at least 4 for BFT")
	}
	if c.QuorumThreshold <= 0.5 || c.QuorumThreshold > 1.0 {
		return errors.New("quorum threshold must be between 0.5 and 1.0")
	}
	if c.LeaderRotation != "round-robin" && c.LeaderRotation != "stake-weighted" {
		return errors.Errorf("invalid leader rotation strategy: %s", c.LeaderRotation)
	}
	return nil
}
