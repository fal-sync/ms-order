package app

import (
	"context"
	"errors"
	"io"
	"net/http"
	"time"

	"boilerplate-skeletoncode/internal/config"
	httpdelivery "boilerplate-skeletoncode/internal/delivery/http"
	"boilerplate-skeletoncode/internal/infrastructure/external/paymentgateway"
	"boilerplate-skeletoncode/internal/infrastructure/persistence/memory"
	mongorepo "boilerplate-skeletoncode/internal/infrastructure/persistence/mongo"
	postgresrepo "boilerplate-skeletoncode/internal/infrastructure/persistence/postgres"
	"boilerplate-skeletoncode/internal/usecase"
)

type App struct {
	server          *http.Server
	shutdownTimeout time.Duration
	closers         []io.Closer
}

func New(cfg config.Config) (*App, error) {
	userRepository := memory.NewUserRepository()
	mongoClient, err := mongorepo.NewClient(context.Background(), cfg.Mongo.URI, cfg.Mongo.Timeout)
	if err != nil {
		return nil, err
	}

	database := mongoClient.Database(cfg.Mongo.Database)
	packageRepository := mongorepo.NewPackageRepository(database)
	orderRepository := mongorepo.NewOrderRepository(database)
	subscriptionRepository := mongorepo.NewSubscriptionRepository(database)
	postgresCtx, postgresCancel := context.WithTimeout(context.Background(), cfg.Postgres.ConnectTimeout)
	defer postgresCancel()
	postgresDB, err := postgresrepo.Open(
		postgresCtx,
		cfg.Postgres.DSN,
		cfg.Postgres.MaxOpenConns,
		cfg.Postgres.MaxIdleConns,
		cfg.Postgres.ConnMaxLifetime,
	)
	if err != nil {
		return nil, err
	}
	if err := postgresrepo.Migrate(postgresCtx, postgresDB); err != nil {
		_ = postgresDB.Close()
		return nil, err
	}
	subscriptionTypeRepository := postgresrepo.NewSubscriptionTypeRepository(postgresDB)
	paymentGateway := paymentgateway.NewHTTPClient(cfg.PaymentGateway.BaseURL, cfg.PaymentGateway.Timeout)
	userCreatedHooks, closers := buildUserCreatedHooks(cfg)
	closers = append(closers, closerFunc(func() error {
		ctx, cancel := context.WithTimeout(context.Background(), cfg.Mongo.Timeout)
		defer cancel()
		return mongoClient.Disconnect(ctx)
	}))
	closers = append(closers, postgresDB)

	userUsecase := usecase.NewUserService(userRepository, userCreatedHooks...)
	orderUsecase := usecase.NewOrderService(packageRepository, orderRepository, subscriptionRepository, paymentGateway, usecase.OrderServiceOptions{
		InvoiceDueDuration:         cfg.Order.InvoiceDueDuration,
		SuccessURLTemplate:         cfg.Order.SuccessURLTemplate,
		FailureURLTemplate:         cfg.Order.FailureURLTemplate,
		NotificationURL:            cfg.Order.NotificationURL,
		SubscriptionTypeRepository: subscriptionTypeRepository,
	})
	router := httpdelivery.NewRouter(userUsecase, orderUsecase, httpdelivery.InternalAccessPolicy{
		AllowedCIDRs:      cfg.InternalAPI.AllowedCIDRs,
		TrustProxyHeaders: cfg.InternalAPI.TrustProxyHeaders,
	})

	server := &http.Server{
		Addr:              ":" + cfg.HTTPPort,
		Handler:           router,
		ReadHeaderTimeout: 5 * time.Second,
	}

	return &App{
		server:          server,
		shutdownTimeout: cfg.ShutdownTimeout,
		closers:         closers,
	}, nil
}

func (a *App) Run() error {
	return a.server.ListenAndServe()
}

func (a *App) Shutdown(ctx context.Context) error {
	shutdownCtx, cancel := context.WithTimeout(ctx, a.shutdownTimeout)
	defer cancel()

	errs := make([]error, 0, len(a.closers)+1)

	if err := a.server.Shutdown(shutdownCtx); err != nil {
		errs = append(errs, err)
	}

	for _, closer := range a.closers {
		if err := closer.Close(); err != nil {
			errs = append(errs, err)
		}
	}

	return errors.Join(errs...)
}

type closerFunc func() error

func (f closerFunc) Close() error {
	return f()
}
