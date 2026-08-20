package handlers

import (
	"database/sql"
	"fmt"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"

	"github.com/guardians-of-the-lake/backend/internal/models"
	"github.com/guardians-of-the-lake/backend/internal/verify"
	"github.com/guardians-of-the-lake/backend/internal/ws"
)

// ReportHandler handles citizen report submissions and listing
type ReportHandler struct {
	DB         *sql.DB
	Hub        *ws.Hub
	UploadsDir string
}

// NewReportHandler creates a new ReportHandler instance
func NewReportHandler(db *sql.DB, hub *ws.Hub, uploadsDir string) *ReportHandler {
	return &ReportHandler{
		DB:         db,
		Hub:        hub,
		UploadsDir: uploadsDir,
	}
}

// SubmitReport handles multipart form and JSON report submissions
func (h *ReportHandler) SubmitReport(c *fiber.Ctx) error {
	var userID int64 = 1 // Default demo user if not supplied
	if uStr := c.FormValue("user_id"); uStr != "" {
		if uid, err := strconv.ParseInt(uStr, 10, 64); err == nil && uid > 0 {
			userID = uid
		}
	}

	latStr := c.FormValue("lat")
	lngStr := c.FormValue("lng")
	if latStr == "" || lngStr == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "lat and lng coordinates are required",
		})
	}

	lat, err := strconv.ParseFloat(latStr, 64)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid lat coordinate"})
	}

	lng, err := strconv.ParseFloat(lngStr, 64)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid lng coordinate"})
	}

	category := strings.ToLower(c.FormValue("category"))
	if category == "" {
		category = string(models.CategoryTurbidity)
	}

	description := c.FormValue("description")
	deviceMeta := c.FormValue("device_meta")
	if deviceMeta == "" {
		cellTower := c.FormValue("cell_tower_id")
		accuracy := c.FormValue("gps_accuracy")
		deviceMeta = fmt.Sprintf(`{"cell_tower_id":"%s","gps_accuracy":%s}`, cellTower, accuracy)
		if accuracy == "" {
			deviceMeta = fmt.Sprintf(`{"cell_tower_id":"%s"}`, cellTower)
		}
	}

	// Handle optional photo upload
	var photoPath string
	file, err := c.FormFile("photo")
	if err == nil && file != nil {
		ext := filepath.Ext(file.Filename)
		if ext == "" {
			ext = ".jpg"
		}
		filename := fmt.Sprintf("report_%s%s", uuid.New().String(), ext)
		destPath := filepath.Join(h.UploadsDir, filename)
		if err := c.SaveFile(file, destPath); err == nil {
			photoPath = "/uploads/" + filename
		}
	}

	// Perform fraud & impossible travel checks
	submissionTime := time.Now().UTC()
	evaluatedMeta, fraudResult, _ := verify.EvaluateReportSubmission(h.DB, userID, lat, lng, submissionTime, deviceMeta)

	status := models.StatusPending
	if len(fraudResult.Flags) > 0 {
		status = models.StatusFlagged
	}

	// Insert report into database
	query := `
		INSERT INTO reports (user_id, lat, lng, photo_path, category, description, device_meta, status, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, CURRENT_TIMESTAMP)
		RETURNING id, user_id, lat, lng, photo_path, category, description, device_meta, status, created_at
	`

	var report models.Report
	var photoNull sql.NullString
	var descNull sql.NullString
	var metaNull sql.NullString

	err = h.DB.QueryRow(query, userID, lat, lng, photoPath, category, description, evaluatedMeta, status).Scan(
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
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": fmt.Sprintf("Failed to save report: %v", err),
		})
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

	// Broadcast live new report notification over WebSocket
	if h.Hub != nil {
		h.Hub.Broadcast("report:new", report)
	}

	return c.Status(fiber.StatusCreated).JSON(report)
}

// ListReports retrieves water reports, supporting status filters and nearest-first GPS sorting
func (h *ReportHandler) ListReports(c *fiber.Ctx) error {
	statusFilter := c.Query("status", string(models.StatusPending))
	hasVerifierLat := c.Query("lat") != ""
	hasVerifierLng := c.Query("lng") != ""

	var verifierLat, verifierLng float64
	var err error
	if hasVerifierLat && hasVerifierLng {
		verifierLat, err = strconv.ParseFloat(c.Query("lat"), 64)
		if err != nil {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid lat parameter"})
		}
		verifierLng, err = strconv.ParseFloat(c.Query("lng"), 64)
		if err != nil {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid lng parameter"})
		}
	}

	var rows *sql.Rows
	if statusFilter == "all" {
		query := `SELECT id, user_id, lat, lng, COALESCE(photo_path, ''), category, COALESCE(description, ''), COALESCE(device_meta, '{}'), status, created_at FROM reports ORDER BY created_at DESC LIMIT 100`
		rows, err = h.DB.Query(query)
	} else {
		query := `SELECT id, user_id, lat, lng, COALESCE(photo_path, ''), category, COALESCE(description, ''), COALESCE(device_meta, '{}'), status, created_at FROM reports WHERE status = ? ORDER BY created_at DESC LIMIT 100`
		rows, err = h.DB.Query(query, statusFilter)
	}

	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": fmt.Sprintf("Failed to query reports: %v", err),
		})
	}
	defer rows.Close()

	reports := make([]models.Report, 0)
	for rows.Next() {
		var r models.Report
		if err := rows.Scan(&r.ID, &r.UserID, &r.Lat, &r.Lng, &r.PhotoPath, &r.Category, &r.Description, &r.DeviceMeta, &r.Status, &r.CreatedAt); err != nil {
			continue
		}

		if hasVerifierLat && hasVerifierLng {
			dist := verify.HaversineDistance(verifierLat, verifierLng, r.Lat, r.Lng)
			r.DistanceM = &dist
		}

		reports = append(reports, r)
	}

	// Sort nearest-first if verifier coordinates were provided
	if hasVerifierLat && hasVerifierLng {
		sort.Slice(reports, func(i, j int) bool {
			if reports[i].DistanceM == nil || reports[j].DistanceM == nil {
				return false
			}
			return *reports[i].DistanceM < *reports[j].DistanceM
		})
	}

	return c.JSON(reports)
}

// GetReport retrieves a single report by ID
func (h *ReportHandler) GetReport(c *fiber.Ctx) error {
	idStr := c.Params("id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid report ID"})
	}

	query := `SELECT id, user_id, lat, lng, COALESCE(photo_path, ''), category, COALESCE(description, ''), COALESCE(device_meta, '{}'), status, created_at FROM reports WHERE id = ?`
	var r models.Report
	err = h.DB.QueryRow(query, id).Scan(&r.ID, &r.UserID, &r.Lat, &r.Lng, &r.PhotoPath, &r.Category, &r.Description, &r.DeviceMeta, &r.Status, &r.CreatedAt)
	if err == sql.ErrNoRows {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "report not found"})
	} else if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": fmt.Sprintf("database error: %v", err)})
	}

	return c.JSON(r)
}
