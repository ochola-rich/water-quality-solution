package models

import "time"

// Role represents a user's permission level
type Role string

const (
	RoleCitizen     Role = "citizen"
	RoleInstitution Role = "institution"
	RoleAdmin       Role = "admin"
)

// ReportCategory represents the water anomaly type
type ReportCategory string

const (
	CategoryTurbidity ReportCategory = "turbidity"
	CategoryAlgae     ReportCategory = "algae"
	CategorySpill     ReportCategory = "spill"
	CategorySmell     ReportCategory = "smell"
	CategoryOther     ReportCategory = "other"
)

// ReportStatus represents the verification lifecycle state of a report
type ReportStatus string

const (
	StatusPending  ReportStatus = "pending"
	StatusVerified ReportStatus = "verified"
	StatusRejected ReportStatus = "rejected"
	StatusFlagged  ReportStatus = "flagged"
)

// VoteType represents peer verification vote
type VoteType string

const (
	VoteConfirm VoteType = "confirm"
	VoteReject  VoteType = "reject"
)

// RewardStatus represents the lightning payout status
type RewardStatus string

const (
	RewardPending RewardStatus = "pending"
	RewardPaid    RewardStatus = "paid"
	RewardFailed  RewardStatus = "failed"
)

// User represents a citizen or institution account
type User struct {
	ID              int64     `json:"id"`
	PhoneHash       string    `json:"phone_hash"`
	DisplayName     string    `json:"display_name"`
	Role            Role      `json:"role"`
	ReputationScore float64   `json:"reputation_score"`
	Tier            string    `json:"tier"`
	CreatedAt       time.Time `json:"created_at"`
}

// DeviceMetadata stores raw client telemetries and fraud warning flags
type DeviceMetadata struct {
	CellTowerID        string   `json:"cell_tower_id,omitempty"`
	GPSAccuracy        float64  `json:"gps_accuracy,omitempty"`
	PrevReportDeltaSec float64  `json:"prev_report_delta_sec,omitempty"`
	Flags              []string `json:"flags,omitempty"`
}

// Report represents a citizen water quality observation
type Report struct {
	ID           int64          `json:"id"`
	UserID       int64          `json:"user_id"`
	Lat          float64        `json:"lat"`
	Lng          float64        `json:"lng"`
	PhotoPath    string         `json:"photo_path"`
	Category     ReportCategory `json:"category"`
	Description  string         `json:"description"`
	DeviceMeta   string         `json:"device_meta"`
	AIPrediction string         `json:"ai_prediction,omitempty"`
	ClientUUID   string         `json:"client_uuid,omitempty"`
	Status       ReportStatus   `json:"status"`
	CreatedAt    time.Time      `json:"created_at"`
	DistanceM    *float64       `json:"distance_m,omitempty"` // For distance-sorted queries
}

// Verification represents a peer verifier vote
type Verification struct {
	ID         int64     `json:"id"`
	ReportID   int64     `json:"report_id"`
	VerifierID int64     `json:"verifier_id"`
	Vote       VoteType  `json:"vote"`
	DistanceM  float64   `json:"distance_m"`
	CreatedAt  time.Time `json:"created_at"`
}

// LedgerEntry records the cryptographic SHA-256 hash of a verified report
type LedgerEntry struct {
	ID          int64     `json:"id"`
	ReportID    int64     `json:"report_id"`
	ContentHash string    `json:"content_hash"`
	ChainRef    *string   `json:"chain_ref,omitempty"`
	VerifiedAt  time.Time `json:"verified_at"`
}

// Reward tracks Bitcoin Lightning micropayment status
type Reward struct {
	ID                 int64        `json:"id"`
	ReportID           int64        `json:"report_id"`
	UserID             int64        `json:"user_id"`
	AmountSats         int64        `json:"amount_sats"`
	Status             RewardStatus `json:"status"`
	LightningInvoiceID *string      `json:"lightning_invoice_id,omitempty"`
	PaidAt             *time.Time   `json:"paid_at,omitempty"`
}

// DashboardSummary aggregates overall platform statistics
type DashboardSummary struct {
	TotalVerifiedReports int64            `json:"total_verified_reports"`
	TotalPendingReports  int64            `json:"total_pending_reports"`
	TotalRewardsSats     int64            `json:"total_rewards_sats"`
	ReportsByCategory    map[string]int64 `json:"reports_by_category"`
	Last24HoursCount     int64            `json:"last_24h_count"`
}

// GeoJSON Types for Leaflet Map
type GeoJSONFeatureCollection struct {
	Type     string           `json:"type"`
	Features []GeoJSONFeature `json:"features"`
}

type GeoJSONFeature struct {
	Type       string                 `json:"type"`
	Geometry   GeoJSONGeometry        `json:"geometry"`
	Properties map[string]interface{} `json:"properties"`
}

type GeoJSONGeometry struct {
	Type        string    `json:"type"`
	Coordinates []float64 `json:"coordinates"` // [lng, lat]
}

// AlertSeverity represents early warning urgency
type AlertSeverity string

