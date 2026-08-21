package alert

import (
	"database/sql"
	"fmt"
	"log"
	"time"

	"github.com/guardians-of-the-lake/backend/internal/models"
	"github.com/guardians-of-the-lake/backend/internal/verify"
	"github.com/guardians-of-the-lake/backend/internal/ws"
)

const (
	// DefaultClusterRadiusMeters is the geographical threshold for grouping anomaly reports (2 km)
	DefaultClusterRadiusMeters = 2000.0
	// DefaultClusterTimeWindow is the rolling time window for cluster detection
	DefaultClusterTimeWindow = 12 * time.Hour
	// MinimumReportsForAlert is the report count threshold to trigger an active alert
	MinimumReportsForAlert = 3
)

// Service manages early warning cluster detection, alert lifecycles, and real-time community broadcasting
type Service struct {
	DB           *sql.DB
	Hub          *ws.Hub
	RadiusMeters float64
	TimeWindow   time.Duration
}

// NewService creates a new early warning alert service
func NewService(db *sql.DB, hub *ws.Hub) *Service {
	return &Service{
		DB:           db,
		Hub:          hub,
		RadiusMeters: DefaultClusterRadiusMeters,
		TimeWindow:   DefaultClusterTimeWindow,
	}
}

// EvaluateCluster checks whether a newly verified report triggers or updates an active anomaly cluster alert
func (s *Service) EvaluateCluster(report models.Report) (*models.Alert, error) {
	// 1. Query recent verified reports within the rolling time window
	windowStart := time.Now().UTC().Add(-s.TimeWindow)
	query := `
		SELECT id, user_id, lat, lng, category, description, created_at
		FROM reports
		WHERE status = 'verified' AND created_at >= ?
	`
	rows, err := s.DB.Query(query, windowStart)
	if err != nil {
		return nil, fmt.Errorf("failed to query verified reports for clustering: %w", err)
	}
	defer rows.Close()

	nearbyCount := int64(1) // Including the current report
	categoryCount := int64(1)

	for rows.Next() {
		var r models.Report
		var desc sql.NullString
		if err := rows.Scan(&r.ID, &r.UserID, &r.Lat, &r.Lng, &r.Category, &desc, &r.CreatedAt); err != nil {
			continue
		}
		if r.ID == report.ID {
			continue
		}

		dist := verify.HaversineDistance(report.Lat, report.Lng, r.Lat, r.Lng)
		if dist <= s.RadiusMeters {
			nearbyCount++
			if r.Category == report.Category {
				categoryCount++
			}
		}
	}

	if nearbyCount < MinimumReportsForAlert {
		return nil, nil // Threshold not reached
	}

	// 2. Determine Severity Level
	severity := models.SeverityModerate
	if report.Category == models.CategorySpill || nearbyCount >= 5 {
		severity = models.SeverityCritical
	} else if nearbyCount >= 3 {
		severity = models.SeverityHigh
	}

	// 3. Check for an existing active alert nearby
	activeAlerts, err := s.GetActiveAlerts()
	if err != nil {
		return nil, fmt.Errorf("failed to check active alerts: %w", err)
	}

	var existingAlert *models.Alert
	for i, alt := range activeAlerts {
		dist := verify.HaversineDistance(report.Lat, report.Lng, alt.ClusterLat, alt.ClusterLng)
		if dist <= s.RadiusMeters && alt.Category == report.Category {
			existingAlert = &activeAlerts[i]
			break
		}
	}

	if existingAlert != nil {
		// Update existing alert
		newCount := existingAlert.ReportCount + 1
		updateQuery := `
			UPDATE alerts 
			SET report_count = ?, severity = CASE WHEN ? = 'critical' THEN 'critical' ELSE severity END
			WHERE id = ?
		`
		_, _ = s.DB.Exec(updateQuery, newCount, string(severity), existingAlert.ID)
		existingAlert.ReportCount = newCount
		if severity == models.SeverityCritical {
			existingAlert.Severity = models.SeverityCritical
		}

		log.Printf("[Alerts] Updated active alert #%d (%s, %d reports)", existingAlert.ID, existingAlert.Title, newCount)

		if s.Hub != nil {
			s.Hub.Broadcast("alert:updated", existingAlert)
		}
		return existingAlert, nil
	}

	// 4. Create a new Early Warning Alert
	title := fmt.Sprintf("Early Warning: %s Anomaly Cluster Detected", formatCategoryTitle(report.Category))
	if severity == models.SeverityCritical {
		title = fmt.Sprintf("EMERGENCY ALERT: Active %s Incident", formatCategoryTitle(report.Category))
	}

	insertQuery := `
		INSERT INTO alerts (title, category, severity, cluster_lat, cluster_lng, radius_m, report_count, status, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, 'active', CURRENT_TIMESTAMP)
		RETURNING id, title, category, severity, cluster_lat, cluster_lng, radius_m, report_count, status, created_at
	`

	var newAlert models.Alert
	err = s.DB.QueryRow(
		insertQuery,
		title,
		string(report.Category),
		string(severity),
		report.Lat,
		report.Lng,
		s.RadiusMeters,
		nearbyCount,
	).Scan(
		&newAlert.ID,
		&newAlert.Title,
		&newAlert.Category,
		&newAlert.Severity,
		&newAlert.ClusterLat,
		&newAlert.ClusterLng,
		&newAlert.RadiusM,
		&newAlert.ReportCount,
		&newAlert.Status,
		&newAlert.CreatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create new alert: %w", err)
	}

	log.Printf("[Alerts] 🚨 TRIGGERED NEW ALERT #%d: %s (Severity: %s, Reports: %d)", newAlert.ID, newAlert.Title, newAlert.Severity, nearbyCount)

	// 5. Broadcast alert over WebSocket
	if s.Hub != nil {
		s.Hub.Broadcast("alert:early_warning", newAlert)
		if newAlert.Severity == models.SeverityCritical {
			s.Hub.Broadcast("alert:critical", newAlert)
		}
	}

	return &newAlert, nil
}

