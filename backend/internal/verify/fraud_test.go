package verify

import (
	"database/sql"
	"testing"
	"time"

	_ "modernc.org/sqlite"
)

func setupTestDB(t *testing.T) *sql.DB {
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("failed to open in-memory db: %v", err)
	}
	db.SetMaxOpenConns(1)

	schema := `
		CREATE TABLE users (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			phone_hash TEXT UNIQUE NOT NULL,
			display_name TEXT,
			role TEXT NOT NULL DEFAULT 'citizen',
			reputation_score REAL NOT NULL DEFAULT 1.0,
			tier TEXT NOT NULL DEFAULT 'water_scout',
			created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
		);

		CREATE TABLE reports (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			user_id INTEGER NOT NULL REFERENCES users(id),
			lat REAL NOT NULL,
			lng REAL NOT NULL,
			photo_path TEXT,
			category TEXT NOT NULL,
			description TEXT,
			device_meta TEXT,
			status TEXT NOT NULL DEFAULT 'pending',
			created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
		);

		CREATE TABLE verifications (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			report_id INTEGER NOT NULL REFERENCES reports(id),
			verifier_id INTEGER NOT NULL REFERENCES users(id),
			vote TEXT NOT NULL,
			distance_m REAL,
			created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
			UNIQUE(report_id, verifier_id)
		);

		CREATE TABLE ledger_entries (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			report_id INTEGER NOT NULL UNIQUE REFERENCES reports(id),
			content_hash TEXT NOT NULL,
			chain_ref TEXT,
			verified_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
		);

		CREATE TABLE rewards (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			report_id INTEGER NOT NULL REFERENCES reports(id),
			user_id INTEGER NOT NULL REFERENCES users(id),
			amount_sats INTEGER NOT NULL,
			status TEXT NOT NULL DEFAULT 'pending',
			lightning_invoice_id TEXT,
			paid_at DATETIME
		);
	`

	if _, err := db.Exec(schema); err != nil {
		t.Fatalf("failed to create schema: %v", err)
	}

	return db
}

func TestHaversineDistance(t *testing.T) {
	// Distance between Kisumu Pier (-0.1022, 34.7617) and Dunga Beach (-0.1444, 34.7391) is ~5.3 km
	dist := HaversineDistance(-0.1022, 34.7617, -0.1444, 34.7391)
	if dist < 5000 || dist > 6000 {
		t.Errorf("expected distance between 5000m and 6000m, got %.2fm", dist)
	}

	// Same point should be 0m
	sameDist := HaversineDistance(-0.1022, 34.7617, -0.1022, 34.7617)
	if sameDist > 0.1 {
		t.Errorf("expected 0m distance for identical coordinates, got %.2fm", sameDist)
	}
}

func TestEvaluateReportSubmission_ImpossibleTravel(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	// Insert user
	_, err := db.Exec(`INSERT INTO users (id, phone_hash, display_name) VALUES (1, 'hash1', 'Test Citizen')`)
	if err != nil {
		t.Fatalf("failed to insert user: %v", err)
	}

	// Insert initial report at Kisumu (-0.1022, 34.7617) 5 minutes ago
	pastTime := time.Now().UTC().Add(-5 * time.Minute)
	_, err = db.Exec(`
		INSERT INTO reports (id, user_id, lat, lng, category, created_at)
		VALUES (1, 1, -0.1022, 34.7617, 'turbidity', ?)
	`, pastTime)
	if err != nil {
		t.Fatalf("failed to insert report: %v", err)
	}

	// New report 5 minutes later in Nairobi (-1.2921, 36.8219), ~265 km away -> Speed > 3000 km/h
	now := time.Now().UTC()
	_, result, err := EvaluateReportSubmission(db, 1, -1.2921, 36.8219, now, `{"cell_tower_id":"SAF-KSM-01"}`)
	if err != nil {
		t.Fatalf("EvaluateReportSubmission failed: %v", err)
	}

	if !result.IsImpossibleTravel {
		t.Errorf("expected impossible travel flag, but got false")
	}

	hasFlag := false
	for _, f := range result.Flags {
		if f == "impossible_movement" {
			hasFlag = true
		}
	}
	if !hasFlag {
		t.Errorf("expected 'impossible_movement' in flags, got %v", result.Flags)
	}
}

func TestEvaluateReportSubmission_NormalTravel(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	// Insert user
	_, _ = db.Exec(`INSERT INTO users (id, phone_hash, display_name) VALUES (1, 'hash1', 'Test Citizen')`)

	// Insert initial report 1 hour ago
	pastTime := time.Now().UTC().Add(-1 * time.Hour)
	_, _ = db.Exec(`INSERT INTO reports (id, user_id, lat, lng, category, created_at) VALUES (1, 1, -0.1022, 34.7617, 'turbidity', ?)`, pastTime)

	// New report 1 hour later 2km away (walking/biking speed ~2 km/h)
	now := time.Now().UTC()
	_, result, err := EvaluateReportSubmission(db, 1, -0.1200, 34.7617, now, `{}`)
	if err != nil {
		t.Fatalf("EvaluateReportSubmission failed: %v", err)
	}

	if result.IsImpossibleTravel {
		t.Errorf("did not expect impossible travel flag for 2km/h travel")
	}
}
