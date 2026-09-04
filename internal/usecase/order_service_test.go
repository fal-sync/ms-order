package usecase

import (
	"context"
	"errors"
	"testing"
	"time"

	"boilerplate-skeletoncode/internal/domain"
	"boilerplate-skeletoncode/internal/infrastructure/persistence/memory"
	"boilerplate-skeletoncode/internal/usecase/port"
)

func TestCheckoutUsesPackageRepositoryAndPaymentGateway(t *testing.T) {
	service, packages, _, _ := newTestOrderService(t, fakePaymentGateway{
		result: port.CreatePaymentResult{
			PaymentID:   "pay_123",
			GatewayCode: "xendit",
			Amount:      150000,
			Currency:    "IDR",
			Status:      "pending",
			CheckoutURL: "https://checkout.test/pay_123",
			ExternalID:  "xendit-pay_123",
			CreatedAt:   time.Date(2026, time.May, 15, 8, 31, 0, 0, time.UTC),
			UpdatedAt:   time.Date(2026, time.May, 15, 8, 31, 0, 0, time.UTC),
		},
	})

	_, err := service.CreatePackage(context.Background(), PackageInput{
		ID:            "pkg_monthly",
		Name:          "Monthly Package",
		DurationCount: 1,
		DurationUnit:  "month",
		PriceAmount:   150000,
		Currency:      "idr",
		Active:        true,
	})
	if err != nil {
		t.Fatalf("CreatePackage returned error: %v", err)
	}

	order, err := service.Checkout(context.Background(), CheckoutInput{
		CustomerID:    "cust_123",
		CompanyID:     "company_123",
		PackageID:     "pkg_monthly",
		GatewayCode:   "xendit",
		CustomerEmail: "naufal@example.com",
		Metadata: map[string]string{
			"source": "ms-gateway",
		},
	})
	if err != nil {
		t.Fatalf("Checkout returned error: %v", err)
	}

	if order.ID != "ord_fixed" {
		t.Fatalf("expected order ID to be ord_fixed, got %q", order.ID)
	}
	if order.Status != domain.OrderStatusPendingPayment {
		t.Fatalf("expected status pending_payment, got %q", order.Status)
	}
	if order.Invoice.TotalAmount != 150000 {
		t.Fatalf("expected invoice total from package repository, got %d", order.Invoice.TotalAmount)
	}
	if order.Payment.PaymentID != "pay_123" {
		t.Fatalf("expected payment ID pay_123, got %q", order.Payment.PaymentID)
	}
	if order.Payment.CheckoutURL == "" {
		t.Fatal("expected checkout URL")
	}

	pkg, err := packages.GetByID(context.Background(), "pkg_monthly")
	if err != nil {
		t.Fatalf("GetByID returned error: %v", err)
	}
	if pkg.Currency != "IDR" {
		t.Fatalf("expected package currency normalized to IDR, got %q", pkg.Currency)
	}
}

