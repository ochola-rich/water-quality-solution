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

// ReportHandler handles citizen report submissions, listing, and offline batch sync
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

// SubmitReport handles multipart form and JSON report submissions with AI metadata and client UUID idempotency
func (h *ReportHandler) SubmitReport(c *fiber.Ctx) error {
	var payload models.ReportSubmitPayload

	// 1. Try parsing JSON body
	if c.Get("Content-Type") == "application/json" || strings.HasPrefix(c.Get("Content-Type"), "application/json") {
		if err := c.BodyParser(&payload); err != nil {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
				"error": fmt.Sprintf("invalid JSON payload: %v", err),
			})
		}
	} else {
		// Fallback to multipart / form values
		if uStr := c.FormValue("user_id"); uStr != "" {
			if uid, err := strconv.ParseInt(uStr, 10, 64); err == nil {
				payload.UserID = uid
			}
		}
		if lat, err := strconv.ParseFloat(c.FormValue("lat"), 64); err == nil {
			payload.Lat = lat
		}
		if lng, err := strconv.ParseFloat(c.FormValue("lng"), 64); err == nil {
			payload.Lng = lng
		}
		payload.Category = c.FormValue("category")
		payload.Description = c.FormValue("description")
		payload.DeviceMeta = c.FormValue("device_meta")
		payload.AIPrediction = c.FormValue("ai_prediction")
		payload.ClientUUID = c.FormValue("client_uuid")
		payload.CellTowerID = c.FormValue("cell_tower_id")
		payload.GPSAccuracy = c.FormValue("gps_accuracy")
	}

	if payload.UserID <= 0 {
		payload.UserID = 1 // Default demo user
	}

	if payload.Lat == 0 && payload.Lng == 0 {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "lat and lng coordinates are required",
		})
	}

	category := strings.ToLower(strings.TrimSpace(payload.Category))
	if category == "" {
		category = string(models.CategoryTurbidity)
	}
	if !isValidReportCategory(category) {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid report category"})
	}

	deviceMeta := payload.DeviceMeta
	if deviceMeta == "" {
		cellTower := payload.CellTowerID
		accuracy := payload.GPSAccuracy
		if cellTower != "" || accuracy != "" {
			if accuracy == "" {
				deviceMeta = fmt.Sprintf(`{"cell_tower_id":"%s"}`, cellTower)
			} else {
				deviceMeta = fmt.Sprintf(`{"cell_tower_id":"%s","gps_accuracy":%s}`, cellTower, accuracy)
			}
		}
	}

	// 2. Check Client UUID Idempotency (prevent duplicate submissions on retry or offline sync)
	if payload.ClientUUID != "" {
		var existingReport models.Report
		var photoNull, descNull, metaNull, aiNull, uuidNull sql.NullString
		checkQuery := `
			SELECT id, user_id, lat, lng, photo_path, category, description, device_meta, ai_prediction, client_uuid, status, created_at
			FROM reports WHERE client_uuid = ?
		`
		err := h.DB.QueryRow(checkQuery, payload.ClientUUID).Scan(
			&existingReport.ID,
			&existingReport.UserID,
			&existingReport.Lat,
			&existingReport.Lng,
			&photoNull,
			&existingReport.Category,
			&descNull,
			&metaNull,
			&aiNull,
			&uuidNull,
			&existingReport.Status,
			&existingReport.CreatedAt,
		)
		if err == nil {
			if photoNull.Valid {
				existingReport.PhotoPath = photoNull.String
			}
			if descNull.Valid {
				existingReport.Description = descNull.String
			}
			if metaNull.Valid {
				existingReport.DeviceMeta = metaNull.String
			}
			if aiNull.Valid {
				existingReport.AIPrediction = aiNull.String
			}
			if uuidNull.Valid {
				existingReport.ClientUUID = uuidNull.String
			}
			return c.Status(fiber.StatusOK).JSON(existingReport)
		}
	}

	// 3. Handle optional photo upload
	photoPath := payload.PhotoPath
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

	// 4. Perform fraud & impossible travel checks
	submissionTime := time.Now().UTC()
	evaluatedMeta, fraudResult, _ := verify.EvaluateReportSubmission(h.DB, payload.UserID, payload.Lat, payload.Lng, submissionTime, deviceMeta)

	status := models.StatusPending
	if len(fraudResult.Flags) > 0 {
		status = models.StatusFlagged
	}

	// 5. Insert report into database
	query := `
		INSERT INTO reports (user_id, lat, lng, photo_path, category, description, device_meta, ai_prediction, client_uuid, status, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, CURRENT_TIMESTAMP)
		RETURNING id, user_id, lat, lng, photo_path, category, description, device_meta, ai_prediction, client_uuid, status, created_at
	`

	var report models.Report
	var photoNull sql.NullString
	var descNull sql.NullString
	var metaNull sql.NullString
	var aiNull sql.NullString
	var uuidNull sql.NullString

	err = h.DB.QueryRow(
		query,
		payload.UserID,
		payload.Lat,
		payload.Lng,
		photoPath,
		category,
		payload.Description,
		evaluatedMeta,
		payload.AIPrediction,
		payload.ClientUUID,
		status,
	).Scan(
		&report.ID,
		&report.UserID,
		&report.Lat,
		&report.Lng,
		&photoNull,
		&report.Category,
		&descNull,
		&metaNull,
		&aiNull,
		&uuidNull,
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
	if aiNull.Valid {
		report.AIPrediction = aiNull.String
	}
	if uuidNull.Valid {
		report.ClientUUID = uuidNull.String
	}

	// Broadcast live new report notification over WebSocket
	if h.Hub != nil {
		h.Hub.Broadcast("report:new", report)
	}

	return c.Status(fiber.StatusCreated).JSON(report)
}

