package domain

import (
	"context"
	"errors"
	"time"
)

var (
	ErrPackageNotFound          = errors.New("package not found")
	ErrPackageAlreadyExists     = errors.New("package already exists")
	ErrOrderNotFound            = errors.New("order not found")
	ErrOrderAlreadyExists       = errors.New("order already exists")
	ErrSubscriptionNotFound     = errors.New("subscription not found")
	ErrSubscriptionTypeNotFound = errors.New("subscription type not found")
)

const (
	SubscriptionTypePackageID            = "8b7d8f9f-3b0a-4c89-a4d4-452ce5d763e1"
	SubscriptionTypeAdditionalFeaturesID = "be4e1df3-8f1e-4817-9815-94e37d4ef894"
	SubscriptionTypeESealDocumentID      = "02e58b8c-6420-4424-88e7-248db7f99079"
)

type OrderStatus string

const (
	OrderStatusPendingPayment OrderStatus = "pending_payment"
	OrderStatusPaid           OrderStatus = "paid"
	OrderStatusPaymentFailed  OrderStatus = "payment_failed"
	OrderStatusCancelled      OrderStatus = "cancelled"
	OrderStatusExpired        OrderStatus = "expired"
)

type PaymentStatus string

const (
	PaymentStatusPending PaymentStatus = "pending"
	PaymentStatusPaid    PaymentStatus = "paid"
	PaymentStatusFailed  PaymentStatus = "failed"
	PaymentStatusExpired PaymentStatus = "expired"
)

type SubscriptionDurationUnit string

const (
	SubscriptionDurationDay   SubscriptionDurationUnit = "day"
	SubscriptionDurationWeek  SubscriptionDurationUnit = "week"
	SubscriptionDurationMonth SubscriptionDurationUnit = "month"
	SubscriptionDurationYear  SubscriptionDurationUnit = "year"
)

type PackageSnapshot struct {
	ID                 string                   `json:"id" bson:"id"`
	SubscriptionTypeID string                   `json:"subscription_type_id" bson:"subscription_type_id"`
	Name               string                   `json:"name" bson:"name"`
	Description        string                   `json:"description,omitempty" bson:"description,omitempty"`
	Features           []string                 `json:"features,omitempty" bson:"features,omitempty"`
	DurationCount      int                      `json:"duration_count" bson:"duration_count"`
	DurationUnit       SubscriptionDurationUnit `json:"duration_unit" bson:"duration_unit"`
	PriceAmount        int64                    `json:"price_amount" bson:"price_amount"`
	Currency           string                   `json:"currency" bson:"currency"`
}

type Package struct {
	ID                 string                   `json:"id" bson:"_id"`
	SubscriptionTypeID string                   `json:"subscription_type_id" bson:"subscription_type_id"`
	Name               string                   `json:"name" bson:"name"`
	Description        string                   `json:"description,omitempty" bson:"description,omitempty"`
	Features           []string                 `json:"features,omitempty" bson:"features,omitempty"`
	DurationCount      int                      `json:"duration_count" bson:"duration_count"`
	DurationUnit       SubscriptionDurationUnit `json:"duration_unit" bson:"duration_unit"`
	PriceAmount        int64                    `json:"price_amount" bson:"price_amount"`
	DiscountPercent    int64                    `json:"discount_percent" bson:"discount_percent"`
	DiscountAmount     int64                    `json:"discount_amount" bson:"-"`
	FinalPriceAmount   int64                    `json:"final_price_amount" bson:"-"`
	Currency           string                   `json:"currency" bson:"currency"`
	Active             bool                     `json:"active" bson:"active"`
	CreatedAt          time.Time                `json:"created_at" bson:"created_at"`
	UpdatedAt          time.Time                `json:"updated_at" bson:"updated_at"`
}

type SubscriptionType struct {
	ID        string     `json:"id"`
	Name      string     `json:"name"`
	CreatedAt time.Time  `json:"created_at"`
	UpdatedAt time.Time  `json:"updated_at"`
	DeletedAt *time.Time `json:"deleted_at,omitempty"`
}

type PackageFilter struct {
	SubscriptionTypeID string
	IncludeInactive    bool
}

type SubscriptionPeriod struct {
	StartsAt      time.Time                `json:"starts_at" bson:"starts_at"`
	EndsAt        time.Time                `json:"ends_at" bson:"ends_at"`
	DurationCount int                      `json:"duration_count" bson:"duration_count"`
	DurationUnit  SubscriptionDurationUnit `json:"duration_unit" bson:"duration_unit"`
}

