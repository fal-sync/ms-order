package usecase

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
	"time"

	"boilerplate-skeletoncode/internal/domain"
	"boilerplate-skeletoncode/internal/usecase/port"
)

var (
	ErrInvalidCustomerID         = errors.New("customer_id is required")
	ErrInvalidCompanyID          = errors.New("company_id is required")
	ErrInvalidActivatedBy        = errors.New("activated_by is required")
	ErrInvalidPackageID          = errors.New("package_id is required")
	ErrInvalidPackageName        = errors.New("package.name is required")
	ErrInvalidPackageDuration    = errors.New("package duration must be greater than zero")
	ErrInvalidDurationUnit       = errors.New("package duration_unit must be one of: day, week, month, year")
	ErrInvalidPackagePrice       = errors.New("package price_amount must be greater than zero")
	ErrInvalidDiscountPercent    = errors.New("package discount_percent must be between 0 and 100")
	ErrInvalidSubscriptionTypeID = errors.New("subscription_type_id must be a valid UUID")
	ErrInvalidCurrency           = errors.New("package.currency must be a valid 3-letter code")
	ErrInvalidGatewayCode        = errors.New("gateway_code is required")
	ErrInvalidCustomerEmail      = errors.New("valid customer_email is required")
	ErrInvalidPaymentStatus      = errors.New("payment status is invalid")
	ErrInvalidDurationKey        = errors.New("duration_key must be one of: monthly, quarter, semiannual, annual")
	ErrPaymentGatewayFailed      = errors.New("payment gateway request failed")
)

const (
	subscriptionDurationMonthly    = "monthly"
	subscriptionDurationQuarter    = "quarter"
	subscriptionDurationSemiannual = "semiannual"
	subscriptionDurationAnnual     = "annual"
)

type checkoutDurationOption struct {
	key             string
	months          int
	discountPercent int64
}

var checkoutDurationOptions = map[string]checkoutDurationOption{
	subscriptionDurationMonthly: {
		key:    subscriptionDurationMonthly,
		months: 1,
	},
	subscriptionDurationQuarter: {
		key:             subscriptionDurationQuarter,
		months:          3,
		discountPercent: 2,
	},
	subscriptionDurationSemiannual: {
		key:             subscriptionDurationSemiannual,
		months:          6,
		discountPercent: 5,
	},
	subscriptionDurationAnnual: {
		key:             subscriptionDurationAnnual,
		months:          12,
		discountPercent: 20,
	},
}

type OrderServiceOptions struct {
	InvoiceDueDuration         time.Duration
	SuccessURLTemplate         string
	FailureURLTemplate         string
	NotificationURL            string
	SubscriptionTypeRepository domain.SubscriptionTypeRepository
}

type PackageInput struct {
	ID                 string
	SubscriptionTypeID string
	Name               string
	Description        string
	Features           []string
	DurationCount      int
	DurationUnit       string
	PriceAmount        int64
	Currency           string
	DiscountPercent    int64
	Active             bool
}

type PackageListInput struct {
	SubscriptionTypeID string
	IncludeInactive    bool
}

type CheckoutInput struct {
	CustomerID      string
	CompanyID       string
	PackageID       string
	DurationKey     string
	GatewayCode     string
	PaymentMethod   string
	EnabledPayments []string
	CustomerEmail   string
	Metadata        map[string]string
}

type PaymentNotificationInput struct {
	OrderID    string
	PaymentID  string
	Status     string
	ExternalID string
}

type PaymentStatusInput struct {
	CustomerID string
	OrderID    string
}

type SubscriptionValidationInput struct {
	CompanyID string
}

type CurrentSubscriptionInput struct {
	CompanyID string
}

type ActivateSubscriptionInput struct {
	CompanyID       string
	PackageID       string
	DurationKey     string
	DiscountPercent *int64
	ActivatedBy     string
	StartsAt        time.Time
}

type CurrentSubscriptionResult struct {
	Active       bool                 `json:"active"`
	Subscription *domain.Subscription `json:"subscription,omitempty"`
}

type CustomerOrderHistoryInput struct {
	CustomerID string
}

type SubscriptionValidationResult struct {
	Allowed bool
	Reason  string
}

type OrderService struct {
	packageRepository          domain.PackageRepository
	orderRepository            domain.OrderRepository
	subscriptionRepository     domain.SubscriptionRepository
	subscriptionTypeRepository domain.SubscriptionTypeRepository
	paymentGateway             port.PaymentGateway
	now                        func() time.Time
	generateID                 func(prefix string) string
	generateInvoiceNo          func(now time.Time) string
	invoiceDueDuration         time.Duration
	successURLTemplate         string
	failureURLTemplate         string
	notificationURL            string
}

