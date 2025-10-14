package hotstuff

import (
	"context"
	"sync"
	"time"

	"github.com/OffchainLabs/prysm/v6/beacon-chain/consensus"
	"github.com/OffchainLabs/prysm/v6/beacon-chain/state"
	"github.com/OffchainLabs/prysm/v6/consensus-types/interfaces"
	"github.com/OffchainLabs/prysm/v6/consensus-types/primitives"
	"github.com/OffchainLabs/prysm/v6/crypto/bls"
	ethpb "github.com/OffchainLabs/prysm/v6/proto/prysm/v1alpha1"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
)

var log = logrus.WithField("prefix", "hotstuff")

// Service implements the HotStuff consensus protocol.
type Service struct {
	// Configuration
	cfg *consensus.HotStuffConfig

	// State
	currentView  uint64
	currentPhase Phase
	genesisTime  time.Time
	validatorIdx primitives.ValidatorIndex

	// Leader election
	leaderElection LeaderElection

	// Block tree
	blocks       map[[32]byte]*BlockNode
	blocksByView map[uint64]*BlockNode
	highestQC    *QuorumCertificate
	lockedQC     *QuorumCertificate
	genesisBlock *BlockNode

	// Voting
	qcBuilders map[uint64]map[Phase]*QCBuilder // view -> phase -> builder
	validators []primitives.ValidatorIndex
	publicKeys []bls.PublicKey
	privateKey bls.SecretKey
	quorumSize uint64

	// Channels
	blockChan    chan *HotStuffBlock
	voteChan     chan *Vote
	viewChangeCh chan *ViewChangeMsg
	stopChan     chan struct{}

	// Timers
	viewTimer   *time.Timer
	viewTimeout time.Duration

	// Synchronization
	mu sync.RWMutex

	// Lifecycle
	ctx     context.Context
	cancel  context.CancelFunc
	started bool
}

// NewService creates a new HotStuff consensus service.
func NewService(ctx context.Context, cfg *consensus.HotStuffConfig) (*Service, error) {
	if cfg == nil {
		return nil, errors.New("config cannot be nil")
	}

	ctx, cancel := context.WithCancel(ctx)

	s := &Service{
		cfg:          cfg,
		currentView:  0,
		currentPhase: PhasePrepare,
		blocks:       make(map[[32]byte]*BlockNode),
		blocksByView: make(map[uint64]*BlockNode),
		qcBuilders:   make(map[uint64]map[Phase]*QCBuilder),
		blockChan:    make(chan *HotStuffBlock, 100),
		voteChan:     make(chan *Vote, 1000),
		viewChangeCh: make(chan *ViewChangeMsg, 100),
		stopChan:     make(chan struct{}),
		viewTimeout:  cfg.ViewTimeout,
		ctx:          ctx,
		cancel:       cancel,
	}

	return s, nil
}

// Start starts the HotStuff consensus service.
func (s *Service) Start() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.started {
		return errors.New("service already started")
	}

	log.Info("Starting HotStuff consensus service")

	// Initialize genesis block
	if err := s.initializeGenesis(); err != nil {
		return errors.Wrap(err, "failed to initialize genesis")
	}

	// Start main loop
	go s.run()

	s.started = true
	return nil
}

// Stop stops the HotStuff consensus service.
func (s *Service) Stop() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if !s.started {
		return nil
	}

	log.Info("Stopping HotStuff consensus service")

	close(s.stopChan)
	s.cancel()

	if s.viewTimer != nil {
		s.viewTimer.Stop()
	}

	s.started = false
	return nil
}

// Status returns the current status of the consensus.
func (s *Service) Status() *consensus.Status {
	s.mu.RLock()
	defer s.mu.RUnlock()

	return &consensus.Status{
		Mode:             consensus.ModeHotStuff,
		Synced:           true, // TODO: Implement proper sync status
		HeadSlot:         primitives.Slot(s.currentView),
		ValidatorCount:   uint64(len(s.validators)),
		ActiveValidators: uint64(len(s.validators)),
	}
}

// initializeGenesis initializes the genesis block.
func (s *Service) initializeGenesis() error {
	// Create genesis block
	genesisBlock := &HotStuffBlock{
		View: 0,
		// BeaconBlock will be set when we integrate with execution layer
	}

	genesisHash := [32]byte{} // Genesis hash is all zeros
	s.genesisBlock = &BlockNode{
		Block:  genesisBlock,
		Status: StatusDecided,
	}

	s.blocks[genesisHash] = s.genesisBlock
	s.blocksByView[0] = s.genesisBlock

	// Initialize with genesis QC
	s.highestQC = &QuorumCertificate{
		View:      0,
		Phase:     PhaseCommit,
		BlockHash: genesisHash,
	}
	s.lockedQC = s.highestQC

	return nil
}

