// Package pos provides a wrapper around Prysm's existing PoS consensus implementation.
// It adapts the blockchain.Service to the consensus.Consensus interface.
package pos

import (
	"context"

	"github.com/OffchainLabs/prysm/v6/beacon-chain/blockchain"
	"github.com/OffchainLabs/prysm/v6/beacon-chain/consensus"
	"github.com/OffchainLabs/prysm/v6/beacon-chain/state"
	"github.com/OffchainLabs/prysm/v6/consensus-types/interfaces"
	"github.com/OffchainLabs/prysm/v6/consensus-types/primitives"
	ethpb "github.com/OffchainLabs/prysm/v6/proto/prysm/v1alpha1"
	"github.com/pkg/errors"
)

// Service wraps the existing blockchain.Service to implement the consensus.Consensus interface.
// This allows the existing PoS implementation to work with the new consensus abstraction.
type Service struct {
	blockchain *blockchain.Service
	config     *consensus.PoSConfig
}

// NewService creates a new PoS consensus service that wraps the existing blockchain service.
func NewService(blockchainService *blockchain.Service, config *consensus.PoSConfig) (*Service, error) {
	if blockchainService == nil {
		return nil, errors.New("blockchain service cannot be nil")
	}
	if config == nil {
		return nil, errors.New("PoS config cannot be nil")
	}

	return &Service{
		blockchain: blockchainService,
		config:     config,
	}, nil
}

// Start starts the PoS consensus service.
func (s *Service) Start() error {
	// The blockchain service is already started by the beacon node
	// This is just a pass-through
	return nil
}

// Stop stops the PoS consensus service.
func (s *Service) Stop() error {
	// The blockchain service is stopped by the beacon node
	// This is just a pass-through
	return nil
}

// Status returns the current status of the PoS consensus.
func (s *Service) Status() *consensus.Status {
	headSlot := s.blockchain.HeadSlot()
	finalizedCheckpoint := s.blockchain.FinalizedCheckpt()
	justifiedCheckpoint := s.blockchain.CurrentJustifiedCheckpt()

	return &consensus.Status{
		Mode:           consensus.ModePoS,
		Synced:         s.blockchain.Synced(),
		HeadSlot:       headSlot,
		FinalizedEpoch: finalizedCheckpoint.Epoch,
		JustifiedEpoch: justifiedCheckpoint.Epoch,
		// ValidatorCount and ActiveValidators would need to be retrieved from state
		// For now, we'll leave them as 0
		ValidatorCount:   0,
		ActiveValidators: 0,
	}
}

// ReceiveBlock receives a new block from the network.
func (s *Service) ReceiveBlock(ctx context.Context, block interfaces.ReadOnlySignedBeaconBlock, blockRoot [32]byte) error {
	return s.blockchain.ReceiveBlock(ctx, block, blockRoot)
}

// ProcessBlock processes a block through the state transition.
func (s *Service) ProcessBlock(ctx context.Context, block interfaces.ReadOnlySignedBeaconBlock) error {
	// The blockchain service handles block processing internally in ReceiveBlock
	// This is a simplified wrapper
	blockRoot, err := block.Block().HashTreeRoot()
	if err != nil {
		return errors.Wrap(err, "could not get block root")
	}
	return s.blockchain.ReceiveBlock(ctx, block, blockRoot)
}

// ProposeBlock proposes a new block for the given slot.
func (s *Service) ProposeBlock(ctx context.Context, slot primitives.Slot) (interfaces.ReadOnlySignedBeaconBlock, error) {
	// Block proposal is handled by the validator client, not the blockchain service
	// This would need to be implemented by integrating with the validator client
	return nil, errors.New("block proposal not implemented in PoS wrapper")
}

// ReceiveAttestation receives a new attestation from the network.
func (s *Service) ReceiveAttestation(ctx context.Context, att *ethpb.Attestation) error {
	return s.blockchain.ReceiveAttestation(ctx, att)
}

// ProcessAttestation processes an attestation.
func (s *Service) ProcessAttestation(ctx context.Context, att *ethpb.Attestation) error {
	return s.blockchain.OnAttestation(ctx, att, 0) // 0 for unknown delay
}

// Head returns the current head block root.
func (s *Service) Head(ctx context.Context) ([32]byte, error) {
	return s.blockchain.HeadRoot(ctx)
}

// HeadSlot returns the slot of the current head block.
func (s *Service) HeadSlot() primitives.Slot {
	return s.blockchain.HeadSlot()
}

// HeadRoot returns the root of the current head block.
func (s *Service) HeadRoot() [32]byte {
	root, err := s.blockchain.HeadRoot(context.Background())
	if err != nil {
		// Return zero hash on error
		return [32]byte{}
	}
	return root
}

// HeadBlock returns the current head block.
func (s *Service) HeadBlock() interfaces.ReadOnlySignedBeaconBlock {
	return s.blockchain.HeadBlock()
}

// HeadState returns the current head state.
func (s *Service) HeadState(ctx context.Context) (state.BeaconState, error) {
	return s.blockchain.HeadState(ctx)
}

// FinalizedCheckpoint returns the current finalized checkpoint.
func (s *Service) FinalizedCheckpoint() *ethpb.Checkpoint {
	return s.blockchain.FinalizedCheckpt()
}

// CurrentJustifiedCheckpoint returns the current justified checkpoint.
func (s *Service) CurrentJustifiedCheckpoint() *ethpb.Checkpoint {
	return s.blockchain.CurrentJustifiedCheckpt()
}

// PreviousJustifiedCheckpoint returns the previous justified checkpoint.
func (s *Service) PreviousJustifiedCheckpoint() *ethpb.Checkpoint {
	return s.blockchain.PreviousJustifiedCheckpt()
}

// IsCanonical checks if a block is part of the canonical chain.
func (s *Service) IsCanonical(ctx context.Context, blockRoot [32]byte) (bool, error) {
	return s.blockchain.IsCanonical(ctx, blockRoot)
}

// IsOptimistic checks if a block is optimistically imported.
func (s *Service) IsOptimistic(blockRoot [32]byte) (bool, error) {
	return s.blockchain.IsOptimistic(context.Background(), blockRoot)
}

// InForkchoice checks if a block is in the forkchoice store.
func (s *Service) InForkchoice(blockRoot [32]byte) bool {
	return s.blockchain.InForkchoice(blockRoot)
}

// CurrentSlot returns the current slot based on genesis time.
func (s *Service) CurrentSlot() primitives.Slot {
	return s.blockchain.CurrentSlot()
}

// GenesisTime returns the genesis time of the chain.
func (s *Service) GenesisTime() uint64 {
	return s.blockchain.GenesisTime().Unix()
}
