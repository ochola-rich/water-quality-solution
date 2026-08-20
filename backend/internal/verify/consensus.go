package verify

import (
	"database/sql"
	"fmt"
	"time"

	"github.com/guardians-of-the-lake/backend/internal/ledger"
	"github.com/guardians-of-the-lake/backend/internal/models"
	"github.com/guardians-of-the-lake/backend/internal/ws"
)

const (
	// MaxVerificationRadiusMeters is the maximum distance in meters a verifier can be from the report
	MaxVerificationRadiusMeters = 500.0
	// DefaultConsensusThreshold is the weighted reputation score needed to verify or reject a report
	DefaultConsensusThreshold = 3.0
	// MaxVerificationsPerHour is the rate limit for a verifier within a rolling hour
	MaxVerificationsPerHour = 20
)

// ConsensusResult represents the outcome of a peer verification vote
type ConsensusResult struct {
	ReportID           int64               `json:"report_id"`
	VoteType           models.VoteType     `json:"vote_type"`
	VerifierID         int64               `json:"verifier_id"`
	VerifierWeight     float64             `json:"verifier_weight"`
	DistanceMeters     float64             `json:"distance_meters"`
	TotalConfirmWeight float64             `json:"total_confirm_weight"`
	TotalRejectWeight  float64             `json:"total_reject_weight"`
	Threshold          float64             `json:"threshold"`
	PreviousStatus     models.ReportStatus `json:"previous_status"`
	NewStatus          models.ReportStatus `json:"new_status"`
	LedgerEntryCreated bool                `json:"ledger_entry_created"`
	RewardEligible     bool                `json:"reward_eligible"`
	LedgerEntry        *models.LedgerEntry `json:"ledger_entry,omitempty"`
}

// RewardProcessor defines an interface for triggering reward payouts when consensus is reached
type RewardProcessor interface {
	ProcessVerifiedReport(reportID int64, authorID int64) (*models.Reward, error)
}

// AlertEvaluator defines an interface for early warning cluster detection
type AlertEvaluator interface {
	EvaluateCluster(report models.Report) (*models.Alert, error)
}

// ConsensusEngine coordinates peer verification voting, weight aggregation, and ledger anchoring
type ConsensusEngine struct {
	DB              *sql.DB
	Hub             *ws.Hub
	RewardService   RewardProcessor
	AlertEvaluator  AlertEvaluator
	Threshold       float64
	MaxRadiusMeters float64
}

// NewConsensusEngine creates a new ConsensusEngine instance
func NewConsensusEngine(db *sql.DB, hub *ws.Hub, rewardService RewardProcessor) *ConsensusEngine {
	return &ConsensusEngine{
		DB:              db,
		Hub:             hub,
		RewardService:   rewardService,
		Threshold:       DefaultConsensusThreshold,
		MaxRadiusMeters: MaxVerificationRadiusMeters,
	}
}

