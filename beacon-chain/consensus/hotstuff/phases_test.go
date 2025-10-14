package hotstuff

import (
	"context"
	"testing"
	"time"

	"github.com/OffchainLabs/prysm/v6/beacon-chain/consensus"
	"github.com/OffchainLabs/prysm/v6/consensus-types/primitives"
	"github.com/OffchainLabs/prysm/v6/testing/assert"
	"github.com/OffchainLabs/prysm/v6/testing/require"
)

func TestService_Start_Stop(t *testing.T) {
	cfg := &consensus.HotStuffConfig{
		ViewTimeout:      10 * time.Second,
		BlockTime:        6 * time.Second,
		MinValidators:    4,
		QuorumThreshold:  0.67,
		LeaderRotation:   "round-robin",
		EnableViewChange: true,
	}

	ctx := context.Background()
	service, err := NewService(ctx, cfg)
	require.NoError(t, err)

	// Start service
	err = service.Start()
	require.NoError(t, err)
	assert.Equal(t, true, service.started)

	// Stop service
	err = service.Stop()
	require.NoError(t, err)
	assert.Equal(t, false, service.started)
}

func TestService_Status(t *testing.T) {
	cfg := &consensus.HotStuffConfig{
		ViewTimeout:      10 * time.Second,
		BlockTime:        6 * time.Second,
		MinValidators:    4,
		QuorumThreshold:  0.67,
		LeaderRotation:   "round-robin",
		EnableViewChange: true,
	}

	ctx := context.Background()
	service, err := NewService(ctx, cfg)
	require.NoError(t, err)

	status := service.Status()
	assert.Equal(t, consensus.ModeHotStuff, status.Mode)
	assert.Equal(t, true, status.Synced)
}

func TestService_InitializeGenesis(t *testing.T) {
	cfg := &consensus.HotStuffConfig{
		ViewTimeout:      10 * time.Second,
		BlockTime:        6 * time.Second,
		MinValidators:    4,
		QuorumThreshold:  0.67,
		LeaderRotation:   "round-robin",
		EnableViewChange: true,
	}

	ctx := context.Background()
	service, err := NewService(ctx, cfg)
	require.NoError(t, err)

	err = service.initializeGenesis()
	require.NoError(t, err)

	// Check genesis block exists
	assert.NotNil(t, service.genesisBlock)
	assert.Equal(t, StatusDecided, service.genesisBlock.Status)

	// Check genesis QC
	assert.NotNil(t, service.highestQC)
	assert.Equal(t, uint64(0), service.highestQC.View)
	assert.Equal(t, PhaseCommit, service.highestQC.Phase)
}

func TestService_ShouldVote(t *testing.T) {
	cfg := &consensus.HotStuffConfig{
		ViewTimeout:      10 * time.Second,
		BlockTime:        6 * time.Second,
		MinValidators:    4,
		QuorumThreshold:  0.67,
		LeaderRotation:   "round-robin",
		EnableViewChange: true,
	}

	ctx := context.Background()
	service, err := NewService(ctx, cfg)
	require.NoError(t, err)

	err = service.initializeGenesis()
	require.NoError(t, err)

	tests := []struct {
		name       string
		block      *HotStuffBlock
		shouldVote bool
	}{
		{
			name: "block with no JustifyQC",
			block: &HotStuffBlock{
				View:      1,
				JustifyQC: nil,
			},
			shouldVote: false,
		},
		{
			name: "block with valid JustifyQC",
			block: &HotStuffBlock{
				View:      1,
				JustifyQC: service.highestQC,
			},
			shouldVote: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := service.shouldVote(tt.block)
			assert.Equal(t, tt.shouldVote, result)
		})
	}
}