type InvoiceItem struct {
	Description string `json:"description" bson:"description"`
	Quantity    int    `json:"quantity" bson:"quantity"`
	UnitAmount  int64  `json:"unit_amount" bson:"unit_amount"`
	TotalAmount int64  `json:"total_amount" bson:"total_amount"`
}

type Invoice struct {
	ID             string        `json:"id" bson:"id"`
	Number         string        `json:"number" bson:"number"`
	OrderID        string        `json:"order_id" bson:"order_id"`
	IssuedAt       time.Time     `json:"issued_at" bson:"issued_at"`
	DueAt          time.Time     `json:"due_at" bson:"due_at"`
	Currency       string        `json:"currency" bson:"currency"`
	SubtotalAmount int64         `json:"subtotal_amount" bson:"subtotal_amount"`
	TotalAmount    int64         `json:"total_amount" bson:"total_amount"`
	Items          []InvoiceItem `json:"items" bson:"items"`
}

type PaymentInfo struct {
	PaymentID     string            `json:"payment_id,omitempty" bson:"payment_id,omitempty"`
	GatewayCode   string            `json:"gateway_code,omitempty" bson:"gateway_code,omitempty"`
	PaymentMethod string            `json:"payment_method,omitempty" bson:"payment_method,omitempty"`
	Details       map[string]string `json:"details,omitempty" bson:"details,omitempty"`
	Status        PaymentStatus     `json:"status,omitempty" bson:"status,omitempty"`
	CheckoutURL   string            `json:"checkout_url,omitempty" bson:"checkout_url,omitempty"`
	ExternalID    string            `json:"external_id,omitempty" bson:"external_id,omitempty"`
	CreatedAt     time.Time         `json:"created_at,omitempty" bson:"created_at,omitempty"`
	UpdatedAt     time.Time         `json:"updated_at,omitempty" bson:"updated_at,omitempty"`
}

type Order struct {
	ID           string             `json:"id" bson:"_id"`
	CustomerID   string             `json:"customer_id" bson:"customer_id"`
	CompanyID    string             `json:"company_id" bson:"company_id"`
	Package      PackageSnapshot    `json:"package" bson:"package"`
	Subscription SubscriptionPeriod `json:"subscription" bson:"subscription"`
	Invoice      Invoice            `json:"invoice" bson:"invoice"`
	Payment      PaymentInfo        `json:"payment,omitempty" bson:"payment,omitempty"`
	Status       OrderStatus        `json:"status" bson:"status"`
	Metadata     map[string]string  `json:"metadata,omitempty" bson:"metadata,omitempty"`
	CreatedAt    time.Time          `json:"created_at" bson:"created_at"`
	UpdatedAt    time.Time          `json:"updated_at" bson:"updated_at"`
}

type Subscription struct {
	ID               string             `json:"id" bson:"_id"`
	CompanyID        string             `json:"company_id" bson:"company_id"`
	CustomerID       string             `json:"customer_id,omitempty" bson:"customer_id,omitempty"`
	OrderID          string             `json:"order_id,omitempty" bson:"order_id,omitempty"`
	ActivatedBy      string             `json:"activated_by" bson:"activated_by"`
	ActivationSource string             `json:"activation_source" bson:"activation_source"`
	Package          PackageSnapshot    `json:"package" bson:"package"`
	Period           SubscriptionPeriod `json:"period" bson:"period"`
	Active           bool               `json:"active" bson:"active"`
	CreatedAt        time.Time          `json:"created_at" bson:"created_at"`
	UpdatedAt        time.Time          `json:"updated_at" bson:"updated_at"`
}

type PackageRepository interface {
	Create(ctx context.Context, pkg Package) error
	Update(ctx context.Context, pkg Package) error
	Delete(ctx context.Context, id string) error
	GetByID(ctx context.Context, id string) (Package, error)
	List(ctx context.Context, filter PackageFilter) ([]Package, error)
	ListActive(ctx context.Context) ([]Package, error)
}

type SubscriptionTypeRepository interface {
	GetByID(ctx context.Context, id string) (SubscriptionType, error)
	ListActive(ctx context.Context) ([]SubscriptionType, error)
}

type OrderRepository interface {
	Create(ctx context.Context, order Order) error
	Update(ctx context.Context, order Order) error
	GetByID(ctx context.Context, id string) (Order, error)
	GetByInvoiceID(ctx context.Context, invoiceID string) (Order, error)
	GetByPaymentID(ctx context.Context, paymentID string) (Order, error)
	ListByCustomerID(ctx context.Context, customerID string) ([]Order, error)
}

type SubscriptionRepository interface {
	GetActiveByCompanyID(ctx context.Context, companyID string, at time.Time) (Subscription, error)
	UpsertByCompanyID(ctx context.Context, subscription Subscription) error
}
