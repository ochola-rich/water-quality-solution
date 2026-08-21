package lightning

import (
	"testing"
)

func TestMockLNbitsClient(t *testing.T) {
	client := NewClient("", "", "")
	if !client.MockMode {
		t.Errorf("expected mock mode to be enabled when empty credentials provided")
	}

	inv, err := client.CreateInvoice(50, "Test Water Bounty")
	if err != nil {
		t.Fatalf("CreateInvoice in mock mode failed: %v", err)
	}
	if inv.PaymentRequest == "" || inv.PaymentHash == "" {
		t.Errorf("expected non-empty payment request and hash")
	}

	payResp, err := client.PayInvoice(inv.PaymentRequest)
	if err != nil {
		t.Fatalf("PayInvoice in mock mode failed: %v", err)
	}
	if !payResp.Paid {
		t.Errorf("expected payment to succeed in mock mode")
	}

	status, err := client.CheckPayment(inv.PaymentHash)
	if err != nil {
		t.Fatalf("CheckPayment in mock mode failed: %v", err)
	}
	if !status.Paid {
		t.Errorf("expected check payment status to be paid in mock mode")
	}
}
