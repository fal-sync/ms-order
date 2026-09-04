package memory

import (
	"context"
	"sort"
	"sync"

	"boilerplate-skeletoncode/internal/domain"
)

type UserRepository struct {
	mu    sync.RWMutex
	users map[string]domain.User
}

func NewUserRepository() *UserRepository {
	return &UserRepository{
		users: make(map[string]domain.User),
	}
}

func (r *UserRepository) Create(_ context.Context, user domain.User) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if _, exists := r.users[user.ID]; exists {
		return domain.ErrUserAlreadyExists
	}

	r.users[user.ID] = user

	return nil
}

func (r *UserRepository) GetByID(_ context.Context, id string) (domain.User, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	user, exists := r.users[id]
	if !exists {
		return domain.User{}, domain.ErrUserNotFound
	}

	return user, nil
}

func (r *UserRepository) List(_ context.Context) ([]domain.User, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	users := make([]domain.User, 0, len(r.users))
	for _, user := range r.users {
		users = append(users, user)
	}

	sort.Slice(users, func(i, j int) bool {
		return users[i].CreatedAt.Before(users[j].CreatedAt)
	})

	return users, nil
}
