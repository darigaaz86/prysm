package hotstuff

import (
	"github.com/pkg/errors"
)

// handleVote handles a received vote.
func (s *Service) handleVote(vote *Vote) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	log.WithFields(map[string]interface{}{
		"view":      vote.View,
		"phase":     vote.Phase,
		"validator": vote.ValidatorIndex,
	}).Debug("Handling vote")

	// Verify vote is for current view
	if vote.View != s.currentView {
		log.WithFields(map[string]interface{}{
			"voteView":    vote.View,
			"currentView": s.currentView,
		}).Debug("Vote for different view, ignoring")
		return nil
	}

	// Verify vote is for current phase
	if vote.Phase != s.currentPhase {
		log.WithFields(map[string]interface{}{
			"votePhase":    vote.Phase,
			"currentPhase": s.currentPhase,
		}).Debug("Vote for different phase, ignoring")
		return nil
	}

	// Verify vote signature
	if err := s.verifyVote(vote); err != nil {
		return errors.Wrap(err, "vote verification failed")
	}

	// Only leader collects votes
	if !s.isLeader() {
		log.Debug("Not leader, ignoring vote")
		return nil
	}

	// Collect vote
	return s.collectVote(vote)
}

// verifyVote verifies a vote's signature.
func (s *Service) verifyVote(vote *Vote) error {
	// Check validator index is valid
	if uint64(vote.ValidatorIndex) >= uint64(len(s.publicKeys)) {
		return errors.Errorf("invalid validator index: %d", vote.ValidatorIndex)
	}

	// Verify signature
	if vote.Signature != nil && len(s.publicKeys) > 0 {
		pubKey := s.publicKeys[vote.ValidatorIndex]
		signingRoot := vote.SigningRoot()
		if !vote.Signature.Verify(pubKey, signingRoot[:]) {
			return errors.New("invalid vote signature")
		}
	}

	return nil
}

// handleTimeout handles a view timeout.
func (s *Service) handleTimeout() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	log.WithField("view", s.currentView).Warn("View timeout")

	// If view change is not enabled, just reset timer
	if !s.cfg.EnableViewChange {
		log.Debug("View change disabled, resetting timer")
		s.resetViewTimer()
		return nil
	}

	// Trigger view change
	return s.triggerViewChange()
}

// triggerViewChange triggers a view change to the next view.
func (s *Service) triggerViewChange() error {
	log.WithFields(map[string]interface{}{
		"currentView": s.currentView,
		"nextView":    s.currentView + 1,
	}).Info("Triggering view change")

	// Create view change message
	msg := &ViewChangeMsg{
		NewView:        s.currentView + 1,
		HighestQC:      s.highestQC,
		ValidatorIndex: s.validatorIdx,
	}

	// Sign the message
	if s.privateKey != nil {
		signingRoot := msg.SigningRoot()
		signature := s.privateKey.Sign(signingRoot[:])
		msg.Signature = signature
	}

	// Broadcast view change message
	return s.broadcastViewChange(msg)
}

// broadcastViewChange broadcasts a view change message.
func (s *Service) broadcastViewChange(msg *ViewChangeMsg) error {
	// TODO: Implement actual network broadcast
	// For now, just add to our own channel
	select {
	case s.viewChangeCh <- msg:
		return nil
	default:
		return errors.New("view change channel full")
	}
}

// handleViewChange handles a view change message.
func (s *Service) handleViewChange(msg *ViewChangeMsg) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	log.WithFields(map[string]interface{}{
		"newView":     msg.NewView,
		"currentView": s.currentView,
		"validator":   msg.ValidatorIndex,
	}).Debug("Handling view change")

	// Verify view change message
	if err := s.verifyViewChange(msg); err != nil {
		return errors.Wrap(err, "view change verification failed")
	}

	// If message is for a future view, consider changing
	if msg.NewView > s.currentView {
		// TODO: Collect view change messages and change when we have quorum
		// For now, just change immediately
		return s.changeView(msg.NewView, msg.HighestQC)
	}

	return nil
}

// verifyViewChange verifies a view change message.
func (s *Service) verifyViewChange(msg *ViewChangeMsg) error {
	// Check validator index is valid
	if uint64(msg.ValidatorIndex) >= uint64(len(s.publicKeys)) {
		return errors.Errorf("invalid validator index: %d", msg.ValidatorIndex)
	}

	// Verify signature
	if msg.Signature != nil && len(s.publicKeys) > 0 {
		pubKey := s.publicKeys[msg.ValidatorIndex]
		signingRoot := msg.SigningRoot()
		if !msg.Signature.Verify(pubKey, signingRoot[:]) {
			return errors.New("invalid view change signature")
		}
	}

	// Verify highest QC
	if msg.HighestQC != nil {
		if err := VerifyQC(msg.HighestQC, s.publicKeys, s.quorumSize); err != nil {
			return errors.Wrap(err, "invalid highest QC in view change")
		}
	}

	return nil
}

// changeView changes to a new view.
func (s *Service) changeView(newView uint64, highestQC *QuorumCertificate) error {
	log.WithFields(map[string]interface{}{
		"from": s.currentView,
		"to":   newView,
	}).Info("Changing view")

	s.currentView = newView
	s.currentPhase = PhasePropose

	// Update highest QC if provided
	if highestQC != nil && CompareQC(highestQC, s.highestQC) > 0 {
		s.highestQC = highestQC
	}

	// Reset view timer
	s.resetViewTimer()

	// If we're the new leader, propose a block
	if s.isLeader() {
		return s.proposeBlock()
	}

	return nil
}