func TestService_CreateVote(t *testing.T) {
	cfg := &consensus.HotStuffConfig{
		ViewTimeout:      10 * time.Second,
		BlockTime:        6 * time.Second,
		MinValidators:    4,
		QuorumThreshold:  0.67,
		LeaderRotation:   "round-robin",
		EnableViewChange: true,
	}

	ctx := context.Background()
	service, err := NewService(ctx, cfg)
	require.NoError(t, err)

	service.validatorIdx = 5

	block := &HotStuffBlock{
		View: 1,
	}
	blockHash := [32]byte{1, 2, 3}

	vote, err := service.createVote(block, blockHash, PhasePrepare)
	require.NoError(t, err)

	assert.Equal(t, uint64(1), vote.View)
	assert.Equal(t, PhasePrepare, vote.Phase)
	assert.Equal(t, blockHash, vote.BlockHash)
	assert.Equal(t, primitives.ValidatorIndex(5), vote.ValidatorIndex)
}

func TestService_AdvancePhase(t *testing.T) {
	cfg := &consensus.HotStuffConfig{
		ViewTimeout:      10 * time.Second,
		BlockTime:        6 * time.Second,
		MinValidators:    4,
		QuorumThreshold:  0.67,
		LeaderRotation:   "round-robin",
		EnableViewChange: true,
	}

	ctx := context.Background()
	service, err := NewService(ctx, cfg)
	require.NoError(t, err)

	err = service.initializeGenesis()
	require.NoError(t, err)

	// Start at PREPARE
	assert.Equal(t, PhasePrepare, service.currentPhase)

	qc := &QuorumCertificate{
		View:  1,
		Phase: PhasePrepare,
	}

	// Advance to PRE-COMMIT
	err = service.advancePhase(PhasePreCommit, qc)
	require.NoError(t, err)
	assert.Equal(t, PhasePreCommit, service.currentPhase)

	// Advance to COMMIT
	qc.Phase = PhasePreCommit
	err = service.advancePhase(PhaseCommit, qc)
	require.NoError(t, err)
	assert.Equal(t, PhaseCommit, service.currentPhase)

	// Advance to DECIDE
	qc.Phase = PhaseCommit
	err = service.advancePhase(PhaseDecide, qc)
	require.NoError(t, err)
	assert.Equal(t, PhaseDecide, service.currentPhase)
}

func TestService_AdvanceView(t *testing.T) {
	cfg := &consensus.HotStuffConfig{
		ViewTimeout:      10 * time.Second,
		BlockTime:        6 * time.Second,
		MinValidators:    4,
		QuorumThreshold:  0.67,
		LeaderRotation:   "round-robin",
		EnableViewChange: true,
	}

	ctx := context.Background()
	service, err := NewService(ctx, cfg)
	require.NoError(t, err)

	err = service.initializeGenesis()
	require.NoError(t, err)

	// Start at view 0
	assert.Equal(t, uint64(0), service.currentView)

	// Advance to view 1
	err = service.advanceView()
	require.NoError(t, err)
	assert.Equal(t, uint64(1), service.currentView)
	assert.Equal(t, PhasePrepare, service.currentPhase)

	// Advance to view 2
	err = service.advanceView()
	require.NoError(t, err)
	assert.Equal(t, uint64(2), service.currentView)
	assert.Equal(t, PhasePrepare, service.currentPhase)
}

func TestService_VerifyBlock(t *testing.T) {
	cfg := &consensus.HotStuffConfig{
		ViewTimeout:      10 * time.Second,
		BlockTime:        6 * time.Second,
		MinValidators:    4,
		QuorumThreshold:  0.67,
		LeaderRotation:   "round-robin",
		EnableViewChange: true,
	}

	ctx := context.Background()
	service, err := NewService(ctx, cfg)
	require.NoError(t, err)

	err = service.initializeGenesis()
	require.NoError(t, err)

	service.currentView = 5

	tests := []struct {
		name    string
		block   *HotStuffBlock
		wantErr bool
	}{
		{
			name: "block for past view",
			block: &HotStuffBlock{
				View: 3,
			},
			wantErr: true,
		},
		{
			name: "block for current view",
			block: &HotStuffBlock{
				View: 5,
			},
			wantErr: false,
		},
		{
			name: "block for future view",
			block: &HotStuffBlock{
				View: 7,
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := service.verifyBlock(tt.block)
			if tt.wantErr {
				assert.NotNil(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}
