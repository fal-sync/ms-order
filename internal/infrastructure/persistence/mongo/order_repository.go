package mongo

import (
	"context"
	"errors"

	"boilerplate-skeletoncode/internal/domain"

	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
)

type OrderRepository struct {
	collection *mongo.Collection
}

func NewOrderRepository(database *mongo.Database) *OrderRepository {
	return &OrderRepository{
		collection: database.Collection("orders"),
	}
}

func (r *OrderRepository) Create(ctx context.Context, order domain.Order) error {
	_, err := r.collection.InsertOne(ctx, order)
	if mongo.IsDuplicateKeyError(err) {
		return domain.ErrOrderAlreadyExists
	}
	return err
}

func (r *OrderRepository) Update(ctx context.Context, order domain.Order) error {
	result, err := r.collection.ReplaceOne(ctx, bson.M{"_id": order.ID}, order)
	if err != nil {
		return err
	}
	if result.MatchedCount == 0 {
		return domain.ErrOrderNotFound
	}
	return nil
}

func (r *OrderRepository) GetByID(ctx context.Context, id string) (domain.Order, error) {
	return r.findOne(ctx, bson.M{"_id": id})
}

func (r *OrderRepository) GetByInvoiceID(ctx context.Context, invoiceID string) (domain.Order, error) {
	return r.findOne(ctx, bson.M{"invoice.id": invoiceID})
}

func (r *OrderRepository) GetByPaymentID(ctx context.Context, paymentID string) (domain.Order, error) {
	return r.findOne(ctx, bson.M{"payment.payment_id": paymentID})
}

func (r *OrderRepository) ListByCustomerID(ctx context.Context, customerID string) ([]domain.Order, error) {
	cursor, err := r.collection.Find(
		ctx,
		bson.M{"customer_id": customerID},
		options.Find().SetSort(bson.D{{Key: "created_at", Value: -1}}),
	)
	if err != nil {
		return nil, err
	}
	defer cursor.Close(ctx)

	var orders []domain.Order
	if err := cursor.All(ctx, &orders); err != nil {
		return nil, err
	}
	if orders == nil {
		orders = []domain.Order{}
	}

	return orders, nil
}

func (r *OrderRepository) findOne(ctx context.Context, filter bson.M) (domain.Order, error) {
	var order domain.Order
	err := r.collection.FindOne(ctx, filter).Decode(&order)
	if errors.Is(err, mongo.ErrNoDocuments) {
		return domain.Order{}, domain.ErrOrderNotFound
	}
	if err != nil {
		return domain.Order{}, err
	}
	return order, nil
}
