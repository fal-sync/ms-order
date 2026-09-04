package usecase

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
	"time"

	"boilerplate-skeletoncode/internal/domain"
	"boilerplate-skeletoncode/internal/usecase/port"
)

// User validation errors.
var (
	ErrInvalidUserName  = errors.New("user name is required")
	ErrInvalidUserEmail = errors.New("valid user email is required")
)

// CreateUserInput contains parameters for creating a new user.
type CreateUserInput struct {
	Name  string `json:"name"`
	Email string `json:"email"`
}

// UserService coordinates user creation, retrieval, and integration hooks.
type UserService struct {
	userRepository   domain.UserRepository
	userCreatedHooks []port.UserCreatedHook
	now              func() time.Time
	generateID       func() string
}

// NewUserService constructs a new UserService with the provided repository and post-creation hooks.
func NewUserService(userRepository domain.UserRepository, userCreatedHooks ...port.UserCreatedHook) *UserService {
	return &UserService{
		userRepository:   userRepository,
		userCreatedHooks: userCreatedHooks,
		now:              time.Now,
		generateID:       generateUserID,
	}
}

// CreateUser validates, stores, and triggers downstream sync hooks for a newly registered user.
func (s *UserService) CreateUser(ctx context.Context, input CreateUserInput) (domain.User, error) {
	name := strings.TrimSpace(input.Name)
	if name == "" {
		return domain.User{}, ErrInvalidUserName
	}

	email := strings.TrimSpace(input.Email)
	if email == "" || !strings.Contains(email, "@") {
		return domain.User{}, ErrInvalidUserEmail
	}

	user := domain.User{
		ID:        s.generateID(),
		Name:      name,
		Email:     email,
		CreatedAt: s.now(),
	}

	if err := s.userRepository.Create(ctx, user); err != nil {
		return domain.User{}, err
	}

	if err := s.runUserCreatedHooks(ctx, user); err != nil {
		return domain.User{}, err
	}

	return user, nil
}

// GetUser retrieves a user by their unique identifier.
func (s *UserService) GetUser(ctx context.Context, id string) (domain.User, error) {
	id = strings.TrimSpace(id)
	if id == "" {
		return domain.User{}, domain.ErrUserNotFound
	}

	return s.userRepository.GetByID(ctx, id)
}

// ListUsers retrieves all registered users.
func (s *UserService) ListUsers(ctx context.Context) ([]domain.User, error) {
	return s.userRepository.List(ctx)
}

func (s *UserService) runUserCreatedHooks(ctx context.Context, user domain.User) error {
	for _, hook := range s.userCreatedHooks {
		if err := hook.HandleUserCreated(ctx, user); err != nil {
			return fmt.Errorf("execute user created hook: %w", err)
		}
	}

	return nil
}

func generateUserID() string {
	buffer := make([]byte, 8)
	if _, err := rand.Read(buffer); err == nil {
		return "usr_" + hex.EncodeToString(buffer)
	}

	return "usr_fallback"
}