func TestCheckoutAppliesDurationVariantAtBackend(t *testing.T) {
	var gatewayInput port.CreatePaymentInput
	service, _, _, _ := newTestOrderService(t, fakePaymentGateway{
		received: &gatewayInput,
		result: port.CreatePaymentResult{
			PaymentID:   "pay_123",
			GatewayCode: "midtrans",
			Amount:      849300,
			Currency:    "IDR",
			Status:      "pending",
			CheckoutURL: "https://checkout.test/pay_123",
			ExternalID:  "snap-token-123",
			CreatedAt:   time.Date(2026, time.May, 15, 8, 31, 0, 0, time.UTC),
			UpdatedAt:   time.Date(2026, time.May, 15, 8, 31, 0, 0, time.UTC),
		},
	})

	_, err := service.CreatePackage(context.Background(), PackageInput{
		ID:            "pkg_pro",
		Name:          "Pro",
		DurationCount: 1,
		DurationUnit:  "month",
		PriceAmount:   149000,
		Currency:      "IDR",
		Active:        true,
	})
	if err != nil {
		t.Fatalf("CreatePackage returned error: %v", err)
	}

	order, err := service.Checkout(context.Background(), CheckoutInput{
		CustomerID:      "cust_123",
		CompanyID:       "company_123",
		PackageID:       "pkg_pro",
		DurationKey:     "semiannual",
		GatewayCode:     "midtrans",
		PaymentMethod:   "bca_va",
		EnabledPayments: []string{"bca_va"},
		CustomerEmail:   "naufal@example.com",
		Metadata: map[string]string{
			"final_price_amount": "1",
		},
	})
	if err != nil {
		t.Fatalf("Checkout returned error: %v", err)
	}

	if order.Package.DurationCount != 6 || order.Package.DurationUnit != domain.SubscriptionDurationMonth {
		t.Fatalf("expected 6 month package snapshot, got %d %s", order.Package.DurationCount, order.Package.DurationUnit)
	}
	if order.Package.PriceAmount != 849300 {
		t.Fatalf("expected package snapshot price 849300, got %d", order.Package.PriceAmount)
	}
	if order.Invoice.TotalAmount != 849300 {
		t.Fatalf("expected invoice total 849300, got %d", order.Invoice.TotalAmount)
	}
	if gatewayInput.Amount != 849300 {
		t.Fatalf("expected gateway amount 849300, got %d", gatewayInput.Amount)
	}
	if gatewayInput.CustomerID != "cust_123" {
		t.Fatalf("expected gateway customer_id cust_123, got %q", gatewayInput.CustomerID)
	}
	if gatewayInput.PaymentMethod != "bca_va" {
		t.Fatalf("expected gateway payment method bca_va, got %q", gatewayInput.PaymentMethod)
	}
	if len(gatewayInput.EnabledPayments) != 1 || gatewayInput.EnabledPayments[0] != "bca_va" {
		t.Fatalf("expected gateway enabled payment bca_va, got %+v", gatewayInput.EnabledPayments)
	}
	if order.Payment.PaymentMethod != "bca_va" {
		t.Fatalf("expected order payment method bca_va, got %q", order.Payment.PaymentMethod)
	}
	if order.Metadata["subscription_duration_key"] != "semiannual" {
		t.Fatalf("expected server duration metadata semiannual, got %q", order.Metadata["subscription_duration_key"])
	}
	if order.Metadata["final_price_amount"] != "849300" {
		t.Fatalf("expected server final price metadata 849300, got %q", order.Metadata["final_price_amount"])
	}
}

func TestCheckoutRejectsInvalidDurationKey(t *testing.T) {
	service, packages, _, _ := newTestOrderService(t, fakePaymentGateway{})
	err := packages.Create(context.Background(), domain.Package{
		ID:            "pkg_pro",
		Name:          "Pro",
		DurationCount: 1,
		DurationUnit:  domain.SubscriptionDurationMonth,
		PriceAmount:   149000,
		Currency:      "IDR",
		Active:        true,
	})
	if err != nil {
		t.Fatalf("Create package returned error: %v", err)
	}

	_, err = service.Checkout(context.Background(), CheckoutInput{
		CustomerID:    "cust_123",
		CompanyID:     "company_123",
		PackageID:     "pkg_pro",
		DurationKey:   "lifetime",
		GatewayCode:   "midtrans",
		CustomerEmail: "naufal@example.com",
	})
	if !errors.Is(err, ErrInvalidDurationKey) {
		t.Fatalf("expected ErrInvalidDurationKey, got %v", err)
	}
}

