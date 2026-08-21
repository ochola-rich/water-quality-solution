package handlers

import (
	"bytes"
	"database/sql"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"math"
	"strconv"
	"time"

	"github.com/gofiber/fiber/v2"

	"github.com/guardians-of-the-lake/backend/internal/models"
)

// DashboardHandler serves B2G and institutional water quality monitoring metrics, GeoJSON feeds, health indices, and data exports
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
				"id":          id,
				"user_id":     userID,
				"user_name":   userName,
				"category":    category,
				"description": desc,
				"photo_path":  photo,
				"status":      status,
				"ledger_hash": hash,
				"device_meta": meta,
				"created_at":  createdAt,
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

// GetHealth computes and returns dynamic Lake Health Index (0-100), category deductions, ratings, and 7-day trend snapshots
func (h *DashboardHandler) GetHealth(c *fiber.Ctx) error {
	// 1. Calculate live 24h health score
	var total24h int64
	breakdown := make(map[string]int64)

	query24h := `
		SELECT category, COUNT(*)
		FROM reports
		WHERE created_at >= DATETIME('now', '-24 hours')
		GROUP BY category
	`
	rows, err := h.DB.Query(query24h)
	if err == nil {
		defer rows.Close()
		for rows.Next() {
			var cat string
			var count int64
			if err := rows.Scan(&cat, &count); err == nil {
				breakdown[cat] = count
				total24h += count
			}
		}
	}

	// Dynamic calculation algorithm:
	// Base: 100
	// Spill: -10 pts each
	// Algae: -5 pts each
	// Turbidity: -2 pts each
	// Smell: -2 pts each
	// Other: -1 pt each
	baseScore := 100.0
	deductions := float64(breakdown[string(models.CategorySpill)]*10 +
		breakdown[string(models.CategoryAlgae)]*5 +
		breakdown[string(models.CategoryTurbidity)]*2 +
		breakdown[string(models.CategorySmell)]*2 +
		breakdown[string(models.CategoryOther)]*1)

	currentScore := math.Max(0.0, math.Min(100.0, baseScore-deductions))

	// Determine Rating
	var rating string
	switch {
	case currentScore >= 85:
		rating = "Pristine"
	case currentScore >= 70:
		rating = "Good"
	case currentScore >= 50:
		rating = "Moderate"
	case currentScore >= 30:
		rating = "Degraded"
	default:
		rating = "Critical"
	}

	// Determine Recommendations
	recommendations := make([]string, 0)
	if breakdown[string(models.CategorySpill)] > 0 {
		recommendations = append(recommendations, "CRITICAL: Deploy hydrocarbon containment booms and notify Maritime Authority immediately.")
	}
	if breakdown[string(models.CategoryAlgae)] >= 2 {
		recommendations = append(recommendations, "WARNING: Elevated microcystin risk; advise against drinking or contacting the water and monitor intake stations. Boiling does not reliably remove cyanotoxins.")
	}
	if breakdown[string(models.CategoryTurbidity)] >= 3 {
		recommendations = append(recommendations, "CAUTION: Heavy runoff siltation detected; inspect upstream catchment soil barriers.")
	}
	if len(recommendations) == 0 {
		recommendations = append(recommendations, "Water quality parameters are within normal baseline thresholds across Lake Victoria monitoring zones.")
	}

	// Store or update snapshot in lake_health table for today
	breakdownJSON, _ := json.Marshal(breakdown)
	todayStr := time.Now().UTC().Format("2006-01-02")
	upsertSnapshot := `
		INSERT INTO lake_health (snapshot_date, health_score, total_reports, breakdown, created_at)
		VALUES (?, ?, ?, ?, CURRENT_TIMESTAMP)
		ON CONFLICT(snapshot_date) DO UPDATE SET
			health_score = excluded.health_score,
			total_reports = excluded.total_reports,
			breakdown = excluded.breakdown
	`
	_, _ = h.DB.Exec(upsertSnapshot, todayStr, currentScore, total24h, string(breakdownJSON))

	// Fetch up to 7 historical snapshots
	trendSnapshots := make([]models.LakeHealthSnapshot, 0)
	snapRows, err := h.DB.Query(`
		SELECT id, snapshot_date, health_score, total_reports, breakdown, created_at
		FROM lake_health
		ORDER BY snapshot_date ASC
		LIMIT 7
	`)
	if err == nil {
		defer snapRows.Close()
		for snapRows.Next() {
			var snap models.LakeHealthSnapshot
			var bStr string
			if err := snapRows.Scan(&snap.ID, &snap.SnapshotDate, &snap.HealthScore, &snap.TotalReports, &bStr, &snap.CreatedAt); err == nil {
				_ = json.Unmarshal([]byte(bStr), &snap.Breakdown)
				if snap.HealthScore >= 85 {
					snap.Rating = "Pristine"
				} else if snap.HealthScore >= 70 {
					snap.Rating = "Good"
				} else if snap.HealthScore >= 50 {
					snap.Rating = "Moderate"
				} else if snap.HealthScore >= 30 {
					snap.Rating = "Degraded"
				} else {
					snap.Rating = "Critical"
				}
				trendSnapshots = append(trendSnapshots, snap)
			}
		}
	}

	resp := models.LakeHealthResponse{
		CurrentScore:    currentScore,
		Rating:          rating,
		Category:        "Lake Victoria Basin",
		TotalReports24h: total24h,
		Breakdown:       breakdown,
		TrendSnapshots:  trendSnapshots,
		Recommendations: recommendations,
		ComputedAt:      time.Now().UTC(),
	}

	return c.JSON(resp)
}

