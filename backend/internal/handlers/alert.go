package handlers

import (
	"database/sql"
	"strconv"

	"github.com/gofiber/fiber/v2"

	"github.com/guardians-of-the-lake/backend/internal/alert"
)

// AlertHandler manages early warning alerts API
type AlertHandler struct {
	DB      *sql.DB
	Service *alert.Service
}

// NewAlertHandler creates a new AlertHandler instance
func NewAlertHandler(db *sql.DB, service *alert.Service) *AlertHandler {
	return &AlertHandler{
		DB:      db,
		Service: service,
	}
}

// GetActiveAlerts handles GET /api/alerts/active
func (h *AlertHandler) GetActiveAlerts(c *fiber.Ctx) error {
	alerts, err := h.Service.GetActiveAlerts()
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": err.Error(),
		})
	}
	return c.JSON(alerts)
}

// GetAllAlerts handles GET /api/alerts
func (h *AlertHandler) GetAllAlerts(c *fiber.Ctx) error {
	alerts, err := h.Service.GetAllAlerts()
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": err.Error(),
		})
	}
	return c.JSON(alerts)
}

// ResolveAlert handles POST /api/alerts/:id/resolve
func (h *AlertHandler) ResolveAlert(c *fiber.Ctx) error {
	idStr := c.Params("id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid alert ID"})
	}

	if err := h.Service.ResolveAlert(id); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": err.Error()})
	}

	return c.JSON(fiber.Map{
		"message":  "Alert marked as resolved",
		"alert_id": id,
	})
}
