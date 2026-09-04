package memory

import (
	"context"
	"sort"
	"sync"

	"boilerplate-skeletoncode/internal/domain"
)

type OrderRepository struct {
	mu           sync.RWMutex
	orders       map[string]domain.Order
	invoiceIndex map[string]string
	paymentIndex map[string]string
}

func NewOrderRepository() *OrderRepository {
	return &OrderRepository{
		orders:       make(map[string]domain.Order),
		invoiceIndex: make(map[string]string),
		paymentIndex: make(map[string]string),
	}
}

func (r *OrderRepository) Create(_ context.Context, order domain.Order) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if _, exists := r.orders[order.ID]; exists {
		return domain.ErrOrderAlreadyExists
	}

	r.orders[order.ID] = cloneOrder(order)
	r.invoiceIndex[order.Invoice.ID] = order.ID
	if order.Payment.PaymentID != "" {
		r.paymentIndex[order.Payment.PaymentID] = order.ID
	}

	return nil
}

func (r *OrderRepository) Update(_ context.Context, order domain.Order) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if _, exists := r.orders[order.ID]; !exists {
		return domain.ErrOrderNotFound
	}

	r.orders[order.ID] = cloneOrder(order)
	r.invoiceIndex[order.Invoice.ID] = order.ID
	if order.Payment.PaymentID != "" {
		r.paymentIndex[order.Payment.PaymentID] = order.ID
	}

	return nil
}

func (r *OrderRepository) GetByID(_ context.Context, id string) (domain.Order, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	order, exists := r.orders[id]
	if !exists {
		return domain.Order{}, domain.ErrOrderNotFound
	}

	return cloneOrder(order), nil
}

func (r *OrderRepository) GetByInvoiceID(_ context.Context, invoiceID string) (domain.Order, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	orderID, exists := r.invoiceIndex[invoiceID]
	if !exists {
		return domain.Order{}, domain.ErrOrderNotFound
	}

	order, exists := r.orders[orderID]
	if !exists {
		return domain.Order{}, domain.ErrOrderNotFound
	}

	return cloneOrder(order), nil
}

func (r *OrderRepository) GetByPaymentID(_ context.Context, paymentID string) (domain.Order, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	orderID, exists := r.paymentIndex[paymentID]
	if !exists {
		return domain.Order{}, domain.ErrOrderNotFound
	}

	order, exists := r.orders[orderID]
	if !exists {
		return domain.Order{}, domain.ErrOrderNotFound
	}

	return cloneOrder(order), nil
}

func (r *OrderRepository) ListByCustomerID(_ context.Context, customerID string) ([]domain.Order, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	orders := make([]domain.Order, 0)
	for _, order := range r.orders {
		if order.CustomerID == customerID {
			orders = append(orders, cloneOrder(order))
		}
	}

	sort.Slice(orders, func(i, j int) bool {
		return orders[i].CreatedAt.After(orders[j].CreatedAt)
	})

	return orders, nil
}

func cloneOrder(order domain.Order) domain.Order {
	if len(order.Metadata) > 0 {
		metadata := make(map[string]string, len(order.Metadata))
		for key, value := range order.Metadata {
			metadata[key] = value
		}
		order.Metadata = metadata
	}

	if len(order.Invoice.Items) > 0 {
		items := make([]domain.InvoiceItem, len(order.Invoice.Items))
		copy(items, order.Invoice.Items)
		order.Invoice.Items = items
	}

	return order
}
