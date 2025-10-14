package hotstuff

import (
	"fmt"

	"github.com/pkg/errors"
)

// handleBlock handles a received block based on the current phase.
func (s *Service) handleBlock(block *HotStuffBlock) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	log.WithFields(map[string]interface{}{
		"view":  block.View,
		"phase": s.currentPhase,
	}).Debug("Handling block")

	// Verify block
	if err := s.verifyBlock(block); err != nil {
		return errors.Wrap(err, "block verification failed")
	}

	// Store block
	blockHash, err := block.Hash()
	if err != nil {
		return errors.Wrap(err, "failed to get block hash")
	}

	node := &BlockNode{
		Block:  block,
		Status: StatusProposed,
	}
	s.blocks[blockHash] = node
	s.blocksByView[block.View] = node

	// Process based on current phase
	switch s.currentPhase {
	case PhasePrepare:
		return s.handlePrepare(block, blockHash)
	case PhasePreCommit:
		return s.handlePreCommit(block, blockHash)
	case PhaseCommit:
		return s.handleCommit(block, blockHash)
	case PhaseDecide:
		return s.handleDecide(block, blockHash)
	default:
		return fmt.Errorf("unknown phase: %s", s.currentPhase)
	}
}

// handlePrepare handles the PREPARE phase.
func (s *Service) handlePrepare(block *HotStuffBlock, blockHash [32]byte) error {
	log.WithField("view", block.View).Debug("Handling PREPARE phase")

	// Check if we should vote for this block
	if !s.shouldVote(block) {
		log.Debug("Not voting for block in PREPARE phase")
		return nil
	}

	// Create PREPARE vote
	vote, err := s.createVote(block, blockHash, PhasePrepare)
	if err != nil {
		return errors.Wrap(err, "failed to create PREPARE vote")
	}

	// If we're the leader, collect votes
	if s.isLeader() {
		return s.collectVote(vote)
	}

	// Otherwise, send vote to leader
	return s.sendVoteToLeader(vote)
}

// handlePreCommit handles the PRE-COMMIT phase.
func (s *Service) handlePreCommit(block *HotStuffBlock, blockHash [32]byte) error {
	log.WithField("view", block.View).Debug("Handling PRE-COMMIT phase")

	// Verify we have PREPARE QC
	node := s.blocks[blockHash]
	if node.PrepareQC == nil {
		return errors.New("missing PREPARE QC for PRE-COMMIT phase")
	}

	// Check if we should vote
	if !s.shouldVote(block) {
		log.Debug("Not voting for block in PRE-COMMIT phase")
		return nil
	}

	// Create PRE-COMMIT vote
	vote, err := s.createVote(block, blockHash, PhasePreCommit)
	if err != nil {
		return errors.Wrap(err, "failed to create PRE-COMMIT vote")
	}

	// If we're the leader, collect votes
	if s.isLeader() {
		return s.collectVote(vote)
	}

	// Otherwise, send vote to leader
	return s.sendVoteToLeader(vote)
}

// handleCommit handles the COMMIT phase.
func (s *Service) handleCommit(block *HotStuffBlock, blockHash [32]byte) error {
	log.WithField("view", block.View).Debug("Handling COMMIT phase")

	// Verify we have PRE-COMMIT QC
	node := s.blocks[blockHash]
	if node.PreCommitQC == nil {
		return errors.New("missing PRE-COMMIT QC for COMMIT phase")
	}

	// Update locked QC
	s.lockedQC = node.PreCommitQC

	// Check if we should vote
	if !s.shouldVote(block) {
		log.Debug("Not voting for block in COMMIT phase")
		return nil
	}

	// Create COMMIT vote
	vote, err := s.createVote(block, blockHash, PhaseCommit)
	if err != nil {
		return errors.Wrap(err, "failed to create COMMIT vote")
	}

	// If we're the leader, collect votes
	if s.isLeader() {
		return s.collectVote(vote)
	}

	// Otherwise, send vote to leader
	return s.sendVoteToLeader(vote)
}

