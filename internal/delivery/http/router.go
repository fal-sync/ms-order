// Package httpdelivery provides HTTP handlers, router registration, and middleware.
package httpdelivery

import (
	"context"
	"net/http"

	"boilerplate-skeletoncode/internal/domain"
	"boilerplate-skeletoncode/internal/usecase"
)

// UserUsecase defines the usecase contract required by user delivery handlers.
type UserUsecase interface {
	CreateUser(rctx context.Context, input usecase.CreateUserInput) (domain.User, error)
	GetUser(rctx context.Context, id string) (domain.User, error)
	ListUsers(rctx context.Context) ([]domain.User, error)
}

// OrderUsecase defines the usecase contract required by order and subscription delivery handlers.
type OrderUsecase interface {
	CreatePackage(rctx context.Context, input usecase.PackageInput) (domain.Package, error)
	UpdatePackage(rctx context.Context, input usecase.PackageInput) (domain.Package, error)
	DeletePackage(rctx context.Context, id string) error
	ListSubscriptionTypes(rctx context.Context) ([]domain.SubscriptionType, error)
	ListPackages(rctx context.Context, input usecase.PackageListInput) ([]domain.Package, error)
	ListActivePackages(rctx context.Context) ([]domain.Package, error)
	Checkout(rctx context.Context, input usecase.CheckoutInput) (domain.Order, error)
	ActivateSubscription(rctx context.Context, input usecase.ActivateSubscriptionInput) (domain.Subscription, error)
	NotifyPayment(rctx context.Context, input usecase.PaymentNotificationInput) (domain.Order, error)
	CheckPayment(rctx context.Context, input usecase.PaymentStatusInput) (domain.Order, error)
	ValidateSubscription(rctx context.Context, input usecase.SubscriptionValidationInput) (usecase.SubscriptionValidationResult, error)
	GetCurrentSubscription(rctx context.Context, input usecase.CurrentSubscriptionInput) (usecase.CurrentSubscriptionResult, error)
	ListCustomerOrders(rctx context.Context, input usecase.CustomerOrderHistoryInput) ([]domain.Order, error)
	GetOrder(rctx context.Context, id string) (domain.Order, error)
	GetInvoice(rctx context.Context, invoiceID string) (domain.Invoice, error)
}

// NewRouter registers all HTTP routes (both public and CIDR-restricted internal routes) and returns the root handler.
func NewRouter(userUsecase UserUsecase, orderUsecase OrderUsecase, internalAccessPolicy InternalAccessPolicy) http.Handler {
	userHandler := newUserHandler(userUsecase)
	orderHandler := newOrderHandler(orderUsecase)
	healthHandler := newHealthHandler()

	internalMux := http.NewServeMux()
	internalMux.HandleFunc("POST /internal/users", userHandler.create)
	internalMux.HandleFunc("GET /internal/users", userHandler.list)
	internalMux.HandleFunc("GET /internal/users/{id}", userHandler.getByID)
	internalMux.HandleFunc("POST /internal/packages", orderHandler.createPackage)
	internalMux.HandleFunc("PUT /internal/packages/{id}", orderHandler.updatePackage)
	internalMux.HandleFunc("DELETE /internal/packages/{id}", orderHandler.deletePackage)
	internalMux.HandleFunc("GET /internal/subscription-types", orderHandler.listSubscriptionTypes)
	internalMux.HandleFunc("GET /internal/orders/{id}", orderHandler.getByID)
	internalMux.HandleFunc("GET /internal/invoices/{id}", orderHandler.getInvoice)
	internalMux.HandleFunc("POST /internal/subscription/validate", orderHandler.validateSubscription)
	internalMux.HandleFunc("POST /internal/payments/notifications", orderHandler.notifyPayment)

	mux := http.NewServeMux()
	mux.HandleFunc("GET /health", healthHandler.check)
	mux.HandleFunc("GET /subscription-types", orderHandler.listSubscriptionTypes)
	mux.HandleFunc("GET /packages", orderHandler.listPackages)
	mux.HandleFunc("POST /packages", orderHandler.createPackage)
	mux.HandleFunc("PUT /packages/{id}", orderHandler.updatePackage)
	mux.HandleFunc("DELETE /packages/{id}", orderHandler.deletePackage)
	mux.HandleFunc("POST /orders/checkout", orderHandler.checkout)
	mux.HandleFunc("POST /subscriptions/activate", orderHandler.activateSubscription)
	mux.HandleFunc("GET /orders/{id}", orderHandler.getCustomerOrder)
	mux.HandleFunc("POST /orders/{id}/payment/check", orderHandler.checkPayment)
	mux.HandleFunc("GET /orders/{id}/payment/events", orderHandler.streamPaymentEvents)
	mux.HandleFunc("GET /orders/history", orderHandler.orderHistory)
	mux.HandleFunc("GET /subscriptions/current", orderHandler.currentSubscription)
	mux.Handle("/internal/", newInternalAccessMiddleware(internalMux, internalAccessPolicy))

	return mux
}
