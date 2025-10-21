// Package hotstuff implements the HotStuff BFT consensus algorithm.
// HotStuff is a leader-based Byzantine fault-tolerant consensus protocol
// with linear communication complexity and optimistic responsiveness.
package hotstuff

import (
	"fmt"

	"github.com/OffchainLabs/prysm/v6/consensus-types/primitives"
	"github.com/OffchainLabs/prysm/v6/crypto/bls"
	ethpb "github.com/OffchainLabs/prysm/v6/proto/prysm/v1alpha1"
)

// View represents a consensus round in HotStuff.
// Each view has a designated leader who proposes a block.
type View struct {
	// Number is the view number (monotonically increasing).
	Number uint64
	// Leader is the validator index of the leader for this view.
	Leader primitives.ValidatorIndex
}

// Phase represents the current phase in the 2-phase HotStuff protocol.
// The protocol has been optimized from 4 phases to 2 phases for faster consensus.
type Phase uint8

const (
	// PhasePropose is the first phase where the leader proposes a block and validators vote.
	// This phase combines the original PREPARE and PRE-COMMIT phases.
	PhasePropose Phase = iota
	// PhaseCommit is the second phase where validators commit and execute the block.
	// This phase combines the original COMMIT and DECIDE phases.
	PhaseCommit
)

// String returns the string representation of the phase.
func (p Phase) String() string {
	switch p {
	case PhasePropose:
		return "PROPOSE"
	case PhaseCommit:
		return "COMMIT"
	default:
		return fmt.Sprintf("UNKNOWN(%d)", p)
	}
}

// QuorumCertificate (QC) represents a collection of 2f+1 signatures on a block.
// A QC proves that a supermajority of validators agreed on a block.
type QuorumCertificate struct {
	// View is the view number when this QC was created.
	View uint64
	// Phase is the phase this QC represents (PROPOSE or COMMIT).
	Phase Phase
	// BlockHash is the hash of the block this QC certifies.
	BlockHash [32]byte
	// Signatures is the aggregated BLS signature from validators.
	Signature bls.Signature
	// Signers is the bitmap of validators who signed.
	Signers []byte
	// SignerIndices is the list of validator indices who signed.
	SignerIndices []primitives.ValidatorIndex
}

// IsValid checks if the QC has enough signatures for a quorum.
func (qc *QuorumCertificate) IsValid(totalValidators, quorumSize uint64) bool {
	if qc == nil {
		return false
	}
	return uint64(len(qc.SignerIndices)) >= quorumSize
}

// Vote represents a validator's vote in a specific phase.
type Vote struct {
	// View is the view number this vote is for.
	View uint64
	// Phase is the phase this vote is for.
	Phase Phase
	// BlockHash is the hash of the block being voted on.
	BlockHash [32]byte
	// ValidatorIndex is the index of the validator who created this vote.
	ValidatorIndex primitives.ValidatorIndex
	// Signature is the BLS signature of the vote.
	Signature bls.Signature
}

// SigningRoot returns the root that should be signed for this vote.
func (v *Vote) SigningRoot() [32]byte {
	// TODO: Implement proper SSZ signing root
	// For now, return a simple hash of the block hash
	return v.BlockHash
}

// HotStuffBlock extends a beacon block with HotStuff-specific metadata.
type HotStuffBlock struct {
	// BeaconBlock is the underlying beacon block.
	BeaconBlock *ethpb.BeaconBlock
	// View is the view number when this block was proposed.
	View uint64
	// ParentQC is the QC for the parent block (proves parent is valid).
	ParentQC *QuorumCertificate
	// JustifyQC is the highest QC known to the proposer.
	JustifyQC *QuorumCertificate
}

// Hash returns the hash of the block.
func (b *HotStuffBlock) Hash() ([32]byte, error) {
	if b.BeaconBlock == nil {
		return [32]byte{}, fmt.Errorf("beacon block is nil")
	}
	return b.BeaconBlock.HashTreeRoot()
}