func TestListPackagesFiltersBySubscriptionTypeID(t *testing.T) {
	service, _, _, _ := newTestOrderService(t, fakePaymentGateway{})

	_, err := service.CreatePackage(context.Background(), PackageInput{
		ID:                 "pkg_starter",
		SubscriptionTypeID: domain.SubscriptionTypePackageID,
		Name:               "Starter",
		DurationCount:      1,
		DurationUnit:       "month",
		PriceAmount:        99000,
		Currency:           "IDR",
		Active:             true,
	})
	if err != nil {
		t.Fatalf("CreatePackage package returned error: %v", err)
	}

	_, err = service.CreatePackage(context.Background(), PackageInput{
		ID:                 "addon_report",
		SubscriptionTypeID: domain.SubscriptionTypeAdditionalFeaturesID,
		Name:               "Report Add-on",
		DurationCount:      1,
		DurationUnit:       "month",
		PriceAmount:        25000,
		Currency:           "IDR",
		Active:             true,
	})
	if err != nil {
		t.Fatalf("CreatePackage add-on returned error: %v", err)
	}

	_, err = service.CreatePackage(context.Background(), PackageInput{
		ID:                 "pkg_inactive",
		SubscriptionTypeID: domain.SubscriptionTypePackageID,
		Name:               "Inactive",
		DurationCount:      1,
		DurationUnit:       "month",
		PriceAmount:        50000,
		Currency:           "IDR",
		Active:             false,
	})
	if err != nil {
		t.Fatalf("CreatePackage inactive returned error: %v", err)
	}

	activePackages, err := service.ListPackages(context.Background(), PackageListInput{
		SubscriptionTypeID: domain.SubscriptionTypePackageID,
	})
	if err != nil {
		t.Fatalf("ListPackages returned error: %v", err)
	}
	if len(activePackages) != 1 || activePackages[0].ID != "pkg_starter" {
		t.Fatalf("expected only active subscription package, got %+v", activePackages)
	}

	additionalFeatures, err := service.ListPackages(context.Background(), PackageListInput{
		SubscriptionTypeID: domain.SubscriptionTypeAdditionalFeaturesID,
	})
	if err != nil {
		t.Fatalf("ListPackages add-on returned error: %v", err)
	}
	if len(additionalFeatures) != 1 || additionalFeatures[0].ID != "addon_report" {
		t.Fatalf("expected only additional feature package, got %+v", additionalFeatures)
	}

	allPackageTypes, err := service.ListPackages(context.Background(), PackageListInput{
		SubscriptionTypeID: domain.SubscriptionTypePackageID,
		IncludeInactive:    true,
	})
	if err != nil {
		t.Fatalf("ListPackages include inactive returned error: %v", err)
	}
	if len(allPackageTypes) != 2 {
		t.Fatalf("expected active and inactive subscription package, got %+v", allPackageTypes)
	}
}

func TestCheckoutRejectsNonSubscriptionPackageType(t *testing.T) {
	service, _, _, _ := newTestOrderService(t, fakePaymentGateway{})

	_, err := service.CreatePackage(context.Background(), PackageInput{
		ID:                 "addon_report",
		SubscriptionTypeID: domain.SubscriptionTypeAdditionalFeaturesID,
		Name:               "Report Add-on",
		DurationCount:      1,
		DurationUnit:       "month",
		PriceAmount:        25000,
		Currency:           "IDR",
		Active:             true,
	})
	if err != nil {
		t.Fatalf("CreatePackage returned error: %v", err)
	}

	_, err = service.Checkout(context.Background(), CheckoutInput{
		CustomerID:    "cust_123",
		CompanyID:     "company_123",
		PackageID:     "addon_report",
		GatewayCode:   "midtrans",
		CustomerEmail: "naufal@example.com",
	})
	if !errors.Is(err, domain.ErrPackageNotFound) {
		t.Fatalf("expected ErrPackageNotFound, got %v", err)
	}
}

func TestCreatePackageRejectsUnknownSubscriptionTypeID(t *testing.T) {
	service, _, _, _ := newTestOrderService(t, fakePaymentGateway{})

	_, err := service.CreatePackage(context.Background(), PackageInput{
		ID:                 "custom_type_package",
		SubscriptionTypeID: "f1eb074f-0ef2-4fda-970d-f8b36d544884",
		Name:               "Custom Type Package",
		DurationCount:      1,
		DurationUnit:       "month",
		PriceAmount:        99000,
		Currency:           "IDR",
		Active:             true,
	})
	if !errors.Is(err, domain.ErrSubscriptionTypeNotFound) {
		t.Fatalf("expected ErrSubscriptionTypeNotFound, got %v", err)
	}
}

