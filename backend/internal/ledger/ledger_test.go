package ledger

import (
	"database/sql"
	"testing"
	"time"

	_ "modernc.org/sqlite"

	"github.com/guardians-of-the-lake/backend/internal/models"
)

func setupLedgerTestDB(t *testing.T) *sql.DB {
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("failed to open test db: %v", err)
	}
	db.SetMaxOpenConns(1)

	schema := `
		CREATE TABLE reports (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			user_id INTEGER NOT NULL,
			lat REAL NOT NULL,
			lng REAL NOT NULL,
			photo_path TEXT,
			category TEXT NOT NULL,
			description TEXT,
			device_meta TEXT,
			status TEXT NOT NULL DEFAULT 'pending',
			created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
		);

		CREATE TABLE ledger_entries (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			report_id INTEGER NOT NULL UNIQUE REFERENCES reports(id),
			content_hash TEXT NOT NULL,
			chain_ref TEXT,
			verified_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
		);
	`
	if _, err := db.Exec(schema); err != nil {
		t.Fatalf("failed to create schema: %v", err)
	}
	return db
}

func TestComputeReportHash_Deterministic(t *testing.T) {
	fixedTime := time.Date(2026, 8, 20, 10, 30, 0, 0, time.UTC)
	report1 := models.Report{
		ID:          42,
		UserID:      7,
		Lat:         -0.1022,
		Lng:         34.7617,
		Category:    models.CategoryTurbidity,
		Description: "Brown muddy plume",
		CreatedAt:   fixedTime,
	}

	hash1, err := ComputeReportHash(report1)
	if err != nil {
		t.Fatalf("ComputeReportHash failed: %v", err)
	}

	hash2, err := ComputeReportHash(report1)
	if err != nil {
		t.Fatalf("ComputeReportHash failed: %v", err)
	}

	if hash1 != hash2 {
		t.Errorf("expected deterministic hash output, got %s vs %s", hash1, hash2)
	}

	if len(hash1) != 64 {
		t.Errorf("expected 64-char hex SHA-256 string, got len %d", len(hash1))
	}
}

func TestRecordEntry_And_VerifyIntegrity(t *testing.T) {
	db := setupLedgerTestDB(t)
	defer db.Close()

	fixedTime := time.Date(2026, 8, 20, 12, 0, 0, 0, time.UTC)
	_, _ = db.Exec(`
		INSERT INTO reports (id, user_id, lat, lng, category, description, status, created_at)
		VALUES (101, 5, -0.1444, 34.7391, 'algae', 'Green bloom at Dunga', 'verified', ?)
	`, fixedTime)

	report := models.Report{
		ID:          101,
		UserID:      5,
		Lat:         -0.1444,
		Lng:         34.7391,
		Category:    models.CategoryAlgae,
		Description: "Green bloom at Dunga",
		CreatedAt:   fixedTime,
	}

	entry, err := RecordEntry(db, report)
	if err != nil {
		t.Fatalf("RecordEntry failed: %v", err)
	}
	if entry.ContentHash == "" {
		t.Errorf("expected non-empty content hash")
	}

	// Verify integrity of unchanged report
	valid, err := VerifyEntryIntegrity(db, 101)
	if err != nil {
		t.Fatalf("VerifyEntryIntegrity failed: %v", err)
	}
	if !valid {
		t.Errorf("expected integrity to be valid for unmodified report")
	}

	// Simulate tampering with report data
	_, _ = db.Exec(`UPDATE reports SET description = 'TAMPERED PLUME DATA' WHERE id = 101`)

	// Tampered report should fail integrity check
	tamperValid, err := VerifyEntryIntegrity(db, 101)
	if err != nil {
		t.Fatalf("VerifyEntryIntegrity after tamper failed: %v", err)
	}
	if tamperValid {
		t.Errorf("expected integrity check to FAIL after data tampering, but got valid")
	}
}
