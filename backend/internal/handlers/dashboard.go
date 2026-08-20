package handlers

import (
	"database/sql"
	"fmt"
	"strconv"
	"time"

	"github.com/gofiber/fiber/v2"

	"github.com/guardians-of-the-lake/backend/internal/models"
)

// DashboardHandler serves B2G and institutional water quality monitoring metrics and GeoJSON feeds
type DashboardHandler struct {
	DB *sql.DB
}

// NewDashboardHandler creates a new DashboardHandler instance
func NewDashboardHandler(db *sql.DB) *DashboardHandler {
	return &DashboardHandler{DB: db}
}

// GetSummary aggregates platform-wide water quality health statistics
func (h *DashboardHandler) GetSummary(c *fiber.Ctx) error {
	summary := models.DashboardSummary{
		ReportsByCategory: make(map[string]int64),
	}

	// 1. Total verified reports
	_ = h.DB.QueryRow(`SELECT COUNT(*) FROM reports WHERE status = 'verified'`).Scan(&summary.TotalVerifiedReports)

	// 2. Total pending reports
	_ = h.DB.QueryRow(`SELECT COUNT(*) FROM reports WHERE status = 'pending'`).Scan(&summary.TotalPendingReports)

	// 3. Total rewards paid in sats
	_ = h.DB.QueryRow(`SELECT COALESCE(SUM(amount_sats), 0) FROM rewards WHERE status = 'paid'`).Scan(&summary.TotalRewardsSats)

	// 4. Reports in the last 24 hours
	_ = h.DB.QueryRow(`SELECT COUNT(*) FROM reports WHERE created_at >= DATETIME('now', '-24 hours')`).Scan(&summary.Last24HoursCount)

	// 5. Category breakdown
	catRows, err := h.DB.Query(`SELECT category, COUNT(*) FROM reports GROUP BY category`)
	if err == nil {
		defer catRows.Close()
		for catRows.Next() {
			var cat string
			var count int64
			if err := catRows.Scan(&cat, &count); err == nil {
				summary.ReportsByCategory[cat] = count
			}
		}
	}

	// Ensure standard categories exist even if zero
	standardCats := []string{
		string(models.CategoryTurbidity),
		string(models.CategoryAlgae),
		string(models.CategorySpill),
		string(models.CategorySmell),
		string(models.CategoryOther),
	}
	for _, sc := range standardCats {
		if _, exists := summary.ReportsByCategory[sc]; !exists {
			summary.ReportsByCategory[sc] = 0
		}
	}

	// Additional analytics
	var activeGuardiansCount int64
	_ = h.DB.QueryRow(`SELECT COUNT(DISTINCT user_id) FROM reports`).Scan(&activeGuardiansCount)

	return c.JSON(fiber.Map{
		"total_verified_reports": summary.TotalVerifiedReports,
		"total_pending_reports":  summary.TotalPendingReports,
		"total_rewards_sats":     summary.TotalRewardsSats,
		"last_24h_count":         summary.Last24HoursCount,
		"active_guardians_count": activeGuardiansCount,
		"reports_by_category":    summary.ReportsByCategory,
		"timestamp":              time.Now().UTC(),
	})
}

// GetPoints returns GeoJSON FeatureCollection formatted specifically for Leaflet map markers
func (h *DashboardHandler) GetPoints(c *fiber.Ctx) error {
	statusFilter := c.Query("status", "all")
	categoryFilter := c.Query("category", "all")

	baseQuery := `
		SELECT r.id, r.user_id, r.lat, r.lng, COALESCE(r.photo_path, ''), r.category, 
		       COALESCE(r.description, ''), COALESCE(r.device_meta, '{}'), r.status, r.created_at,
		       COALESCE(l.content_hash, ''), COALESCE(u.display_name, 'Water Scout')
		FROM reports r
		LEFT JOIN ledger_entries l ON r.id = l.report_id
		LEFT JOIN users u ON r.user_id = u.id
		WHERE 1=1
	`
	args := make([]interface{}, 0)

	if statusFilter != "all" && statusFilter != "" {
		baseQuery += " AND r.status = ?"
		args = append(args, statusFilter)
	}

	if categoryFilter != "all" && categoryFilter != "" {
		baseQuery += " AND r.category = ?"
		args = append(args, categoryFilter)
	}

	baseQuery += " ORDER BY r.created_at DESC LIMIT 500"

	rows, err := h.DB.Query(baseQuery, args...)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": fmt.Sprintf("failed to query map points: %v", err),
		})
	}
	defer rows.Close()

	features := make([]models.GeoJSONFeature, 0)

	for rows.Next() {
		var id, userID int64
		var lat, lng float64
		var photo, category, desc, meta, status, hash, userName string
		var createdAt time.Time

		if err := rows.Scan(&id, &userID, &lat, &lng, &photo, &category, &desc, &meta, &status, &createdAt, &hash, &userName); err != nil {
			continue
		}

		feature := models.GeoJSONFeature{
			Type: "Feature",
			Geometry: models.GeoJSONGeometry{
				Type:        "Point",
				Coordinates: []float64{lng, lat}, // GeoJSON standard: [longitude, latitude]
			},
			Properties: map[string]interface{}{
				"id":           id,
				"user_id":      userID,
				"user_name":    userName,
				"category":     category,
				"description":  desc,
				"photo_path":   photo,
				"status":       status,
				"ledger_hash":  hash,
				"device_meta":  meta,
				"created_at":   createdAt,
			},
		}

		features = append(features, feature)
	}

	return c.JSON(models.GeoJSONFeatureCollection{
		Type:     "FeatureCollection",
		Features: features,
	})
}

// GetTrends provides 7-day or 30-day time-series aggregate report counts for Chart.js
func (h *DashboardHandler) GetTrends(c *fiber.Ctx) error {
	daysStr := c.Query("days", "7")
	days, err := strconv.Atoi(daysStr)
	if err != nil || days <= 0 || days > 90 {
		days = 7
	}

	query := `
		SELECT DATE(created_at) as report_date, category, COUNT(*) as cnt
		FROM reports
		WHERE created_at >= DATETIME('now', ?)
		GROUP BY DATE(created_at), category
		ORDER BY report_date ASC
	`
	modifier := fmt.Sprintf("-%d days", days)
	rows, err := h.DB.Query(query, modifier)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": fmt.Sprintf("failed to query trends: %v", err),
		})
	}
	defer rows.Close()

	type TrendPoint struct {
		Date     string `json:"date"`
		Category string `json:"category"`
		Count    int64  `json:"count"`
	}

	trends := make([]TrendPoint, 0)
	for rows.Next() {
		var tp TrendPoint
		if err := rows.Scan(&tp.Date, &tp.Category, &tp.Count); err != nil {
			continue
		}
		trends = append(trends, tp)
	}

	return c.JSON(trends)
}