func NewOrderService(
	packageRepository domain.PackageRepository,
	orderRepository domain.OrderRepository,
	subscriptionRepository domain.SubscriptionRepository,
	paymentGateway port.PaymentGateway,
	options OrderServiceOptions,
) *OrderService {
	if options.InvoiceDueDuration <= 0 {
		options.InvoiceDueDuration = 24 * time.Hour
	}

	return &OrderService{
		packageRepository:          packageRepository,
		orderRepository:            orderRepository,
		subscriptionRepository:     subscriptionRepository,
		subscriptionTypeRepository: options.SubscriptionTypeRepository,
		paymentGateway:             paymentGateway,
		now:                        time.Now,
		generateID:                 generateOrderID,
		generateInvoiceNo:          generateInvoiceNumber,
		invoiceDueDuration:         options.InvoiceDueDuration,
		successURLTemplate:         options.SuccessURLTemplate,
		failureURLTemplate:         options.FailureURLTemplate,
		notificationURL:            options.NotificationURL,
	}
}

func (s *OrderService) CreatePackage(ctx context.Context, input PackageInput) (domain.Package, error) {
	pkg, err := normalizePackage(input)
	if err != nil {
		return domain.Package{}, err
	}
	if _, err := s.ensureSubscriptionType(ctx, pkg.SubscriptionTypeID); err != nil {
		return domain.Package{}, err
	}

	now := s.now().UTC()
	pkg.CreatedAt = now
	pkg.UpdatedAt = now

	if err := s.packageRepository.Create(ctx, pkg); err != nil {
		return domain.Package{}, err
	}

	return withPackageDiscount(pkg), nil
}

func (s *OrderService) UpdatePackage(ctx context.Context, input PackageInput) (domain.Package, error) {
	pkg, err := normalizePackage(input)
	if err != nil {
		return domain.Package{}, err
	}
	if _, err := s.ensureSubscriptionType(ctx, pkg.SubscriptionTypeID); err != nil {
		return domain.Package{}, err
	}

	existing, err := s.packageRepository.GetByID(ctx, pkg.ID)
	if err != nil {
		return domain.Package{}, err
	}

	pkg.CreatedAt = existing.CreatedAt
	pkg.UpdatedAt = s.now().UTC()

	if err := s.packageRepository.Update(ctx, pkg); err != nil {
		return domain.Package{}, err
	}

	return withPackageDiscount(pkg), nil
}

func (s *OrderService) DeletePackage(ctx context.Context, id string) error {
	id = strings.TrimSpace(id)
	if id == "" {
		return domain.ErrPackageNotFound
	}

	return s.packageRepository.Delete(ctx, id)
}

func (s *OrderService) ListSubscriptionTypes(ctx context.Context) ([]domain.SubscriptionType, error) {
	if s.subscriptionTypeRepository == nil {
		return defaultSubscriptionTypes(), nil
	}

	return s.subscriptionTypeRepository.ListActive(ctx)
}

func (s *OrderService) ListPackages(ctx context.Context, input PackageListInput) ([]domain.Package, error) {
	subscriptionTypeID, err := normalizeSubscriptionTypeID(input.SubscriptionTypeID)
	if err != nil {
		return nil, err
	}
	if _, err := s.ensureSubscriptionType(ctx, subscriptionTypeID); err != nil {
		return nil, err
	}

	packages, err := s.packageRepository.List(ctx, domain.PackageFilter{
		SubscriptionTypeID: subscriptionTypeID,
		IncludeInactive:    input.IncludeInactive,
	})
	if err != nil {
		return nil, err
	}
	for i := range packages {
		packages[i] = withPackageDiscount(packages[i])
	}
	return packages, nil
}

func (s *OrderService) ListActivePackages(ctx context.Context) ([]domain.Package, error) {
	return s.ListPackages(ctx, PackageListInput{
		SubscriptionTypeID: domain.SubscriptionTypePackageID,
	})
}

func (s *OrderService) ensureSubscriptionType(ctx context.Context, id string) (domain.SubscriptionType, error) {
	id, err := normalizeSubscriptionTypeID(id)
	if err != nil {
		return domain.SubscriptionType{}, err
	}

	if s.subscriptionTypeRepository != nil {
		return s.subscriptionTypeRepository.GetByID(ctx, id)
	}

	for _, subscriptionType := range defaultSubscriptionTypes() {
		if subscriptionType.ID == id {
			return subscriptionType, nil
		}
	}

	return domain.SubscriptionType{}, domain.ErrSubscriptionTypeNotFound
}

