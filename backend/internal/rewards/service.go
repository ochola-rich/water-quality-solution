package rewards

import (
	"database/sql"
	"fmt"
	"log"
	"time"

	"github.com/guardians-of-the-lake/backend/internal/lightning"
	"github.com/guardians-of-the-lake/backend/internal/models"
	"github.com/guardians-of-the-lake/backend/internal/ws"
)

const (
	// DefaultReportRewardSats is the standard reward for a verified report
	DefaultReportRewardSats = 50
	// ReputationBonusPerVerifiedReport is the reputation increase for the citizen
	ReputationBonusPerVerifiedReport = 0.2
)

// Service manages reward calculations, database recording, and LNbits lightning payouts
type Service struct {
	DB         *sql.DB
	LNClient   *lightning.Client
	Hub        *ws.Hub
	RewardSats int64
}

// NewService creates a new rewards service
func NewService(db *sql.DB, lnClient *lightning.Client, hub *ws.Hub) *Service {
	return &Service{
		DB:         db,
		LNClient:   lnClient,
		Hub:        hub,
		RewardSats: DefaultReportRewardSats,
	}
}

// ProcessVerifiedReport executes the reward payment flow for a verified report
func (s *Service) ProcessVerifiedReport(reportID int64, authorID int64) (*models.Reward, error) {
	// 1. Check if a reward entry already exists for this report
	var existingID int64
	var existingStatus string
	checkQuery := `SELECT id, status FROM rewards WHERE report_id = ?`
	err := s.DB.QueryRow(checkQuery, reportID).Scan(&existingID, &existingStatus)
	if err == nil && existingStatus == string(models.RewardPaid) {
		return nil, fmt.Errorf("reward for report %d has already been paid", reportID)
	}

	amountSats := s.RewardSats

	// 2. Generate Lightning invoice / payment tracking via LNbits
	memo := fmt.Sprintf("Guardians of the Lake reward for report #%d", reportID)
	invResp, err := s.LNClient.CreateInvoice(amountSats, memo)
	var invoiceID string
	if err == nil && invResp != nil {
		invoiceID = invResp.PaymentRequest
		if invoiceID == "" {
			invoiceID = invResp.PaymentHash
		}
	} else {
		invoiceID = fmt.Sprintf("mock_inv_%d_%d", reportID, time.Now().Unix())
	}

	// 3. Insert or update the reward record as pending
	var rewardID int64
	if existingID > 0 {
		updateQuery := `UPDATE rewards SET amount_sats = ?, status = ?, lightning_invoice_id = ? WHERE id = ? RETURNING id`
		_ = s.DB.QueryRow(updateQuery, amountSats, models.RewardPending, invoiceID, existingID).Scan(&rewardID)
	} else {
		insertQuery := `
			INSERT INTO rewards (report_id, user_id, amount_sats, status, lightning_invoice_id)
			VALUES (?, ?, ?, ?, ?)
			RETURNING id
		`
		if err := s.DB.QueryRow(insertQuery, reportID, authorID, amountSats, models.RewardPending, invoiceID).Scan(&rewardID); err != nil {
			return nil, fmt.Errorf("failed to insert reward record: %w", err)
		}
	}

	// 4. Pay the invoice through platform wallet
	payStatus, payErr := s.LNClient.PayInvoice(invoiceID)

	var reward models.Reward
	reward.ID = rewardID
	reward.ReportID = reportID
	reward.UserID = authorID
	reward.AmountSats = amountSats
	reward.LightningInvoiceID = &invoiceID

	now := time.Now().UTC()
	if payErr == nil && (payStatus == nil || payStatus.Paid) {
		reward.Status = models.RewardPaid
		reward.PaidAt = &now

		updatePaidQuery := `UPDATE rewards SET status = ?, paid_at = CURRENT_TIMESTAMP WHERE id = ?`
		_, _ = s.DB.Exec(updatePaidQuery, models.RewardPaid, rewardID)

		// 5. Increase user reputation and update tier
		updateUserRepQuery := `
			UPDATE users 
			SET reputation_score = reputation_score + ?,
			    tier = CASE 
			      WHEN reputation_score + ? >= 5.0 THEN 'lake_guardian'
			      WHEN reputation_score + ? >= 3.0 THEN 'trusted_verifier'
			      ELSE 'water_scout'
			    END
			WHERE id = ?
		`
		_, _ = s.DB.Exec(updateUserRepQuery, ReputationBonusPerVerifiedReport, ReputationBonusPerVerifiedReport, ReputationBonusPerVerifiedReport, authorID)

		log.Printf("[Rewards] Successfully paid %d sats reward to user %d for verified report %d", amountSats, authorID, reportID)

		// 6. Broadcast reward:paid over WebSocket
		if s.Hub != nil {
			s.Hub.Broadcast("reward:paid", map[string]interface{}{
				"reward_id":            rewardID,
				"report_id":            reportID,
				"user_id":              authorID,
				"amount_sats":          amountSats,
				"status":               models.RewardPaid,
				"lightning_invoice_id": invoiceID,
				"paid_at":              now,
			})
		}
	} else {
		reward.Status = models.RewardFailed
		updateFailedQuery := `UPDATE rewards SET status = ? WHERE id = ?`
		_, _ = s.DB.Exec(updateFailedQuery, models.RewardFailed, rewardID)
		log.Printf("[Rewards] Failed to payout reward for report %d: %v", reportID, payErr)
	}

	return &reward, nil
}

// GetUserRewards retrieves all rewards earned by a specific user
func (s *Service) GetUserRewards(userID int64) ([]models.Reward, error) {
	query := `
		SELECT id, report_id, user_id, amount_sats, status, COALESCE(lightning_invoice_id, ''), paid_at
		FROM rewards WHERE user_id = ? ORDER BY id DESC
	`
	rows, err := s.DB.Query(query, userID)
	if err != nil {
		return nil, fmt.Errorf("failed to query user rewards: %w", err)
	}
	defer rows.Close()

	rewards := make([]models.Reward, 0)
	for rows.Next() {
		var r models.Reward
		var invNull string
		var paidNull sql.NullTime
		if err := rows.Scan(&r.ID, &r.ReportID, &r.UserID, &r.AmountSats, &r.Status, &invNull, &paidNull); err != nil {
			continue
		}
		if invNull != "" {
			r.LightningInvoiceID = &invNull
		}
		if paidNull.Valid {
			r.PaidAt = &paidNull.Time
		}
		rewards = append(rewards, r)
	}
	return rewards, nil
}
