//! Light client proof support for independent verification.
//!
//! Provides Merkle proof generation and verification for:
//! - Finalized sequence commitments (batch chain)
//! - Fairness roots
//! - Execution receipts
//! - Accountability anchors

use sha2::{Sha256, Digest};

/// A single node in a Merkle proof path.
#[derive(Debug, Clone, PartialEq, Eq, serde::Serialize, serde::Deserialize)]
pub struct MerkleProofNode {
    /// Hash of the sibling at this level.
    pub sibling_hash: [u8; 32],
    /// Whether the sibling is on the left (true) or right (false).
    pub is_left: bool,
}

/// A Merkle inclusion proof for a leaf in a commitment tree.
#[derive(Debug, Clone, serde::Serialize, serde::Deserialize)]
pub struct MerkleProof {
    /// The leaf hash being proven.
    pub leaf_hash: [u8; 32],
    /// Path from leaf to root.
    pub path: Vec<MerkleProofNode>,
    /// The expected root hash.
    pub root_hash: [u8; 32],
}

impl MerkleProof {
    /// Verify this proof by recomputing the root from the leaf and path.
    pub fn verify(&self) -> bool {
        let mut current = self.leaf_hash;
        for node in &self.path {
            let mut h = Sha256::new();
            if node.is_left {
                h.update(&node.sibling_hash);
                h.update(&current);
            } else {
                h.update(&current);
                h.update(&node.sibling_hash);
            }
            current = h.finalize().into();
        }
        current == self.root_hash
    }
}

/// Compute a Merkle root from a list of leaf hashes.
/// Returns the root and the tree layers (for proof extraction).
pub fn compute_merkle_root(leaves: &[[u8; 32]]) -> ([u8; 32], Vec<Vec<[u8; 32]>>) {
    if leaves.is_empty() {
        return ([0u8; 32], Vec::new());
    }

    let mut layers: Vec<Vec<[u8; 32]>> = Vec::new();
    layers.push(leaves.to_vec());

    let mut current = leaves.to_vec();
    while current.len() > 1 {
        // Pad to even
        if current.len() % 2 != 0 {
            current.push(*current.last().unwrap());
        }

        let mut next = Vec::new();
        for pair in current.chunks(2) {
            let mut h = Sha256::new();
            h.update(&pair[0]);
            h.update(&pair[1]);
            next.push(h.finalize().into());
        }
        layers.push(next.clone());
        current = next;
    }

    (current[0], layers)
}

/// Generate a Merkle proof for a specific leaf index.
pub fn generate_proof(leaves: &[[u8; 32]], index: usize) -> Option<MerkleProof> {
    if index >= leaves.len() || leaves.is_empty() {
        return None;
    }

    let (root, layers) = compute_merkle_root(leaves);
    let mut path = Vec::new();
    let mut idx = index;

    for layer in &layers[..layers.len().saturating_sub(1)] {
        // Pad layer to even for sibling lookup
        let mut padded = layer.clone();
        if padded.len() % 2 != 0 {
            padded.push(*padded.last().unwrap());
        }

        let sibling_idx = if idx % 2 == 0 { idx + 1 } else { idx - 1 };
        let sibling_hash = padded[sibling_idx.min(padded.len() - 1)];
        let is_left = idx % 2 != 0; // sibling is on left if we're on right

        path.push(MerkleProofNode { sibling_hash, is_left });
        idx /= 2;
    }

    Some(MerkleProof {
        leaf_hash: leaves[index],
        path,
        root_hash: root,
    })
}

/// A finality proof for a specific batch in the finalized chain.
#[derive(Debug, Clone, serde::Serialize, serde::Deserialize)]
pub struct FinalityProof {
    /// The batch being proven.
    pub batch_id: [u8; 32],
    /// Epoch and slot of finalization.
    pub epoch: u64,
    pub slot: u64,
    /// Finalization hash of this batch.
    pub finalization_hash: [u8; 32],
    /// Merkle proof of inclusion in the epoch's finalized batch tree.
    pub inclusion_proof: MerkleProof,
    /// Number of quorum approvals.
    pub quorum_approvals: usize,
    /// Committee size at time of finalization.
    pub committee_size: usize,
}

