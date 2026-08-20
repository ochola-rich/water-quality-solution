package rewards

import (
	"database/sql"
	"testing"

	_ "modernc.org/sqlite"

	"github.com/guardians-of-the-lake/backend/internal/lightning"
	"github.com/guardians-of-the-lake/backend/internal/models"
)

func setupRewardsTestDB(t *testing.T) *sql.DB {
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("failed to open test db: %v", err)
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

func TestRewardsService_ProcessVerifiedReport(t *testing.T) {
	db := setupRewardsTestDB(t)
	defer db.Close()

	// Seed user with 1.0 reputation
	_, _ = db.Exec(`INSERT INTO users (id, phone_hash, display_name, reputation_score, tier) VALUES (1, 'hash_u1', 'Citizen A', 1.0, 'water_scout')`)
	// Seed verified report
	_, _ = db.Exec(`INSERT INTO reports (id, user_id, lat, lng, category, status) VALUES (50, 1, -0.1022, 34.7617, 'turbidity', 'verified')`)

	lnClient := lightning.NewClient("", "", "")
	service := NewService(db, lnClient, nil)
	service.RewardSats = 50

	reward, err := service.ProcessVerifiedReport(50, 1)
	if err != nil {
		t.Fatalf("ProcessVerifiedReport failed: %v", err)
	}

	if reward.Status != models.RewardPaid {
		t.Errorf("expected reward status 'paid', got '%s'", reward.Status)
	}
	if reward.AmountSats != 50 {
		t.Errorf("expected 50 sats reward, got %d", reward.AmountSats)
	}

	// Verify user reputation was incremented
	var newRep float64
	_ = db.QueryRow(`SELECT reputation_score FROM users WHERE id = 1`).Scan(&newRep)
	if newRep < 1.19 || newRep > 1.21 { // 1.0 + 0.2 = 1.2
		t.Errorf("expected user reputation ~1.2, got %.2f", newRep)
	}

	// Double payout attempt should return error
	_, err = service.ProcessVerifiedReport(50, 1)
	if err == nil {
		t.Fatalf("expected error on duplicate payout attempt, got nil")
	}

	// Check GetUserRewards
	rewardsList, err := service.GetUserRewards(1)
	if err != nil {
		t.Fatalf("GetUserRewards failed: %v", err)
	}
	if len(rewardsList) != 1 {
		t.Errorf("expected 1 reward item, got %d", len(rewardsList))
	}
}