func TestCheckoutPaymentGatewayFailureMarksOrderFailed(t *testing.T) {
	service, packages, orders, _ := newTestOrderService(t, fakePaymentGateway{err: errors.New("payment down")})
	err := packages.Create(context.Background(), domain.Package{
		ID:            "pkg_monthly",
		Name:          "Monthly Package",
		DurationCount: 1,
		DurationUnit:  domain.SubscriptionDurationMonth,
		PriceAmount:   150000,
		Currency:      "IDR",
		Active:        true,
	})
	if err != nil {
		t.Fatalf("Create package returned error: %v", err)
	}

	order, err := service.Checkout(context.Background(), CheckoutInput{
		CustomerID:    "cust_123",
		CompanyID:     "company_123",
		PackageID:     "pkg_monthly",
		GatewayCode:   "xendit",
		CustomerEmail: "naufal@example.com",
	})
	if !errors.Is(err, ErrPaymentGatewayFailed) {
		t.Fatalf("expected ErrPaymentGatewayFailed, got %v", err)
	}

	stored, err := orders.GetByID(context.Background(), order.ID)
	if err != nil {
		t.Fatalf("GetByID returned error: %v", err)
	}
	if stored.Status != domain.OrderStatusPaymentFailed {
		t.Fatalf("expected stored order payment_failed, got %q", stored.Status)
	}
}

func TestNotifyPaymentPaidActivatesSubscription(t *testing.T) {
	service, packages, _, subscriptions := newTestOrderService(t, fakePaymentGateway{
		result: port.CreatePaymentResult{
			PaymentID:   "pay_123",
			GatewayCode: "xendit",
			Amount:      150000,
			Currency:    "IDR",
			Status:      "pending",
			CheckoutURL: "https://checkout.test/pay_123",
		},
	})
	_ = packages.Create(context.Background(), domain.Package{
		ID:            "pkg_monthly",
		Name:          "Monthly Package",
		DurationCount: 1,
		DurationUnit:  domain.SubscriptionDurationMonth,
		PriceAmount:   150000,
		Currency:      "IDR",
		Active:        true,
	})

	order, err := service.Checkout(context.Background(), CheckoutInput{
		CustomerID:    "cust_123",
		CompanyID:     "company_123",
		PackageID:     "pkg_monthly",
		GatewayCode:   "xendit",
		CustomerEmail: "naufal@example.com",
	})
	if err != nil {
		t.Fatalf("Checkout returned error: %v", err)
	}

	paidOrder, err := service.NotifyPayment(context.Background(), PaymentNotificationInput{
		OrderID:   order.ID,
		PaymentID: "pay_123",
		Status:    "paid",
	})
	if err != nil {
		t.Fatalf("NotifyPayment returned error: %v", err)
	}
	if paidOrder.Status != domain.OrderStatusPaid {
		t.Fatalf("expected order paid, got %q", paidOrder.Status)
	}

	subscription, err := subscriptions.GetActiveByCompanyID(context.Background(), "company_123", service.now())
	if err != nil {
		t.Fatalf("expected active subscription, got %v", err)
	}
	if subscription.OrderID != order.ID {
		t.Fatalf("expected subscription order %q, got %q", order.ID, subscription.OrderID)
	}
	if subscription.CompanyID != "company_123" || subscription.CustomerID != "cust_123" {
		t.Fatalf("unexpected subscription ownership: %+v", subscription)
	}

	validation, err := service.ValidateSubscription(context.Background(), SubscriptionValidationInput{CompanyID: "company_123"})
	if err != nil {
		t.Fatalf("ValidateSubscription returned error: %v", err)
	}
	if !validation.Allowed {
		t.Fatal("expected subscription validation to allow")
	}

	currentSubscription, err := service.GetCurrentSubscription(context.Background(), CurrentSubscriptionInput{CompanyID: "company_123"})
	if err != nil {
		t.Fatalf("GetCurrentSubscription returned error: %v", err)
	}
	if !currentSubscription.Active || currentSubscription.Subscription == nil {
		t.Fatal("expected current subscription to be active")
	}

	history, err := service.ListCustomerOrders(context.Background(), CustomerOrderHistoryInput{CustomerID: "cust_123"})
	if err != nil {
		t.Fatalf("ListCustomerOrders returned error: %v", err)
	}
	if len(history) != 1 || history[0].ID != order.ID {
		t.Fatalf("unexpected order history: %+v", history)
	}
}

