package handlers

import (
	"database/sql"
	"fmt"
	"strconv"

	"github.com/gofiber/fiber/v2"

	"github.com/guardians-of-the-lake/backend/internal/models"
	"github.com/guardians-of-the-lake/backend/internal/rewards"
)

// RewardHandler manages Lightning payout triggers and reward lookups
type RewardHandler struct {
	DB      *sql.DB
	Service *rewards.Service
}

// NewRewardHandler creates a new RewardHandler instance
func NewRewardHandler(db *sql.DB, service *rewards.Service) *RewardHandler {
	return &RewardHandler{
		DB:      db,
		Service: service,
	}
}

// ManualPayout handles POST /internal/rewards/:report_id/payout
func (h *RewardHandler) ManualPayout(c *fiber.Ctx) error {
	reportIDStr := c.Params("report_id")
	reportID, err := strconv.ParseInt(reportIDStr, 10, 64)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid report ID"})
	}

	// Fetch report and author
	var authorID int64
	var status string
	err = h.DB.QueryRow(`SELECT user_id, status FROM reports WHERE id = ?`, reportID).Scan(&authorID, &status)
	if err == sql.ErrNoRows {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "report not found"})
	} else if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": fmt.Sprintf("database error: %v", err)})
	}

	if status != string(models.StatusVerified) {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": fmt.Sprintf("cannot payout reward for report with status '%s'; must be 'verified'", status),
		})
	}

	reward, err := h.Service.ProcessVerifiedReport(reportID, authorID)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}

	return c.JSON(fiber.Map{
		"message": "Reward payout processed",
		"reward":  reward,
	})
}

// GetUserRewards handles GET /api/users/:id/rewards
func (h *RewardHandler) GetUserRewards(c *fiber.Ctx) error {
	userIDStr := c.Params("id")
	userID, err := strconv.ParseInt(userIDStr, 10, 64)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid user ID"})
	}

	userRewards, err := h.Service.GetUserRewards(userID)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": fmt.Sprintf("failed to get rewards: %v", err)})
	}

	var totalSats int64
	for _, r := range userRewards {
		if r.Status == models.RewardPaid {
			totalSats += r.AmountSats
		}
	}

	return c.JSON(fiber.Map{
		"user_id":         userID,
		"total_paid_sats": totalSats,
		"rewards_count":   len(userRewards),
		"rewards":         userRewards,
	})
}

// GetRewardStats handles GET /api/rewards/summary
func (h *RewardHandler) GetRewardStats(c *fiber.Ctx) error {
	var totalSats int64
	var paidCount int64
	var pendingCount int64

	_ = h.DB.QueryRow(`SELECT COALESCE(SUM(amount_sats), 0), COUNT(*) FROM rewards WHERE status = 'paid'`).Scan(&totalSats, &paidCount)
	_ = h.DB.QueryRow(`SELECT COUNT(*) FROM rewards WHERE status = 'pending'`).Scan(&pendingCount)

	return c.JSON(fiber.Map{
		"total_rewards_paid_sats": totalSats,
		"total_rewards_paid_kes":  float64(totalSats) * 0.13, // Rough sats-to-KES conversion for demo
		"paid_payouts_count":      paidCount,
		"pending_payouts_count":   pendingCount,
	})
}
