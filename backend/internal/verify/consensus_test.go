package verify

import (
	"testing"

	"github.com/guardians-of-the-lake/backend/internal/models"
)

type mockRewardProcessor struct {
	processedReports []int64
}

func (m *mockRewardProcessor) ProcessVerifiedReport(reportID int64, authorID int64) (*models.Reward, error) {
	m.processedReports = append(m.processedReports, reportID)
	return &models.Reward{
		ReportID:   reportID,
		UserID:     authorID,
		AmountSats: 50,
		Status:     models.RewardPaid,
	}, nil
}

func TestConsensusEngine_SubmitVote_Flow(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	// Seed users:
	// User 1: Report Author (Reputation 1.0)
	// User 2: Verifier 1 (Reputation 2.0)
	// User 3: Verifier 2 (Reputation 1.5)
	_, _ = db.Exec(`INSERT INTO users (id, phone_hash, display_name, reputation_score) VALUES (1, 'u1', 'Author', 1.0)`)
	_, _ = db.Exec(`INSERT INTO users (id, phone_hash, display_name, reputation_score) VALUES (2, 'u2', 'Verifier1', 2.0)`)
	_, _ = db.Exec(`INSERT INTO users (id, phone_hash, display_name, reputation_score) VALUES (3, 'u3', 'Verifier2', 1.5)`)

	// Create pending report at (-0.1022, 34.7617)
	_, _ = db.Exec(`
		INSERT INTO reports (id, user_id, lat, lng, category, description, status)
		VALUES (10, 1, -0.1022, 34.7617, 'turbidity', 'Muddy water at pier', 'pending')
	`)

	mockRewards := &mockRewardProcessor{}
	engine := NewConsensusEngine(db, nil, mockRewards)
	engine.Threshold = 3.0 // Requires 3.0 total weighted confirm points

	// 1. Author attempts self-verification -> Prohibited
	_, err := engine.SubmitVote(10, 1, models.VoteConfirm, -0.1022, 34.7617)
	if err == nil {
		t.Fatalf("expected error on self-verification attempt, got nil")
	}

	// 2. Verifier 1 votes confirm from 200m away (valid radius)
	// Kisumu point roughly ~200m away
	v1Lat, v1Lng := -0.1030, 34.7620
	res1, err := engine.SubmitVote(10, 2, models.VoteConfirm, v1Lat, v1Lng)
	if err != nil {
		t.Fatalf("unexpected error on verifier 1 vote: %v", err)
	}
	if res1.NewStatus != models.StatusPending {
		t.Errorf("expected report to remain pending (score 2.0 < 3.0), got %s", res1.NewStatus)
	}
	if res1.TotalConfirmWeight != 2.0 {
		t.Errorf("expected total confirm weight 2.0, got %.2f", res1.TotalConfirmWeight)
	}

	// 3. Verifier 1 attempts duplicate vote -> Prohibited
	_, err = engine.SubmitVote(10, 2, models.VoteConfirm, v1Lat, v1Lng)
	if err == nil {
		t.Fatalf("expected error on duplicate vote, got nil")
	}

	// 4. Verifier 2 votes confirm from 10km away -> Prohibited by radius check (> 500m)
	v2FarLat, v2FarLng := -0.2000, 34.8500
	_, err = engine.SubmitVote(10, 3, models.VoteConfirm, v2FarLat, v2FarLng)
	if err == nil {
		t.Fatalf("expected error on out-of-radius vote, got nil")
	}

	// 5. Verifier 2 votes confirm from 50m away -> Reaches 3.5 total confirm weight (>= 3.0 threshold) -> Verified!
	v2CloseLat, v2CloseLng := -0.1023, 34.7618
	res2, err := engine.SubmitVote(10, 3, models.VoteConfirm, v2CloseLat, v2CloseLng)
	if err != nil {
		t.Fatalf("unexpected error on verifier 2 vote: %v", err)
	}

	if res2.NewStatus != models.StatusVerified {
		t.Errorf("expected report to transition to verified (score 3.5 >= 3.0), got %s", res2.NewStatus)
	}
	if !res2.LedgerEntryCreated {
		t.Errorf("expected cryptographic ledger entry to be created upon reaching consensus")
	}

	// Verify database report status was updated
	var currentStatus string
	_ = db.QueryRow(`SELECT status FROM reports WHERE id = 10`).Scan(&currentStatus)
	if currentStatus != string(models.StatusVerified) {
		t.Errorf("expected db report status 'verified', got '%s'", currentStatus)
	}
}