impl FinalityProof {
    /// Verify the finality proof.
    pub fn verify(&self) -> bool {
        // 1. Verify Merkle inclusion
        if !self.inclusion_proof.verify() {
            return false;
        }

        // 2. Verify the leaf is the finalization hash of this batch
        let mut leaf_hasher = Sha256::new();
        leaf_hasher.update(&self.batch_id);
        leaf_hasher.update(&self.finalization_hash);
        leaf_hasher.update(&self.epoch.to_be_bytes());
        leaf_hasher.update(&self.slot.to_be_bytes());
        let expected_leaf: [u8; 32] = leaf_hasher.finalize().into();

        if expected_leaf != self.inclusion_proof.leaf_hash {
            return false;
        }

        // 3. Verify quorum threshold (2/3 of committee)
        let threshold = (self.committee_size * 2 + 2) / 3; // ceiling division
        self.quorum_approvals >= threshold
    }
}

/// Build a leaf hash for a finalized batch.
pub fn finality_leaf_hash(batch_id: &[u8; 32], finalization_hash: &[u8; 32], epoch: u64, slot: u64) -> [u8; 32] {
    let mut h = Sha256::new();
    h.update(batch_id);
    h.update(finalization_hash);
    h.update(&epoch.to_be_bytes());
    h.update(&slot.to_be_bytes());
    h.finalize().into()
}

/// A fairness proof for a specific batch's ordering.
#[derive(Debug, Clone, serde::Serialize, serde::Deserialize)]
pub struct FairnessProof {
    pub batch_id: [u8; 32],
    pub fairness_root: [u8; 32],
    pub ordered_submission_ids: Vec<[u8; 32]>,
}

impl FairnessProof {
    /// Verify that the fairness root matches the ordered submissions.
    pub fn verify(&self) -> bool {
        let mut h = Sha256::new();
        for id in &self.ordered_submission_ids {
            h.update(id);
        }
        let computed: [u8; 32] = h.finalize().into();
        computed == self.fairness_root
    }
}

#[cfg(test)]
mod tests {
    use super::*;

    fn make_leaves(n: usize) -> Vec<[u8; 32]> {
        (0..n).map(|i| {
            let mut h = Sha256::new();
            h.update(&(i as u64).to_be_bytes());
            h.finalize().into()
        }).collect()
    }

    #[test]
    fn test_merkle_root_single_leaf() {
        let leaves = make_leaves(1);
        let (root, _) = compute_merkle_root(&leaves);
        assert_eq!(root, leaves[0]);
    }

    #[test]
    fn test_merkle_root_two_leaves() {
        let leaves = make_leaves(2);
        let (root, _) = compute_merkle_root(&leaves);

        let mut h = Sha256::new();
        h.update(&leaves[0]);
        h.update(&leaves[1]);
        let expected: [u8; 32] = h.finalize().into();
        assert_eq!(root, expected);
    }

    #[test]
    fn test_merkle_root_deterministic() {
        let leaves = make_leaves(8);
        let (r1, _) = compute_merkle_root(&leaves);
        let (r2, _) = compute_merkle_root(&leaves);
        assert_eq!(r1, r2);
    }

    #[test]
    fn test_merkle_root_empty() {
        let (root, _) = compute_merkle_root(&[]);
        assert_eq!(root, [0u8; 32]);
    }

    #[test]
    fn test_merkle_proof_verify_leaf_0() {
        let leaves = make_leaves(4);
        let proof = generate_proof(&leaves, 0).unwrap();
        assert!(proof.verify());
    }

    #[test]
    fn test_merkle_proof_verify_leaf_3() {
        let leaves = make_leaves(4);
        let proof = generate_proof(&leaves, 3).unwrap();
        assert!(proof.verify());
    }

