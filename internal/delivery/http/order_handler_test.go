package httpdelivery

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"boilerplate-skeletoncode/internal/domain"
	"boilerplate-skeletoncode/internal/usecase"
)

func TestRouterListsPublicPackages(t *testing.T) {
	orderUsecase := &stubOrderUsecase{
		packages: []domain.Package{{
			ID:                 "pkg_monthly",
			SubscriptionTypeID: domain.SubscriptionTypePackageID,
			Name:               "Monthly Package",
			DurationCount:      1,
			DurationUnit:       domain.SubscriptionDurationMonth,
			PriceAmount:        150000,
			Currency:           "IDR",
			Active:             true,
		}},
	}
	handler := NewRouter(stubUserUsecase{}, orderUsecase, InternalAccessPolicy{})

	response := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/packages", nil)
	handler.ServeHTTP(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, response.Code)
	}

	var body struct {
		Packages []domain.Package `json:"packages"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &body); err != nil {
		t.Fatalf("invalid response JSON: %v", err)
	}
	if len(body.Packages) != 1 || body.Packages[0].ID != "pkg_monthly" {
		t.Fatalf("unexpected packages response: %+v", body.Packages)
	}
	if orderUsecase.packageListInput.SubscriptionTypeID != "" {
		t.Fatalf("expected default subscription_type_id from usecase, got %q", orderUsecase.packageListInput.SubscriptionTypeID)
	}
	if orderUsecase.packageListInput.IncludeInactive {
		t.Fatal("expected public package list to exclude inactive packages")
	}
}

func TestRouterListsPackagesWithSubscriptionTypeQuery(t *testing.T) {
	orderUsecase := &stubOrderUsecase{
		packages: []domain.Package{{
			ID:                 "addon_report",
			SubscriptionTypeID: domain.SubscriptionTypeAdditionalFeaturesID,
			Name:               "Report Add-on",
			DurationCount:      1,
			DurationUnit:       domain.SubscriptionDurationMonth,
			PriceAmount:        25000,
			Currency:           "IDR",
			Active:             false,
		}},
	}
	handler := NewRouter(stubUserUsecase{}, orderUsecase, InternalAccessPolicy{})

	response := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/packages?subscription_type_id="+domain.SubscriptionTypeAdditionalFeaturesID+"&include_inactive=true", nil)
	handler.ServeHTTP(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, response.Code)
	}
	if orderUsecase.packageListInput.SubscriptionTypeID != domain.SubscriptionTypeAdditionalFeaturesID {
		t.Fatalf("expected additional features type filter, got %q", orderUsecase.packageListInput.SubscriptionTypeID)
	}
	if !orderUsecase.packageListInput.IncludeInactive {
		t.Fatal("expected include_inactive query to be forwarded")
	}
}

func TestRouterCheckoutUsesUserHeader(t *testing.T) {
	now := time.Date(2026, time.May, 15, 8, 30, 0, 0, time.UTC)
	orderUsecase := &stubOrderUsecase{
		order: domain.Order{
			ID:         "ord_123",
			CustomerID: "user_123",
			Payment: domain.PaymentInfo{
				PaymentID:   "pay_123",
				CheckoutURL: "https://checkout.test/pay_123",
			},
			Status:    domain.OrderStatusPendingPayment,
			CreatedAt: now,
			UpdatedAt: now,
		},
	}
	handler := NewRouter(stubUserUsecase{}, orderUsecase, InternalAccessPolicy{})

	response := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/orders/checkout", bytes.NewBufferString(`{
		"package_id": "pkg_monthly",
		"duration_key": "semiannual",
		"gateway_code": "xendit",
		"payment_method": "bca_va",
		"enabled_payments": ["bca_va"],
		"customer_email": "naufal@example.com"
	}`))
	request.Header.Set("X-User-ID", "user_123")
	request.Header.Set("X-Company-ID", "company_123")
	handler.ServeHTTP(response, request)

	if response.Code != http.StatusCreated {
		t.Fatalf("expected status %d, got %d", http.StatusCreated, response.Code)
	}
	if orderUsecase.checkoutInput.CustomerID != "user_123" {
		t.Fatalf("expected customer from X-User-ID, got %q", orderUsecase.checkoutInput.CustomerID)
	}
	if orderUsecase.checkoutInput.CompanyID != "company_123" {
		t.Fatalf("expected company from X-Company-ID, got %q", orderUsecase.checkoutInput.CompanyID)
	}
	if orderUsecase.checkoutInput.PackageID != "pkg_monthly" {
		t.Fatalf("expected package_id pkg_monthly, got %q", orderUsecase.checkoutInput.PackageID)
	}
	if orderUsecase.checkoutInput.DurationKey != "semiannual" {
		t.Fatalf("expected duration_key semiannual, got %q", orderUsecase.checkoutInput.DurationKey)
	}
	if orderUsecase.checkoutInput.PaymentMethod != "bca_va" {
		t.Fatalf("expected payment_method bca_va, got %q", orderUsecase.checkoutInput.PaymentMethod)
	}
	if len(orderUsecase.checkoutInput.EnabledPayments) != 1 || orderUsecase.checkoutInput.EnabledPayments[0] != "bca_va" {
		t.Fatalf("expected enabled_payments bca_va, got %+v", orderUsecase.checkoutInput.EnabledPayments)
	}
}

func TestRouterSubscriptionValidationReturnsPaymentRequired(t *testing.T) {
	orderUsecase := &stubOrderUsecase{
		validation: usecase.SubscriptionValidationResult{
			Allowed: false,
			Reason:  "subscription not active or expired",
		},
	}
	handler := NewRouter(stubUserUsecase{}, orderUsecase, InternalAccessPolicy{AllowedCIDRs: []string{"127.0.0.1/32"}})

	response := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/internal/subscription/validate", bytes.NewBufferString(`{}`))
	request.RemoteAddr = "127.0.0.1:12345"
	request.Header.Set("X-Company-ID", "company_123")
	handler.ServeHTTP(response, request)

	if response.Code != http.StatusPaymentRequired {
		t.Fatalf("expected status %d, got %d", http.StatusPaymentRequired, response.Code)
	}
	if orderUsecase.validationInput.CompanyID != "company_123" {
		t.Fatalf("expected validation company company_123, got %q", orderUsecase.validationInput.CompanyID)
	}
}

func TestRouterSuperAdminActivatesCompanySubscription(t *testing.T) {
	orderUsecase := &stubOrderUsecase{
		subscriptionResult: domain.Subscription{ID: "sub_123", CompanyID: "company_123", Active: true},
	}
	handler := NewRouter(stubUserUsecase{}, orderUsecase, InternalAccessPolicy{})

	response := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/subscriptions/activate", bytes.NewBufferString(`{
		"company_id": "company_123",
		"package_id": "pkg_monthly"
	}`))
	request.Header.Set("X-User-ID", "super_123")
	handler.ServeHTTP(response, request)

	if response.Code != http.StatusCreated {
		t.Fatalf("expected status %d, got %d", http.StatusCreated, response.Code)
	}
	if orderUsecase.activationInput.CompanyID != "company_123" || orderUsecase.activationInput.ActivatedBy != "super_123" {
		t.Fatalf("unexpected activation input: %+v", orderUsecase.activationInput)
	}
}

func TestRouterCheckPaymentUsesCustomerHeader(t *testing.T) {
	now := time.Date(2026, time.May, 15, 8, 30, 0, 0, time.UTC)
	orderUsecase := &stubOrderUsecase{
		order: domain.Order{
			ID:         "ord_123",
			CustomerID: "user_123",
			Payment: domain.PaymentInfo{
				PaymentID: "pay_123",
				Status:    domain.PaymentStatusPaid,
			},
			Status:    domain.OrderStatusPaid,
			CreatedAt: now,
			UpdatedAt: now,
		},
	}
	handler := NewRouter(stubUserUsecase{}, orderUsecase, InternalAccessPolicy{})

	response := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/orders/ord_123/payment/check", nil)
	request.Header.Set("X-User-ID", "user_123")
	handler.ServeHTTP(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, response.Code)
	}
	if orderUsecase.paymentStatusInput.CustomerID != "user_123" {
		t.Fatalf("expected customer user_123, got %q", orderUsecase.paymentStatusInput.CustomerID)
	}
	if orderUsecase.paymentStatusInput.OrderID != "ord_123" {
		t.Fatalf("expected order ord_123, got %q", orderUsecase.paymentStatusInput.OrderID)
	}
}

type stubUserUsecase struct{}

func (stubUserUsecase) CreateUser(context.Context, usecase.CreateUserInput) (domain.User, error) {
	return domain.User{}, nil
}

func (stubUserUsecase) GetUser(context.Context, string) (domain.User, error) {
	return domain.User{}, nil
}

func (stubUserUsecase) ListUsers(context.Context) ([]domain.User, error) {
	return nil, nil
}

type stubOrderUsecase struct {
	packages           []domain.Package
	packageListInput   usecase.PackageListInput
	order              domain.Order
	orders             []domain.Order
	subscription       usecase.CurrentSubscriptionResult
	subscriptionResult domain.Subscription
	validation         usecase.SubscriptionValidationResult
	checkoutInput      usecase.CheckoutInput
	validationInput    usecase.SubscriptionValidationInput
	paymentStatusInput usecase.PaymentStatusInput
	activationInput    usecase.ActivateSubscriptionInput
}

func (s *stubOrderUsecase) CreatePackage(context.Context, usecase.PackageInput) (domain.Package, error) {
	return domain.Package{}, nil
}

func (s *stubOrderUsecase) UpdatePackage(context.Context, usecase.PackageInput) (domain.Package, error) {
	return domain.Package{}, nil
}

func (s *stubOrderUsecase) DeletePackage(context.Context, string) error {
	return nil
}

func (s *stubOrderUsecase) ListSubscriptionTypes(context.Context) ([]domain.SubscriptionType, error) {
	return nil, nil
}

func (s *stubOrderUsecase) ListPackages(_ context.Context, input usecase.PackageListInput) ([]domain.Package, error) {
	s.packageListInput = input
	return s.packages, nil
}

func (s *stubOrderUsecase) ListActivePackages(context.Context) ([]domain.Package, error) {
	return s.packages, nil
}

func (s *stubOrderUsecase) Checkout(_ context.Context, input usecase.CheckoutInput) (domain.Order, error) {
	s.checkoutInput = input
	return s.order, nil
}

func (s *stubOrderUsecase) ActivateSubscription(_ context.Context, input usecase.ActivateSubscriptionInput) (domain.Subscription, error) {
	s.activationInput = input
	return s.subscriptionResult, nil
}

func (s *stubOrderUsecase) NotifyPayment(context.Context, usecase.PaymentNotificationInput) (domain.Order, error) {
	return s.order, nil
}

func (s *stubOrderUsecase) CheckPayment(_ context.Context, input usecase.PaymentStatusInput) (domain.Order, error) {
	s.paymentStatusInput = input
	return s.order, nil
}

func (s *stubOrderUsecase) ValidateSubscription(_ context.Context, input usecase.SubscriptionValidationInput) (usecase.SubscriptionValidationResult, error) {
	s.validationInput = input
	return s.validation, nil
}

func (s *stubOrderUsecase) GetCurrentSubscription(context.Context, usecase.CurrentSubscriptionInput) (usecase.CurrentSubscriptionResult, error) {
	return s.subscription, nil
}

func (s *stubOrderUsecase) ListCustomerOrders(context.Context, usecase.CustomerOrderHistoryInput) ([]domain.Order, error) {
	return s.orders, nil
}

func (s *stubOrderUsecase) GetOrder(context.Context, string) (domain.Order, error) {
	return s.order, nil
}

func (s *stubOrderUsecase) GetInvoice(context.Context, string) (domain.Invoice, error) {
	return domain.Invoice{}, nil
}
