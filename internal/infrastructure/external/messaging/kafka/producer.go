package kafka

import "context"

type Producer interface {
	Publish(ctx context.Context, message Message) error
}
