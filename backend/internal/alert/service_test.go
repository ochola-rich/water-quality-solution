package alert

import (
	"database/sql"
	"testing"
	"time"

	_ "modernc.org/sqlite"

	"github.com/guardians-of-the-lake/backend/internal/models"
)

func setupAlertsTestDB(t *testing.T) *sql.DB {
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("failed to open test db: %v", err)
	}
	db.SetMaxOpenConns(1)

	schema := `
		CREATE TABLE reports (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			user_id INTEGER NOT NULL,
			lat REAL NOT NULL,
			lng REAL NOT NULL,
			photo_path TEXT,
			category TEXT NOT NULL,
			description TEXT,
			device_meta TEXT,
			status TEXT NOT NULL DEFAULT 'pending',
			created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
		);

		CREATE TABLE alerts (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			title TEXT NOT NULL,
			category TEXT NOT NULL,
			severity TEXT NOT NULL DEFAULT 'moderate',
			cluster_lat REAL NOT NULL,
			cluster_lng REAL NOT NULL,
			radius_m REAL NOT NULL DEFAULT 2000.0,
			report_count INTEGER NOT NULL DEFAULT 1,
			status TEXT NOT NULL DEFAULT 'active',
			created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
			resolved_at DATETIME
		);
	`
	if _, err := db.Exec(schema); err != nil {
		t.Fatalf("failed to create test schema: %v", err)
	}
	return db
}

func TestAlertService_EvaluateCluster(t *testing.T) {
	db := setupAlertsTestDB(t)
	defer db.Close()

	service := NewService(db, nil)
	service.RadiusMeters = 2000.0 // 2km

	now := time.Now().UTC()

	// 1. Insert 2 verified reports near Kisumu Pier (-0.1022, 34.7617)
	_, _ = db.Exec(`INSERT INTO reports (id, user_id, lat, lng, category, status, created_at) VALUES (1, 1, -0.1022, 34.7617, 'spill', 'verified', ?)`, now.Add(-1*time.Hour))
	_, _ = db.Exec(`INSERT INTO reports (id, user_id, lat, lng, category, status, created_at) VALUES (2, 2, -0.1030, 34.7620, 'spill', 'verified', ?)`, now.Add(-30*time.Minute))

	// 3rd report in same area (300m away) -> Should cross MinimumReportsForAlert (3) and trigger Alert
	report3 := models.Report{
		ID:        3,
		UserID:    3,
		Lat:       -0.1040,
		Lng:       34.7625,
		Category:  models.CategorySpill,
		Status:    models.StatusVerified,
		CreatedAt: now,
	}

	alert, err := service.EvaluateCluster(report3)
	if err != nil {
		t.Fatalf("EvaluateCluster failed: %v", err)
	}
	if alert == nil {
		t.Fatalf("expected active alert to be triggered, got nil")
	}

	if alert.Severity != models.SeverityCritical {
		t.Errorf("expected severity 'critical' for chemical spill cluster, got '%s'", alert.Severity)
	}
	if alert.ReportCount != 3 {
		t.Errorf("expected 3 reports in cluster, got %d", alert.ReportCount)
	}

	// Verify alert is in active list
	activeList, err := service.GetActiveAlerts()
	if err != nil {
		t.Fatalf("GetActiveAlerts failed: %v", err)
	}
	if len(activeList) != 1 {
		t.Errorf("expected 1 active alert, got %d", len(activeList))
	}

	// Resolve the alert
	err = service.ResolveAlert(alert.ID)
	if err != nil {
		t.Fatalf("ResolveAlert failed: %v", err)
	}

	activeAfterResolve, _ := service.GetActiveAlerts()
	if len(activeAfterResolve) != 0 {
		t.Errorf("expected 0 active alerts after resolution, got %d", len(activeAfterResolve))
	}
}
