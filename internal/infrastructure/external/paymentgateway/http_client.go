package paymentgateway

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"boilerplate-skeletoncode/internal/usecase/port"
)

type HTTPClient struct {
	baseURL    string
	httpClient *http.Client
}

type createPaymentRequest struct {
	ReferenceID     string   `json:"reference_id"`
	GatewayCode     string   `json:"gateway_code"`
	PaymentMethod   string   `json:"payment_method,omitempty"`
	EnabledPayments []string `json:"enabled_payments,omitempty"`
	Amount          int64    `json:"amount"`
	Currency        string   `json:"currency"`
	CustomerID      string   `json:"customer_id,omitempty"`
	CustomerEmail   string   `json:"customer_email"`
	SuccessURL      string   `json:"success_url"`
	FailureURL      string   `json:"failure_url"`
	NotificationURL string   `json:"notification_url"`
}

type createPaymentResponse struct {
	PaymentID     string            `json:"payment_id"`
	ReferenceID   string            `json:"reference_id"`
	GatewayCode   string            `json:"gateway_code"`
	PaymentMethod string            `json:"payment_method"`
	Details       map[string]string `json:"details"`
	Amount        int64             `json:"amount"`
	Currency      string            `json:"currency"`
	CustomerID    string            `json:"customer_id"`
	CustomerEmail string            `json:"customer_email"`
	Status        string            `json:"status"`
	CheckoutURL   string            `json:"checkout_url"`
	ExternalID    string            `json:"external_id"`
	CreatedAt     string            `json:"created_at"`
	UpdatedAt     string            `json:"updated_at"`
}

func NewHTTPClient(baseURL string, timeout time.Duration) *HTTPClient {
	if timeout <= 0 {
		timeout = 15 * time.Second
	}

	return &HTTPClient{
		baseURL: strings.TrimRight(baseURL, "/"),
		httpClient: &http.Client{
			Timeout: timeout,
		},
	}
}

func (c *HTTPClient) CreatePayment(ctx context.Context, input port.CreatePaymentInput) (port.CreatePaymentResult, error) {
	payload := createPaymentRequest{
		ReferenceID:     input.ReferenceID,
		GatewayCode:     input.GatewayCode,
		PaymentMethod:   input.PaymentMethod,
		EnabledPayments: input.EnabledPayments,
		Amount:          input.Amount,
		Currency:        input.Currency,
		CustomerID:      input.CustomerID,
		CustomerEmail:   input.CustomerEmail,
		SuccessURL:      input.SuccessURL,
		FailureURL:      input.FailureURL,
		NotificationURL: input.NotificationURL,
	}

	var body bytes.Buffer
	if err := json.NewEncoder(&body).Encode(payload); err != nil {
		return port.CreatePaymentResult{}, fmt.Errorf("encode payment request: %w", err)
	}

	request, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/internal/payments", &body)
	if err != nil {
		return port.CreatePaymentResult{}, fmt.Errorf("create payment request: %w", err)
	}
	request.Header.Set("Accept", "application/json")
	request.Header.Set("Content-Type", "application/json")

	response, err := c.httpClient.Do(request)
	if err != nil {
		return port.CreatePaymentResult{}, fmt.Errorf("perform payment request: %w", err)
	}
	defer response.Body.Close()

	data, err := io.ReadAll(io.LimitReader(response.Body, 1<<20))
	if err != nil {
		return port.CreatePaymentResult{}, fmt.Errorf("read payment response: %w", err)
	}

	if response.StatusCode < http.StatusOK || response.StatusCode >= http.StatusMultipleChoices {
		return port.CreatePaymentResult{}, fmt.Errorf("payment gateway returned status %d: %s", response.StatusCode, strings.TrimSpace(string(data)))
	}

	return decodePaymentResponse(data)
}

func (c *HTTPClient) SyncPayment(ctx context.Context, input port.SyncPaymentInput) (port.SyncPaymentResult, error) {
	paymentID := strings.TrimSpace(input.PaymentID)
	if paymentID == "" {
		return port.SyncPaymentResult{}, fmt.Errorf("payment_id is required")
	}

	request, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/internal/payments/"+url.PathEscape(paymentID)+"/sync", nil)
	if err != nil {
		return port.SyncPaymentResult{}, fmt.Errorf("create sync payment request: %w", err)
	}
	request.Header.Set("Accept", "application/json")

	response, err := c.httpClient.Do(request)
	if err != nil {
		return port.SyncPaymentResult{}, fmt.Errorf("perform sync payment request: %w", err)
	}
	defer response.Body.Close()

	data, err := io.ReadAll(io.LimitReader(response.Body, 1<<20))
	if err != nil {
		return port.SyncPaymentResult{}, fmt.Errorf("read sync payment response: %w", err)
	}

	if response.StatusCode < http.StatusOK || response.StatusCode >= http.StatusMultipleChoices {
		return port.SyncPaymentResult{}, fmt.Errorf("payment gateway returned status %d: %s", response.StatusCode, strings.TrimSpace(string(data)))
	}

	return decodePaymentResponse(data)
}

func decodePaymentResponse(data []byte) (port.CreatePaymentResult, error) {
	var decoded createPaymentResponse
	if err := json.Unmarshal(data, &decoded); err != nil {
		return port.CreatePaymentResult{}, fmt.Errorf("decode payment response: %w", err)
	}

	createdAt, _ := time.Parse(time.RFC3339Nano, decoded.CreatedAt)
	updatedAt, _ := time.Parse(time.RFC3339Nano, decoded.UpdatedAt)

	return port.CreatePaymentResult{
		PaymentID:     decoded.PaymentID,
		ReferenceID:   decoded.ReferenceID,
		GatewayCode:   decoded.GatewayCode,
		PaymentMethod: decoded.PaymentMethod,
		Details:       decoded.Details,
		Amount:        decoded.Amount,
		Currency:      decoded.Currency,
		CustomerEmail: decoded.CustomerEmail,
		Status:        decoded.Status,
		CheckoutURL:   decoded.CheckoutURL,
		ExternalID:    decoded.ExternalID,
		CreatedAt:     createdAt,
		UpdatedAt:     updatedAt,
	}, nil
}