// run is the main consensus loop.
func (s *Service) run() {
	s.startViewTimer()

	for {
		select {
		case <-s.stopChan:
			return

		case block := <-s.blockChan:
			if err := s.handleBlock(block); err != nil {
				log.WithError(err).Error("Failed to handle block")
			}

		case vote := <-s.voteChan:
			if err := s.handleVote(vote); err != nil {
				log.WithError(err).Error("Failed to handle vote")
			}

		case msg := <-s.viewChangeCh:
			if err := s.handleViewChange(msg); err != nil {
				log.WithError(err).Error("Failed to handle view change")
			}

		case <-s.viewTimer.C:
			if err := s.handleTimeout(); err != nil {
				log.WithError(err).Error("Failed to handle timeout")
			}
		}
	}
}

// startViewTimer starts the view timeout timer.
func (s *Service) startViewTimer() {
	if s.viewTimer != nil {
		s.viewTimer.Stop()
	}
	s.viewTimer = time.NewTimer(s.viewTimeout)
}

// resetViewTimer resets the view timeout timer.
func (s *Service) resetViewTimer() {
	s.startViewTimer()
}

// isLeader checks if this validator is the leader for the current view.
func (s *Service) isLeader() bool {
	if s.leaderElection == nil {
		return false
	}
	return s.leaderElection.IsLeader(s.currentView, s.validatorIdx)
}

// getLeader returns the leader for the current view.
func (s *Service) getLeader() (primitives.ValidatorIndex, error) {
	if s.leaderElection == nil {
		return 0, errors.New("leader election not initialized")
	}
	return s.leaderElection.GetLeader(s.currentView)
}

// Consensus interface implementation stubs (to be implemented in integration)

func (s *Service) ReceiveBlock(ctx context.Context, block interfaces.ReadOnlySignedBeaconBlock, blockRoot [32]byte) error {
	return errors.New("not implemented")
}

func (s *Service) ProcessBlock(ctx context.Context, block interfaces.ReadOnlySignedBeaconBlock) error {
	return errors.New("not implemented")
}

func (s *Service) ProposeBlock(ctx context.Context, slot primitives.Slot) (interfaces.ReadOnlySignedBeaconBlock, error) {
	return nil, errors.New("not implemented")
}

func (s *Service) ReceiveAttestation(ctx context.Context, att *ethpb.Attestation) error {
	return errors.New("not implemented")
}

func (s *Service) ProcessAttestation(ctx context.Context, att *ethpb.Attestation) error {
	return errors.New("not implemented")
}

func (s *Service) Head(ctx context.Context) ([32]byte, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if s.highestQC == nil {
		return [32]byte{}, nil
	}
	return s.highestQC.BlockHash, nil
}

func (s *Service) HeadSlot() primitives.Slot {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return primitives.Slot(s.currentView)
}

func (s *Service) HeadRoot() [32]byte {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if s.highestQC == nil {
		return [32]byte{}
	}
	return s.highestQC.BlockHash
}

func (s *Service) HeadBlock() interfaces.ReadOnlySignedBeaconBlock {
	return nil // TODO: Implement
}

func (s *Service) HeadState(ctx context.Context) (state.BeaconState, error) {
	return nil, errors.New("not implemented")
}

func (s *Service) FinalizedCheckpoint() *ethpb.Checkpoint {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if s.lockedQC == nil {
		return &ethpb.Checkpoint{}
	}

	return &ethpb.Checkpoint{
		Epoch: primitives.Epoch(s.lockedQC.View),
		Root:  s.lockedQC.BlockHash[:],
	}
}

func (s *Service) CurrentJustifiedCheckpoint() *ethpb.Checkpoint {
	return s.FinalizedCheckpoint() // Simplified for now
}

func (s *Service) PreviousJustifiedCheckpoint() *ethpb.Checkpoint {
	return s.FinalizedCheckpoint() // Simplified for now
}

func (s *Service) IsCanonical(ctx context.Context, blockRoot [32]byte) (bool, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	_, exists := s.blocks[blockRoot]
	return exists, nil
}

func (s *Service) IsOptimistic(blockRoot [32]byte) (bool, error) {
	return false, nil // HotStuff doesn't use optimistic sync
}

func (s *Service) InForkchoice(blockRoot [32]byte) bool {
	s.mu.RLock()
	defer s.mu.RUnlock()

	_, exists := s.blocks[blockRoot]
	return exists
}

func (s *Service) CurrentSlot() primitives.Slot {
	return s.HeadSlot()
}

func (s *Service) GenesisTime() uint64 {
	return uint64(s.genesisTime.Unix())
}
