package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"github.com/gofiber/contrib/websocket"
	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/cors"
	"github.com/gofiber/fiber/v2/middleware/logger"
	"github.com/gofiber/fiber/v2/middleware/recover"
	"github.com/joho/godotenv"

	"github.com/guardians-of-the-lake/backend/internal/ai"
	"github.com/guardians-of-the-lake/backend/internal/alert"
	"github.com/guardians-of-the-lake/backend/internal/db"
	"github.com/guardians-of-the-lake/backend/internal/handlers"
	"github.com/guardians-of-the-lake/backend/internal/ledger"
	"github.com/guardians-of-the-lake/backend/internal/lightning"
	"github.com/guardians-of-the-lake/backend/internal/rewards"
	"github.com/guardians-of-the-lake/backend/internal/verify"
	"github.com/guardians-of-the-lake/backend/internal/ws"
)

func main() {
	// 1. CLI flags
	migrateFlag := flag.Bool("migrate", false, "Run database schema migrations")
	seedFlag := flag.Bool("seed", false, "Seed database with baseline test data")
	portFlag := flag.String("port", "", "Port to run the HTTP server on")
	flag.Parse()

	// 2. Load .env if present
	_ = godotenv.Load(".env")
	_ = godotenv.Load("../.env")

	port := os.Getenv("PORT")
	if *portFlag != "" {
		port = *portFlag
	}
	if port == "" {
		port = "8080"
	}

	dbPath := os.Getenv("DB_PATH")
	if dbPath == "" {
		dbPath = "./guardians.db"
	}

	migrationsDir := os.Getenv("MIGRATIONS_DIR")
	if migrationsDir == "" {
		migrationsDir = "./migrations"
	}

	uploadsDir := os.Getenv("UPLOADS_DIR")
	if uploadsDir == "" {
		uploadsDir = "./uploads"
	}
	_ = os.MkdirAll(uploadsDir, 0755)

	// 3. Connect to SQLite
	database, err := db.InitDB(dbPath)
	if err != nil {
		log.Fatalf("Failed to initialize database: %v", err)
	}
	defer database.Close()

	// 4. Run migrations if requested or ensure tables exist
	if *migrateFlag || os.Getenv("AUTO_MIGRATE") == "true" {
		if err := db.RunMigrations(database, migrationsDir); err != nil {
			log.Fatalf("Failed to run migrations: %v", err)
		}
	} else {
		_ = db.RunMigrations(database, migrationsDir)
	}

	// 5. Seed data if requested
	if *seedFlag {
		if err := db.SeedData(database); err != nil {
			log.Printf("Warning: Failed to seed data: %v", err)
		}
	}

	// 6. Initialize WebSocket Hub
	hub := ws.NewHub()
	go hub.Run()

	// 7. Initialize Lightning & Rewards Services
	lnClient := lightning.NewClient(
		os.Getenv("LNBITS_URL"),
		os.Getenv("LNBITS_ADMIN_KEY"),
		os.Getenv("LNBITS_INVOICE_KEY"),
	)
	rewardService := rewards.NewService(database, lnClient, hub)

	// 8. Initialize Early Warning & AI Prediction Services
	alertService := alert.NewService(database, hub)
	aiService := ai.NewService()

	// 9. Initialize Consensus Engine
	consensusEngine := verify.NewConsensusEngine(database, hub, rewardService)
	consensusEngine.AlertEvaluator = alertService

	// 10. Initialize HTTP Handlers
	reportHandler := handlers.NewReportHandler(database, hub, uploadsDir)
	verifyHandler := handlers.NewVerifyHandler(database, hub, consensusEngine)
	dashboardHandler := handlers.NewDashboardHandler(database)
	ledgerHandler := handlers.NewLedgerHandler(database)
	rewardHandler := handlers.NewRewardHandler(database, rewardService)
	userHandler := handlers.NewUserHandler(database)
	alertHandler := handlers.NewAlertHandler(database, alertService)
	aiHandler := handlers.NewAIHandler(aiService)

	// 11. Initialize Fiber App
	app := fiber.New(fiber.Config{
		AppName:      "Guardians of the Lake API v1.0",
		BodyLimit:    20 * 1024 * 1024, // 20 MB for photo uploads
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 10 * time.Second,
	})

	// Middlewares
	app.Use(recover.New())
	app.Use(logger.New(logger.Config{
		Format: "[${time}] ${status} - ${latency} ${method} ${path}\n",
	}))
	app.Use(cors.New(cors.Config{
		AllowOrigins: "*",
		AllowHeaders: "Origin, Content-Type, Accept, Authorization",
		AllowMethods: "GET, POST, PUT, DELETE, OPTIONS",
	}))

	// Serve Static Frontend & Photo Uploads
	staticDir := os.Getenv("STATIC_DIR")
	if staticDir == "" {
		staticDir = "../frontend"
	}
	if absStatic, err := filepath.Abs(staticDir); err == nil {
		app.Static("/", absStatic)
	}
	if absUploads, err := filepath.Abs(uploadsDir); err == nil {
		app.Static("/uploads", absUploads)
	}

	// Health Check
	app.Get("/healthz", func(c *fiber.Ctx) error {
		return c.JSON(fiber.Map{
			"status":    "healthy",
			"timestamp": time.Now().UTC(),
			"ws_peers":  hub.ClientCount(),
		})
	})

	// API Routes Group
	api := app.Group("/api")

	// Reports
	api.Post("/reports", reportHandler.SubmitReport)
	api.Get("/reports", reportHandler.ListReports)
	api.Get("/reports/:id", reportHandler.GetReport)
	api.Post("/reports/:id/verify", verifyHandler.SubmitVerificationVote)
	api.Get("/reports/:id/verifications", verifyHandler.ListVerifications)

	// Dashboard
	api.Get("/dashboard/summary", dashboardHandler.GetSummary)
	api.Get("/dashboard/points", dashboardHandler.GetPoints)
	api.Get("/dashboard/trends", dashboardHandler.GetTrends)

	// Hash Ledger
	api.Get("/ledger", ledgerHandler.ListEntries)
	api.Get("/ledger/:report_id", ledgerHandler.GetEntry)
	api.Get("/ledger/:report_id/verify", ledgerHandler.VerifyIntegrity)

	// Early Warning Alerts
	api.Get("/alerts/active", alertHandler.GetActiveAlerts)
	api.Get("/alerts", alertHandler.GetAllAlerts)
	api.Post("/alerts/:id/resolve", alertHandler.ResolveAlert)

	// AI Water Quality Prediction
	api.Post("/ai/assess", aiHandler.Assess)

	// Rewards
	api.Get("/rewards/summary", rewardHandler.GetRewardStats)

	// Users & Leaderboard
	api.Get("/users/leaderboard", userHandler.GetLeaderboard)
	api.Get("/users/:id", userHandler.GetUserProfile)
	api.Get("/users/:id/rewards", rewardHandler.GetUserRewards)

	// Internal Routes (Service-to-Service)
	internal := app.Group("/internal")
	internal.Post("/rewards/:report_id/payout", rewardHandler.ManualPayout)
	internal.Post("/ledger/anchor", func(c *fiber.Ctx) error {
		network := c.Query("network", "hedera-testnet")
		root, txRef, count, err := ledger.AnchorBatchToChain(database, network)
		if err != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
		}
		return c.JSON(fiber.Map{
			"status":         "anchored",
			"network":        network,
			"merkle_root":    root,
			"tx_ref":         txRef,
			"anchored_count": count,
			"timestamp":      time.Now().UTC(),
		})
	})

	// WebSocket Dashboard Endpoint
	app.Use("/ws", func(c *fiber.Ctx) error {
		if websocket.IsWebSocketUpgrade(c) {
			c.Locals("allowed", true)
			return c.Next()
		}
		return fiber.ErrUpgradeRequired
	})

	app.Get("/ws/dashboard", websocket.New(func(conn *websocket.Conn) {
		hub.Register(conn)
		defer hub.Unregister(conn)

		for {
			_, _, err := conn.ReadMessage()
			if err != nil {
				break
			}
		}
	}))

	// Start server
	go func() {
		addr := fmt.Sprintf(":%s", port)
		log.Printf("🌊 Guardians of the Lake server running on http://localhost%s", addr)
		if err := app.Listen(addr); err != nil {
			log.Printf("Server shut down: %v", err)
		}
	}()

	// Graceful Shutdown
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, os.Interrupt, syscall.SIGTERM)
	<-quit

	log.Println("Shutting down server gracefully...")
	_ = app.Shutdown()
	log.Println("Server stopped.")
}
