package usecase

import (
	"context"
	"testing"

	"boilerplate-skeletoncode/internal/domain"
	"boilerplate-skeletoncode/internal/infrastructure/persistence/memory"
)

func TestCreateUser(t *testing.T) {
	service := NewUserService(memory.NewUserRepository())

	user, err := service.CreateUser(context.Background(), CreateUserInput{
		Name:  "Naufal",
		Email: "naufal@example.com",
	})
	if err != nil {
		t.Fatalf("CreateUser returned error: %v", err)
	}

	if user.ID == "" {
		t.Fatal("expected generated user ID")
	}

	if user.Name != "Naufal" {
		t.Fatalf("expected Name to be Naufal, got %q", user.Name)
	}
}

func TestCreateUserValidation(t *testing.T) {
	service := NewUserService(memory.NewUserRepository())

	_, err := service.CreateUser(context.Background(), CreateUserInput{
		Name:  "",
		Email: "naufal@example.com",
	})
	if err == nil {
		t.Fatal("expected validation error")
	}

	if err != ErrInvalidUserName {
		t.Fatalf("expected ErrInvalidUserName, got %v", err)
	}
}

func TestGetUserNotFound(t *testing.T) {
	service := NewUserService(memory.NewUserRepository())

	_, err := service.GetUser(context.Background(), "missing-id")
	if err == nil {
		t.Fatal("expected not found error")
	}

	if err != domain.ErrUserNotFound {
		t.Fatalf("expected domain.ErrUserNotFound, got %v", err)
	}
}

func TestCreateUserRunsHooks(t *testing.T) {
	hook := &hookSpy{}
	service := NewUserService(memory.NewUserRepository(), hook)

	user, err := service.CreateUser(context.Background(), CreateUserInput{
		Name:  "Naufal",
		Email: "naufal@example.com",
	})
	if err != nil {
		t.Fatalf("CreateUser returned error: %v", err)
	}

	if !hook.called {
		t.Fatal("expected user created hook to be called")
	}

	if hook.user.ID != user.ID {
		t.Fatalf("expected hook user ID to be %q, got %q", user.ID, hook.user.ID)
	}
}

type hookSpy struct {
	called bool
	user   domain.User
}

func (h *hookSpy) HandleUserCreated(_ context.Context, user domain.User) error {
	h.called = true
	h.user = user
	return nil
}
