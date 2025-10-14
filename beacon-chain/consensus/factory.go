package consensus

import (
	"context"

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
	// This will be implemented in later steps
	return nil, errors.New("HotStuff consensus not yet implemented")
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
