package ai

import (
	"math"
	"strings"
	"time"

	"github.com/guardians-of-the-lake/backend/internal/models"
)

// AssessmentRequest input for automated AI water quality assessment
type AssessmentRequest struct {
	Category     string   `json:"category"`
	Description  string   `json:"description"`
	Lat          float64  `json:"lat"`
	Lng          float64  `json:"lng"`
	TurbidityNTU *float64 `json:"turbidity_ntu,omitempty"`
	ColorHex     string   `json:"color_hex,omitempty"`
}

// Service provides heuristic water quality index (WQI) scoring and classification
type Service struct{}

// NewService creates a new AI prediction service
func NewService() *Service {
	return &Service{}
}

// AssessWaterQuality analyzes anomaly text, category, and optional optical metrics to estimate WQI and severity
func (s *Service) AssessWaterQuality(req AssessmentRequest) models.AIAssessmentResult {
	cat := models.ReportCategory(strings.ToLower(strings.TrimSpace(req.Category)))
	desc := strings.ToLower(req.Description)

	wqi := 75 // Baseline moderate score
	confidence := 0.75
	severity := "normal"
	advisory := "Standard monitoring recommended."
	features := make([]string, 0)

	// 1. Evaluate Category Heuristics
	switch cat {
	case models.CategorySpill:
		wqi -= 50
		confidence = 0.92
		severity = "critical"
		advisory = "CRITICAL: Potential hydrocarbon or chemical hazard. Avoid contact and alert maritime & municipal authorities."
		features = append(features, "hydrocarbon_sheen", "chemical_contaminant", "toxicity_risk")

	case models.CategoryAlgae:
		wqi -= 35
		confidence = 0.88
		severity = "warning"
		advisory = "WARNING: Cyanobacteria bloom detected. Potential microcystin toxins present. Boil water advisory recommended."
		features = append(features, "chlorophyll_a_spike", "eutrophication", "dissolved_oxygen_drop")

	case models.CategoryTurbidity:
		wqi -= 25
		confidence = 0.82
		severity = "warning"
		advisory = "CAUTION: Elevated suspended sediments. Filtration and chlorination required before consumption."
		features = append(features, "suspended_particulates", "runoff_sedimentation", "reduced_secchi_depth")

	case models.CategorySmell:
		wqi -= 30
		confidence = 0.78
		severity = "warning"
		advisory = "CAUTION: Strong organic decomposition or anaerobic hydrogen sulfide decay detected."
		features = append(features, "anaerobic_decay", "organic_decomposition", "high_bod")

	default:
		wqi -= 10
		confidence = 0.65
		advisory = "General anomaly reported. Awaiting peer validation."
		features = append(features, "unclassified_anomaly")
	}

	// 2. Natural Language Keyword NLP heuristics
	criticalKeywords := []string{"oil", "fuel", "dead fish", "chemical", "toxic", "foam", "discharge"}
	for _, kw := range criticalKeywords {
		if strings.Contains(desc, kw) {
			wqi -= 15
			confidence = math.Min(0.98, confidence+0.08)
			severity = "critical"
			features = append(features, "nlp_critical_keyword:"+kw)
			break
		}
	}

	// 3. Optical Turbidity Measurement (if available from sensor / photo analysis)
	if req.TurbidityNTU != nil {
		ntu := *req.TurbidityNTU
		if ntu > 100 { // Normal lake water is < 10 NTU
			wqi -= 20
			features = append(features, "sensor_extreme_turbidity")
		} else if ntu > 25 {
			wqi -= 10
			features = append(features, "sensor_moderate_turbidity")
		}
	}

	// 4. Color Spectrum Heuristics
	if req.ColorHex != "" {
		cHex := strings.ToLower(strings.TrimPrefix(req.ColorHex, "#"))
		if strings.HasPrefix(cHex, "00") || strings.HasPrefix(cHex, "22") { // Dark oily / black
			features = append(features, "spectral_hydrocarbon_dark")
		} else if strings.Contains(cHex, "00ff") || strings.Contains(cHex, "2e8b") { // Bright green bloom
			features = append(features, "spectral_chlorophyll_green")
		} else if strings.Contains(cHex, "8b45") || strings.Contains(cHex, "a052") { // Brown silt
			features = append(features, "spectral_silt_brown")
		}
	}

	// Clamp WQI to [0, 100]
	if wqi < 0 {
		wqi = 0
	} else if wqi > 100 {
		wqi = 100
	}

	if wqi < 40 {
		severity = "critical"
	} else if wqi < 65 && severity != "critical" {
		severity = "warning"
	}

	return models.AIAssessmentResult{
		CategoryEstimate:  cat,
		WaterQualityIndex: wqi,
		ConfidenceScore:   math.Round(confidence*100) / 100,
		Severity:          severity,
		AdvisoryNotice:    advisory,
		DetectedFeatures:  features,
		EvaluatedAt:       time.Now().UTC(),
	}
}