const (
	SeverityModerate AlertSeverity = "moderate"
	SeverityHigh     AlertSeverity = "high"
	SeverityCritical AlertSeverity = "critical"
)

// AlertStatus represents lifecycle of an early warning alert
type AlertStatus string

const (
	AlertActive   AlertStatus = "active"
	AlertResolved AlertStatus = "resolved"
)

// Alert represents an early warning cluster detection alert
type Alert struct {
	ID          int64          `json:"id"`
	Title       string         `json:"title"`
	Category    ReportCategory `json:"category"`
	Severity    AlertSeverity  `json:"severity"`
	ClusterLat  float64        `json:"cluster_lat"`
	ClusterLng  float64        `json:"cluster_lng"`
	RadiusM     float64        `json:"radius_m"`
	ReportCount int64          `json:"report_count"`
	Status      AlertStatus    `json:"status"`
	CreatedAt   time.Time      `json:"created_at"`
	ResolvedAt  *time.Time     `json:"resolved_at,omitempty"`
}

// AIAssessmentResult represents the automated water quality prediction outcome
type AIAssessmentResult struct {
	CategoryEstimate  ReportCategory `json:"category_estimate"`
	WaterQualityIndex int            `json:"water_quality_index"` // 0-100 (0=Hazardous, 100=Pristine)
	ConfidenceScore   float64        `json:"confidence_score"`    // 0.0 - 1.0
	Severity          string         `json:"severity"`            // normal | warning | critical
	AdvisoryNotice    string         `json:"advisory_notice"`
	DetectedFeatures  []string       `json:"detected_features"`
	EvaluatedAt       time.Time      `json:"evaluated_at"`
}

// LakeHealthSnapshot represents a daily historical lake health index record
type LakeHealthSnapshot struct {
	ID           int64            `json:"id"`
	SnapshotDate string           `json:"snapshot_date"`
	HealthScore  float64          `json:"health_score"`
	TotalReports int64            `json:"total_reports"`
	Breakdown    map[string]int64 `json:"breakdown"`
	Rating       string           `json:"rating"` // Pristine | Good | Moderate | Degraded | Critical
	CreatedAt    time.Time        `json:"created_at"`
}

// LakeHealthResponse returns the current real-time computed score, breakdown, and historical trend
type LakeHealthResponse struct {
	CurrentScore    float64              `json:"current_score"` // 0-100
	Rating          string               `json:"rating"`        // Pristine | Good | Moderate | Degraded | Critical
	Category        string               `json:"category"`      // Primary environmental factor
	TotalReports24h int64                `json:"total_reports_24h"`
	Breakdown       map[string]int64     `json:"breakdown"`
	TrendSnapshots  []LakeHealthSnapshot `json:"trend_snapshots"`
	Recommendations []string             `json:"recommendations"`
	ComputedAt      time.Time            `json:"computed_at"`
}

// ReportSubmitPayload holds input for single report submission or batch offline sync
type ReportSubmitPayload struct {
	ClientUUID   string  `json:"client_uuid,omitempty" form:"client_uuid"`
	UserID       int64   `json:"user_id" form:"user_id"`
	Lat          float64 `json:"lat" form:"lat"`
	Lng          float64 `json:"lng" form:"lng"`
	Category     string  `json:"category" form:"category"`
	Description  string  `json:"description" form:"description"`
	PhotoPath    string  `json:"photo_path,omitempty" form:"photo_path"`
	DeviceMeta   string  `json:"device_meta,omitempty" form:"device_meta"`
	AIPrediction string  `json:"ai_prediction,omitempty" form:"ai_prediction"`
	CellTowerID  string  `json:"cell_tower_id,omitempty" form:"cell_tower_id"`
	GPSAccuracy  string  `json:"gps_accuracy,omitempty" form:"gps_accuracy"`
}

// SyncReportRequest represents batch sync payload from offline localStorage queue
type SyncReportRequest struct {
	Reports []ReportSubmitPayload `json:"reports"`
}

// SyncReportResponse returns synchronization summary
type SyncReportResponse struct {
	SyncedCount    int      `json:"synced_count"`
	DuplicateCount int      `json:"duplicate_count"`
	Reports        []Report `json:"reports"`
	Errors         []string `json:"errors,omitempty"`
}

// AlertSubscriptionRequest represents Web Push / device subscription payload
type AlertSubscriptionRequest struct {
	Endpoint string `json:"endpoint"`
	AuthKey  string `json:"auth_key,omitempty"`
	P256DH   string `json:"p256dh,omitempty"`
	UserID   *int64 `json:"user_id,omitempty"`
}

// AlertSubscription records push notification tokens for early warnings
type AlertSubscription struct {
	ID        int64     `json:"id"`
	Endpoint  string    `json:"endpoint"`
	AuthKey   string    `json:"auth_key,omitempty"`
	P256DH    string    `json:"p256dh,omitempty"`
	UserID    *int64    `json:"user_id,omitempty"`
	CreatedAt time.Time `json:"created_at"`
}
