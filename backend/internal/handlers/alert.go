package handlers

import (
	"database/sql"
	"fmt"
	"strconv"
	"strings"

	"github.com/gofiber/fiber/v2"

	"github.com/guardians-of-the-lake/backend/internal/alert"
	"github.com/guardians-of-the-lake/backend/internal/models"
)

// AlertHandler manages early warning alerts API and push subscriptions
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

// Subscribe handles POST /api/alerts/subscribe for Web Push notifications
func (h *AlertHandler) Subscribe(c *fiber.Ctx) error {
	var req models.AlertSubscriptionRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid request body"})
	}

	req.Endpoint = strings.TrimSpace(req.Endpoint)
	if req.Endpoint == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "subscription endpoint is required"})
	}

	query := `
		INSERT INTO alert_subscriptions (endpoint, auth_key, p256dh_key, user_id, created_at)
		VALUES (?, ?, ?, ?, CURRENT_TIMESTAMP)
		ON CONFLICT(endpoint) DO UPDATE SET
			auth_key = excluded.auth_key,
			p256dh_key = excluded.p256dh_key,
			user_id = excluded.user_id
		RETURNING id, endpoint, created_at
	`
	var id int64
	var endpoint, createdAt string
	err := h.DB.QueryRow(query, req.Endpoint, req.AuthKey, req.P256DH, req.UserID).Scan(&id, &endpoint, &createdAt)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": fmt.Sprintf("failed to save push subscription: %v", err),
		})
	}

	return c.Status(fiber.StatusCreated).JSON(fiber.Map{
		"status":          "subscribed",
		"subscription_id": id,
		"endpoint":        endpoint,
		"created_at":      createdAt,
	})
}
