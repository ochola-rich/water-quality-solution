package lightning

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"
	"time"
)

// Client handles communication with an LNbits instance for Lightning payments
type Client struct {
	BaseURL    string
	AdminKey   string
	InvoiceKey string
	HTTPClient *http.Client
	MockMode   bool
}

// NewClient creates a new LNbits API client
func NewClient(baseURL, adminKey, invoiceKey string) *Client {
	baseURL = strings.TrimSuffix(baseURL, "/")
	mockMode := baseURL == "" || adminKey == ""

	if mockMode {
		log.Println("[Lightning] LNbits credentials not configured; running in mock Lightning mode.")
	} else {
		log.Printf("[Lightning] Connected to LNbits instance at %s", baseURL)
	}

	return &Client{
		BaseURL:    baseURL,
		AdminKey:   adminKey,
		InvoiceKey: invoiceKey,
		HTTPClient: &http.Client{Timeout: 10 * time.Second},
		MockMode:   mockMode,
	}
}

// CreateInvoiceRequest payload for LNbits POST /api/v1/payments
type CreateInvoiceRequest struct {
	Out          bool   `json:"out"`
	Amount       int64  `json:"amount"` // in sats
	Memo         string `json:"memo"`
	Unit         string `json:"unit,omitempty"`
	Webhook      string `json:"webhook,omitempty"`
	InternalWire bool   `json:"internal,omitempty"`
}

// CreateInvoiceResponse response from LNbits POST /api/v1/payments
type CreateInvoiceResponse struct {
	PaymentHash    string `json:"payment_hash"`
	PaymentRequest string `json:"payment_request"`
	CheckingID     string `json:"checking_id"`
	LNURLResponse  string `json:"lnurl_response,omitempty"`
}

// PaymentStatus response for checking invoice status
type PaymentStatus struct {
	Paid     bool   `json:"paid"`
	Preimage string `json:"preimage,omitempty"`
	Details  struct {
		Amount int64  `json:"amount"`
		Memo   string `json:"memo"`
		Time   int64  `json:"time"`
	} `json:"details,omitempty"`
}

// WalletDetails represents LNbits wallet info
type WalletDetails struct {
	ID      string `json:"id"`
	Name    string `json:"name"`
	Balance int64  `json:"balance"` // in msats (divide by 1000 for sats)
}

// CreateInvoice generates a Lightning invoice for a given satoshi amount
func (c *Client) CreateInvoice(amountSats int64, memo string) (*CreateInvoiceResponse, error) {
	if c.MockMode {
		// Mock invoice generation
		mockHash := fmt.Sprintf("mock_hash_%d_%d", time.Now().UnixNano(), amountSats)
		mockInvoice := fmt.Sprintf("lnbc%04dp1mockguardianslake_%s", amountSats, mockHash[:16])
		return &CreateInvoiceResponse{
			PaymentHash:    mockHash,
			PaymentRequest: mockInvoice,
			CheckingID:     mockHash,
		}, nil
	}

	url := fmt.Sprintf("%s/api/v1/payments", c.BaseURL)
	reqBody := CreateInvoiceRequest{
		Out:    false,
		Amount: amountSats,
		Memo:   memo,
	}

	payloadBytes, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal invoice request: %w", err)
	}

	req, err := http.NewRequest("POST", url, bytes.NewBuffer(payloadBytes))
	if err != nil {
		return nil, fmt.Errorf("failed to create http request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Api-Key", c.InvoiceKey)

	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("LNbits request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("LNbits error status %d: %s", resp.StatusCode, string(body))
	}

	var invResp CreateInvoiceResponse
	if err := json.NewDecoder(resp.Body).Decode(&invResp); err != nil {
		return nil, fmt.Errorf("failed to decode LNbits response: %w", err)
	}

	return &invResp, nil
}

// PayInvoice pays a BOLT11 Lightning invoice from the platform wallet
func (c *Client) PayInvoice(bolt11Invoice string) (*PaymentStatus, error) {
	if c.MockMode {
		return &PaymentStatus{
			Paid:     true,
			Preimage: fmt.Sprintf("mock_preimage_%d", time.Now().UnixNano()),
		}, nil
	}

	url := fmt.Sprintf("%s/api/v1/payments", c.BaseURL)
	payload := map[string]interface{}{
		"out":    true,
		"bolt11": bolt11Invoice,
	}

	payloadBytes, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal payment payload: %w", err)
	}

	req, err := http.NewRequest("POST", url, bytes.NewBuffer(payloadBytes))
	if err != nil {
		return nil, fmt.Errorf("failed to create payment request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Api-Key", c.AdminKey)

	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("LNbits payment request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("LNbits payment failed (status %d): %s", resp.StatusCode, string(body))
	}

	return &PaymentStatus{Paid: true}, nil
}

// CheckPayment verifies if an invoice has been settled
func (c *Client) CheckPayment(paymentHash string) (*PaymentStatus, error) {
	if c.MockMode {
		return &PaymentStatus{Paid: true}, nil
	}

	url := fmt.Sprintf("%s/api/v1/payments/%s", c.BaseURL, paymentHash)
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create status request: %w", err)
	}

	req.Header.Set("X-Api-Key", c.InvoiceKey)
	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("LNbits check status failed: %w", err)
	}
	defer resp.Body.Close()

	var status PaymentStatus
	if err := json.NewDecoder(resp.Body).Decode(&status); err != nil {
		return nil, fmt.Errorf("failed to parse payment status: %w", err)
	}

	return &status, nil
}
