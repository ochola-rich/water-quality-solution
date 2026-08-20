package handlers

import (
	"database/sql"
	"fmt"
	"strconv"

	"github.com/gofiber/fiber/v2"

	"github.com/guardians-of-the-lake/backend/internal/models"
)

// UserHandler manages user profile and leaderboard queries
type UserHandler struct {
	DB *sql.DB
}

// NewUserHandler creates a new UserHandler instance
func NewUserHandler(db *sql.DB) *UserHandler {
	return &UserHandler{DB: db}
}

// GetUserProfile handles GET /api/users/:id
func (h *UserHandler) GetUserProfile(c *fiber.Ctx) error {
	idStr := c.Params("id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid user ID"})
	}

	var user models.User
	query := `SELECT id, phone_hash, display_name, role, reputation_score, tier, created_at FROM users WHERE id = ?`
	err = h.DB.QueryRow(query, id).Scan(
		&user.ID, &user.PhoneHash, &user.DisplayName, &user.Role, &user.ReputationScore, &user.Tier, &user.CreatedAt,
	)
	if err == sql.ErrNoRows {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "user not found"})
	} else if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": fmt.Sprintf("database error: %v", err)})
	}

	// Fetch user activity summary
	var totalReports, verifiedReports, totalVerifications int64
	var totalSatsEarned int64

	_ = h.DB.QueryRow(`SELECT COUNT(*) FROM reports WHERE user_id = ?`, id).Scan(&totalReports)
	_ = h.DB.QueryRow(`SELECT COUNT(*) FROM reports WHERE user_id = ? AND status = 'verified'`, id).Scan(&verifiedReports)
	_ = h.DB.QueryRow(`SELECT COUNT(*) FROM verifications WHERE verifier_id = ?`, id).Scan(&totalVerifications)
	_ = h.DB.QueryRow(`SELECT COALESCE(SUM(amount_sats), 0) FROM rewards WHERE user_id = ? AND status = 'paid'`, id).Scan(&totalSatsEarned)

	return c.JSON(fiber.Map{
		"user":                user,
		"total_reports":       totalReports,
		"verified_reports":    verifiedReports,
		"total_verifications": totalVerifications,
		"total_sats_earned":   totalSatsEarned,
	})
}

// GetLeaderboard handles GET /api/users/leaderboard
func (h *UserHandler) GetLeaderboard(c *fiber.Ctx) error {
	query := `
		SELECT u.id, u.display_name, u.role, u.reputation_score, u.tier,
		       COUNT(DISTINCT r.id) as reports_count,
		       COUNT(DISTINCT v.id) as verifications_count,
		       COALESCE(SUM(rw.amount_sats), 0) as total_sats
		FROM users u
		LEFT JOIN reports r ON u.id = r.user_id AND r.status = 'verified'
		LEFT JOIN verifications v ON u.id = v.verifier_id
		LEFT JOIN rewards rw ON u.id = rw.user_id AND rw.status = 'paid'
		GROUP BY u.id
		ORDER BY u.reputation_score DESC, total_sats DESC
		LIMIT 20
	`
	rows, err := h.DB.Query(query)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": fmt.Sprintf("database error: %v", err)})
	}
	defer rows.Close()

	type LeaderboardEntry struct {
		ID                 int64   `json:"id"`
		DisplayName        string  `json:"display_name"`
		Role               string  `json:"role"`
		ReputationScore    float64 `json:"reputation_score"`
		Tier               string  `json:"tier"`
		ReportsCount       int64   `json:"reports_count"`
		VerificationsCount int64   `json:"verifications_count"`
		TotalSats          int64   `json:"total_sats"`
	}

	leaderboard := make([]LeaderboardEntry, 0)
	for rows.Next() {
		var entry LeaderboardEntry
		if err := rows.Scan(
			&entry.ID, &entry.DisplayName, &entry.Role, &entry.ReputationScore, &entry.Tier,
			&entry.ReportsCount, &entry.VerificationsCount, &entry.TotalSats,
		); err != nil {
			continue
		}
		leaderboard = append(leaderboard, entry)
	}

	return c.JSON(leaderboard)
}
