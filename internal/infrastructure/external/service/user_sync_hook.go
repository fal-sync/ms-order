package service

import (
	"context"
	"fmt"
	"time"

	"boilerplate-skeletoncode/internal/domain"
	"boilerplate-skeletoncode/internal/infrastructure/external/httpclient"
)

type UserSyncHook struct {
	client   *httpclient.Client
	syncPath string
}

type syncUserRequest struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	Email     string    `json:"email"`
	CreatedAt time.Time `json:"created_at"`
}

func NewUserSyncHook(baseURL string, timeout time.Duration, syncPath string) *UserSyncHook {
	return &UserSyncHook{
		client:   httpclient.New(baseURL, timeout),
		syncPath: syncPath,
	}
}

func (h *UserSyncHook) HandleUserCreated(ctx context.Context, user domain.User) error {
	if err := h.client.PostJSON(ctx, h.syncPath, syncUserRequest{
		ID:        user.ID,
		Name:      user.Name,
		Email:     user.Email,
		CreatedAt: user.CreatedAt,
	}); err != nil {
		return fmt.Errorf("sync user to external service: %w", err)
	}

	return nil
}