    #[test]
    fn test_merkle_proof_verify_all_leaves() {
        for n in [2, 3, 4, 5, 7, 8, 16] {
            let leaves = make_leaves(n);
            for i in 0..n {
                let proof = generate_proof(&leaves, i).unwrap();
                assert!(proof.verify(), "proof failed for leaf {i} in tree of size {n}");
            }
        }
    }

    #[test]
    fn test_merkle_proof_invalid_index() {
        let leaves = make_leaves(4);
        assert!(generate_proof(&leaves, 4).is_none());
        assert!(generate_proof(&leaves, 100).is_none());
    }

    #[test]
    fn test_merkle_proof_tampered_leaf() {
        let leaves = make_leaves(4);
        let mut proof = generate_proof(&leaves, 0).unwrap();
        proof.leaf_hash[0] ^= 1;
        assert!(!proof.verify());
    }

    #[test]
    fn test_merkle_proof_tampered_sibling() {
        let leaves = make_leaves(4);
        let mut proof = generate_proof(&leaves, 0).unwrap();
        if !proof.path.is_empty() {
            proof.path[0].sibling_hash[0] ^= 1;
        }
        assert!(!proof.verify());
    }

    #[test]
    fn test_merkle_proof_tampered_root() {
        let leaves = make_leaves(4);
        let mut proof = generate_proof(&leaves, 0).unwrap();
        proof.root_hash[0] ^= 1;
        assert!(!proof.verify());
    }

    #[test]
    fn test_finality_proof_valid() {
        let batch_id = [1u8; 32];
        let fin_hash = [2u8; 32];
        let epoch = 5;
        let slot = 10;

        let leaf = finality_leaf_hash(&batch_id, &fin_hash, epoch, slot);
        let leaves = vec![leaf, [0xAA; 32], [0xBB; 32], [0xCC; 32]];
        let inclusion = generate_proof(&leaves, 0).unwrap();

        let proof = FinalityProof {
            batch_id, epoch, slot,
            finalization_hash: fin_hash,
            inclusion_proof: inclusion,
            quorum_approvals: 3,
            committee_size: 4,
        };
        assert!(proof.verify());
    }

    #[test]
    fn test_finality_proof_insufficient_quorum() {
        let batch_id = [1u8; 32];
        let fin_hash = [2u8; 32];
        let leaf = finality_leaf_hash(&batch_id, &fin_hash, 5, 10);
        let leaves = vec![leaf];
        let inclusion = generate_proof(&leaves, 0).unwrap();

        let proof = FinalityProof {
            batch_id, epoch: 5, slot: 10,
            finalization_hash: fin_hash,
            inclusion_proof: inclusion,
            quorum_approvals: 1,   // Only 1 of 4
            committee_size: 4,
        };
        assert!(!proof.verify());
    }

    #[test]
    fn test_fairness_proof_valid() {
        let ids = vec![[1u8; 32], [2u8; 32], [3u8; 32]];
        let mut h = Sha256::new();
        for id in &ids {
            h.update(id);
        }
        let root: [u8; 32] = h.finalize().into();

        let proof = FairnessProof {
            batch_id: [0u8; 32],
            fairness_root: root,
            ordered_submission_ids: ids,
        };
        assert!(proof.verify());
    }

    #[test]
    fn test_fairness_proof_tampered_order() {
        let ids = vec![[1u8; 32], [2u8; 32], [3u8; 32]];
        let mut h = Sha256::new();
        for id in &ids {
            h.update(id);
        }
        let root: [u8; 32] = h.finalize().into();

        // Swap order
        let proof = FairnessProof {
            batch_id: [0u8; 32],
            fairness_root: root,
            ordered_submission_ids: vec![[2u8; 32], [1u8; 32], [3u8; 32]],
        };
        assert!(!proof.verify());
    }

    #[test]
    fn test_proof_serialization_roundtrip() {
        let leaves = make_leaves(4);
        let proof = generate_proof(&leaves, 2).unwrap();
        let json = serde_json::to_string(&proof).unwrap();
        let parsed: MerkleProof = serde_json::from_str(&json).unwrap();
        assert!(parsed.verify());
        assert_eq!(parsed.root_hash, proof.root_hash);
    }
}
