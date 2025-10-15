package consensus

import (
	"context"

	"github.com/OffchainLabs/prysm/v6/beacon-chain/consensus/hotstuff"
	"github.com/OffchainLabs/prysm/v6/beacon-chain/state"
	"github.com/OffchainLabs/prysm/v6/consensus-types/interfaces"
	"github.com/OffchainLabs/prysm/v6/consensus-types/primitives"
	ethpb "github.com/OffchainLabs/prysm/v6/proto/prysm/v1alpha1"
	"github.com/pkg/errors"
)

// Factory creates consensus instances based on configuration.
type Factory struct {
	config *Config
}

// NewFactory creates a new consensus factory.
func NewFactory(config *Config) (*Factory, error) {
	if config == nil {
		config = DefaultConfig()
	}

	if err := config.Validate(); err != nil {
		return nil, errors.Wrap(err, "invalid consensus configuration")
	}

	return &Factory{
		config: config,
	}, nil
}

// Create creates a new consensus instance based on the configured mode.
func (f *Factory) Create(ctx context.Context, opts ...Option) (Consensus, error) {
	switch f.config.Mode {
	case ModePoS:
		return f.createPoS(ctx, opts...)
	case ModeHotStuff:
		return f.createHotStuff(ctx, opts...)
	default:
		return nil, errors.Errorf("unsupported consensus mode: %s", f.config.Mode)
	}
}

// createPoS creates a PoS consensus instance.
func (f *Factory) createPoS(ctx context.Context, opts ...Option) (Consensus, error) {
	// Note: The actual blockchain.Service needs to be passed in via options
	// This is because the blockchain service is created by the beacon node
	// and has many dependencies that we don't want to recreate here.

	// For now, we return an error indicating that the blockchain service
	// must be provided via WithBlockchainService option
	return nil, errors.New("PoS consensus requires blockchain service to be provided via WithBlockchainService option")
}

// createHotStuff creates a HotStuff consensus instance.
func (f *Factory) createHotStuff(ctx context.Context, opts ...Option) (Consensus, error) {
	// Get HotStuff-specific config
	if f.config.HotStuff == nil {
		return nil, errors.New("HotStuff configuration not found")
	}

	// Convert to hotstuff.Config
	hotStuffCfg := &hotstuff.Config{
		ViewTimeout:      f.config.HotStuff.ViewTimeout,
		BlockTime:        f.config.HotStuff.BlockTime,
		MinValidators:    f.config.HotStuff.MinValidators,
		QuorumThreshold:  f.config.HotStuff.QuorumThreshold,
		LeaderRotation:   f.config.HotStuff.LeaderRotation,
		EnableViewChange: f.config.HotStuff.EnableViewChange,
	}

	// Create the HotStuff service
	service, err := hotstuff.NewService(ctx, hotStuffCfg)
	if err != nil {
		return nil, errors.Wrap(err, "failed to create HotStuff service")
	}

	// Wrap in adapter to match Consensus interface
	return &hotStuffAdapter{service: service}, nil
}

// Mode returns the configured consensus mode.
func (f *Factory) Mode() Mode {
	return f.config.Mode
}

// Config returns the consensus configuration.
func (f *Factory) Config() *Config {
	return f.config
}

// Option is a functional option for configuring consensus instances.
type Option func(interface{}) error

// WithBlockchainService provides the existing blockchain service for PoS mode.
// This is required when using PoS consensus mode.
func WithBlockchainService(blockchainService interface{}) Option {
	return func(c interface{}) error {
		// This will be used when creating PoS consensus
		// The actual implementation will be in the PoS service creation
		return nil
	}
}

// WithGenesisTime sets the genesis time for the consensus instance.
func WithGenesisTime(genesisTime uint64) Option {
	return func(c interface{}) error {
		// This will be implemented when we create the actual consensus services
		return nil
	}
}

