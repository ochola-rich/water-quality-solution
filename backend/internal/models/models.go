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
	ID          int64          `json:"id"`
	UserID      int64          `json:"user_id"`
	Lat         float64        `json:"lat"`
	Lng         float64        `json:"lng"`
	PhotoPath   string         `json:"photo_path"`
	Category    ReportCategory `json:"category"`
	Description string         `json:"description"`
	DeviceMeta  string         `json:"device_meta"`
	Status      ReportStatus   `json:"status"`
	CreatedAt   time.Time      `json:"created_at"`
	DistanceM   *float64       `json:"distance_m,omitempty"` // For distance-sorted queries
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