// SyncOfflineReports handles POST /api/reports/sync for batch offline report synchronization
func (h *ReportHandler) SyncOfflineReports(c *fiber.Ctx) error {
	var req models.SyncReportRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": fmt.Sprintf("invalid sync payload format: %v", err),
		})
	}

	if len(req.Reports) == 0 {
		return c.JSON(models.SyncReportResponse{
			SyncedCount:    0,
			DuplicateCount: 0,
			Reports:        []models.Report{},
		})
	}

	syncedReports := make([]models.Report, 0)
	errors := make([]string, 0)
	syncedCount := 0
	duplicateCount := 0

	for _, item := range req.Reports {
		if item.UserID <= 0 {
			item.UserID = 1
		}
		if item.Lat == 0 && item.Lng == 0 {
			errors = append(errors, fmt.Sprintf("skipped item (missing coordinates for uuid: %s)", item.ClientUUID))
			continue
		}

		category := strings.ToLower(strings.TrimSpace(item.Category))
		if category == "" {
			category = string(models.CategoryTurbidity)
		}
		if !isValidReportCategory(category) {
			errors = append(errors, fmt.Sprintf("skipped item (invalid category for uuid: %s)", item.ClientUUID))
			continue
		}

		// Check idempotency if client_uuid is present
		if item.ClientUUID != "" {
			var existing models.Report
			var photoNull, descNull, metaNull, aiNull, uuidNull sql.NullString
			dupQuery := `
				SELECT id, user_id, lat, lng, photo_path, category, description, device_meta, ai_prediction, client_uuid, status, created_at
				FROM reports WHERE client_uuid = ?
			`
			err := h.DB.QueryRow(dupQuery, item.ClientUUID).Scan(
				&existing.ID, &existing.UserID, &existing.Lat, &existing.Lng, &photoNull,
				&existing.Category, &descNull, &metaNull, &aiNull, &uuidNull, &existing.Status, &existing.CreatedAt,
			)
			if err == nil {
				duplicateCount++
				if photoNull.Valid {
					existing.PhotoPath = photoNull.String
				}
				if descNull.Valid {
					existing.Description = descNull.String
				}
				if metaNull.Valid {
					existing.DeviceMeta = metaNull.String
				}
				if aiNull.Valid {
					existing.AIPrediction = aiNull.String
				}
				if uuidNull.Valid {
					existing.ClientUUID = uuidNull.String
				}
				syncedReports = append(syncedReports, existing)
				continue
			}
		}

		// Fraud evaluation
		evaluatedMeta, fraudResult, _ := verify.EvaluateReportSubmission(h.DB, item.UserID, item.Lat, item.Lng, time.Now().UTC(), item.DeviceMeta)
		status := models.StatusPending
		if len(fraudResult.Flags) > 0 {
			status = models.StatusFlagged
		}

		insertQuery := `
			INSERT INTO reports (user_id, lat, lng, photo_path, category, description, device_meta, ai_prediction, client_uuid, status, created_at)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, CURRENT_TIMESTAMP)
			RETURNING id, user_id, lat, lng, photo_path, category, description, device_meta, ai_prediction, client_uuid, status, created_at
		`
		var newReport models.Report
		var photoNull, descNull, metaNull, aiNull, uuidNull sql.NullString
		err := h.DB.QueryRow(
			insertQuery,
			item.UserID, item.Lat, item.Lng, item.PhotoPath, category, item.Description, evaluatedMeta, item.AIPrediction, item.ClientUUID, status,
		).Scan(
			&newReport.ID, &newReport.UserID, &newReport.Lat, &newReport.Lng, &photoNull,
			&newReport.Category, &descNull, &metaNull, &aiNull, &uuidNull, &newReport.Status, &newReport.CreatedAt,
		)
		if err != nil {
			errors = append(errors, fmt.Sprintf("failed to save report for uuid %s: %v", item.ClientUUID, err))
			continue
		}

		if photoNull.Valid {
			newReport.PhotoPath = photoNull.String
		}
		if descNull.Valid {
			newReport.Description = descNull.String
		}
		if metaNull.Valid {
			newReport.DeviceMeta = metaNull.String
		}
		if aiNull.Valid {
			newReport.AIPrediction = aiNull.String
		}
		if uuidNull.Valid {
			newReport.ClientUUID = uuidNull.String
		}

		syncedCount++
		syncedReports = append(syncedReports, newReport)

		if h.Hub != nil {
			h.Hub.Broadcast("report:new", newReport)
		}
	}

	return c.Status(fiber.StatusOK).JSON(models.SyncReportResponse{
		SyncedCount:    syncedCount,
		DuplicateCount: duplicateCount,
		Reports:        syncedReports,
		Errors:         errors,
	})
}