// GetActiveAlerts returns all currently active anomaly cluster alerts
func (s *Service) GetActiveAlerts() ([]models.Alert, error) {
	query := `
		SELECT id, title, category, severity, cluster_lat, cluster_lng, radius_m, report_count, status, created_at, resolved_at
		FROM alerts
		WHERE status = 'active'
		ORDER BY created_at DESC
	`
	rows, err := s.DB.Query(query)
	if err != nil {
		return nil, fmt.Errorf("failed to query active alerts: %w", err)
	}
	defer rows.Close()

	alerts := make([]models.Alert, 0)
	for rows.Next() {
		var a models.Alert
		var resNull sql.NullTime
		if err := rows.Scan(
			&a.ID, &a.Title, &a.Category, &a.Severity, &a.ClusterLat, &a.ClusterLng,
			&a.RadiusM, &a.ReportCount, &a.Status, &a.CreatedAt, &resNull,
		); err != nil {
			continue
		}
		if resNull.Valid {
			a.ResolvedAt = &resNull.Time
		}
		alerts = append(alerts, a)
	}
	return alerts, nil
}

// GetAllAlerts returns alert history including resolved alerts
func (s *Service) GetAllAlerts() ([]models.Alert, error) {
	query := `
		SELECT id, title, category, severity, cluster_lat, cluster_lng, radius_m, report_count, status, created_at, resolved_at
		FROM alerts
		ORDER BY created_at DESC
		LIMIT 100
	`
	rows, err := s.DB.Query(query)
	if err != nil {
		return nil, fmt.Errorf("failed to query alerts history: %w", err)
	}
	defer rows.Close()

	alerts := make([]models.Alert, 0)
	for rows.Next() {
		var a models.Alert
		var resNull sql.NullTime
		if err := rows.Scan(
			&a.ID, &a.Title, &a.Category, &a.Severity, &a.ClusterLat, &a.ClusterLng,
			&a.RadiusM, &a.ReportCount, &a.Status, &a.CreatedAt, &resNull,
		); err != nil {
			continue
		}
		if resNull.Valid {
			a.ResolvedAt = &resNull.Time
		}
		alerts = append(alerts, a)
	}
	return alerts, nil
}

// ResolveAlert marks an alert as resolved
func (s *Service) ResolveAlert(alertID int64) error {
	query := `UPDATE alerts SET status = 'resolved', resolved_at = CURRENT_TIMESTAMP WHERE id = ? AND status = 'active'`
	res, err := s.DB.Exec(query, alertID)
	if err != nil {
		return fmt.Errorf("failed to resolve alert %d: %w", alertID, err)
	}
	rowsAffected, _ := res.RowsAffected()
	if rowsAffected == 0 {
		return fmt.Errorf("alert %d not found or already resolved", alertID)
	}

	if s.Hub != nil {
		s.Hub.Broadcast("alert:resolved", map[string]interface{}{
			"alert_id":    alertID,
			"status":      models.AlertResolved,
			"resolved_at": time.Now().UTC(),
		})
	}
	return nil
}

func formatCategoryTitle(c models.ReportCategory) string {
	switch c {
	case models.CategoryAlgae:
		return "Algal Bloom"
	case models.CategorySpill:
		return "Chemical / Fuel Spill"
	case models.CategoryTurbidity:
		return "High Turbidity Sediment"
	case models.CategorySmell:
		return "Severe Odor"
	default:
		return "Water Contamination"
	}
}
