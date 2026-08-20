package handlers

import (
	"database/sql"
	"fmt"
	"strconv"
	"time"

	"github.com/gofiber/fiber/v2"

	"github.com/guardians-of-the-lake/backend/internal/ledger"
	"github.com/guardians-of-the-lake/backend/internal/models"
)

// LedgerHandler manages cryptographic ledger inspection and tamper-verification endpoints
type LedgerHandler struct {
	DB *sql.DB
}

// NewLedgerHandler creates a new LedgerHandler instance
func NewLedgerHandler(db *sql.DB) *LedgerHandler {
	return &LedgerHandler{DB: db}
}

// ListEntries returns historical cryptographic proof entries
func (h *LedgerHandler) ListEntries(c *fiber.Ctx) error {
	query := `
		SELECT l.id, l.report_id, l.content_hash, l.chain_ref, l.verified_at,
		       r.user_id, r.category, r.lat, r.lng, r.description, r.status
		FROM ledger_entries l
		JOIN reports r ON l.report_id = r.id
		ORDER BY l.verified_at DESC
		LIMIT 100
	`
	rows, err := h.DB.Query(query)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": fmt.Sprintf("database error: %v", err),
		})
	}
	defer rows.Close()

	type LedgerDetail struct {
		models.LedgerEntry
		UserID      int64   `json:"user_id"`
		Category    string  `json:"category"`
		Lat         float64 `json:"lat"`
		Lng         float64 `json:"lng"`
		Description string  `json:"description"`
		Status      string  `json:"status"`
	}

	entries := make([]LedgerDetail, 0)
	for rows.Next() {
		var item LedgerDetail
		var chainRef sql.NullString
		var desc sql.NullString
		if err := rows.Scan(
			&item.ID, &item.ReportID, &item.ContentHash, &chainRef, &item.VerifiedAt,
			&item.UserID, &item.Category, &item.Lat, &item.Lng, &desc, &item.Status,
		); err != nil {
			continue
		}
		if chainRef.Valid {
			item.ChainRef = &chainRef.String
		}
		if desc.Valid {
			item.Description = desc.String
		}
		entries = append(entries, item)
	}

	return c.JSON(entries)
}

// GetEntry returns a single ledger entry by report ID
func (h *LedgerHandler) GetEntry(c *fiber.Ctx) error {
	reportIDStr := c.Params("report_id")
	reportID, err := strconv.ParseInt(reportIDStr, 10, 64)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid report ID"})
	}

	query := `
		SELECT id, report_id, content_hash, chain_ref, verified_at
		FROM ledger_entries WHERE report_id = ?
	`
	var entry models.LedgerEntry
	var chainRef sql.NullString
	err = h.DB.QueryRow(query, reportID).Scan(
		&entry.ID, &entry.ReportID, &entry.ContentHash, &chainRef, &entry.VerifiedAt,
	)
	if err == sql.ErrNoRows {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "no ledger entry found for this report"})
	} else if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": fmt.Sprintf("database error: %v", err)})
	}

	if chainRef.Valid {
		entry.ChainRef = &chainRef.String
	}

	return c.JSON(entry)
}

// VerifyIntegrity recalculates the SHA-256 hash in real time to verify zero tampering
func (h *LedgerHandler) VerifyIntegrity(c *fiber.Ctx) error {
	reportIDStr := c.Params("report_id")
	reportID, err := strconv.ParseInt(reportIDStr, 10, 64)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid report ID"})
	}

	// Fetch report
	var r models.Report
	var desc sql.NullString
	reportQuery := `SELECT id, user_id, lat, lng, category, description, created_at FROM reports WHERE id = ?`
	err = h.DB.QueryRow(reportQuery, reportID).Scan(&r.ID, &r.UserID, &r.Lat, &r.Lng, &r.Category, &desc, &r.CreatedAt)
	if err == sql.ErrNoRows {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "report not found"})
	} else if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": fmt.Sprintf("error querying report: %v", err)})
	}
	if desc.Valid {
		r.Description = desc.String
	}

	// Fetch stored hash
	var recordedHash string
	var verifiedAt time.Time
	ledgerQuery := `SELECT content_hash, verified_at FROM ledger_entries WHERE report_id = ?`
	err = h.DB.QueryRow(ledgerQuery, reportID).Scan(&recordedHash, &verifiedAt)
	if err == sql.ErrNoRows {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "report is not yet anchored in the cryptographic ledger"})
	} else if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": fmt.Sprintf("error querying ledger: %v", err)})
	}

	computedHash, err := ledger.ComputeReportHash(r)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": fmt.Sprintf("failed to recompute hash: %v", err)})
	}

	matches := computedHash == recordedHash

	return c.JSON(fiber.Map{
		"report_id":     reportID,
		"stored_hash":   recordedHash,
		"computed_hash": computedHash,
		"integrity":     matches,
		"verified_at":   verifiedAt,
		"status": func() string {
			if matches {
				return "VALID_UNALTERED"
			}
			return "INTEGRITY_COMPROMISED"
		}(),
	})
}
