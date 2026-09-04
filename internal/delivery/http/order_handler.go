package httpdelivery

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"boilerplate-skeletoncode/internal/domain"
	"boilerplate-skeletoncode/internal/usecase"
)

type orderHandler struct {
	orderUsecase OrderUsecase
}

type packageRequest struct {
	ID                 string   `json:"id"`
	SubscriptionTypeID string   `json:"subscription_type_id"`
	Name               string   `json:"name"`
	Description        string   `json:"description,omitempty"`
	Features           []string `json:"features,omitempty"`
	DurationCount      int      `json:"duration_count"`
	DurationUnit       string   `json:"duration_unit"`
	PriceAmount        int64    `json:"price_amount"`
	Currency           string   `json:"currency"`
	Active             *bool    `json:"active,omitempty"`
}

type checkoutRequest struct {
	PackageID       string            `json:"package_id"`
	DurationKey     string            `json:"duration_key"`
	GatewayCode     string            `json:"gateway_code"`
	PaymentMethod   string            `json:"payment_method,omitempty"`
	EnabledPayments []string          `json:"enabled_payments,omitempty"`
	CustomerEmail   string            `json:"customer_email"`
	Metadata        map[string]string `json:"metadata,omitempty"`
}

type subscriptionValidationRequest struct {
	UserID    string `json:"user_id"`
	CompanyID string `json:"company_id"`
}

type activateSubscriptionRequest struct {
	CompanyID   string     `json:"company_id"`
	PackageID   string     `json:"package_id"`
	DurationKey string     `json:"duration_key,omitempty"`
	StartsAt    *time.Time `json:"starts_at,omitempty"`
}

type paymentNotificationRequest struct {
	OrderID    string `json:"order_id"`
	PaymentID  string `json:"payment_id"`
	Status     string `json:"status"`
	ExternalID string `json:"external_id"`
}

type paymentStatusEvent struct {
	OrderID       string               `json:"order_id"`
	Status        domain.OrderStatus   `json:"status"`
	PaymentStatus domain.PaymentStatus `json:"payment_status"`
	Paid          bool                 `json:"paid"`
	Order         domain.Order         `json:"order"`
	UpdatedAt     string               `json:"updated_at"`
}

func newOrderHandler(orderUsecase OrderUsecase) orderHandler {
	return orderHandler{
		orderUsecase: orderUsecase,
	}
}

func (h orderHandler) createPackage(w http.ResponseWriter, r *http.Request) {
	request, ok := decodePackageRequest(w, r)
	if !ok {
		return
	}

	pkg, err := h.orderUsecase.CreatePackage(r.Context(), packageInput(request, ""))
	if err != nil {
		writeOrderError(w, err)
		return
	}

	writeJSON(w, http.StatusCreated, pkg)
}

func (h orderHandler) updatePackage(w http.ResponseWriter, r *http.Request) {
	request, ok := decodePackageRequest(w, r)
	if !ok {
		return
	}

	pkg, err := h.orderUsecase.UpdatePackage(r.Context(), packageInput(request, r.PathValue("id")))
	if err != nil {
		writeOrderError(w, err)
		return
	}

	writeJSON(w, http.StatusOK, pkg)
}