// WithValidatorCount sets the validator count for the consensus instance.
func WithValidatorCount(count uint64) Option {
	return func(c interface{}) error {
		// This will be implemented when we create the actual consensus services
		return nil
	}
}

// hotStuffAdapter adapts the HotStuff service to the Consensus interface.
type hotStuffAdapter struct {
	service *hotstuff.Service
}

func (a *hotStuffAdapter) Start() error {
	return a.service.Start()
}

func (a *hotStuffAdapter) Stop() error {
	return a.service.Stop()
}

func (a *hotStuffAdapter) Status() *Status {
	hs := a.service.Status()
	return &Status{
		Mode:             ModeHotStuff,
		Synced:           hs.Synced,
		HeadSlot:         hs.HeadSlot,
		FinalizedEpoch:   hs.FinalizedEpoch,
		JustifiedEpoch:   hs.JustifiedEpoch,
		ValidatorCount:   hs.ValidatorCount,
		ActiveValidators: hs.ActiveValidators,
	}
}

func (a *hotStuffAdapter) ReceiveBlock(ctx context.Context, block interfaces.ReadOnlySignedBeaconBlock, blockRoot [32]byte) error {
	return a.service.ReceiveBlock(ctx, block, blockRoot)
}

func (a *hotStuffAdapter) ProcessBlock(ctx context.Context, block interfaces.ReadOnlySignedBeaconBlock) error {
	return a.service.ProcessBlock(ctx, block)
}

func (a *hotStuffAdapter) ProposeBlock(ctx context.Context, slot primitives.Slot) (interfaces.ReadOnlySignedBeaconBlock, error) {
	return a.service.ProposeBlock(ctx, slot)
}

func (a *hotStuffAdapter) ReceiveAttestation(ctx context.Context, att *ethpb.Attestation) error {
	return a.service.ReceiveAttestation(ctx, att)
}

func (a *hotStuffAdapter) ProcessAttestation(ctx context.Context, att *ethpb.Attestation) error {
	return a.service.ProcessAttestation(ctx, att)
}

func (a *hotStuffAdapter) Head(ctx context.Context) ([32]byte, error) {
	return a.service.Head(ctx)
}

func (a *hotStuffAdapter) HeadSlot() primitives.Slot {
	return a.service.HeadSlot()
}

func (a *hotStuffAdapter) HeadRoot() [32]byte {
	return a.service.HeadRoot()
}

func (a *hotStuffAdapter) HeadBlock() interfaces.ReadOnlySignedBeaconBlock {
	return a.service.HeadBlock()
}

func (a *hotStuffAdapter) HeadState(ctx context.Context) (state.BeaconState, error) {
	return a.service.HeadState(ctx)
}

func (a *hotStuffAdapter) FinalizedCheckpoint() *ethpb.Checkpoint {
	return a.service.FinalizedCheckpoint()
}

func (a *hotStuffAdapter) CurrentJustifiedCheckpoint() *ethpb.Checkpoint {
	return a.service.CurrentJustifiedCheckpoint()
}

func (a *hotStuffAdapter) PreviousJustifiedCheckpoint() *ethpb.Checkpoint {
	return a.service.PreviousJustifiedCheckpoint()
}

func (a *hotStuffAdapter) IsCanonical(ctx context.Context, blockRoot [32]byte) (bool, error) {
	return a.service.IsCanonical(ctx, blockRoot)
}

func (a *hotStuffAdapter) IsOptimistic(blockRoot [32]byte) (bool, error) {
	return a.service.IsOptimistic(blockRoot)
}

func (a *hotStuffAdapter) InForkchoice(blockRoot [32]byte) bool {
	return a.service.InForkchoice(blockRoot)
}

func (a *hotStuffAdapter) CurrentSlot() primitives.Slot {
	return a.service.CurrentSlot()
}

func (a *hotStuffAdapter) GenesisTime() uint64 {
	return a.service.GenesisTime()
}
