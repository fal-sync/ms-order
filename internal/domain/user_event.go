package domain

import "time"

// UserCreatedEvent payload emitted when a new user is created.
type UserCreatedEvent struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	Email     string    `json:"email"`
	CreatedAt time.Time `json:"created_at"`
}

// NewUserCreatedEvent constructs a UserCreatedEvent from a User entity.
func NewUserCreatedEvent(user User) UserCreatedEvent {
	return UserCreatedEvent{
		ID:        user.ID,
		Name:      user.Name,
		Email:     user.Email,
		CreatedAt: user.CreatedAt,
	}
}