func (s *OrderService) Checkout(ctx context.Context, input CheckoutInput) (domain.Order, error) {
	customerID := strings.TrimSpace(input.CustomerID)
	if customerID == "" {
		return domain.Order{}, ErrInvalidCustomerID
	}
	companyID := strings.TrimSpace(input.CompanyID)
	if companyID == "" {
		return domain.Order{}, ErrInvalidCompanyID
	}

	packageID := strings.TrimSpace(input.PackageID)
	if packageID == "" {
		return domain.Order{}, ErrInvalidPackageID
	}

	gatewayCode := strings.ToLower(strings.TrimSpace(input.GatewayCode))
	if gatewayCode == "" {
		return domain.Order{}, ErrInvalidGatewayCode
	}
	paymentMethod := strings.ToLower(strings.TrimSpace(input.PaymentMethod))
	enabledPayments := normalizeStringList(input.EnabledPayments)

	customerEmail := strings.TrimSpace(input.CustomerEmail)
	if customerEmail == "" || !strings.Contains(customerEmail, "@") {
		return domain.Order{}, ErrInvalidCustomerEmail
	}

	pkg, err := s.packageRepository.GetByID(ctx, packageID)
	if err != nil {
		return domain.Order{}, err
	}
	if !pkg.Active {
		return domain.Order{}, domain.ErrPackageNotFound
	}
	packageSubscriptionTypeID, err := normalizeSubscriptionTypeID(pkg.SubscriptionTypeID)
	if err != nil {
		return domain.Order{}, err
	}
	if packageSubscriptionTypeID != domain.SubscriptionTypePackageID {
		return domain.Order{}, domain.ErrPackageNotFound
	}
	pkg.SubscriptionTypeID = packageSubscriptionTypeID

	issuedAt := s.now().UTC()
	packageSnapshot := packageSnapshot(pkg)
	var checkoutMetadata map[string]string
	if strings.TrimSpace(input.DurationKey) != "" {
		durationOption, err := parseCheckoutDurationOption(input.DurationKey)
		if err != nil {
			return domain.Order{}, err
		}
		packageSnapshot, checkoutMetadata, err = checkoutPackageSnapshot(pkg, durationOption)
		if err != nil {
			return domain.Order{}, err
		}
	}
	subscription, err := s.subscriptionPeriodForCompany(ctx, companyID, packageSnapshot, issuedAt)
	if err != nil {
		return domain.Order{}, err
	}

	orderID := s.generateID("ord")
	invoice := s.buildInvoice(orderID, packageSnapshot, issuedAt)
	order := domain.Order{
		ID:           orderID,
		CustomerID:   customerID,
		CompanyID:    companyID,
		Package:      packageSnapshot,
		Subscription: subscription,
		Invoice:      invoice,
		Payment: domain.PaymentInfo{
			GatewayCode:   gatewayCode,
			PaymentMethod: paymentMethod,
			Status:        domain.PaymentStatusPending,
			CreatedAt:     issuedAt,
			UpdatedAt:     issuedAt,
		},
		Status:    domain.OrderStatusPendingPayment,
		Metadata:  mergeMetadata(cloneMetadata(input.Metadata), checkoutMetadata),
		CreatedAt: issuedAt,
		UpdatedAt: issuedAt,
	}

	if err := s.orderRepository.Create(ctx, order); err != nil {
		return domain.Order{}, err
	}

	if s.paymentGateway == nil {
		order = s.markPaymentFailed(ctx, order, issuedAt)
		return order, ErrPaymentGatewayFailed
	}

	payment, err := s.paymentGateway.CreatePayment(ctx, port.CreatePaymentInput{
		ReferenceID:     order.ID,
		GatewayCode:     gatewayCode,
		PaymentMethod:   paymentMethod,
		EnabledPayments: enabledPayments,
		Amount:          packageSnapshot.PriceAmount,
		Currency:        packageSnapshot.Currency,
		CustomerID:      customerID,
		CustomerEmail:   customerEmail,
		SuccessURL:      renderURLTemplate(s.successURLTemplate, order),
		FailureURL:      renderURLTemplate(s.failureURLTemplate, order),
		NotificationURL: s.notificationURL,
	})
	if err != nil {
		order = s.markPaymentFailed(ctx, order, s.now().UTC())
		return order, fmt.Errorf("%w: %v", ErrPaymentGatewayFailed, err)
	}

	paymentStatus := parsePaymentStatusOrPending(payment.Status)
	order.Payment = domain.PaymentInfo{
		PaymentID:     payment.PaymentID,
		GatewayCode:   payment.GatewayCode,
		PaymentMethod: firstNonEmpty(payment.PaymentMethod, paymentMethod),
		Details:       cloneMetadata(payment.Details),
		Status:        paymentStatus,
		CheckoutURL:   payment.CheckoutURL,
		ExternalID:    payment.ExternalID,
		CreatedAt:     payment.CreatedAt,
		UpdatedAt:     payment.UpdatedAt,
	}
	order.UpdatedAt = s.now().UTC()
	if paymentStatus != domain.PaymentStatusPending {
		return s.applyPaymentUpdate(ctx, order, payment.PaymentID, payment.ExternalID, paymentStatus, payment.Details)
	}

	if err := s.orderRepository.Update(ctx, order); err != nil {
		return domain.Order{}, err
	}

	return order, nil
}

func (s *OrderService) NotifyPayment(ctx context.Context, input PaymentNotificationInput) (domain.Order, error) {
	status, err := parsePaymentStatus(input.Status)
	if err != nil {
		return domain.Order{}, err
	}

	order, err := s.orderByNotification(ctx, input)
	if err != nil {
		return domain.Order{}, err
	}

	return s.applyPaymentUpdate(ctx, order, input.PaymentID, input.ExternalID, status, nil)
}