func (h orderHandler) deletePackage(w http.ResponseWriter, r *http.Request) {
	if err := h.orderUsecase.DeletePackage(r.Context(), r.PathValue("id")); err != nil {
		writeOrderError(w, err)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

func (h orderHandler) listSubscriptionTypes(w http.ResponseWriter, r *http.Request) {
	subscriptionTypes, err := h.orderUsecase.ListSubscriptionTypes(r.Context())
	if err != nil {
		writeOrderError(w, err)
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{"subscription_types": subscriptionTypes})
}

func (h orderHandler) listPackages(w http.ResponseWriter, r *http.Request) {
	query := r.URL.Query()
	packages, err := h.orderUsecase.ListPackages(r.Context(), usecase.PackageListInput{
		SubscriptionTypeID: query.Get("subscription_type_id"),
		IncludeInactive:    parseBoolQuery(query.Get("include_inactive")),
	})
	if err != nil {
		writeOrderError(w, err)
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{"packages": packages})
}

func (h orderHandler) checkout(w http.ResponseWriter, r *http.Request) {
	var request checkoutRequest
	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	order, err := h.orderUsecase.Checkout(r.Context(), usecase.CheckoutInput{
		CustomerID:      r.Header.Get("X-User-ID"),
		CompanyID:       r.Header.Get("X-Company-ID"),
		PackageID:       request.PackageID,
		DurationKey:     request.DurationKey,
		GatewayCode:     request.GatewayCode,
		PaymentMethod:   request.PaymentMethod,
		EnabledPayments: request.EnabledPayments,
		CustomerEmail:   request.CustomerEmail,
		Metadata:        request.Metadata,
	})
	if err != nil {
		writeOrderError(w, err)
		return
	}

	writeJSON(w, http.StatusCreated, order)
}

func (h orderHandler) validateSubscription(w http.ResponseWriter, r *http.Request) {
	var request subscriptionValidationRequest
	_ = json.NewDecoder(r.Body).Decode(&request)

	companyID := r.Header.Get("X-Company-ID")
	if companyID == "" {
		companyID = request.CompanyID
	}
	if companyID == "" {
		companyID = request.UserID
	}

	result, err := h.orderUsecase.ValidateSubscription(r.Context(), usecase.SubscriptionValidationInput{
		CompanyID: companyID,
	})
	if err != nil {
		writeOrderError(w, err)
		return
	}
	if !result.Allowed {
		writeJSON(w, http.StatusPaymentRequired, map[string]any{
			"allowed":     false,
			"reason":      result.Reason,
			"status_code": http.StatusPaymentRequired,
		})
		return
	}

	writeJSON(w, http.StatusOK, map[string]bool{"allowed": true})
}

func (h orderHandler) currentSubscription(w http.ResponseWriter, r *http.Request) {
	result, err := h.orderUsecase.GetCurrentSubscription(r.Context(), usecase.CurrentSubscriptionInput{
		CompanyID: r.Header.Get("X-Company-ID"),
	})
	if err != nil {
		writeOrderError(w, err)
		return
	}

	writeJSON(w, http.StatusOK, result)
}

func (h orderHandler) activateSubscription(w http.ResponseWriter, r *http.Request) {
	var request activateSubscriptionRequest
	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	var startsAt time.Time
	if request.StartsAt != nil {
		startsAt = request.StartsAt.UTC()
	}
	subscription, err := h.orderUsecase.ActivateSubscription(r.Context(), usecase.ActivateSubscriptionInput{
		CompanyID:   request.CompanyID,
		PackageID:   request.PackageID,
		DurationKey: request.DurationKey,
		ActivatedBy: r.Header.Get("X-User-ID"),
		StartsAt:    startsAt,
	})
	if err != nil {
		writeOrderError(w, err)
		return
	}

	writeJSON(w, http.StatusCreated, subscription)
}

func (h orderHandler) orderHistory(w http.ResponseWriter, r *http.Request) {
	orders, err := h.orderUsecase.ListCustomerOrders(r.Context(), usecase.CustomerOrderHistoryInput{
		CustomerID: r.Header.Get("X-User-ID"),
	})
	if err != nil {
		writeOrderError(w, err)
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{"orders": orders})
}

func (h orderHandler) getCustomerOrder(w http.ResponseWriter, r *http.Request) {
	customerID := strings.TrimSpace(r.Header.Get("X-User-ID"))
	if customerID == "" {
		writeOrderError(w, usecase.ErrInvalidCustomerID)
		return
	}

	order, err := h.orderUsecase.GetOrder(r.Context(), r.PathValue("id"))
	if err != nil {
		writeOrderError(w, err)
		return
	}
	if order.CustomerID != customerID {
		writeOrderError(w, domain.ErrOrderNotFound)
		return
	}

	writeJSON(w, http.StatusOK, order)
}

func (h orderHandler) checkPayment(w http.ResponseWriter, r *http.Request) {
	order, err := h.orderUsecase.CheckPayment(r.Context(), usecase.PaymentStatusInput{
		CustomerID: r.Header.Get("X-User-ID"),
		OrderID:    r.PathValue("id"),
	})
	if err != nil {
		writeOrderError(w, err)
		return
	}

	writeJSON(w, http.StatusOK, order)
}

func (h orderHandler) streamPaymentEvents(w http.ResponseWriter, r *http.Request) {
	flusher, ok := w.(http.Flusher)
	if !ok {
		writeError(w, http.StatusInternalServerError, "streaming is not supported")
		return
	}

	order, err := h.orderUsecase.CheckPayment(r.Context(), usecase.PaymentStatusInput{
		CustomerID: r.Header.Get("X-User-ID"),
		OrderID:    r.PathValue("id"),
	})
	if err != nil {
		writeOrderError(w, err)
		return
	}

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache, no-transform")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("X-Accel-Buffering", "no")

	if err := writePaymentStatusEvent(w, "payment-status", order); err != nil {
		return
	}
	flusher.Flush()

	ticker := time.NewTicker(2 * time.Second)
	heartbeat := time.NewTicker(15 * time.Second)
	timeout := time.NewTimer(30 * time.Minute)
	defer ticker.Stop()
	defer heartbeat.Stop()
	defer timeout.Stop()

	lastStatusKey := paymentStatusKey(order)

	for {
		select {
		case <-r.Context().Done():
			return
		case <-timeout.C:
			_, _ = fmt.Fprint(w, "event: close\ndata: {}\n\n")
			flusher.Flush()
			return
		case <-heartbeat.C:
			_, _ = fmt.Fprint(w, ": keepalive\n\n")
			flusher.Flush()
		case <-ticker.C:
			nextOrder, err := h.orderUsecase.CheckPayment(r.Context(), usecase.PaymentStatusInput{
				CustomerID: r.Header.Get("X-User-ID"),
				OrderID:    r.PathValue("id"),
			})
			if err != nil {
				_ = writePaymentErrorEvent(w, err)
				flusher.Flush()
				return
			}

			nextStatusKey := paymentStatusKey(nextOrder)
			if nextStatusKey == lastStatusKey {
				continue
			}

			lastStatusKey = nextStatusKey
			if err := writePaymentStatusEvent(w, "payment-status", nextOrder); err != nil {
				return
			}
			flusher.Flush()
		}
	}
}

func (h orderHandler) notifyPayment(w http.ResponseWriter, r *http.Request) {
	var request paymentNotificationRequest
	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	order, err := h.orderUsecase.NotifyPayment(r.Context(), usecase.PaymentNotificationInput{
		OrderID:    request.OrderID,
		PaymentID:  request.PaymentID,
		Status:     request.Status,
		ExternalID: request.ExternalID,
	})
	if err != nil {
		writeOrderError(w, err)
		return
	}

	writeJSON(w, http.StatusOK, order)
}

func (h orderHandler) getByID(w http.ResponseWriter, r *http.Request) {
	order, err := h.orderUsecase.GetOrder(r.Context(), r.PathValue("id"))
	if err != nil {
		writeOrderError(w, err)
		return
	}

	writeJSON(w, http.StatusOK, order)
}

func (h orderHandler) getInvoice(w http.ResponseWriter, r *http.Request) {
	invoice, err := h.orderUsecase.GetInvoice(r.Context(), r.PathValue("id"))
	if err != nil {
		writeOrderError(w, err)
		return
	}

	writeJSON(w, http.StatusOK, invoice)
}

func writePaymentStatusEvent(w http.ResponseWriter, eventName string, order domain.Order) error {
	payload := paymentStatusEvent{
		OrderID:       order.ID,
		Status:        order.Status,
		PaymentStatus: order.Payment.Status,
		Paid:          order.Status == domain.OrderStatusPaid || order.Payment.Status == domain.PaymentStatusPaid,
		Order:         order,
		UpdatedAt:     order.UpdatedAt.UTC().Format(time.RFC3339Nano),
	}

	data, err := json.Marshal(payload)
	if err != nil {
		return err
	}

	_, err = fmt.Fprintf(w, "event: %s\ndata: %s\n\n", eventName, data)
	return err
}

func writePaymentErrorEvent(w http.ResponseWriter, err error) error {
	payload := map[string]string{
		"message": err.Error(),
	}
	data, marshalErr := json.Marshal(payload)
	if marshalErr != nil {
		return marshalErr
	}

	_, writeErr := fmt.Fprintf(w, "event: error\ndata: %s\n\n", data)
	return writeErr
}

func paymentStatusKey(order domain.Order) string {
	return strings.Join([]string{
		string(order.Status),
		string(order.Payment.Status),
		order.Payment.ExternalID,
		order.UpdatedAt.UTC().Format(time.RFC3339Nano),
	}, "|")
}

func decodePackageRequest(w http.ResponseWriter, r *http.Request) (packageRequest, bool) {
	var request packageRequest
	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return packageRequest{}, false
	}
	return request, true
}

func packageInput(request packageRequest, pathID string) usecase.PackageInput {
	id := request.ID
	if pathID != "" {
		id = pathID
	}

	active := true
	if request.Active != nil {
		active = *request.Active
	}

	return usecase.PackageInput{
		ID:                 id,
		SubscriptionTypeID: request.SubscriptionTypeID,
		Name:               request.Name,
		Description:        request.Description,
		Features:           request.Features,
		DurationCount:      request.DurationCount,
		DurationUnit:       request.DurationUnit,
		PriceAmount:        request.PriceAmount,
		Currency:           request.Currency,
		Active:             active,
	}
}

func parseBoolQuery(value string) bool {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "1", "true", "yes", "on":
		return true
	default:
		return false
	}
}

