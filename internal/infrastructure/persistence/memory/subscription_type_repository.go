package memory

import (
	"context"
	"sync"
	"time"

	"boilerplate-skeletoncode/internal/domain"
)

type SubscriptionTypeRepository struct {
	mu                sync.RWMutex
	subscriptionTypes map[string]domain.SubscriptionType
}

func NewSubscriptionTypeRepository() *SubscriptionTypeRepository {
	now := time.Date(2026, time.January, 1, 0, 0, 0, 0, time.UTC)

	return &SubscriptionTypeRepository{
		subscriptionTypes: map[string]domain.SubscriptionType{
			domain.SubscriptionTypePackageID: {
				ID:        domain.SubscriptionTypePackageID,
				Name:      "Subscription Package",
				CreatedAt: now,
				UpdatedAt: now,
			},
			domain.SubscriptionTypeAdditionalFeaturesID: {
				ID:        domain.SubscriptionTypeAdditionalFeaturesID,
				Name:      "Additional Features",
				CreatedAt: now,
				UpdatedAt: now,
			},
			domain.SubscriptionTypeESealDocumentID: {
				ID:        domain.SubscriptionTypeESealDocumentID,
				Name:      "E-Seal Document",
				CreatedAt: now,
				UpdatedAt: now,
			},
		},
	}
}

func (r *SubscriptionTypeRepository) GetByID(_ context.Context, id string) (domain.SubscriptionType, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	subscriptionType, exists := r.subscriptionTypes[id]
	if !exists || subscriptionType.DeletedAt != nil {
		return domain.SubscriptionType{}, domain.ErrSubscriptionTypeNotFound
	}

	return subscriptionType, nil
}

func (r *SubscriptionTypeRepository) ListActive(_ context.Context) ([]domain.SubscriptionType, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	subscriptionTypes := make([]domain.SubscriptionType, 0, len(r.subscriptionTypes))
	for _, subscriptionType := range r.subscriptionTypes {
		if subscriptionType.DeletedAt == nil {
			subscriptionTypes = append(subscriptionTypes, subscriptionType)
		}
	}

	return subscriptionTypes, nil
}