func (s *OrderService) CheckPayment(ctx context.Context, input PaymentStatusInput) (domain.Order, error) {
	customerID := strings.TrimSpace(input.CustomerID)
	if customerID == "" {
		return domain.Order{}, ErrInvalidCustomerID
	}

	orderID := strings.TrimSpace(input.OrderID)
	if orderID == "" {
		return domain.Order{}, domain.ErrOrderNotFound
	}

	order, err := s.orderRepository.GetByID(ctx, orderID)
	if err != nil {
		return domain.Order{}, err
	}
	if order.CustomerID != customerID {
		return domain.Order{}, domain.ErrOrderNotFound
	}

	if s.paymentGateway == nil || strings.TrimSpace(order.Payment.PaymentID) == "" || isTerminalOrderStatus(order.Status) {
		return order, nil
	}

	payment, err := s.paymentGateway.SyncPayment(ctx, port.SyncPaymentInput{
		PaymentID: order.Payment.PaymentID,
	})
	if err != nil {
		return domain.Order{}, fmt.Errorf("%w: %v", ErrPaymentGatewayFailed, err)
	}

	status := parsePaymentStatusOrPending(payment.Status)
	if !paymentUpdateChanged(order, payment, status) {
		return order, nil
	}

	return s.applyPaymentUpdate(ctx, order, payment.PaymentID, payment.ExternalID, status, payment.Details)
}

func (s *OrderService) applyPaymentUpdate(ctx context.Context, order domain.Order, paymentID string, externalID string, status domain.PaymentStatus, details map[string]string) (domain.Order, error) {
	alreadyPaid := order.Status == domain.OrderStatusPaid
	now := s.now().UTC()

	if strings.TrimSpace(paymentID) != "" {
		order.Payment.PaymentID = strings.TrimSpace(paymentID)
	}
	if strings.TrimSpace(externalID) != "" {
		order.Payment.ExternalID = strings.TrimSpace(externalID)
	}
	if len(details) > 0 {
		order.Payment.Details = mergeMetadata(cloneMetadata(order.Payment.Details), details)
	}
	order.Payment.Status = status
	order.Payment.UpdatedAt = now
	order.UpdatedAt = now

	switch status {
	case domain.PaymentStatusPaid:
		order.Status = domain.OrderStatusPaid
		if !alreadyPaid {
			companyID := order.CompanyID
			if companyID == "" {
				companyID = order.CustomerID
			}
			period, err := s.subscriptionPeriodForCompany(ctx, companyID, order.Package, now)
			if err != nil {
				return domain.Order{}, err
			}
			order.Subscription = period
			subscription := domain.Subscription{
				ID:               s.generateID("sub"),
				CompanyID:        companyID,
				CustomerID:       order.CustomerID,
				OrderID:          order.ID,
				ActivatedBy:      order.CustomerID,
				ActivationSource: "payment",
				Package:          order.Package,
				Period:           period,
				Active:           true,
				CreatedAt:        now,
				UpdatedAt:        now,
			}
			if err := s.subscriptionRepository.UpsertByCompanyID(ctx, subscription); err != nil {
				return domain.Order{}, err
			}
		}
	case domain.PaymentStatusFailed:
		order.Status = domain.OrderStatusPaymentFailed
	case domain.PaymentStatusExpired:
		order.Status = domain.OrderStatusExpired
	case domain.PaymentStatusPending:
		order.Status = domain.OrderStatusPendingPayment
	}

	if err := s.orderRepository.Update(ctx, order); err != nil {
		return domain.Order{}, err
	}

	return order, nil
}

func isTerminalOrderStatus(status domain.OrderStatus) bool {
	switch status {
	case domain.OrderStatusPaid, domain.OrderStatusPaymentFailed, domain.OrderStatusCancelled, domain.OrderStatusExpired:
		return true
	default:
		return false
	}
}

func paymentUpdateChanged(order domain.Order, payment port.SyncPaymentResult, status domain.PaymentStatus) bool {
	if order.Payment.Status != status {
		return true
	}
	if strings.TrimSpace(payment.PaymentID) != "" && strings.TrimSpace(payment.PaymentID) != order.Payment.PaymentID {
		return true
	}
	if strings.TrimSpace(payment.ExternalID) != "" && strings.TrimSpace(payment.ExternalID) != order.Payment.ExternalID {
		return true
	}

	for key, value := range payment.Details {
		key = strings.TrimSpace(key)
		value = strings.TrimSpace(value)
		if key != "" && value != "" && order.Payment.Details[key] != value {
			return true
		}
	}

	return false
}

func (s *OrderService) ValidateSubscription(ctx context.Context, input SubscriptionValidationInput) (SubscriptionValidationResult, error) {
	companyID := strings.TrimSpace(input.CompanyID)
	if companyID == "" {
		return SubscriptionValidationResult{}, ErrInvalidCompanyID
	}

	_, err := s.subscriptionRepository.GetActiveByCompanyID(ctx, companyID, s.now().UTC())
	if err != nil {
		if errors.Is(err, domain.ErrSubscriptionNotFound) {
			return SubscriptionValidationResult{
				Allowed: false,
				Reason:  "subscription not active or expired",
			}, nil
		}
		return SubscriptionValidationResult{}, err
	}

	return SubscriptionValidationResult{Allowed: true}, nil
}

func (s *OrderService) GetCurrentSubscription(ctx context.Context, input CurrentSubscriptionInput) (CurrentSubscriptionResult, error) {
	companyID := strings.TrimSpace(input.CompanyID)
	if companyID == "" {
		return CurrentSubscriptionResult{}, ErrInvalidCompanyID
	}

	subscription, err := s.subscriptionRepository.GetActiveByCompanyID(ctx, companyID, s.now().UTC())
	if err != nil {
		if errors.Is(err, domain.ErrSubscriptionNotFound) {
			return CurrentSubscriptionResult{Active: false}, nil
		}

		return CurrentSubscriptionResult{}, err
	}

	return CurrentSubscriptionResult{
		Active:       true,
		Subscription: &subscription,
	}, nil
}