func writeOrderError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, usecase.ErrInvalidCustomerID),
		errors.Is(err, usecase.ErrInvalidCompanyID),
		errors.Is(err, usecase.ErrInvalidActivatedBy),
		errors.Is(err, usecase.ErrInvalidPackageID),
		errors.Is(err, usecase.ErrInvalidPackageName),
		errors.Is(err, usecase.ErrInvalidPackageDuration),
		errors.Is(err, usecase.ErrInvalidDurationUnit),
		errors.Is(err, usecase.ErrInvalidPackagePrice),
		errors.Is(err, usecase.ErrInvalidSubscriptionTypeID),
		errors.Is(err, usecase.ErrInvalidCurrency),
		errors.Is(err, usecase.ErrInvalidGatewayCode),
		errors.Is(err, usecase.ErrInvalidCustomerEmail),
		errors.Is(err, usecase.ErrInvalidPaymentStatus),
		errors.Is(err, usecase.ErrInvalidDurationKey):
		writeError(w, http.StatusBadRequest, err.Error())
	case errors.Is(err, domain.ErrPackageNotFound),
		errors.Is(err, domain.ErrOrderNotFound),
		errors.Is(err, domain.ErrSubscriptionNotFound),
		errors.Is(err, domain.ErrSubscriptionTypeNotFound):
		writeError(w, http.StatusNotFound, err.Error())
	case errors.Is(err, domain.ErrPackageAlreadyExists),
		errors.Is(err, domain.ErrOrderAlreadyExists):
		writeError(w, http.StatusConflict, err.Error())
	case errors.Is(err, usecase.ErrPaymentGatewayFailed):
		writeError(w, http.StatusServiceUnavailable, err.Error())
	default:
		writeError(w, http.StatusInternalServerError, "internal server error")
	}
}