func TestValidateSubscriptionDeniedWhenMissing(t *testing.T) {
	service, _, _, _ := newTestOrderService(t, fakePaymentGateway{})

	validation, err := service.ValidateSubscription(context.Background(), SubscriptionValidationInput{CompanyID: "company_123"})
	if err != nil {
		t.Fatalf("ValidateSubscription returned error: %v", err)
	}
	if validation.Allowed {
		t.Fatal("expected subscription validation to deny")
	}

	currentSubscription, err := service.GetCurrentSubscription(context.Background(), CurrentSubscriptionInput{CompanyID: "company_123"})
	if err != nil {
		t.Fatalf("GetCurrentSubscription returned error: %v", err)
	}
	if currentSubscription.Active || currentSubscription.Subscription != nil {
		t.Fatalf("expected missing current subscription, got %+v", currentSubscription)
	}
}

func TestSuperAdminActivationCreatesCompanySubscriptionWithoutPayment(t *testing.T) {
	service, packages, _, subscriptions := newTestOrderService(t, fakePaymentGateway{})
	if err := packages.Create(context.Background(), domain.Package{
		ID:            "pkg_monthly",
		Name:          "Monthly Package",
		DurationCount: 1,
		DurationUnit:  domain.SubscriptionDurationMonth,
		PriceAmount:   150000,
		Currency:      "IDR",
		Active:        true,
	}); err != nil {
		t.Fatalf("Create package returned error: %v", err)
	}

	subscription, err := service.ActivateSubscription(context.Background(), ActivateSubscriptionInput{
		CompanyID:   "company_123",
		PackageID:   "pkg_monthly",
		ActivatedBy: "super_123",
	})
	if err != nil {
		t.Fatalf("ActivateSubscription returned error: %v", err)
	}
	if subscription.ActivationSource != "superadmin" || subscription.ActivatedBy != "super_123" {
		t.Fatalf("unexpected activation audit: %+v", subscription)
	}
	if _, err := subscriptions.GetActiveByCompanyID(context.Background(), "company_123", service.now()); err != nil {
		t.Fatalf("expected active company subscription: %v", err)
	}
}

func TestCalculateSubscriptionPeriodClampsCalendarMonthEnd(t *testing.T) {
	startsAt := time.Date(2026, time.January, 31, 10, 0, 0, 0, time.UTC)

	period, err := CalculateSubscriptionPeriod(startsAt, 1, domain.SubscriptionDurationMonth)
	if err != nil {
		t.Fatalf("CalculateSubscriptionPeriod returned error: %v", err)
	}

	expectedEndsAt := time.Date(2026, time.February, 28, 10, 0, 0, 0, time.UTC)
	if !period.EndsAt.Equal(expectedEndsAt) {
		t.Fatalf("expected subscription ends_at %s, got %s", expectedEndsAt, period.EndsAt)
	}
}

