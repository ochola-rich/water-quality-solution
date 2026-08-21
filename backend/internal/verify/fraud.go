package verify

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"math"
	"strings"
	"time"

	"github.com/guardians-of-the-lake/backend/internal/models"
)

const (
	// EarthRadiusMeters is the approximate mean radius of the Earth in meters
	EarthRadiusMeters = 6371000.0
	// MaxAllowedSpeedKmH is the maximum feasible travel speed before flagging impossible movement
	MaxAllowedSpeedKmH = 120.0
)

// HaversineDistance calculates the great-circle distance between two GPS coordinates in meters
func HaversineDistance(lat1, lon1, lat2, lon2 float64) float64 {
	dLat := (lat2 - lat1) * math.Pi / 180.0
	dLon := (lon2 - lon1) * math.Pi / 180.0

	radLat1 := lat1 * math.Pi / 180.0
	radLat2 := lat2 * math.Pi / 180.0

	a := math.Sin(dLat/2)*math.Sin(dLat/2) +
		math.Sin(dLon/2)*math.Sin(dLon/2)*math.Cos(radLat1)*math.Cos(radLat2)
	c := 2 * math.Atan2(math.Sqrt(a), math.Sqrt(1-a))

	return EarthRadiusMeters * c
}

// FraudCheckResult holds the evaluation outcome of report metadata
type FraudCheckResult struct {
	Flags              []string `json:"flags"`
	ImpliedSpeedKmH    *float64 `json:"implied_speed_kmh,omitempty"`
	DistanceDeltaM     *float64 `json:"distance_delta_m,omitempty"`
	TimeDeltaSec       *float64 `json:"time_delta_sec,omitempty"`
	CellTowerMismatch  bool     `json:"cell_tower_mismatch"`
	IsImpossibleTravel bool     `json:"is_impossible_travel"`
}

// EvaluateReportSubmission evaluates speed anomalies and cell tower sanity
func EvaluateReportSubmission(db *sql.DB, userID int64, newLat, newLng float64, newTime time.Time, rawDeviceMeta string) (string, FraudCheckResult, error) {
	var result FraudCheckResult
	result.Flags = make([]string, 0)

	var meta models.DeviceMetadata
	if strings.TrimSpace(rawDeviceMeta) != "" {
		if err := json.Unmarshal([]byte(rawDeviceMeta), &meta); err != nil {
			// Non-fatal, create fresh metadata
			meta = models.DeviceMetadata{}
		}
	}

	// 1. Query user's previous report
	var prevLat, prevLng float64
	var prevCreatedAt time.Time
	query := `SELECT lat, lng, created_at FROM reports WHERE user_id = ? ORDER BY created_at DESC LIMIT 1`
	err := db.QueryRow(query, userID).Scan(&prevLat, &prevLng, &prevCreatedAt)

	if err == nil {
		distM := HaversineDistance(prevLat, prevLng, newLat, newLng)
		timeDelta := newTime.Sub(prevCreatedAt).Seconds()
		if timeDelta <= 0 {
			timeDelta = 1 // Prevent division by zero
		}

		speedKmH := (distM / 1000.0) / (timeDelta / 3600.0)
		result.DistanceDeltaM = &distM
		result.TimeDeltaSec = &timeDelta
		result.ImpliedSpeedKmH = &speedKmH
		meta.PrevReportDeltaSec = timeDelta

		if speedKmH > MaxAllowedSpeedKmH {
			result.IsImpossibleTravel = true
			result.Flags = append(result.Flags, "impossible_movement")
		}
	} else if err != sql.ErrNoRows {
		return "", result, fmt.Errorf("error querying previous report for fraud check: %w", err)
	}

	// 2. Cell tower consistency check (e.g. Lake Victoria region vs tower ID)
	if meta.CellTowerID != "" {
		tower := strings.ToUpper(meta.CellTowerID)
		// Lake Victoria basin is roughly Lat [-3.0 to +1.0], Lng [31.5 to 35.5]
		isLakeVictoriaRegion := newLat >= -3.5 && newLat <= 1.5 && newLng >= 31.0 && newLng <= 36.0
		if strings.HasPrefix(tower, "MSA-") && isLakeVictoriaRegion { // Mombasa tower ID in Kisumu region
			result.CellTowerMismatch = true
			result.Flags = append(result.Flags, "location_mismatch")
		} else if strings.HasPrefix(tower, "NRB-") && (newLat < -0.5 || newLng < 33.0) {
			result.CellTowerMismatch = true
			result.Flags = append(result.Flags, "location_mismatch")
		}
	}

	// Append any existing flags from client meta
	if len(meta.Flags) > 0 {
		for _, f := range meta.Flags {
			exists := false
			for _, rf := range result.Flags {
				if rf == f {
					exists = true
					break
				}
			}
			if !exists {
				result.Flags = append(result.Flags, f)
			}
		}
	}
	meta.Flags = result.Flags

	updatedMetaBytes, err := json.Marshal(meta)
	if err != nil {
		return rawDeviceMeta, result, nil
	}

	return string(updatedMetaBytes), result, nil
}
