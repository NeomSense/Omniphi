package types

import (
	"crypto/sha256"
	"encoding/binary"

	"cosmossdk.io/math"
)

// ============================================================================
// Constructors
// ============================================================================

// NewApp creates a new App instance
func NewApp(id uint64, name, owner, schemaCid string, challengePeriod int64, minVerifiers uint32, createdAt int64) App {
	return App{
		AppId:           id,
		Name:            name,
		Owner:           owner,
		SchemaCid:       schemaCid,
		ChallengePeriod: challengePeriod,
		MinVerifiers:    minVerifiers,
		Status:          AppStatusActive,
		CreatedAt:       createdAt,
	}
}

// NewVerifierMember creates a new VerifierMember instance
func NewVerifierMember(address string, weight math.Int, joinedAt int64) VerifierMember {
	return VerifierMember{
		Address:  address,
		Weight:   weight,
		JoinedAt: joinedAt,
	}
}

// NewVerifierSet creates a new VerifierSet instance
func NewVerifierSet(id, epoch uint64, members []VerifierMember, minAttestations uint32, quorumPct math.LegacyDec, appId uint64) VerifierSet {
	return VerifierSet{
		VerifierSetId:   id,
		Epoch:           epoch,
		Members:         members,
		MinAttestations: minAttestations,
		QuorumPct:       quorumPct,
		AppId:           appId,
	}
}

// NewBatchCommitment creates a new BatchCommitment instance
func NewBatchCommitment(id, epoch uint64, merkleRoot []byte, recordCount, appId, verifierSetId uint64, submitter string, challengeEndTime, submittedAt int64) BatchCommitment {
	return BatchCommitment{
		BatchId:          id,
		Epoch:            epoch,
		RecordMerkleRoot: merkleRoot,
		RecordCount:      recordCount,
		AppId:            appId,
		VerifierSetId:    verifierSetId,
		Submitter:        submitter,
		ChallengeEndTime: challengeEndTime,
		Status:           BatchStatusSubmitted,
		SubmittedAt:      submittedAt,
		FinalizedAt:      0,
	}
}

// NewBatchCommitmentWithOptions creates a BatchCommitment with optional DA/PoSeq fields
func NewBatchCommitmentWithOptions(id, epoch uint64, merkleRoot []byte, recordCount, appId, verifierSetId uint64, submitter string, challengeEndTime, submittedAt int64, daHash, poseqHash []byte) BatchCommitment {
	bc := NewBatchCommitment(id, epoch, merkleRoot, recordCount, appId, verifierSetId, submitter, challengeEndTime, submittedAt)
	bc.DACommitmentHash = daHash
	bc.PoSeqCommitmentHash = poseqHash
	return bc
}

// NewAttestation creates a new Attestation instance
func NewAttestation(batchId uint64, verifier string, signature []byte, confidenceWeight math.LegacyDec, timestamp int64) Attestation {
	return Attestation{
		BatchId:          batchId,
		VerifierAddress:  verifier,
		Signature:        signature,
		ConfidenceWeight: confidenceWeight,
		Timestamp:        timestamp,
	}
}

// NewChallenge creates a new Challenge instance
func NewChallenge(id, batchId uint64, challenger string, challengeType ChallengeType, proofData []byte, timestamp int64) Challenge {
	return Challenge{
		ChallengeId:   id,
		BatchId:       batchId,
		Challenger:    challenger,
		ChallengeType: challengeType,
		ProofData:     proofData,
		Status:        ChallengeStatusOpen,
		Timestamp:     timestamp,
		ResolvedAt:    0,
		ResolvedBy:    "",
		BondAmount:    math.ZeroInt(),
	}
}

// NewChallengeWithBond creates a Challenge with a bond amount (F4)
func NewChallengeWithBond(id, batchId uint64, challenger string, challengeType ChallengeType, proofData []byte, timestamp int64, bondAmount math.Int) Challenge {
	c := NewChallenge(id, batchId, challenger, challengeType, proofData, timestamp)
	c.BondAmount = bondAmount
	return c
}

// ComputeAttestationSignBytes computes the deterministic sign bytes for an attestation (F5).
// Returns SHA256(batchID || merkleRoot || epoch || verifierAddress) — a batch-binding
// commitment that proves the verifier intentionally attested to THIS specific batch.
//
// SECURITY: The verifier address is included to prevent third-party forgery. Without it,
// anyone who observes batch data on-chain could compute the hash and submit attestations
// on behalf of arbitrary verifier set members.
func ComputeAttestationSignBytes(batchID uint64, merkleRoot []byte, epoch uint64, verifierAddress string) []byte {
	h := sha256.New()
	batchBytes := make([]byte, 8)
	binary.BigEndian.PutUint64(batchBytes, batchID)
	h.Write(batchBytes)
	h.Write(merkleRoot)
	epochBytes := make([]byte, 8)
	binary.BigEndian.PutUint64(epochBytes, epoch)
	h.Write(epochBytes)
	h.Write([]byte(verifierAddress))
	return h.Sum(nil)
}

// NewVerifierReputation creates a new VerifierReputation with initial values
func NewVerifierReputation(address string) VerifierReputation {
	return VerifierReputation{
		Address:             address,
		TotalAttestations:   0,
		CorrectAttestations: 0,
		SlashedCount:        0,
		ReputationScore:     math.ZeroInt(),
	}
}
