// Package consensus provides an abstraction layer for different consensus mechanisms.
// It allows switching between Ethereum PoS and alternative consensus algorithms like HotStuff.
package consensus

import (
	"context"

	"github.com/OffchainLabs/prysm/v6/beacon-chain/state"
	"github.com/OffchainLabs/prysm/v6/consensus-types/interfaces"
	"github.com/OffchainLabs/prysm/v6/consensus-types/primitives"
	ethpb "github.com/OffchainLabs/prysm/v6/proto/prysm/v1alpha1"
)

// Consensus defines the interface for consensus mechanisms.
// This interface abstracts the core consensus operations, allowing different
// implementations (PoS, HotStuff, etc.) to be plugged in.
type Consensus interface {
	// Lifecycle management
	Start() error
	Stop() error
	Status() *Status

	// Block operations
	ReceiveBlock(ctx context.Context, block interfaces.ReadOnlySignedBeaconBlock, blockRoot [32]byte) error
	ProcessBlock(ctx context.Context, block interfaces.ReadOnlySignedBeaconBlock) error
	ProposeBlock(ctx context.Context, slot primitives.Slot) (interfaces.ReadOnlySignedBeaconBlock, error)

	// Vote/Attestation operations
	ReceiveAttestation(ctx context.Context, att *ethpb.Attestation) error
	ProcessAttestation(ctx context.Context, att *ethpb.Attestation) error

	// Chain head operations
	Head(ctx context.Context) ([32]byte, error)
	HeadSlot() primitives.Slot
	HeadRoot() [32]byte
	HeadBlock() interfaces.ReadOnlySignedBeaconBlock
	HeadState(ctx context.Context) (state.BeaconState, error)

	// Finality operations
	FinalizedCheckpoint() *ethpb.Checkpoint
	CurrentJustifiedCheckpoint() *ethpb.Checkpoint
	PreviousJustifiedCheckpoint() *ethpb.Checkpoint

	// Chain info
	IsCanonical(ctx context.Context, blockRoot [32]byte) (bool, error)
	IsOptimistic(blockRoot [32]byte) (bool, error)
	InForkchoice(blockRoot [32]byte) bool

	// State operations
	CurrentSlot() primitives.Slot
	GenesisTime() uint64
}

// Status represents the current status of the consensus mechanism.
type Status struct {
	Mode             Mode
	Synced           bool
	HeadSlot         primitives.Slot
	FinalizedEpoch   primitives.Epoch
	JustifiedEpoch   primitives.Epoch
	ValidatorCount   uint64
	ActiveValidators uint64
}

// Mode represents the consensus mode.
type Mode string

const (
	// ModePoS represents Ethereum Proof of Stake consensus.
	ModePoS Mode = "pos"
	// ModeHotStuff represents HotStuff BFT consensus.
	ModeHotStuff Mode = "hotstuff"
)

// Vote represents a validator's vote in the consensus mechanism.
// This is a generic interface that can represent:
// - Attestations in PoS
// - Votes in HotStuff
type Vote interface {
	BlockRoot() [32]byte
	Slot() primitives.Slot
	ValidatorIndex() primitives.ValidatorIndex
	Signature() []byte
}

// Checkpoint represents a finality checkpoint.
type Checkpoint struct {
	Epoch primitives.Epoch
	Root  [32]byte
}
