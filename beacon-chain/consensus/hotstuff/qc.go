package hotstuff

import (
	"fmt"

	"github.com/OffchainLabs/prysm/v6/consensus-types/primitives"
	"github.com/OffchainLabs/prysm/v6/crypto/bls"
	"github.com/pkg/errors"
)

// QCBuilder helps build Quorum Certificates from votes.
type QCBuilder struct {
	view       uint64
	phase      Phase
	blockHash  [32]byte
	votes      map[primitives.ValidatorIndex]*Vote
	quorumSize uint64
	totalVals  uint64
}

// NewQCBuilder creates a new QC builder.
func NewQCBuilder(view uint64, phase Phase, blockHash [32]byte, quorumSize, totalVals uint64) *QCBuilder {
	return &QCBuilder{
		view:       view,
		phase:      phase,
		blockHash:  blockHash,
		votes:      make(map[primitives.ValidatorIndex]*Vote),
		quorumSize: quorumSize,
		totalVals:  totalVals,
	}
}

// AddVote adds a vote to the QC builder.
// Returns true if the vote was added, false if it was a duplicate.
func (b *QCBuilder) AddVote(vote *Vote) (bool, error) {
	// Validate vote
	if vote.View != b.view {
		return false, fmt.Errorf("vote view %d does not match builder view %d", vote.View, b.view)
	}
	if vote.Phase != b.phase {
		return false, fmt.Errorf("vote phase %s does not match builder phase %s", vote.Phase, b.phase)
	}
	if vote.BlockHash != b.blockHash {
		return false, fmt.Errorf("vote block hash does not match builder block hash")
	}

	// Check if already voted
	if _, exists := b.votes[vote.ValidatorIndex]; exists {
		return false, nil // Duplicate vote
	}

	// Add vote
	b.votes[vote.ValidatorIndex] = vote
	return true, nil
}

// HasQuorum returns true if we have enough votes for a quorum.
func (b *QCBuilder) HasQuorum() bool {
	return uint64(len(b.votes)) >= b.quorumSize
}

// VoteCount returns the current number of votes.
func (b *QCBuilder) VoteCount() uint64 {
	return uint64(len(b.votes))
}

// Build builds a QC from the collected votes.
// Returns an error if there aren't enough votes for a quorum.
func (b *QCBuilder) Build() (*QuorumCertificate, error) {
	if !b.HasQuorum() {
		return nil, fmt.Errorf("not enough votes: have %d, need %d", len(b.votes), b.quorumSize)
	}

	// Collect signatures and signer indices
	signatures := make([]bls.Signature, 0, len(b.votes))
	signerIndices := make([]primitives.ValidatorIndex, 0, len(b.votes))

	for idx, vote := range b.votes {
		signatures = append(signatures, vote.Signature)
		signerIndices = append(signerIndices, idx)
	}

	// Aggregate signatures
	aggregatedSig := bls.AggregateSignatures(signatures)

	// Create signer bitmap
	signers := createSignerBitmap(signerIndices, b.totalVals)

	return &QuorumCertificate{
		View:          b.view,
		Phase:         b.phase,
		BlockHash:     b.blockHash,
		Signature:     aggregatedSig,
		Signers:       signers,
		SignerIndices: signerIndices,
	}, nil
}

// VerifyQC verifies a Quorum Certificate.
func VerifyQC(qc *QuorumCertificate, publicKeys []bls.PublicKey, quorumSize uint64) error {
	if qc == nil {
		return errors.New("QC is nil")
	}

	// Check quorum size
	if uint64(len(qc.SignerIndices)) < quorumSize {
		return fmt.Errorf("not enough signers: have %d, need %d", len(qc.SignerIndices), quorumSize)
	}

	// Collect public keys of signers as byte slices
	signerPubKeys := make([][]byte, 0, len(qc.SignerIndices))
	for _, idx := range qc.SignerIndices {
		if uint64(idx) >= uint64(len(publicKeys)) {
			return fmt.Errorf("signer index %d out of range", idx)
		}
		signerPubKeys = append(signerPubKeys, publicKeys[idx].Marshal())
	}

	// Aggregate public keys
	aggregatedPubKey, err := bls.AggregatePublicKeys(signerPubKeys)
	if err != nil {
		return errors.Wrap(err, "failed to aggregate public keys")
	}

	// Verify signature
	// TODO: Use proper signing root based on view, phase, and block hash
	signingRoot := qc.BlockHash
	if !qc.Signature.Verify(aggregatedPubKey, signingRoot[:]) {
		return errors.New("QC signature verification failed")
	}

	return nil
}

// createSignerBitmap creates a bitmap of signers.
func createSignerBitmap(signerIndices []primitives.ValidatorIndex, totalVals uint64) []byte {
	// Calculate number of bytes needed
	numBytes := (totalVals + 7) / 8
	bitmap := make([]byte, numBytes)

	// Set bits for each signer
	for _, idx := range signerIndices {
		byteIdx := uint64(idx) / 8
		bitIdx := uint64(idx) % 8
		if byteIdx < numBytes {
			bitmap[byteIdx] |= 1 << bitIdx
		}
	}

	return bitmap
}

// GetSignerIndices extracts signer indices from a bitmap.
func GetSignerIndices(bitmap []byte, totalVals uint64) []primitives.ValidatorIndex {
	indices := make([]primitives.ValidatorIndex, 0)

	for i := uint64(0); i < totalVals; i++ {
		byteIdx := i / 8
		bitIdx := i % 8

		if byteIdx < uint64(len(bitmap)) {
			if (bitmap[byteIdx] & (1 << bitIdx)) != 0 {
				indices = append(indices, primitives.ValidatorIndex(i))
			}
		}
	}

	return indices
}

// CompareQC compares two QCs and returns the higher one.
// Returns 1 if qc1 is higher, -1 if qc2 is higher, 0 if equal.
func CompareQC(qc1, qc2 *QuorumCertificate) int {
	if qc1 == nil && qc2 == nil {
		return 0
	}
	if qc1 == nil {
		return -1
	}
	if qc2 == nil {
		return 1
	}

	// Compare by view first
	if qc1.View > qc2.View {
		return 1
	}
	if qc1.View < qc2.View {
		return -1
	}

	// If same view, compare by phase
	if qc1.Phase > qc2.Phase {
		return 1
	}
	if qc1.Phase < qc2.Phase {
		return -1
	}

	return 0
}

// HighestQC returns the highest QC from a list.
func HighestQC(qcs []*QuorumCertificate) *QuorumCertificate {
	if len(qcs) == 0 {
		return nil
	}

	highest := qcs[0]
	for _, qc := range qcs[1:] {
		if CompareQC(qc, highest) > 0 {
			highest = qc
		}
	}

	return highest
}
