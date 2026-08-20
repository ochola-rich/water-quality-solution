package ai

import (
	"testing"

	"github.com/guardians-of-the-lake/backend/internal/models"
)

func TestAIService_AssessWaterQuality(t *testing.T) {
	service := NewService()

	// 1. Test Chemical Spill assessment
	spillReq := AssessmentRequest{
		Category:    "spill",
		Description: "Oily diesel sheen with dead fish near the boat dock",
		Lat:         -0.1022,
		Lng:         34.7617,
	}
	res1 := service.AssessWaterQuality(spillReq)
	if res1.Severity != "critical" {
		t.Errorf("expected severity 'critical' for spill, got %s", res1.Severity)
	}
	if res1.WaterQualityIndex > 30 {
		t.Errorf("expected low WQI (<30) for toxic spill, got %d", res1.WaterQualityIndex)
	}
	if res1.ConfidenceScore < 0.85 {
		t.Errorf("expected high confidence score for spill, got %.2f", res1.ConfidenceScore)
	}

	// 2. Test Algae Bloom assessment
	algaeReq := AssessmentRequest{
		Category:    "algae",
		Description: "Thick green slime covering the bay surface",
		Lat:         -0.1444,
		Lng:         34.7391,
		ColorHex:    "#00FF66",
	}
	res2 := service.AssessWaterQuality(algaeReq)
	if res2.CategoryEstimate != models.CategoryAlgae {
		t.Errorf("expected category 'algae', got %s", res2.CategoryEstimate)
	}
	if res2.Severity != "warning" {
		t.Errorf("expected severity 'warning', got %s", res2.Severity)
	}
}