func (s *OrderService) ActivateSubscription(ctx context.Context, input ActivateSubscriptionInput) (domain.Subscription, error) {
	companyID := strings.TrimSpace(input.CompanyID)
	if companyID == "" {
		return domain.Subscription{}, ErrInvalidCompanyID
	}
	activatedBy := strings.TrimSpace(input.ActivatedBy)
	if activatedBy == "" {
		return domain.Subscription{}, ErrInvalidActivatedBy
	}
	packageID := strings.TrimSpace(input.PackageID)
	if packageID == "" {
		return domain.Subscription{}, ErrInvalidPackageID
	}

	pkg, err := s.packageRepository.GetByID(ctx, packageID)
	if err != nil || !pkg.Active {
		if err != nil {
			return domain.Subscription{}, err
		}
		return domain.Subscription{}, domain.ErrPackageNotFound
	}
	if pkg.SubscriptionTypeID != "" && pkg.SubscriptionTypeID != domain.SubscriptionTypePackageID {
		return domain.Subscription{}, domain.ErrPackageNotFound
	}

	packageSnapshot := packageSnapshot(pkg)
	if strings.TrimSpace(input.DurationKey) != "" {
		durationOption, err := parseCheckoutDurationOption(input.DurationKey)
		if err != nil {
			return domain.Subscription{}, err
		}
		packageSnapshot, _, err = checkoutPackageSnapshot(pkg, durationOption)
		if err != nil {
			return domain.Subscription{}, err
		}
	} else if pkg.DiscountPercent > 0 {
		discountAmount := roundDivide(packageSnapshot.PriceAmount*pkg.DiscountPercent, 100)
		packageSnapshot.PriceAmount = packageSnapshot.PriceAmount - discountAmount
	}

	if input.DiscountPercent != nil {
		discount := *input.DiscountPercent
		if discount < 0 {
			discount = 0
		}
		if discount > 100 {
			discount = 100
		}
		discountAmount := roundDivide(packageSnapshot.PriceAmount*discount, 100)
		packageSnapshot.PriceAmount = packageSnapshot.PriceAmount - discountAmount
	}

	startsAt := input.StartsAt.UTC()
	if input.StartsAt.IsZero() {
		startsAt = s.now().UTC()
	}
	period, err := s.subscriptionPeriodForCompany(ctx, companyID, packageSnapshot, startsAt)
	if err != nil {
		return domain.Subscription{}, err
	}
	now := s.now().UTC()
	subscription := domain.Subscription{
		ID:               s.generateID("sub"),
		CompanyID:        companyID,
		ActivatedBy:      activatedBy,
		ActivationSource: "superadmin",
		Package:          packageSnapshot,
		Period:           period,
		Active:           true,
		CreatedAt:        now,
		UpdatedAt:        now,
	}
	if err := s.subscriptionRepository.UpsertByCompanyID(ctx, subscription); err != nil {
		return domain.Subscription{}, err
	}
	return subscription, nil
}

func (s *OrderService) ListCustomerOrders(ctx context.Context, input CustomerOrderHistoryInput) ([]domain.Order, error) {
	customerID := strings.TrimSpace(input.CustomerID)
	if customerID == "" {
		return nil, ErrInvalidCustomerID
	}

	return s.orderRepository.ListByCustomerID(ctx, customerID)
}

// GetOrder fetches an order by its unique ID.
func (s *OrderService) GetOrder(ctx context.Context, id string) (domain.Order, error) {
	id = strings.TrimSpace(id)
	if id == "" {
		return domain.Order{}, domain.ErrOrderNotFound
	}

	return s.orderRepository.GetByID(ctx, id)
}

// GetInvoice retrieves an invoice by its invoice ID.
func (s *OrderService) GetInvoice(ctx context.Context, invoiceID string) (domain.Invoice, error) {
	invoiceID = strings.TrimSpace(invoiceID)
	if invoiceID == "" {
		return domain.Invoice{}, domain.ErrOrderNotFound
	}

	order, err := s.orderRepository.GetByInvoiceID(ctx, invoiceID)
	if err != nil {
		return domain.Invoice{}, err
	}

	return order.Invoice, nil
}

// CalculateSubscriptionPeriod computes the start and end dates of a subscription given a duration count and unit.
func CalculateSubscriptionPeriod(startsAt time.Time, durationCount int, durationUnit domain.SubscriptionDurationUnit) (domain.SubscriptionPeriod, error) {
	if durationCount <= 0 {
		return domain.SubscriptionPeriod{}, ErrInvalidPackageDuration
	}

	var endsAt time.Time
	switch durationUnit {
	case domain.SubscriptionDurationDay:
		endsAt = startsAt.AddDate(0, 0, durationCount)
	case domain.SubscriptionDurationWeek:
		endsAt = startsAt.AddDate(0, 0, durationCount*7)
	case domain.SubscriptionDurationMonth:
		endsAt = addCalendarMonths(startsAt, durationCount)
	case domain.SubscriptionDurationYear:
		endsAt = addCalendarMonths(startsAt, durationCount*12)
	default:
		return domain.SubscriptionPeriod{}, ErrInvalidDurationUnit
	}

	return domain.SubscriptionPeriod{
		StartsAt:      startsAt,
		EndsAt:        endsAt,
		DurationCount: durationCount,
		DurationUnit:  durationUnit,
	}, nil
}

