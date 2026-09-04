package mongo

import (
	"context"
	"errors"
	"time"

	"boilerplate-skeletoncode/internal/domain"

	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
)

type SubscriptionRepository struct {
	collection *mongo.Collection
}

func NewSubscriptionRepository(database *mongo.Database) *SubscriptionRepository {
	return &SubscriptionRepository{
		collection: database.Collection("subscriptions"),
	}
}

func (r *SubscriptionRepository) GetActiveByCompanyID(ctx context.Context, companyID string, at time.Time) (domain.Subscription, error) {
	var subscription domain.Subscription
	err := r.collection.FindOne(ctx, bson.M{
		"$or": bson.A{
			bson.M{"company_id": companyID},
			bson.M{"company_id": bson.M{"$exists": false}, "customer_id": companyID},
		},
		"active":         true,
		"period.ends_at": bson.M{"$gt": at},
	}).Decode(&subscription)
	if errors.Is(err, mongo.ErrNoDocuments) {
		return domain.Subscription{}, domain.ErrSubscriptionNotFound
	}
	if err != nil {
		return domain.Subscription{}, err
	}
	return subscription, nil
}

func (r *SubscriptionRepository) UpsertByCompanyID(ctx context.Context, subscription domain.Subscription) error {
	update := bson.M{
		"$set": bson.M{
			"company_id":        subscription.CompanyID,
			"customer_id":       subscription.CustomerID,
			"order_id":          subscription.OrderID,
			"activated_by":      subscription.ActivatedBy,
			"activation_source": subscription.ActivationSource,
			"package":           subscription.Package,
			"period":            subscription.Period,
			"active":            subscription.Active,
			"updated_at":        subscription.UpdatedAt,
		},
		"$setOnInsert": bson.M{
			"_id":        subscription.ID,
			"created_at": subscription.CreatedAt,
		},
	}

	_, err := r.collection.UpdateOne(
		ctx,
		bson.M{"$or": bson.A{
			bson.M{"company_id": subscription.CompanyID},
			bson.M{"company_id": bson.M{"$exists": false}, "customer_id": subscription.CompanyID},
		}},
		update,
		options.UpdateOne().SetUpsert(true),
	)
	return err
}

func (r *SubscriptionRepository) GetActiveByCustomerID(ctx context.Context, customerID string, at time.Time) (domain.Subscription, error) {
	return r.GetActiveByCompanyID(ctx, customerID, at)
}