// handleDecide handles the DECIDE phase.
func (s *Service) handleDecide(block *HotStuffBlock, blockHash [32]byte) error {
	log.WithField("view", block.View).Info("Handling DECIDE phase - executing block")

	// Verify we have COMMIT QC
	node := s.blocks[blockHash]
	if node.CommitQC == nil {
		return errors.New("missing COMMIT QC for DECIDE phase")
	}

	// Execute block
	if err := s.executeBlock(block, blockHash); err != nil {
		return errors.Wrap(err, "failed to execute block")
	}

	// Update status
	node.Status = StatusDecided

	// Advance to next view
	return s.advanceView()
}

// shouldVote determines if we should vote for a block.
func (s *Service) shouldVote(block *HotStuffBlock) bool {
	// Safety rule 1: Block extends from highest QC we know
	if block.JustifyQC == nil {
		return false
	}

	if CompareQC(block.JustifyQC, s.highestQC) < 0 {
		log.Debug("Block does not extend from highest QC")
		return false
	}

	// Safety rule 2: Block extends from locked QC or has higher QC
	if s.lockedQC != nil {
		if CompareQC(block.JustifyQC, s.lockedQC) < 0 {
			log.Debug("Block does not extend from locked QC")
			return false
		}
	}

	// TODO: Add more safety rules (e.g., valid state transition)

	return true
}

// createVote creates a vote for a block in a specific phase.
func (s *Service) createVote(block *HotStuffBlock, blockHash [32]byte, phase Phase) (*Vote, error) {
	vote := &Vote{
		View:           block.View,
		Phase:          phase,
		BlockHash:      blockHash,
		ValidatorIndex: s.validatorIdx,
	}

	// Sign the vote
	if s.privateKey != nil {
		signingRoot := vote.SigningRoot()
		signature := s.privateKey.Sign(signingRoot[:])
		vote.Signature = signature
	}

	return vote, nil
}

// collectVote collects a vote (called by leader).
func (s *Service) collectVote(vote *Vote) error {
	// Get or create QC builder for this view and phase
	if s.qcBuilders[vote.View] == nil {
		s.qcBuilders[vote.View] = make(map[Phase]*QCBuilder)
	}

	builder := s.qcBuilders[vote.View][vote.Phase]
	if builder == nil {
		builder = NewQCBuilder(vote.View, vote.Phase, vote.BlockHash, s.quorumSize, uint64(len(s.validators)))
		s.qcBuilders[vote.View][vote.Phase] = builder
	}

	// Add vote to builder
	added, err := builder.AddVote(vote)
	if err != nil {
		return errors.Wrap(err, "failed to add vote to builder")
	}

	if !added {
		// Duplicate vote, ignore
		return nil
	}

	log.WithFields(map[string]interface{}{
		"view":       vote.View,
		"phase":      vote.Phase,
		"voteCount":  builder.VoteCount(),
		"quorumSize": s.quorumSize,
	}).Debug("Collected vote")

	// Check if we have quorum
	if builder.HasQuorum() {
		return s.onQuorumReached(vote.View, vote.Phase, vote.BlockHash, builder)
	}

	return nil
}

// onQuorumReached is called when a quorum is reached for a phase.
func (s *Service) onQuorumReached(view uint64, phase Phase, blockHash [32]byte, builder *QCBuilder) error {
	log.WithFields(map[string]interface{}{
		"view":  view,
		"phase": phase,
	}).Info("Quorum reached")

	// Build QC
	qc, err := builder.Build()
	if err != nil {
		return errors.Wrap(err, "failed to build QC")
	}

	// Update block node with QC
	node := s.blocks[blockHash]
	if node == nil {
		return errors.New("block not found")
	}

	switch phase {
	case PhasePrepare:
		node.PrepareQC = qc
		node.Status = StatusPrepared
		return s.advancePhase(PhasePreCommit, qc)

	case PhasePreCommit:
		node.PreCommitQC = qc
		node.Status = StatusPreCommitted
		return s.advancePhase(PhaseCommit, qc)

	case PhaseCommit:
		node.CommitQC = qc
		node.Status = StatusCommitted
		return s.advancePhase(PhaseDecide, qc)

	default:
		return fmt.Errorf("unexpected phase for quorum: %s", phase)
	}
}

