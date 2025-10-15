package hotstuff

import (
	"testing"
)

// Note: Leader election tests are limited due to import cycle constraints with primitives package
// Full integration tests should be done at a higher level

func TestPhaseOrdering(t *testing.T) {
	// Test that phases have correct ordering
	phases := []Phase{PhasePrepare, PhasePreCommit, PhaseCommit, PhaseDecide}

	for i := 0; i < len(phases)-1; i++ {
		if phases[i] >= phases[i+1] {
			t.Errorf("Phase ordering incorrect: %v should be < %v", phases[i], phases[i+1])
		}
	}
}

func TestBlockStatusOrdering(t *testing.T) {
	// Test that block statuses have correct ordering
	statuses := []BlockStatus{StatusProposed, StatusPrepared, StatusPreCommitted, StatusCommitted, StatusDecided}

	for i := 0; i < len(statuses)-1; i++ {
		if statuses[i] >= statuses[i+1] {
			t.Errorf("BlockStatus ordering incorrect: %v should be < %v", statuses[i], statuses[i+1])
		}
	}
}