func (s *OrderService) subscriptionPeriodForCompany(ctx context.Context, companyID string, packageSnapshot domain.PackageSnapshot, startsAt time.Time) (domain.SubscriptionPeriod, error) {
	activeSubscription, err := s.subscriptionRepository.GetActiveByCompanyID(ctx, companyID, startsAt)
	if err == nil && activeSubscription.Period.EndsAt.After(startsAt) {
		startsAt = activeSubscription.Period.EndsAt
	} else if err != nil && !errors.Is(err, domain.ErrSubscriptionNotFound) {
		return domain.SubscriptionPeriod{}, err
	}

	return CalculateSubscriptionPeriod(startsAt, packageSnapshot.DurationCount, packageSnapshot.DurationUnit)
}

func (s *OrderService) buildInvoice(orderID string, packageSnapshot domain.PackageSnapshot, issuedAt time.Time) domain.Invoice {
	item := domain.InvoiceItem{
		Description: fmt.Sprintf("%s subscription (%d %s)", packageSnapshot.Name, packageSnapshot.DurationCount, packageSnapshot.DurationUnit),
		Quantity:    1,
		UnitAmount:  packageSnapshot.PriceAmount,
		TotalAmount: packageSnapshot.PriceAmount,
	}

	return domain.Invoice{
		ID:             s.generateID("inv"),
		Number:         s.generateInvoiceNo(issuedAt),
		OrderID:        orderID,
		IssuedAt:       issuedAt,
		DueAt:          issuedAt.Add(s.invoiceDueDuration),
		Currency:       packageSnapshot.Currency,
		SubtotalAmount: item.TotalAmount,
		TotalAmount:    item.TotalAmount,
		Items:          []domain.InvoiceItem{item},
	}
}

func (s *OrderService) markPaymentFailed(ctx context.Context, order domain.Order, updatedAt time.Time) domain.Order {
	order.Status = domain.OrderStatusPaymentFailed
	order.Payment.Status = domain.PaymentStatusFailed
	order.Payment.UpdatedAt = updatedAt
	order.UpdatedAt = updatedAt
	_ = s.orderRepository.Update(ctx, order)
	return order
}

func (s *OrderService) orderByNotification(ctx context.Context, input PaymentNotificationInput) (domain.Order, error) {
	orderID := strings.TrimSpace(input.OrderID)
	if orderID != "" {
		return s.GetOrder(ctx, orderID)
	}

	paymentID := strings.TrimSpace(input.PaymentID)
	if paymentID != "" {
		return s.orderRepository.GetByPaymentID(ctx, paymentID)
	}

	return domain.Order{}, domain.ErrOrderNotFound
}

func normalizePackage(input PackageInput) (domain.Package, error) {
	packageID := strings.TrimSpace(input.ID)
	if packageID == "" {
		return domain.Package{}, ErrInvalidPackageID
	}

	subscriptionTypeID, err := normalizeSubscriptionTypeID(input.SubscriptionTypeID)
	if err != nil {
		return domain.Package{}, err
	}

	packageName := strings.TrimSpace(input.Name)
	if packageName == "" {
		return domain.Package{}, ErrInvalidPackageName
	}

	durationUnit, err := parseDurationUnit(input.DurationUnit)
	if err != nil {
		return domain.Package{}, err
	}

	if input.DurationCount <= 0 {
		return domain.Package{}, ErrInvalidPackageDuration
	}

	if input.PriceAmount <= 0 {
		return domain.Package{}, ErrInvalidPackagePrice
	}

	currency := strings.ToUpper(strings.TrimSpace(input.Currency))
	if len(currency) != 3 {
		return domain.Package{}, ErrInvalidCurrency
	}

	if input.DiscountPercent < 0 || input.DiscountPercent > 100 {
		return domain.Package{}, ErrInvalidDiscountPercent
	}

	return domain.Package{
		ID:                 packageID,
		SubscriptionTypeID: subscriptionTypeID,
		Name:               packageName,
		Description:        strings.TrimSpace(input.Description),
		Features:           normalizePackageFeatures(input.Features),
		DurationCount:      input.DurationCount,
		DurationUnit:       durationUnit,
		PriceAmount:        input.PriceAmount,
		DiscountPercent:    input.DiscountPercent,
		Currency:           currency,
		Active:             input.Active,
	}, nil
}

func packageSnapshot(pkg domain.Package) domain.PackageSnapshot {
	return domain.PackageSnapshot{
		ID:                 pkg.ID,
		SubscriptionTypeID: pkg.SubscriptionTypeID,
		Name:               pkg.Name,
		Description:        pkg.Description,
		Features:           cloneStrings(pkg.Features),
		DurationCount:      pkg.DurationCount,
		DurationUnit:       pkg.DurationUnit,
		PriceAmount:        pkg.PriceAmount,
		Currency:           pkg.Currency,
	}
}