// advancePhase advances to the next phase.
func (s *Service) advancePhase(nextPhase Phase, qc *QuorumCertificate) error {
	log.WithFields(map[string]interface{}{
		"from": s.currentPhase,
		"to":   nextPhase,
	}).Info("Advancing phase")

	s.currentPhase = nextPhase

	// Update highest QC
	if CompareQC(qc, s.highestQC) > 0 {
		s.highestQC = qc
	}

	// Reset view timer
	s.resetViewTimer()

	// If we're the leader, broadcast next phase message
	if s.isLeader() {
		return s.broadcastPhaseMessage(nextPhase, qc)
	}

	return nil
}

// advanceView advances to the next view.
func (s *Service) advanceView() error {
	s.currentView++
	s.currentPhase = PhasePrepare

	log.WithField("view", s.currentView).Info("Advanced to next view")

	// Reset view timer
	s.resetViewTimer()

	// If we're the new leader, propose a block
	if s.isLeader() {
		return s.proposeBlock()
	}

	return nil
}

// proposeBlock proposes a new block (called by leader).
func (s *Service) proposeBlock() error {
	log.WithField("view", s.currentView).Info("Proposing block as leader")

	// Create new block
	block := &HotStuffBlock{
		View:      s.currentView,
		JustifyQC: s.highestQC,
		// BeaconBlock will be created when we integrate with execution layer
	}

	// Broadcast block
	return s.broadcastBlock(block)
}

// broadcastBlock broadcasts a block to all validators.
func (s *Service) broadcastBlock(block *HotStuffBlock) error {
	// TODO: Implement actual network broadcast
	// For now, just add to our own channel
	select {
	case s.blockChan <- block:
		return nil
	default:
		return errors.New("block channel full")
	}
}

// broadcastPhaseMessage broadcasts a phase message to all validators.
func (s *Service) broadcastPhaseMessage(phase Phase, qc *QuorumCertificate) error {
	log.WithFields(map[string]interface{}{
		"phase": phase,
		"view":  s.currentView,
	}).Debug("Broadcasting phase message")

	// TODO: Implement actual network broadcast
	// For now, this is a placeholder

	return nil
}

// sendVoteToLeader sends a vote to the leader.
func (s *Service) sendVoteToLeader(vote *Vote) error {
	// TODO: Implement actual network send
	// For now, just add to our own channel (for testing)
	select {
	case s.voteChan <- vote:
		return nil
	default:
		return errors.New("vote channel full")
	}
}

// executeBlock executes a decided block.
func (s *Service) executeBlock(block *HotStuffBlock, blockHash [32]byte) error {
	log.WithField("view", block.View).Info("Executing block")

	// TODO: Integrate with execution layer
	// For now, just mark as executed

	return nil
}

// verifyBlock verifies a block.
func (s *Service) verifyBlock(block *HotStuffBlock) error {
	// Verify block is for current or future view
	if block.View < s.currentView {
		return fmt.Errorf("block view %d is less than current view %d", block.View, s.currentView)
	}

	// Verify JustifyQC
	if block.JustifyQC != nil {
		if err := VerifyQC(block.JustifyQC, s.publicKeys, s.quorumSize); err != nil {
			return errors.Wrap(err, "invalid JustifyQC")
		}
	}

	// TODO: Add more verification (e.g., leader signature, state transition)

	return nil
}
