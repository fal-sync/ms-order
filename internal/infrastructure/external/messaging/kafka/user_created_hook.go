package kafka

import (
	"context"
	"encoding/json"
	"fmt"

	"boilerplate-skeletoncode/internal/domain"
)

type UserCreatedHook struct {
	producer Producer
	topic    string
}

func NewUserCreatedHook(producer Producer, topic string) *UserCreatedHook {
	return &UserCreatedHook{
		producer: producer,
		topic:    topic,
	}
}

func (h *UserCreatedHook) HandleUserCreated(ctx context.Context, user domain.User) error {
	payload, err := json.Marshal(domain.NewUserCreatedEvent(user))
	if err != nil {
		return fmt.Errorf("marshal user created event: %w", err)
	}

	if err := h.producer.Publish(ctx, Message{
		Topic: h.topic,
		Key:   user.ID,
		Value: payload,
		Headers: map[string]string{
			"event_type": "user.created",
		},
	}); err != nil {
		return fmt.Errorf("publish user created event: %w", err)
	}

	return nil
}
