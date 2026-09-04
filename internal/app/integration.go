package app

import (
	"io"

	"boilerplate-skeletoncode/internal/config"
	"boilerplate-skeletoncode/internal/infrastructure/external/messaging/kafka"
	"boilerplate-skeletoncode/internal/infrastructure/external/service"
	"boilerplate-skeletoncode/internal/usecase/port"
)

func buildUserCreatedHooks(cfg config.Config) ([]port.UserCreatedHook, []io.Closer) {
	hooks := make([]port.UserCreatedHook, 0, 2)
	closers := make([]io.Closer, 0, 1)

	if cfg.External.Service.UserSyncBaseURL != "" {
		hooks = append(hooks, service.NewUserSyncHook(
			cfg.External.Service.UserSyncBaseURL,
			cfg.External.Service.UserSyncTimeout,
			cfg.External.Service.UserSyncPath,
		))
	}

	if cfg.External.Kafka.Enabled && len(cfg.External.Kafka.Brokers) > 0 && cfg.External.Kafka.UserCreatedTopic != "" {
		writer := kafka.NewWriter(cfg.External.Kafka.Brokers)
		hooks = append(hooks, kafka.NewUserCreatedHook(writer, cfg.External.Kafka.UserCreatedTopic))
		closers = append(closers, writer)
	}

	return hooks, closers
}
