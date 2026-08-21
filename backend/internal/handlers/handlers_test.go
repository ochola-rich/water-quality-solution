package handlers

import (
	"bytes"
	"database/sql"
	"encoding/json"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gofiber/fiber/v2"
	_ "modernc.org/sqlite"

	"github.com/guardians-of-the-lake/backend/internal/lightning"
	"github.com/guardians-of-the-lake/backend/internal/models"
	"github.com/guardians-of-the-lake/backend/internal/rewards"
	"github.com/guardians-of-the-lake/backend/internal/verify"
	"github.com/guardians-of-the-lake/backend/internal/ws"
)

func setupTestApp(t *testing.T) (*fiber.App, *sql.DB) {
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("failed to open in-memory db: %v", err)
	}
	db.SetMaxOpenConns(1)

	schema := `
		CREATE TABLE users (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			phone_hash TEXT UNIQUE NOT NULL,
			display_name TEXT,
			role TEXT NOT NULL DEFAULT 'citizen',
			reputation_score REAL NOT NULL DEFAULT 1.0,
			tier TEXT NOT NULL DEFAULT 'water_scout',
			created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
		);

		CREATE TABLE reports (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			user_id INTEGER NOT NULL REFERENCES users(id),
			lat REAL NOT NULL,
			lng REAL NOT NULL,
			photo_path TEXT,
			category TEXT NOT NULL,
			description TEXT,
			device_meta TEXT,
			status TEXT NOT NULL DEFAULT 'pending',
			created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
		);

		CREATE TABLE verifications (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			report_id INTEGER NOT NULL REFERENCES reports(id),
			verifier_id INTEGER NOT NULL REFERENCES users(id),
			vote TEXT NOT NULL,
			distance_m REAL,
			created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
			UNIQUE(report_id, verifier_id)
		);

		CREATE TABLE ledger_entries (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			report_id INTEGER NOT NULL UNIQUE REFERENCES reports(id),
			content_hash TEXT NOT NULL,
			chain_ref TEXT,
			verified_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
		);

		CREATE TABLE rewards (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			report_id INTEGER NOT NULL REFERENCES reports(id),
			user_id INTEGER NOT NULL REFERENCES users(id),
			amount_sats INTEGER NOT NULL,
			status TEXT NOT NULL DEFAULT 'pending',
			lightning_invoice_id TEXT,
			paid_at DATETIME
		);
	`
	if _, err := db.Exec(schema); err != nil {
		t.Fatalf("failed to setup schema: %v", err)
	}

	// Seed basic users
	_, _ = db.Exec(`INSERT INTO users (id, phone_hash, display_name, role, reputation_score) VALUES (1, 'u1_hash', 'Scout Dunga', 'citizen', 1.0)`)
	_, _ = db.Exec(`INSERT INTO users (id, phone_hash, display_name, role, reputation_score) VALUES (2, 'u2_hash', 'Guardian Pier', 'citizen', 3.0)`)

	hub := ws.NewHub()
	lnClient := lightning.NewClient("", "", "")
	rewardService := rewards.NewService(db, lnClient, hub)
	consensusEngine := verify.NewConsensusEngine(db, hub, rewardService)

	reportHandler := NewReportHandler(db, hub, "./tmp_uploads")
	verifyHandler := NewVerifyHandler(db, hub, consensusEngine)
	dashboardHandler := NewDashboardHandler(db)
	ledgerHandler := NewLedgerHandler(db)
	rewardHandler := NewRewardHandler(db, rewardService)
	userHandler := NewUserHandler(db)

	app := fiber.New()
	api := app.Group("/api")

	api.Post("/reports", reportHandler.SubmitReport)
	api.Get("/reports", reportHandler.ListReports)
	api.Get("/reports/:id", reportHandler.GetReport)
	api.Post("/reports/:id/verify", verifyHandler.SubmitVerificationVote)
	api.Get("/reports/:id/verifications", verifyHandler.ListVerifications)

	api.Get("/dashboard/summary", dashboardHandler.GetSummary)
	api.Get("/dashboard/points", dashboardHandler.GetPoints)
	api.Get("/dashboard/trends", dashboardHandler.GetTrends)

	api.Get("/ledger", ledgerHandler.ListEntries)
	api.Get("/ledger/:report_id", ledgerHandler.GetEntry)
	api.Get("/ledger/:report_id/verify", ledgerHandler.VerifyIntegrity)

	api.Get("/rewards/summary", rewardHandler.GetRewardStats)
	api.Get("/users/leaderboard", userHandler.GetLeaderboard)
	api.Get("/users/:id", userHandler.GetUserProfile)

	return app, db
}