// SubmitVote processes a verifier's confirm/reject vote for a report
func (ce *ConsensusEngine) SubmitVote(reportID int64, verifierID int64, vote models.VoteType, verifierLat, verifierLng float64) (*ConsensusResult, error) {
	// 1. Fetch Report
	var report models.Report
	var photoNull, descNull, metaNull sql.NullString
	reportQuery := `
		SELECT id, user_id, lat, lng, photo_path, category, description, device_meta, status, created_at
		FROM reports WHERE id = ?
	`
	err := ce.DB.QueryRow(reportQuery, reportID).Scan(
		&report.ID,
		&report.UserID,
		&report.Lat,
		&report.Lng,
		&photoNull,
		&report.Category,
		&descNull,
		&metaNull,
		&report.Status,
		&report.CreatedAt,
	)
	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("report %d not found", reportID)
	} else if err != nil {
		return nil, fmt.Errorf("failed to fetch report %d: %w", reportID, err)
	}

	if photoNull.Valid {
		report.PhotoPath = photoNull.String
	}
	if descNull.Valid {
		report.Description = descNull.String
	}
	if metaNull.Valid {
		report.DeviceMeta = metaNull.String
	}

	// 2. Validate report is in a votable state
	if report.Status == models.StatusVerified {
		return nil, fmt.Errorf("report %d is already verified", reportID)
	}
	if report.Status == models.StatusRejected {
		return nil, fmt.Errorf("report %d is already rejected", reportID)
	}

	// 3. Self-verification prevention: author cannot verify their own report
	if report.UserID == verifierID {
		return nil, fmt.Errorf("self-verification prohibited: author cannot verify own report")
	}

	// 4. Fetch Verifier details & reputation
	var verifier models.User
	userQuery := `SELECT id, phone_hash, display_name, role, reputation_score, tier, created_at FROM users WHERE id = ?`
	err = ce.DB.QueryRow(userQuery, verifierID).Scan(
		&verifier.ID,
		&verifier.PhoneHash,
		&verifier.DisplayName,
		&verifier.Role,
		&verifier.ReputationScore,
		&verifier.Tier,
		&verifier.CreatedAt,
	)
	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("verifier user %d not found", verifierID)
	} else if err != nil {
		return nil, fmt.Errorf("failed to fetch verifier %d: %w", verifierID, err)
	}

	// 5. Proximity / Geofencing Check: Distance must be within MaxRadiusMeters (500m)
	distanceM := HaversineDistance(verifierLat, verifierLng, report.Lat, report.Lng)
	if distanceM > ce.MaxRadiusMeters {
		return nil, fmt.Errorf("verifier is %.1fm away, exceeding maximum allowed radius of %.0fm", distanceM, ce.MaxRadiusMeters)
	}

	// 6. Rate Limit Check: max N verifications in last hour
	var hourlyCount int
	rateLimitQuery := `
		SELECT COUNT(*) FROM verifications 
		WHERE verifier_id = ? AND created_at >= DATETIME('now', '-1 hour')
	`
	if err := ce.DB.QueryRow(rateLimitQuery, verifierID).Scan(&hourlyCount); err != nil {
		return nil, fmt.Errorf("rate limit query failed: %w", err)
	}
	if hourlyCount >= MaxVerificationsPerHour {
		return nil, fmt.Errorf("verification rate limit exceeded (%d/hour)", MaxVerificationsPerHour)
	}

	// 7. Duplicate Vote Check
	var existingVoteCount int
	dupQuery := `SELECT COUNT(*) FROM verifications WHERE report_id = ? AND verifier_id = ?`
	if err := ce.DB.QueryRow(dupQuery, reportID, verifierID).Scan(&existingVoteCount); err != nil {
		return nil, fmt.Errorf("duplicate vote check failed: %w", err)
	}
	if existingVoteCount > 0 {
		return nil, fmt.Errorf("user %d has already submitted a verification vote for report %d", verifierID, reportID)
	}

	// 8. Record the Verification Vote
	insertVoteQuery := `
		INSERT INTO verifications (report_id, verifier_id, vote, distance_m, created_at)
		VALUES (?, ?, ?, ?, CURRENT_TIMESTAMP)
	`
	if _, err := ce.DB.Exec(insertVoteQuery, reportID, verifierID, string(vote), distanceM); err != nil {
		return nil, fmt.Errorf("failed to record verification vote: %w", err)
	}

	// 9. Calculate Accumulated Weighted Scores
	// Sum(reputation_score) grouped by vote type
	calcQuery := `
		SELECT v.vote, COALESCE(SUM(u.reputation_score), 0.0)
		FROM verifications v
		JOIN users u ON v.verifier_id = u.id
		WHERE v.report_id = ?
		GROUP BY v.vote
	`
	rows, err := ce.DB.Query(calcQuery, reportID)
	if err != nil {
		return nil, fmt.Errorf("failed to calculate verification totals: %w", err)
	}
	defer rows.Close()

	var totalConfirmWeight float64
	var totalRejectWeight float64

	for rows.Next() {
		var vType string
		var weight float64
		if err := rows.Scan(&vType, &weight); err != nil {
			continue
		}
		if vType == string(models.VoteConfirm) {
			totalConfirmWeight = weight
		} else if vType == string(models.VoteReject) {
			totalRejectWeight = weight
		}
	}

	result := &ConsensusResult{
		ReportID:           reportID,
		VoteType:           vote,
		VerifierID:         verifierID,
		VerifierWeight:     verifier.ReputationScore,
		DistanceMeters:     distanceM,
		TotalConfirmWeight: totalConfirmWeight,
		TotalRejectWeight:  totalRejectWeight,
		Threshold:          ce.Threshold,
		PreviousStatus:     report.Status,
		NewStatus:          report.Status,
	}

	// 10. Check Consensus Threshold
	if totalConfirmWeight >= ce.Threshold {
		// Consensus Reached: Mark Report Verified
		result.NewStatus = models.StatusVerified
		updateQuery := `UPDATE reports SET status = ? WHERE id = ?`
		if _, err := ce.DB.Exec(updateQuery, models.StatusVerified, reportID); err != nil {
			return nil, fmt.Errorf("failed to update report status to verified: %w", err)
		}
		report.Status = models.StatusVerified

		// Record Cryptographic Hash in SHA-256 Ledger
		ledgerEntry, err := ledger.RecordEntry(ce.DB, report)
		if err == nil && ledgerEntry != nil {
			result.LedgerEntryCreated = true
			result.LedgerEntry = ledgerEntry
		}

		// Reward Payout Trigger
		result.RewardEligible = true
		if ce.RewardService != nil {
			go func() {
				_, _ = ce.RewardService.ProcessVerifiedReport(report.ID, report.UserID)
			}()
		}

		// Early Warning Cluster Evaluation
		if ce.AlertEvaluator != nil {
			go func(r models.Report) {
				_, _ = ce.AlertEvaluator.EvaluateCluster(r)
			}(report)
		}

		// Broadcast WebSocket Event: report:verified
		if ce.Hub != nil {
			ce.Hub.Broadcast("report:verified", map[string]interface{}{
				"report_id":      report.ID,
				"status":         models.StatusVerified,
				"category":       report.Category,
				"lat":            report.Lat,
				"lng":            report.Lng,
				"confirm_weight": totalConfirmWeight,
				"ledger_hash":    ledgerEntry.ContentHash,
				"verified_at":    time.Now().UTC(),
			})
		}
	} else if totalRejectWeight >= ce.Threshold {
		// Consensus Reached: Mark Report Rejected
		result.NewStatus = models.StatusRejected
		updateQuery := `UPDATE reports SET status = ? WHERE id = ?`
		if _, err := ce.DB.Exec(updateQuery, models.StatusRejected, reportID); err != nil {
			return nil, fmt.Errorf("failed to update report status to rejected: %w", err)
		}

		// Broadcast WebSocket Event: report:rejected
		if ce.Hub != nil {
			ce.Hub.Broadcast("report:rejected", map[string]interface{}{
				"report_id":     report.ID,
				"status":        models.StatusRejected,
				"reject_weight": totalRejectWeight,
			})
		}
	} else {
		// Still Pending / In Progress
		if ce.Hub != nil {
			ce.Hub.Broadcast("verification:vote", map[string]interface{}{
				"report_id":      report.ID,
				"verifier_id":    verifierID,
				"vote":           vote,
				"confirm_weight": totalConfirmWeight,
				"reject_weight":  totalRejectWeight,
				"threshold":      ce.Threshold,
			})
		}
	}

	return result, nil
}
