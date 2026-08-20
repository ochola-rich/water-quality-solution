package handlers

import (
	"database/sql"
	"fmt"
	"strconv"
	"strings"

	"github.com/gofiber/fiber/v2"

	"github.com/guardians-of-the-lake/backend/internal/models"
	"github.com/guardians-of-the-lake/backend/internal/verify"
	"github.com/guardians-of-the-lake/backend/internal/ws"
)

// VerifyHandler manages peer verification voting endpoints
type VerifyHandler struct {
	DB     *sql.DB
	Hub    *ws.Hub
	Engine *verify.ConsensusEngine
}

// NewVerifyHandler creates a new VerifyHandler instance
func NewVerifyHandler(db *sql.DB, hub *ws.Hub, engine *verify.ConsensusEngine) *VerifyHandler {
	return &VerifyHandler{
		DB:     db,
		Hub:    hub,
		Engine: engine,
	}
}

// VoteRequest payload for POST /api/reports/:id/verify
type VoteRequest struct {
	VerifierID int64   `json:"verifier_id" form:"verifier_id"`
	Vote       string  `json:"vote" form:"vote"` // "confirm" or "reject"
	Lat        float64 `json:"lat" form:"lat"`
	Lng        float64 `json:"lng" form:"lng"`
}

// SubmitVerificationVote handles POST /api/reports/:id/verify
func (h *VerifyHandler) SubmitVerificationVote(c *fiber.Ctx) error {
	reportIDStr := c.Params("id")
	reportID, err := strconv.ParseInt(reportIDStr, 10, 64)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid report ID"})
	}

	var req VoteRequest
	// Try parsing JSON body first
	if err := c.BodyParser(&req); err != nil || req.Vote == "" {
		// Fallback to form values
		req.Vote = c.FormValue("vote")
		if vID, err := strconv.ParseInt(c.FormValue("verifier_id"), 10, 64); err == nil {
			req.VerifierID = vID
		}
		if lat, err := strconv.ParseFloat(c.FormValue("lat"), 64); err == nil {
			req.Lat = lat
		}
		if lng, err := strconv.ParseFloat(c.FormValue("lng"), 64); err == nil {
			req.Lng = lng
		}
	}

	// Default demo verifier ID if not supplied
	if req.VerifierID <= 0 {
		req.VerifierID = 2
	}

	voteType := models.VoteType(strings.ToLower(strings.TrimSpace(req.Vote)))
	if voteType != models.VoteConfirm && voteType != models.VoteReject {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "vote must be either 'confirm' or 'reject'",
		})
	}

	if req.Lat == 0 && req.Lng == 0 {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "verifier lat and lng coordinates are required for geo-verification",
		})
	}

	result, err := h.Engine.SubmitVote(reportID, req.VerifierID, voteType, req.Lat, req.Lng)
	if err != nil {
		errMsg := err.Error()
		if strings.Contains(errMsg, "not found") {
			return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": errMsg})
		}
		if strings.Contains(errMsg, "prohibited") || strings.Contains(errMsg, "already") || strings.Contains(errMsg, "exceeding") || strings.Contains(errMsg, "limit") {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": errMsg})
		}
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": errMsg})
	}

	return c.Status(fiber.StatusOK).JSON(fiber.Map{
		"message": "Verification vote recorded successfully",
		"result":  result,
	})
}

// ListVerifications handles GET /api/reports/:id/verifications
func (h *VerifyHandler) ListVerifications(c *fiber.Ctx) error {
	reportIDStr := c.Params("id")
	reportID, err := strconv.ParseInt(reportIDStr, 10, 64)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid report ID"})
	}

	query := `
		SELECT v.id, v.report_id, v.verifier_id, v.vote, v.distance_m, v.created_at,
		       u.display_name, u.role, u.reputation_score, u.tier
		FROM verifications v
		JOIN users u ON v.verifier_id = u.id
		WHERE v.report_id = ?
		ORDER BY v.created_at ASC
	`
	rows, err := h.DB.Query(query, reportID)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": fmt.Sprintf("database error: %v", err)})
	}
	defer rows.Close()

	type VerificationDetail struct {
		models.Verification
		DisplayName     string  `json:"display_name"`
		Role            string  `json:"role"`
		ReputationScore float64 `json:"reputation_score"`
		Tier            string  `json:"tier"`
	}

	list := make([]VerificationDetail, 0)
	for rows.Next() {
		var item VerificationDetail
		if err := rows.Scan(
			&item.ID, &item.ReportID, &item.VerifierID, &item.Vote, &item.DistanceM, &item.CreatedAt,
			&item.DisplayName, &item.Role, &item.ReputationScore, &item.Tier,
		); err != nil {
			continue
		}
		list = append(list, item)
	}

	return c.JSON(list)
}
