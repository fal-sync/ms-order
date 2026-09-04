package port

import (
	"context"

	"boilerplate-skeletoncode/internal/domain"
)

type UserCreatedHook interface {
	HandleUserCreated(ctx context.Context, user domain.User) error
}
