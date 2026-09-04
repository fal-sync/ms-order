package memory

import (
	"context"
	"sync"
	"time"

	"boilerplate-skeletoncode/internal/domain"
)

type SubscriptionRepository struct {
	mu            sync.RWMutex
	subscriptions map[string]domain.Subscription
}

func NewSubscriptionRepository() *SubscriptionRepository {
	return &SubscriptionRepository{
		subscriptions: make(map[string]domain.Subscription),
	}
}

func (r *SubscriptionRepository) GetActiveByCompanyID(_ context.Context, companyID string, at time.Time) (domain.Subscription, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	subscription, exists := r.subscriptions[companyID]
	if !exists || !subscription.Active || !subscription.Period.EndsAt.After(at) {
		return domain.Subscription{}, domain.ErrSubscriptionNotFound
	}

	return subscription, nil
}

func (r *SubscriptionRepository) UpsertByCompanyID(_ context.Context, subscription domain.Subscription) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.subscriptions[subscription.CompanyID] = subscription
	return nil
}

func (r *SubscriptionRepository) GetActiveByCustomerID(ctx context.Context, customerID string, at time.Time) (domain.Subscription, error) {
	return r.GetActiveByCompanyID(ctx, customerID, at)
}
