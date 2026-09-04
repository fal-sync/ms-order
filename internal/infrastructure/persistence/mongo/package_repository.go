package mongo

import (
	"context"
	"errors"

	"boilerplate-skeletoncode/internal/domain"

	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
)

type PackageRepository struct {
	collection *mongo.Collection
}

func NewPackageRepository(database *mongo.Database) *PackageRepository {
	return &PackageRepository{
		collection: database.Collection("packages"),
	}
}

func (r *PackageRepository) Create(ctx context.Context, pkg domain.Package) error {
	_, err := r.collection.InsertOne(ctx, pkg)
	if mongo.IsDuplicateKeyError(err) {
		return domain.ErrPackageAlreadyExists
	}
	return err
}

func (r *PackageRepository) Update(ctx context.Context, pkg domain.Package) error {
	result, err := r.collection.ReplaceOne(ctx, bson.M{"_id": pkg.ID}, pkg)
	if err != nil {
		return err
	}
	if result.MatchedCount == 0 {
		return domain.ErrPackageNotFound
	}
	return nil
}

func (r *PackageRepository) Delete(ctx context.Context, id string) error {
	result, err := r.collection.DeleteOne(ctx, bson.M{"_id": id})
	if err != nil {
		return err
	}
	if result.DeletedCount == 0 {
		return domain.ErrPackageNotFound
	}
	return nil
}

func (r *PackageRepository) GetByID(ctx context.Context, id string) (domain.Package, error) {
	var pkg domain.Package
	err := r.collection.FindOne(ctx, bson.M{"_id": id}).Decode(&pkg)
	if errors.Is(err, mongo.ErrNoDocuments) {
		return domain.Package{}, domain.ErrPackageNotFound
	}
	if err != nil {
		return domain.Package{}, err
	}
	return pkg, nil
}

func (r *PackageRepository) List(ctx context.Context, filter domain.PackageFilter) ([]domain.Package, error) {
	query := bson.M{}
	if filter.SubscriptionTypeID != "" {
		query["subscription_type_id"] = filter.SubscriptionTypeID
	}
	if !filter.IncludeInactive {
		query["active"] = true
	}

	return r.find(ctx, query)
}

func (r *PackageRepository) ListActive(ctx context.Context) ([]domain.Package, error) {
	return r.find(ctx, bson.M{"active": true})
}

func (r *PackageRepository) find(ctx context.Context, filter bson.M) ([]domain.Package, error) {
	cursor, err := r.collection.Find(
		ctx,
		filter,
		options.Find().SetSort(bson.D{{Key: "created_at", Value: -1}, {Key: "_id", Value: 1}}),
	)
	if err != nil {
		return nil, err
	}
	defer cursor.Close(ctx)

	var packages []domain.Package
	if err := cursor.All(ctx, &packages); err != nil {
		return nil, err
	}
	if packages == nil {
		packages = []domain.Package{}
	}

	return packages, nil
}