func checkoutPackageSnapshot(pkg domain.Package, option checkoutDurationOption) (domain.PackageSnapshot, map[string]string, error) {
	snapshot := packageSnapshot(pkg)
	priceBeforeDiscount, discountAmount, finalPrice, err := checkoutPrice(pkg, option)
	if err != nil {
		return domain.PackageSnapshot{}, nil, err
	}

	snapshot.DurationCount, snapshot.DurationUnit = checkoutDuration(option.months)
	snapshot.PriceAmount = finalPrice

	metadata := map[string]string{
		"package_variant_id":           fmt.Sprintf("%s:%s", pkg.ID, option.key),
		"base_package_id":              pkg.ID,
		"package_name":                 pkg.Name,
		"subscription_duration_key":    option.key,
		"subscription_duration_label":  checkoutDurationLabel(option.months),
		"subscription_duration_months": fmt.Sprintf("%d", option.months),
		"subscription_duration_count":  fmt.Sprintf("%d", snapshot.DurationCount),
		"subscription_duration_unit":   string(snapshot.DurationUnit),
		"discount_percent":             fmt.Sprintf("%d", option.discountPercent),
		"discount_amount":              fmt.Sprintf("%d", discountAmount),
		"base_price_amount":            fmt.Sprintf("%d", pkg.PriceAmount),
		"price_before_discount_amount": fmt.Sprintf("%d", priceBeforeDiscount),
		"final_price_amount":           fmt.Sprintf("%d", finalPrice),
		"currency":                     snapshot.Currency,
	}

	return snapshot, metadata, nil
}

func checkoutPrice(pkg domain.Package, option checkoutDurationOption) (int64, int64, int64, error) {
	baseMonths, err := packageDurationMonths(pkg.DurationCount, pkg.DurationUnit)
	if err != nil {
		return 0, 0, 0, err
	}
	if baseMonths <= 0 {
		return 0, 0, 0, ErrInvalidPackageDuration
	}

	priceBeforeDiscount := roundDivide(pkg.PriceAmount*int64(option.months), int64(baseMonths))
	packageDiscount := roundDivide(priceBeforeDiscount*pkg.DiscountPercent, 100)
	priceAfterPackageDiscount := priceBeforeDiscount - packageDiscount
	durationDiscount := roundDivide(priceAfterPackageDiscount*option.discountPercent, 100)
	discountAmount := packageDiscount + durationDiscount
	finalPrice := priceAfterPackageDiscount - durationDiscount
	if finalPrice <= 0 {
		return 0, 0, 0, ErrInvalidPackagePrice
	}

	return priceBeforeDiscount, discountAmount, finalPrice, nil
}

func withPackageDiscount(pkg domain.Package) domain.Package {
	pkg.DiscountAmount = roundDivide(pkg.PriceAmount*pkg.DiscountPercent, 100)
	pkg.FinalPriceAmount = pkg.PriceAmount - pkg.DiscountAmount
	if pkg.FinalPriceAmount < 0 {
		pkg.FinalPriceAmount = 0
	}
	return pkg
}

func packageDurationMonths(durationCount int, durationUnit domain.SubscriptionDurationUnit) (int, error) {
	if durationCount <= 0 {
		return 0, ErrInvalidPackageDuration
	}

	switch durationUnit {
	case domain.SubscriptionDurationMonth:
		return durationCount, nil
	case domain.SubscriptionDurationYear:
		return durationCount * 12, nil
	default:
		return 0, ErrInvalidDurationUnit
	}
}

func checkoutDuration(months int) (int, domain.SubscriptionDurationUnit) {
	if months == 12 {
		return 1, domain.SubscriptionDurationYear
	}

	return months, domain.SubscriptionDurationMonth
}

func checkoutDurationLabel(months int) string {
	if months == 12 {
		return "1 year"
	}
	if months == 1 {
		return "1 month"
	}

	return fmt.Sprintf("%d months", months)
}

func normalizeSubscriptionTypeID(value string) (string, error) {
	subscriptionTypeID := strings.ToLower(strings.TrimSpace(value))
	if subscriptionTypeID == "" {
		return domain.SubscriptionTypePackageID, nil
	}

	if !isUUID(subscriptionTypeID) {
		return "", ErrInvalidSubscriptionTypeID
	}

	return subscriptionTypeID, nil
}

func isUUID(value string) bool {
	if len(value) != 36 {
		return false
	}

	for index, char := range value {
		switch index {
		case 8, 13, 18, 23:
			if char != '-' {
				return false
			}
		default:
			if !isHex(char) {
				return false
			}
		}
	}

	return true
}

func isHex(char rune) bool {
	return (char >= '0' && char <= '9') ||
		(char >= 'a' && char <= 'f') ||
		(char >= 'A' && char <= 'F')
}

