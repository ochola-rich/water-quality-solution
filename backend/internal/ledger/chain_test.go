package ledger

import (
	"testing"
)

func TestComputeMerkleRoot(t *testing.T) {
	// Empty slice
	if ComputeMerkleRoot([]string{}) != "" {
		t.Errorf("expected empty string for empty slice")
	}

	// Single hash
	single := "a665a45920422f9d417e4867efdc4fb8a04a1f3fff1fa07e998e86f7f7a27ae3"
	if ComputeMerkleRoot([]string{single}) != single {
		t.Errorf("expected same hash for single element Merkle root")
	}

	// Multiple hashes (deterministic calculation)
	h1 := "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855"
	h2 := "ca978112ca1bbdcafac231b39a23dc4da786eff8147c4e72b9807785afee48bb"
	h3 := "4e07408562bedb8b60ce05c1decfe3ad16b72230967de01f640b7e4729b49fce"

	root1 := ComputeMerkleRoot([]string{h1, h2, h3})
	root2 := ComputeMerkleRoot([]string{h1, h2, h3})

	if root1 != root2 {
		t.Errorf("expected deterministic Merkle root, got %s vs %s", root1, root2)
	}
	if len(root1) != 64 {
		t.Errorf("expected 64-char hex string, got len %d", len(root1))
	}
}

func TestAnchorBatchToChain(t *testing.T) {
	db := setupLedgerTestDB(t)
	defer db.Close()

	// Seed 2 ledger entries without chain_ref
	_, _ = db.Exec(`INSERT INTO ledger_entries (id, report_id, content_hash) VALUES (1, 10, 'hash_abc1')`)
	_, _ = db.Exec(`INSERT INTO ledger_entries (id, report_id, content_hash) VALUES (2, 20, 'hash_abc2')`)

	root, txRef, count, err := AnchorBatchToChain(db, "hedera-testnet")
	if err != nil {
		t.Fatalf("AnchorBatchToChain failed: %v", err)
	}
	if count != 2 {
		t.Errorf("expected 2 anchored entries, got %d", count)
	}
	if root == "" || txRef == "" {
		t.Errorf("expected valid root and txRef")
	}

	// Check that chain_ref was recorded in the database
	var savedRef string
	_ = db.QueryRow(`SELECT chain_ref FROM ledger_entries WHERE id = 1`).Scan(&savedRef)
	if savedRef != txRef {
		t.Errorf("expected saved chain_ref '%s', got '%s'", txRef, savedRef)
	}
}
