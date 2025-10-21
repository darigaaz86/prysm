package hotstuff

import (
	"fmt"

	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
)

// handleBlock handles a received block based on the current phase.
func (s *Service) handleBlock(block *HotStuffBlock) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	log.WithFields(logrus.Fields{
		"view":  block.View,
		"phase": s.currentPhase,
	}).Info("🔥 HotStuff: Processing block")

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

	// Process based on current phase (2-phase model)
	switch s.currentPhase {
	case PhasePropose:
		return s.handlePropose(block, blockHash)
	case PhaseCommit:
		return s.handleCommit(block, blockHash)
	default:
		return fmt.Errorf("unknown phase: %s", s.currentPhase)
	}
}

// handlePropose handles the PROPOSE phase.
// This phase combines the original PREPARE and PRE-COMMIT phases for faster consensus.
func (s *Service) handlePropose(block *HotStuffBlock, blockHash [32]byte) error {
	log.WithFields(logrus.Fields{
		"view":      block.View,
		"blockHash": fmt.Sprintf("%#x", blockHash[:8]),
	}).Info("🔥 HotStuff: Handling PROPOSE phase")

	// Check if we should vote for this block (combined PREPARE + PRE-COMMIT safety rules)
	if !s.shouldVotePropose(block) {
		log.WithField("view", block.View).Debug("🔥 HotStuff: Not voting for block in PROPOSE phase")
		return nil
	}

	// Create PROPOSE vote
	vote, err := s.createVote(block, blockHash, PhasePropose)
	if err != nil {
		return errors.Wrap(err, "failed to create PROPOSE vote")
	}

	log.WithField("view", block.View).Debug("🔥 HotStuff: Created PROPOSE vote")

	// If we're the leader, collect votes
	if s.isLeader() {
		return s.collectVote(vote)
	}

	// Otherwise, send vote to leader
	return s.sendVoteToLeader(vote)
}

// handleCommit handles the COMMIT phase.
// This phase combines the original COMMIT and DECIDE phases - it collects votes,
// builds the CommitQC, executes the block, and advances to the next view.
func (s *Service) handleCommit(block *HotStuffBlock, blockHash [32]byte) error {
	log.WithFields(logrus.Fields{
		"view":      block.View,
		"blockHash": fmt.Sprintf("%#x", blockHash[:8]),
	}).Info("🔥 HotStuff: Handling COMMIT phase")

	// Verify we have PROPOSE QC
	node := s.blocks[blockHash]
	if node.ProposeQC == nil {
		return errors.New("missing PROPOSE QC for COMMIT phase")
	}

	// Update locked QC (prevents rollback to earlier blocks)
	s.lockedQC = node.ProposeQC
	log.WithFields(logrus.Fields{
		"view":         block.View,
		"lockedQCView": node.ProposeQC.View,
	}).Debug("🔥 HotStuff: Updated locked QC")

	// Check if we should vote
	if !s.shouldVoteCommit(block, node) {
		log.WithField("view", block.View).Debug("🔥 HotStuff: Not voting for block in COMMIT phase")
		return nil
	}

	// Create COMMIT vote
	vote, err := s.createVote(block, blockHash, PhaseCommit)
	if err != nil {
		return errors.Wrap(err, "failed to create COMMIT vote")
	}

	log.WithField("view", block.View).Debug("🔥 HotStuff: Created COMMIT vote")

	// If we're the leader, collect votes
	if s.isLeader() {
		return s.collectVote(vote)
	}

	// Otherwise, send vote to leader
	return s.sendVoteToLeader(vote)
}

// shouldVotePropose determines if we should vote for a block in the PROPOSE phase.
// This combines the safety rules from the original PREPARE and PRE-COMMIT phases.
func (s *Service) shouldVotePropose(block *HotStuffBlock) bool {
	// Safety rule 1: Block must have a JustifyQC
	if block.JustifyQC == nil {
		log.WithField("view", block.View).Debug("🔥 HotStuff: Block has no JustifyQC, not voting")
		return false
	}

	// Safety rule 2: Block extends from highest QC we know (PREPARE rule)
	if CompareQC(block.JustifyQC, s.highestQC) < 0 {
		log.WithFields(logrus.Fields{
			"view":          block.View,
			"justifyQCView": block.JustifyQC.View,
			"highestQCView": s.highestQC.View,
		}).Debug("🔥 HotStuff: Block does not extend from highest QC, not voting")
		return false
	}

	// Safety rule 3: Block extends from locked QC or has higher QC (PRE-COMMIT rule)
	if s.lockedQC != nil {
		if CompareQC(block.JustifyQC, s.lockedQC) < 0 {
			log.WithFields(logrus.Fields{
				"view":          block.View,
				"justifyQCView": block.JustifyQC.View,
				"lockedQCView":  s.lockedQC.View,
			}).Debug("🔥 HotStuff: Block does not extend from locked QC, not voting")
			return false
		}
	}

	log.WithField("view", block.View).Debug("🔥 HotStuff: Safety rules passed, will vote in PROPOSE phase")
	return true
}