// ExportCSV exports verified water reports in RFC 4180 CSV format for B2G environmental agency compliance
func (h *DashboardHandler) ExportCSV(c *fiber.Ctx) error {
	statusFilter := c.Query("status", "verified")

	query := `
		SELECT r.id, r.user_id, r.category, r.lat, r.lng, COALESCE(r.description, ''),
		       r.status, COALESCE(l.content_hash, ''), COALESCE(l.chain_ref, ''),
		       r.created_at, l.verified_at
		FROM reports r
		LEFT JOIN ledger_entries l ON r.id = l.report_id
	`
	var rows *sql.Rows
	var err error

	if statusFilter == "all" {
		query += ` ORDER BY r.created_at DESC`
		rows, err = h.DB.Query(query)
	} else {
		query += ` WHERE r.status = ? ORDER BY r.created_at DESC`
		rows, err = h.DB.Query(query, statusFilter)
	}

	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": fmt.Sprintf("database query failed: %v", err),
		})
	}
	defer rows.Close()

	buf := new(bytes.Buffer)
	writer := csv.NewWriter(buf)

	// Write CSV Header
	header := []string{
		"Report ID",
		"User ID",
		"Anomaly Category",
		"Latitude",
		"Longitude",
		"Description",
		"Status",
		"SHA-256 Ledger Hash",
		"Blockchain Anchor Reference",
		"Created At (UTC)",
		"Verified At (UTC)",
	}
	if err := writer.Write(header); err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "failed to write CSV header"})
	}

	for rows.Next() {
		var id, userID int64
		var category, desc, status, hash, chainRef string
		var lat, lng float64
		var createdAt time.Time
		var verifiedAt sql.NullTime

		if err := rows.Scan(&id, &userID, &category, &lat, &lng, &desc, &status, &hash, &chainRef, &createdAt, &verifiedAt); err != nil {
			continue
		}

		verifiedAtStr := ""
		if verifiedAt.Valid {
			verifiedAtStr = verifiedAt.Time.UTC().Format(time.RFC3339)
		}

		record := []string{
			strconv.FormatInt(id, 10),
			strconv.FormatInt(userID, 10),
			category,
			fmt.Sprintf("%.6f", lat),
			fmt.Sprintf("%.6f", lng),
			desc,
			status,
			hash,
			chainRef,
			createdAt.UTC().Format(time.RFC3339),
			verifiedAtStr,
		}
		_ = writer.Write(record)
	}

	writer.Flush()

	c.Set("Content-Type", "text/csv; charset=utf-8")
	c.Set("Content-Disposition", "attachment; filename=\"lake_victoria_water_quality_reports.csv\"")
	return c.Send(buf.Bytes())
}
