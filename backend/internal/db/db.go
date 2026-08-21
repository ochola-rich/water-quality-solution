package db

import (
	"database/sql"
	"fmt"
	"log"
	"os"
	"path/filepath"

	_ "modernc.org/sqlite"
)

// InitDB initializes SQLite database with foreign keys and WAL mode enabled
func InitDB(dbPath string) (*sql.DB, error) {
	dir := filepath.Dir(dbPath)
	if dir != "" && dir != "." {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return nil, fmt.Errorf("failed to create db directory %s: %w", dir, err)
		}
	}

	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		return nil, fmt.Errorf("failed to open sqlite database at %s: %w", dbPath, err)
	}

	// Configure connection pragmas for performance and integrity
	pragmas := []string{
		"PRAGMA foreign_keys = ON;",
		"PRAGMA journal_mode = WAL;",
		"PRAGMA synchronous = NORMAL;",
		"PRAGMA busy_timeout = 5000;",
	}

	for _, pragma := range pragmas {
		if _, err := db.Exec(pragma); err != nil {
			return nil, fmt.Errorf("failed to execute pragma '%s': %w", pragma, err)
		}
	}

	if err := db.Ping(); err != nil {
		return nil, fmt.Errorf("database ping failed: %w", err)
	}

	log.Printf("[DB] Connected to SQLite database at: %s", dbPath)
	return db, nil
}

// RunMigrations executes SQL migration files from the migrations folder
func RunMigrations(db *sql.DB, migrationsDir string) error {
	entries, err := os.ReadDir(migrationsDir)
	if err != nil {
		return fmt.Errorf("failed to read migrations directory %s: %w", migrationsDir, err)
	}

	for _, entry := range entries {
		if filepath.Ext(entry.Name()) != ".sql" {
			continue
		}

		filePath := filepath.Join(migrationsDir, entry.Name())
		content, err := os.ReadFile(filePath)
		if err != nil {
			return fmt.Errorf("failed to read migration file %s: %w", filePath, err)
		}

		log.Printf("[DB] Applying migration: %s", entry.Name())
		if _, err := db.Exec(string(content)); err != nil {
			return fmt.Errorf("migration %s failed: %w", entry.Name(), err)
		}
	}

	log.Println("[DB] All migrations applied successfully.")
	return nil
}

// SeedData inserts test users and baseline reports across Lake Victoria
func SeedData(db *sql.DB) error {
	log.Println("[DB] Seeding baseline test data...")

	// Check if reports table already has data to avoid duplicate seeding
	var reportCount int
	if err := db.QueryRow("SELECT COUNT(*) FROM reports").Scan(&reportCount); err != nil {
		return fmt.Errorf("failed to check reports table count: %w", err)
	}
	if reportCount > 0 {
		log.Println("[DB] Reports table already has data, skipping seed")
		return nil
	}

	// 1. Seed test users
	users := []struct {
		phoneHash       string
		displayName     string
		role            string
		reputationScore float64
		tier            string
	}{
		{"phone_hash_wanja_001", "Wanja Rouwel", "admin", 5.0, "lake_guardian"},
		{"phone_hash_berna_002", "Bernadette Akinyi", "institution", 4.5, "trusted_verifier"},
		{"phone_hash_rich_003", "Otieno Richard", "citizen", 3.8, "water_scout"},
		{"phone_hash_citizen_004", "Achieng Onyango (Dunga)", "citizen", 2.5, "water_scout"},
		{"phone_hash_citizen_005", "Juma Mwangi (Kendu)", "citizen", 1.8, "water_scout"},
		{"phone_hash_citizen_006", "Brian Okoth (Homa Bay)", "citizen", 2.0, "water_scout"},
	}

	for _, u := range users {
		query := `INSERT OR IGNORE INTO users (phone_hash, display_name, role, reputation_score, tier) VALUES (?, ?, ?, ?, ?)`
		if _, err := db.Exec(query, u.phoneHash, u.displayName, u.role, u.reputationScore, u.tier); err != nil {
			return fmt.Errorf("failed to seed user %s: %w", u.displayName, err)
		}
	}

	// 2. Seed baseline reports around Lake Victoria (Kisumu Bay, Dunga, Homa Bay, etc.)
	reports := []struct {
		userID      int64
		lat         float64
		lng         float64
		category    string
		description string
		deviceMeta  string
		status      string
	}{
		{
			userID:      1,
			lat:         -0.1022,
			lng:         34.7617,
			category:    "turbidity",
			description: "High sedimentation and brownish water near Kisumu Pier",
			deviceMeta:  `{"gps_accuracy": 4.2, "cell_tower_id": "SAF-KSM-01", "flags": []}`,
			status:      "verified",
		},
		{
			userID:      2,
			lat:         -0.1444,
			lng:         34.7391,
			category:    "algae",
			description: "Dense green algae bloom spreading near Dunga Beach fish landing site",
			deviceMeta:  `{"gps_accuracy": 3.8, "cell_tower_id": "SAF-DNG-02", "flags": []}`,
			status:      "verified",
		},
		{
			userID:      3,
			lat:         -0.5273,
			lng:         34.4566,
			category:    "spill",
			description: "Visible oily sheen and chemical odor near Homa Bay harbor",
			deviceMeta:  `{"gps_accuracy": 5.0, "cell_tower_id": "AIR-HMB-01", "flags": []}`,
			status:      "pending",
		},
		{
			userID:      4,
			lat:         -0.3589,
			lng:         34.6433,
			category:    "smell",
			description: "Strong decaying organic odor near Kendu Bay reed beds",
			deviceMeta:  `{"gps_accuracy": 6.1, "cell_tower_id": "SAF-KND-03", "flags": []}`,
			status:      "pending",
		},
		{
			userID:      5,
			lat:         -0.4820,
			lng:         34.2050,
			category:    "turbidity",
			description: "Heavy runoff mud entering the lake near Mbita causeway",
			deviceMeta:  `{"gps_accuracy": 4.5, "cell_tower_id": "SAF-MBT-01", "flags": []}`,
			status:      "pending",
		},
	}

	for _, r := range reports {
		query := `INSERT OR IGNORE INTO reports (user_id, lat, lng, category, description, device_meta, status) VALUES (?, ?, ?, ?, ?, ?, ?)`
		res, err := db.Exec(query, r.userID, r.lat, r.lng, r.category, r.description, r.deviceMeta, r.status)
		if err != nil {
			return fmt.Errorf("failed to seed report: %w", err)
		}

		reportID, _ := res.LastInsertId()
		if r.status == "verified" && reportID > 0 {
			// Seed ledger entry and reward for verified baseline report
			contentHash := fmt.Sprintf("sha256_mock_hash_for_report_%d", reportID)
			ledgerQuery := `INSERT OR IGNORE INTO ledger_entries (report_id, content_hash) VALUES (?, ?)`
			_, _ = db.Exec(ledgerQuery, reportID, contentHash)

			rewardQuery := `INSERT OR IGNORE INTO rewards (report_id, user_id, amount_sats, status, lightning_invoice_id, paid_at) VALUES (?, ?, ?, ?, ?, CURRENT_TIMESTAMP)`
			_, _ = db.Exec(rewardQuery, reportID, r.userID, 50, "paid", fmt.Sprintf("lnbc_mock_inv_%d", reportID))
		}
	}

	log.Println("[DB] Baseline test data seeded successfully.")
	return nil
}
