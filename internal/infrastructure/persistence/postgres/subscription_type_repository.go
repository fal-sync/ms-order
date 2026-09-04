package postgres

import (
	"context"
	"database/sql"
	"errors"

	"boilerplate-skeletoncode/internal/domain"
)

type SubscriptionTypeRepository struct {
	db *sql.DB
}

func NewSubscriptionTypeRepository(db *sql.DB) *SubscriptionTypeRepository {
	return &SubscriptionTypeRepository{
		db: db,
	}
}

func (r *SubscriptionTypeRepository) GetByID(ctx context.Context, id string) (domain.SubscriptionType, error) {
	row := r.db.QueryRowContext(ctx, `
		SELECT id::text, name, created_at, updated_at, deleted_at
		FROM subscription_types
		WHERE id = $1 AND deleted_at IS NULL
	`, id)

	subscriptionType, err := scanSubscriptionType(row)
	if errors.Is(err, sql.ErrNoRows) {
		return domain.SubscriptionType{}, domain.ErrSubscriptionTypeNotFound
	}
	if err != nil {
		return domain.SubscriptionType{}, err
	}

	return subscriptionType, nil
}

func (r *SubscriptionTypeRepository) ListActive(ctx context.Context) ([]domain.SubscriptionType, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT id::text, name, created_at, updated_at, deleted_at
		FROM subscription_types
		WHERE deleted_at IS NULL
		ORDER BY created_at ASC, id ASC
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	subscriptionTypes := []domain.SubscriptionType{}
	for rows.Next() {
		subscriptionType, err := scanSubscriptionType(rows)
		if err != nil {
			return nil, err
		}

		subscriptionTypes = append(subscriptionTypes, subscriptionType)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	return subscriptionTypes, nil
}

type subscriptionTypeScanner interface {
	Scan(dest ...any) error
}

func scanSubscriptionType(scanner subscriptionTypeScanner) (domain.SubscriptionType, error) {
	var subscriptionType domain.SubscriptionType
	err := scanner.Scan(
		&subscriptionType.ID,
		&subscriptionType.Name,
		&subscriptionType.CreatedAt,
		&subscriptionType.UpdatedAt,
		&subscriptionType.DeletedAt,
	)
	if err != nil {
		return domain.SubscriptionType{}, err
	}

	return subscriptionType, nil
}