func isValidReportCategory(category string) bool {
	switch models.ReportCategory(category) {
	case models.CategoryTurbidity, models.CategoryAlgae, models.CategorySpill, models.CategorySmell, models.CategoryOther:
		return true
	default:
		return false
	}
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
		query := `SELECT id, user_id, lat, lng, COALESCE(photo_path, ''), category, COALESCE(description, ''), COALESCE(device_meta, '{}'), COALESCE(ai_prediction, ''), COALESCE(client_uuid, ''), status, created_at FROM reports ORDER BY created_at DESC LIMIT 100`
		rows, err = h.DB.Query(query)
	} else {
		query := `SELECT id, user_id, lat, lng, COALESCE(photo_path, ''), category, COALESCE(description, ''), COALESCE(device_meta, '{}'), COALESCE(ai_prediction, ''), COALESCE(client_uuid, ''), status, created_at FROM reports WHERE status = ? ORDER BY created_at DESC LIMIT 100`
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
		if err := rows.Scan(&r.ID, &r.UserID, &r.Lat, &r.Lng, &r.PhotoPath, &r.Category, &r.Description, &r.DeviceMeta, &r.AIPrediction, &r.ClientUUID, &r.Status, &r.CreatedAt); err != nil {
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

	query := `SELECT id, user_id, lat, lng, COALESCE(photo_path, ''), category, COALESCE(description, ''), COALESCE(device_meta, '{}'), COALESCE(ai_prediction, ''), COALESCE(client_uuid, ''), status, created_at FROM reports WHERE id = ?`
	var r models.Report
	err = h.DB.QueryRow(query, id).Scan(&r.ID, &r.UserID, &r.Lat, &r.Lng, &r.PhotoPath, &r.Category, &r.Description, &r.DeviceMeta, &r.AIPrediction, &r.ClientUUID, &r.Status, &r.CreatedAt)
	if err == sql.ErrNoRows {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "report not found"})
	} else if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": fmt.Sprintf("database error: %v", err)})
	}

	return c.JSON(r)
}