func TestReportSubmissionAndRetrieval(t *testing.T) {
	app, db := setupTestApp(t)
	defer db.Close()

	// 1. Submit Report via multipart form
	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	_ = writer.WriteField("user_id", "1")
	_ = writer.WriteField("lat", "-0.1022")
	_ = writer.WriteField("lng", "34.7617")
	_ = writer.WriteField("category", "turbidity")
	_ = writer.WriteField("description", "Murky brown waters near Kisumu port")
	_ = writer.Close()

	req := httptest.NewRequest("POST", "/api/reports", body)
	req.Header.Set("Content-Type", writer.FormDataContentType())

	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("POST /api/reports failed: %v", err)
	}
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("expected status 201 Created, got %d", resp.StatusCode)
	}

	var createdReport models.Report
	if err := json.NewDecoder(resp.Body).Decode(&createdReport); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if createdReport.ID == 0 || createdReport.Category != models.CategoryTurbidity {
		t.Errorf("unexpected report content: %+v", createdReport)
	}

	// 2. Query reports list with distance sorting
	listReq := httptest.NewRequest("GET", "/api/reports?status=pending&lat=-0.1020&lng=34.7615", nil)
	listResp, err := app.Test(listReq)
	if err != nil {
		t.Fatalf("GET /api/reports failed: %v", err)
	}
	if listResp.StatusCode != http.StatusOK {
		t.Fatalf("expected status 200 OK, got %d", listResp.StatusCode)
	}

	var reports []models.Report
	if err := json.NewDecoder(listResp.Body).Decode(&reports); err != nil {
		t.Fatalf("failed to decode reports list: %v", err)
	}
	if len(reports) != 1 {
		t.Errorf("expected 1 report in list, got %d", len(reports))
	}
	if reports[0].DistanceM == nil {
		t.Errorf("expected distance_m to be populated when verifier coords are provided")
	}

	// 3. Verify the report (User 2 has 3.0 reputation points, meeting threshold)
	voteBody := bytes.NewBufferString(`{"verifier_id": 2, "vote": "confirm", "lat": -0.1023, "lng": 34.7618}`)
	verifyReq := httptest.NewRequest("POST", "/api/reports/1/verify", voteBody)
	verifyReq.Header.Set("Content-Type", "application/json")

	verifyResp, err := app.Test(verifyReq)
	if err != nil {
		t.Fatalf("POST /api/reports/1/verify failed: %v", err)
	}
	if verifyResp.StatusCode != http.StatusOK {
		t.Fatalf("expected status 200 OK for verification, got %d", verifyResp.StatusCode)
	}

	// 4. Test Dashboard Summary
	dashReq := httptest.NewRequest("GET", "/api/dashboard/summary", nil)
	dashResp, err := app.Test(dashReq)
	if err != nil {
		t.Fatalf("GET /api/dashboard/summary failed: %v", err)
	}
	if dashResp.StatusCode != http.StatusOK {
		t.Fatalf("expected status 200 OK, got %d", dashResp.StatusCode)
	}

	// 5. Test GeoJSON points
	pointsReq := httptest.NewRequest("GET", "/api/dashboard/points", nil)
	pointsResp, err := app.Test(pointsReq)
	if err != nil {
		t.Fatalf("GET /api/dashboard/points failed: %v", err)
	}
	if pointsResp.StatusCode != http.StatusOK {
		buf := new(bytes.Buffer)
		buf.ReadFrom(pointsResp.Body)
		t.Fatalf("expected status 200 OK for points, got %d: %s", pointsResp.StatusCode, buf.String())
	}

	var geoJSON models.GeoJSONFeatureCollection
	_ = json.NewDecoder(pointsResp.Body).Decode(&geoJSON)
	if len(geoJSON.Features) != 1 {
		t.Errorf("expected 1 GeoJSON feature, got %d", len(geoJSON.Features))
	}
	if geoJSON.Features[0].Geometry.Type != "Point" {
		t.Errorf("expected geometry type 'Point', got %s", geoJSON.Features[0].Geometry.Type)
	}

	// 6. Test Cryptographic Ledger Integrity Check
	ledgerVerifyReq := httptest.NewRequest("GET", "/api/ledger/1/verify", nil)
	ledgerVerifyResp, err := app.Test(ledgerVerifyReq)
	if err != nil {
		t.Fatalf("GET /api/ledger/1/verify failed: %v", err)
	}
	if ledgerVerifyResp.StatusCode != http.StatusOK {
		t.Fatalf("expected status 200 OK for ledger verify, got %d", ledgerVerifyResp.StatusCode)
	}

	var ledgerResult map[string]interface{}
	_ = json.NewDecoder(ledgerVerifyResp.Body).Decode(&ledgerResult)
	if ledgerResult["status"] != "VALID_UNALTERED" {
		t.Errorf("expected status 'VALID_UNALTERED', got %v", ledgerResult["status"])
	}
}