// ViewChangeMsg represents a view change message sent when a view times out.
type ViewChangeMsg struct {
	// NewView is the view number being proposed.
	NewView uint64
	// HighestQC is the highest QC known to this validator.
	HighestQC *QuorumCertificate
	// ValidatorIndex is the index of the validator sending this message.
	ValidatorIndex primitives.ValidatorIndex
	// Signature is the BLS signature of this message.
	Signature bls.Signature
}

// SigningRoot returns the root that should be signed for this view change message.
func (v *ViewChangeMsg) SigningRoot() [32]byte {
	// TODO: Implement proper SSZ signing root
	// For now, return a hash based on the new view number
	var root [32]byte
	root[0] = byte(v.NewView)
	root[1] = byte(v.NewView >> 8)
	root[2] = byte(v.NewView >> 16)
	root[3] = byte(v.NewView >> 24)
	return root
}

// PrepareMsg represents a PREPARE message from the leader.
type PrepareMsg struct {
	// View is the view number.
	View uint64
	// Block is the proposed block.
	Block *HotStuffBlock
	// LeaderSignature is the leader's signature on the block.
	LeaderSignature bls.Signature
}

// PreCommitMsg represents a PRE-COMMIT message from the leader.
type PreCommitMsg struct {
	// View is the view number.
	View uint64
	// PrepareQC is the QC from the PREPARE phase.
	PrepareQC *QuorumCertificate
	// LeaderSignature is the leader's signature.
	LeaderSignature bls.Signature
}

// CommitMsg represents a COMMIT message from the leader.
type CommitMsg struct {
	// View is the view number.
	View uint64
	// PreCommitQC is the QC from the PRE-COMMIT phase.
	PreCommitQC *QuorumCertificate
	// LeaderSignature is the leader's signature.
	LeaderSignature bls.Signature
}

// DecideMsg represents a DECIDE message from the leader.
type DecideMsg struct {
	// View is the view number.
	View uint64
	// CommitQC is the QC from the COMMIT phase.
	CommitQC *QuorumCertificate
	// LeaderSignature is the leader's signature.
	LeaderSignature bls.Signature
}

// BlockStatus represents the status of a block in the HotStuff protocol.
type BlockStatus uint8

const (
	// StatusUnknown means the block status is unknown.
	StatusUnknown BlockStatus = iota
	// StatusProposed means the block has been proposed in the PROPOSE phase.
	StatusProposed
	// StatusCommitted means the block has a PROPOSE QC and is in the COMMIT phase.
	StatusCommitted
	// StatusExecuted means the block has a COMMIT QC and has been executed.
	StatusExecuted
)

// String returns the string representation of the block status.
func (s BlockStatus) String() string {
	switch s {
	case StatusUnknown:
		return "UNKNOWN"
	case StatusProposed:
		return "PROPOSED"
	case StatusCommitted:
		return "COMMITTED"
	case StatusExecuted:
		return "EXECUTED"
	default:
		return fmt.Sprintf("UNKNOWN(%d)", s)
	}
}

// BlockNode represents a node in the block tree.
type BlockNode struct {
	// Block is the HotStuff block.
	Block *HotStuffBlock
	// Status is the current status of this block.
	Status BlockStatus
	// ProposeQC is the PROPOSE QC for this block (if any).
	// This QC proves that a quorum voted for the block in the PROPOSE phase.
	ProposeQC *QuorumCertificate
	// CommitQC is the COMMIT QC for this block (if any).
	// This QC proves that a quorum committed to the block in the COMMIT phase.
	CommitQC *QuorumCertificate
	// Parent is the parent block node.
	Parent *BlockNode
	// Children are the child block nodes.
	Children []*BlockNode
}

// Hash returns the hash of the block.
func (n *BlockNode) Hash() ([32]byte, error) {
	if n.Block == nil {
		return [32]byte{}, fmt.Errorf("block is nil")
	}
	return n.Block.Hash()
}

// IsCommitted returns true if the block has been committed.
func (n *BlockNode) IsCommitted() bool {
	return n.Status >= StatusCommitted
}

// IsExecuted returns true if the block has been executed.
func (n *BlockNode) IsExecuted() bool {
	return n.Status == StatusExecuted
}
