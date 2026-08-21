package ledger

import (
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"time"

	"github.com/guardians-of-the-lake/backend/internal/models"
)

// CanonicalReportPayload represents the tamper-evident payload structure for hashing
type CanonicalReportPayload struct {
	ReportID    int64     `json:"report_id"`
	UserID      int64     `json:"user_id"`
	Lat         float64   `json:"lat"`
	Lng         float64   `json:"lng"`
	Category    string    `json:"category"`
	Description string    `json:"description"`
	CreatedAt   time.Time `json:"created_at"`
}

// ComputeReportHash generates a deterministic SHA-256 hash for a verified report
func ComputeReportHash(report models.Report) (string, error) {
	payload := CanonicalReportPayload{
		ReportID:    report.ID,
		UserID:      report.UserID,
		Lat:         report.Lat,
		Lng:         report.Lng,
		Category:    string(report.Category),
		Description: report.Description,
		CreatedAt:   report.CreatedAt.UTC().Truncate(time.Second),
	}

	rawBytes, err := json.Marshal(payload)
	if err != nil {
		return "", fmt.Errorf("failed to marshal canonical report payload: %w", err)
	}

	hash := sha256.Sum256(rawBytes)
	return hex.EncodeToString(hash[:]), nil
}

// RecordEntry records the cryptographic proof of a verified report into ledger_entries
func RecordEntry(db *sql.DB, report models.Report) (*models.LedgerEntry, error) {
	contentHash, err := ComputeReportHash(report)
	if err != nil {
		return nil, fmt.Errorf("error computing report hash: %w", err)
	}

	query := `
		INSERT INTO ledger_entries (report_id, content_hash, verified_at)
		VALUES (?, ?, CURRENT_TIMESTAMP)
		ON CONFLICT(report_id) DO UPDATE SET content_hash = excluded.content_hash, verified_at = CURRENT_TIMESTAMP
		RETURNING id, report_id, content_hash, chain_ref, verified_at
	`

	var entry models.LedgerEntry
	var chainRef sql.NullString
	err = db.QueryRow(query, report.ID, contentHash).Scan(
		&entry.ID,
		&entry.ReportID,
		&entry.ContentHash,
		&chainRef,
		&entry.VerifiedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to record ledger entry for report %d: %w", report.ID, err)
	}

	if chainRef.Valid {
		entry.ChainRef = &chainRef.String
	}

	return &entry, nil
}

// VerifyEntryIntegrity validates whether a ledger entry matches the calculated report hash
func VerifyEntryIntegrity(db *sql.DB, reportID int64) (bool, error) {
	var r models.Report
	var recordedHash string

	reportQuery := `SELECT id, user_id, lat, lng, category, description, created_at FROM reports WHERE id = ?`
	err := db.QueryRow(reportQuery, reportID).Scan(&r.ID, &r.UserID, &r.Lat, &r.Lng, &r.Category, &r.Description, &r.CreatedAt)
	if err != nil {
		return false, fmt.Errorf("report not found: %w", err)
	}

	ledgerQuery := `SELECT content_hash FROM ledger_entries WHERE report_id = ?`
	err = db.QueryRow(ledgerQuery, reportID).Scan(&recordedHash)
	if err != nil {
		return false, fmt.Errorf("ledger entry not found for report %d: %w", reportID, err)
	}

	computedHash, err := ComputeReportHash(r)
	if err != nil {
		return false, fmt.Errorf("failed to recompute report hash: %w", err)
	}

	return computedHash == recordedHash, nil
}
