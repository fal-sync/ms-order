package port

import (
	"context"
	"time"
)

type CreatePaymentInput struct {
	ReferenceID     string
	GatewayCode     string
	PaymentMethod   string
	EnabledPayments []string
	Amount          int64
	Currency        string
	CustomerID      string
	CustomerEmail   string
	SuccessURL      string
	FailureURL      string
	NotificationURL string
}

type CreatePaymentResult struct {
	PaymentID     string
	ReferenceID   string
	GatewayCode   string
	PaymentMethod string
	Details       map[string]string
	Amount        int64
	Currency      string
	CustomerEmail string
	Status        string
	CheckoutURL   string
	ExternalID    string
	CreatedAt     time.Time
	UpdatedAt     time.Time
}

type SyncPaymentInput struct {
	PaymentID string
}

type SyncPaymentResult = CreatePaymentResult

type PaymentGateway interface {
	CreatePayment(ctx context.Context, input CreatePaymentInput) (CreatePaymentResult, error)
	SyncPayment(ctx context.Context, input SyncPaymentInput) (SyncPaymentResult, error)
}