// shouldVoteCommit determines if we should vote for a block in the COMMIT phase.
// This phase requires that the block has a valid ProposeQC.
func (s *Service) shouldVoteCommit(block *HotStuffBlock, node *BlockNode) bool {
	// Must have ProposeQC
	if node.ProposeQC == nil {
		log.WithField("view", block.View).Debug("🔥 HotStuff: Block has no ProposeQC, not voting in COMMIT")
		return false
	}

	// Verify ProposeQC is valid
	if err := VerifyQC(node.ProposeQC, s.publicKeys, s.quorumSize); err != nil {
		log.WithFields(logrus.Fields{
			"view":  block.View,
			"error": err.Error(),
		}).Warn("🔥 HotStuff: ProposeQC verification failed, not voting in COMMIT")
		return false
	}

	log.WithField("view", block.View).Debug("🔥 HotStuff: ProposeQC valid, will vote in COMMIT phase")
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
		log.WithFields(logrus.Fields{
			"view":  vote.View,
			"phase": vote.Phase,
		}).Debug("🔥 HotStuff: Created new QC builder")
	}

	// Add vote to builder
	added, err := builder.AddVote(vote)
	if err != nil {
		return errors.Wrap(err, "failed to add vote to builder")
	}

	if !added {
		// Duplicate vote, ignore
		log.WithFields(logrus.Fields{
			"view":           vote.View,
			"phase":          vote.Phase,
			"validatorIndex": vote.ValidatorIndex,
		}).Debug("🔥 HotStuff: Duplicate vote ignored")
		return nil
	}

	log.WithFields(logrus.Fields{
		"view":       vote.View,
		"phase":      vote.Phase,
		"voteCount":  builder.VoteCount(),
		"quorumSize": s.quorumSize,
	}).Debug("🔥 HotStuff: Collected vote")

	// Check if we have quorum
	if builder.HasQuorum() {
		// Clean up old builders (keep only last 2 views)
		s.cleanupOldBuilders(vote.View)
		return s.onQuorumReached(vote.View, vote.Phase, vote.BlockHash, builder)
	}

	return nil
}

// cleanupOldBuilders removes QC builders for old views to prevent memory leaks.
func (s *Service) cleanupOldBuilders(currentView uint64) {
	for view := range s.qcBuilders {
		// Keep builders for current view and previous view only
		if view < currentView-1 {
			delete(s.qcBuilders, view)
			log.WithField("view", view).Debug("🔥 HotStuff: Cleaned up old QC builders")
		}
	}
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
	case PhasePropose:
		// PROPOSE quorum reached - build ProposeQC and advance to COMMIT
		node.ProposeQC = qc
		node.Status = StatusProposed
		log.WithFields(logrus.Fields{
			"view":      view,
			"blockHash": fmt.Sprintf("%#x", blockHash[:8]),
		}).Info("🔥 HotStuff: ProposeQC built, advancing to COMMIT phase")
		return s.advancePhase(PhaseCommit, qc)

	case PhaseCommit:
		// COMMIT quorum reached - build CommitQC, execute block, advance to next view
		node.CommitQC = qc
		node.Status = StatusCommitted
		log.WithFields(logrus.Fields{
			"view":      view,
			"blockHash": fmt.Sprintf("%#x", blockHash[:8]),
		}).Info("🔥 HotStuff: CommitQC built, executing block")

		// Execute the block
		if err := s.executeBlock(node.Block, blockHash); err != nil {
			return errors.Wrap(err, "failed to execute block")
		}

		// Update status to executed
		node.Status = StatusExecuted
		log.WithField("view", view).Info("🔥 HotStuff: Block executed, advancing to next view")

		// Advance to next view
		return s.advanceView()

	default:
		return fmt.Errorf("unexpected phase for quorum: %s", phase)
	}
}

// advancePhase advances to the next phase.
// In the 2-phase model, this only handles PhasePropose → PhaseCommit transition.
func (s *Service) advancePhase(nextPhase Phase, qc *QuorumCertificate) error {
	log.WithFields(logrus.Fields{
		"from": s.currentPhase,
		"to":   nextPhase,
		"view": s.currentView,
	}).Info("🔥 HotStuff: Advancing phase")

	// Validate phase transition (only PROPOSE → COMMIT is valid)
	if s.currentPhase == PhasePropose && nextPhase != PhaseCommit {
		return fmt.Errorf("invalid phase transition from %s to %s", s.currentPhase, nextPhase)
	}

	s.currentPhase = nextPhase

	// Update highest QC
	if CompareQC(qc, s.highestQC) > 0 {
		s.highestQC = qc
		log.WithFields(logrus.Fields{
			"view":   s.currentView,
			"qcView": qc.View,
		}).Debug("🔥 HotStuff: Updated highest QC")
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
// This is called after a block is executed in the COMMIT phase.
func (s *Service) advanceView() error {
	s.currentView++
	s.currentPhase = PhasePropose

	log.WithFields(logrus.Fields{
		"view":  s.currentView,
		"phase": s.currentPhase,
	}).Info("🔥 HotStuff: Advanced to next view")

	// Reset view timer
	s.resetViewTimer()

	// If we're the new leader, propose a block
	if s.isLeader() {
		log.WithField("view", s.currentView).Info("🔥 HotStuff: I am the leader, proposing block")
		return s.proposeBlock()
	}

	return nil
}

// proposeBlock proposes a new block (called by leader).
func (s *Service) proposeBlock() error {
	log.WithFields(logrus.Fields{
		"view":      s.currentView,
		"phase":     s.currentPhase,
		"highestQC": s.highestQC.View,
	}).Info("🔥 HotStuff: Proposing block as leader")

	// Create new block
	block := &HotStuffBlock{
		View:      s.currentView,
		JustifyQC: s.highestQC,
		// BeaconBlock will be created when we integrate with execution layer
	}

	log.WithField("view", s.currentView).Info("🔥 HotStuff: Block created, ready for execution layer integration")

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