func defaultSubscriptionTypes() []domain.SubscriptionType {
	now := time.Date(2026, time.January, 1, 0, 0, 0, 0, time.UTC)

	return []domain.SubscriptionType{
		{
			ID:        domain.SubscriptionTypePackageID,
			Name:      "Subscription Package",
			CreatedAt: now,
			UpdatedAt: now,
		},
		{
			ID:        domain.SubscriptionTypeAdditionalFeaturesID,
			Name:      "Additional Features",
			CreatedAt: now,
			UpdatedAt: now,
		},
		{
			ID:        domain.SubscriptionTypeESealDocumentID,
			Name:      "E-Seal Document",
			CreatedAt: now,
			UpdatedAt: now,
		},
	}
}

func roundDivide(value, divisor int64) int64 {
	if divisor <= 0 {
		return 0
	}

	result := value / divisor
	remainder := value % divisor
	if remainder*2 >= divisor {
		result++
	}

	return result
}

func parseCheckoutDurationOption(value string) (checkoutDurationOption, error) {
	key := strings.ToLower(strings.TrimSpace(value))
	option, ok := checkoutDurationOptions[key]
	if !ok {
		return checkoutDurationOption{}, ErrInvalidDurationKey
	}

	return option, nil
}

func parseDurationUnit(value string) (domain.SubscriptionDurationUnit, error) {
	unit := domain.SubscriptionDurationUnit(strings.ToLower(strings.TrimSpace(value)))
	switch unit {
	case domain.SubscriptionDurationDay, domain.SubscriptionDurationWeek, domain.SubscriptionDurationMonth, domain.SubscriptionDurationYear:
		return unit, nil
	default:
		return "", ErrInvalidDurationUnit
	}
}

func parsePaymentStatus(value string) (domain.PaymentStatus, error) {
	status := domain.PaymentStatus(strings.ToLower(strings.TrimSpace(value)))
	switch status {
	case domain.PaymentStatusPending, domain.PaymentStatusPaid, domain.PaymentStatusFailed, domain.PaymentStatusExpired:
		return status, nil
	default:
		return "", ErrInvalidPaymentStatus
	}
}

func parsePaymentStatusOrPending(value string) domain.PaymentStatus {
	status, err := parsePaymentStatus(value)
	if err != nil {
		return domain.PaymentStatusPending
	}
	return status
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}

func renderURLTemplate(template string, order domain.Order) string {
	value := strings.ReplaceAll(template, "{order_id}", order.ID)
	value = strings.ReplaceAll(value, "{invoice_id}", order.Invoice.ID)
	return value
}

func addCalendarMonths(value time.Time, months int) time.Time {
	year, month, day := value.Date()
	hour, minute, second := value.Clock()
	nanosecond := value.Nanosecond()
	location := value.Location()

	firstOfTargetMonth := time.Date(year, month+time.Month(months), 1, hour, minute, second, nanosecond, location)
	lastDayOfTargetMonth := firstOfTargetMonth.AddDate(0, 1, -1).Day()
	if day > lastDayOfTargetMonth {
		day = lastDayOfTargetMonth
	}

	return time.Date(firstOfTargetMonth.Year(), firstOfTargetMonth.Month(), day, hour, minute, second, nanosecond, location)
}

func cloneMetadata(metadata map[string]string) map[string]string {
	if len(metadata) == 0 {
		return nil
	}

	cloned := make(map[string]string, len(metadata))
	for key, value := range metadata {
		key = strings.TrimSpace(key)
		if key != "" {
			cloned[key] = value
		}
	}

	return cloned
}

func mergeMetadata(base map[string]string, overrides map[string]string) map[string]string {
	if len(overrides) == 0 {
		return base
	}
	if base == nil {
		base = make(map[string]string, len(overrides))
	}

	for key, value := range overrides {
		key = strings.TrimSpace(key)
		if key != "" {
			base[key] = value
		}
	}

	return base
}

func normalizePackageFeatures(features []string) []string {
	if len(features) == 0 {
		return nil
	}

	normalized := make([]string, 0, len(features))
	for _, feature := range features {
		feature = strings.TrimSpace(feature)
		if feature != "" {
			normalized = append(normalized, feature)
		}
	}
	if len(normalized) == 0 {
		return nil
	}

	return normalized
}

func normalizeStringList(values []string) []string {
	if len(values) == 0 {
		return nil
	}

	normalized := make([]string, 0, len(values))
	for _, value := range values {
		value = strings.ToLower(strings.TrimSpace(value))
		if value != "" {
			normalized = append(normalized, value)
		}
	}
	if len(normalized) == 0 {
		return nil
	}

	return normalized
}

func cloneStrings(values []string) []string {
	if len(values) == 0 {
		return nil
	}

	cloned := make([]string, len(values))
	copy(cloned, values)
	return cloned
}

func generateOrderID(prefix string) string {
	buffer := make([]byte, 8)
	if _, err := rand.Read(buffer); err == nil {
		return prefix + "_" + hex.EncodeToString(buffer)
	}

	return prefix + "_fallback"
}

func generateInvoiceNumber(now time.Time) string {
	buffer := make([]byte, 3)
	suffix := "fallback"
	if _, err := rand.Read(buffer); err == nil {
		suffix = strings.ToUpper(hex.EncodeToString(buffer))
	}

	return fmt.Sprintf("INV-%s-%s", now.UTC().Format("20060102"), suffix)
}