func TestCheckPaymentSyncsGatewayAndActivatesSubscription(t *testing.T) {
	var syncInput port.SyncPaymentInput
	service, packages, _, subscriptions := newTestOrderService(t, fakePaymentGateway{
		result: port.CreatePaymentResult{
			PaymentID:   "pay_123",
			GatewayCode: "midtrans",
			Status:      "pending",
			CreatedAt:   time.Date(2026, time.May, 15, 8, 31, 0, 0, time.UTC),
			UpdatedAt:   time.Date(2026, time.May, 15, 8, 31, 0, 0, time.UTC),
		},
		syncReceived: &syncInput,
		syncResult: port.SyncPaymentResult{
			PaymentID:   "pay_123",
			GatewayCode: "midtrans",
			Status:      "paid",
			ExternalID:  "midtrans-trx-123",
			Details: map[string]string{
				"transaction_status": "settlement",
			},
			UpdatedAt: time.Date(2026, time.May, 15, 8, 32, 0, 0, time.UTC),
		},
	})
	err := packages.Create(context.Background(), domain.Package{
		ID:            "pkg_pro",
		Name:          "Pro",
		DurationCount: 1,
		DurationUnit:  domain.SubscriptionDurationMonth,
		PriceAmount:   149000,
		Currency:      "IDR",
		Active:        true,
	})
	if err != nil {
		t.Fatalf("Create package returned error: %v", err)
	}

	order, err := service.Checkout(context.Background(), CheckoutInput{
		CustomerID:    "cust_123",
		CompanyID:     "company_123",
		PackageID:     "pkg_pro",
		GatewayCode:   "midtrans",
		CustomerEmail: "naufal@example.com",
	})
	if err != nil {
		t.Fatalf("Checkout returned error: %v", err)
	}

	paidOrder, err := service.CheckPayment(context.Background(), PaymentStatusInput{
		CustomerID: "cust_123",
		OrderID:    order.ID,
	})
	if err != nil {
		t.Fatalf("CheckPayment returned error: %v", err)
	}
	if syncInput.PaymentID != "pay_123" {
		t.Fatalf("expected sync payment_id pay_123, got %q", syncInput.PaymentID)
	}
	if paidOrder.Status != domain.OrderStatusPaid || paidOrder.Payment.Status != domain.PaymentStatusPaid {
		t.Fatalf("expected paid order, got order=%q payment=%q", paidOrder.Status, paidOrder.Payment.Status)
	}
	if paidOrder.Payment.ExternalID != "midtrans-trx-123" {
		t.Fatalf("expected external id, got %q", paidOrder.Payment.ExternalID)
	}
	if paidOrder.Payment.Details["transaction_status"] != "settlement" {
		t.Fatalf("expected payment details from sync, got %+v", paidOrder.Payment.Details)
	}
	if _, err := subscriptions.GetActiveByCompanyID(context.Background(), "company_123", service.now()); err != nil {
		t.Fatalf("expected active subscription: %v", err)
	}
}

func newTestOrderService(t *testing.T, paymentGateway fakePaymentGateway) (*OrderService, *memory.PackageRepository, *memory.OrderRepository, *memory.SubscriptionRepository) {
	t.Helper()

	packageRepository := memory.NewPackageRepository()
	orderRepository := memory.NewOrderRepository()
	subscriptionRepository := memory.NewSubscriptionRepository()
	service := NewOrderService(packageRepository, orderRepository, subscriptionRepository, paymentGateway, OrderServiceOptions{
		InvoiceDueDuration: 48 * time.Hour,
		SuccessURLTemplate: "https://app.test/success?order_id={order_id}",
		FailureURLTemplate: "https://app.test/failed?order_id={order_id}",
		NotificationURL:    "https://order.test/internal/payments/notifications",
	})
	now := time.Date(2026, time.May, 15, 8, 30, 0, 0, time.UTC)
	service.now = func() time.Time { return now }
	service.generateID = func(prefix string) string { return prefix + "_fixed" }
	service.generateInvoiceNo = func(time.Time) string { return "INV-20260515-FIXED" }

	return service, packageRepository, orderRepository, subscriptionRepository
}

type fakePaymentGateway struct {
	received     *port.CreatePaymentInput
	syncReceived *port.SyncPaymentInput
	result       port.CreatePaymentResult
	syncResult   port.SyncPaymentResult
	err          error
	syncErr      error
}

func (f fakePaymentGateway) CreatePayment(ctx context.Context, input port.CreatePaymentInput) (port.CreatePaymentResult, error) {
	if f.received != nil {
		*f.received = input
	}
	return f.result, f.err
}

func (f fakePaymentGateway) SyncPayment(ctx context.Context, input port.SyncPaymentInput) (port.SyncPaymentResult, error) {
	if f.syncReceived != nil {
		*f.syncReceived = input
	}
	if f.syncErr != nil {
		return port.SyncPaymentResult{}, f.syncErr
	}
	return f.syncResult, nil
}
