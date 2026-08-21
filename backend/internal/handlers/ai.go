package handlers

import (
	"github.com/gofiber/fiber/v2"

	"github.com/guardians-of-the-lake/backend/internal/ai"
)

// AIHandler manages AI prediction and classification HTTP requests
type AIHandler struct {
	Service *ai.Service
}

// NewAIHandler creates a new AIHandler instance
func NewAIHandler(service *ai.Service) *AIHandler {
	return &AIHandler{Service: service}
}

// Assess handles POST /api/ai/assess
func (h *AIHandler) Assess(c *fiber.Ctx) error {
	var req ai.AssessmentRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "invalid request body",
		})
	}

	result := h.Service.AssessWaterQuality(req)
	return c.JSON(result)
}
