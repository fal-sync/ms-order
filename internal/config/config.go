// Package config loads and parses environment variables for the service.
package config

import (
	"errors"
	"fmt"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/joho/godotenv"
)

// Config aggregates all runtime configuration settings for ms-order.
type Config struct {
	AppName         string
	HTTPPort        string
	ShutdownTimeout time.Duration
	InternalAPI     InternalAPIConfig
	Mongo           MongoConfig
	Postgres        PostgresConfig
	Order           OrderConfig
	PaymentGateway  PaymentGatewayConfig
	External        ExternalConfig
}

// InternalAPIConfig holds network security settings for internal endpoints.
type InternalAPIConfig struct {
	AllowedCIDRs      []string
	TrustProxyHeaders bool
}

// OrderConfig specifies invoice and payment redirect URL settings.
type OrderConfig struct {
	InvoiceDueDuration time.Duration
	SuccessURLTemplate string
	FailureURLTemplate string
	NotificationURL    string
}

// MongoConfig contains connection parameters for MongoDB.
type MongoConfig struct {
	URI      string
	Database string
	Timeout  time.Duration
}

// PostgresConfig contains connection and pool parameters for PostgreSQL.
type PostgresConfig struct {
	DSN             string
	MaxOpenConns    int
	MaxIdleConns    int
	ConnMaxLifetime time.Duration
	ConnectTimeout  time.Duration
}

// PaymentGatewayConfig holds client connection settings for ms-payment-gateway.
type PaymentGatewayConfig struct {
	BaseURL string
	Timeout time.Duration
}

// ExternalConfig configures external downstream sync and message brokers.
type ExternalConfig struct {
	Service ServiceConfig
	Kafka   KafkaConfig
}

// ServiceConfig configures downstream user sync HTTP client.
type ServiceConfig struct {
	UserSyncBaseURL string
	UserSyncPath    string
	UserSyncTimeout time.Duration
}

// KafkaConfig configures Apache Kafka producer settings.
type KafkaConfig struct {
	Enabled          bool
	Brokers          []string
	UserCreatedTopic string
}

// Load reads configuration from .env and environment variables, validating required values.
func Load() (Config, error) {
	if err := loadDotEnv(); err != nil {
		return Config{}, err
	}

	var configErrors []string

	cfg := Config{
		AppName:         getRequiredEnv("APP_NAME", &configErrors),
		HTTPPort:        getRequiredEnv("APP_PORT", &configErrors),
		ShutdownTimeout: getRequiredDurationEnv("APP_SHUTDOWN_TIMEOUT", &configErrors),
		InternalAPI: InternalAPIConfig{
			AllowedCIDRs:      getRequiredListEnv("INTERNAL_ALLOWED_CIDRS", &configErrors),
			TrustProxyHeaders: getRequiredBoolEnv("INTERNAL_TRUST_PROXY_HEADERS", &configErrors),
		},
		Mongo: MongoConfig{
			URI:      getRequiredEnv("MONGO_URI", &configErrors),
			Database: getRequiredEnv("MONGO_DATABASE", &configErrors),
			Timeout:  getRequiredDurationEnv("MONGO_TIMEOUT", &configErrors),
		},
		Postgres: PostgresConfig{
			DSN:             databaseDSN(),
			MaxOpenConns:    getIntEnv("DB_MAX_OPEN_CONNS", 10),
			MaxIdleConns:    getIntEnv("DB_MAX_IDLE_CONNS", 5),
			ConnMaxLifetime: getDurationEnv("DB_CONN_MAX_LIFETIME", 30*time.Minute),
			ConnectTimeout:  getDurationEnv("DB_CONNECT_TIMEOUT", 5*time.Second),
		},
		Order: OrderConfig{
			InvoiceDueDuration: getRequiredDurationEnv("ORDER_INVOICE_DUE_DURATION", &configErrors),
			SuccessURLTemplate: getRequiredEnv("PAYMENT_SUCCESS_URL_TEMPLATE", &configErrors),
			FailureURLTemplate: getRequiredEnv("PAYMENT_FAILURE_URL_TEMPLATE", &configErrors),
			NotificationURL:    getRequiredEnv("PAYMENT_NOTIFICATION_URL", &configErrors),
		},
		PaymentGateway: PaymentGatewayConfig{
			BaseURL: getRequiredEnv("PAYMENT_GATEWAY_BASE_URL", &configErrors),
			Timeout: getRequiredDurationEnv("PAYMENT_GATEWAY_TIMEOUT", &configErrors),
		},
		External: ExternalConfig{
			Service: ServiceConfig{
				UserSyncBaseURL: getOptionalEnv("EXTERNAL_USER_SYNC_BASE_URL"),
				UserSyncPath:    getRequiredEnv("EXTERNAL_USER_SYNC_PATH", &configErrors),
				UserSyncTimeout: getRequiredDurationEnv("EXTERNAL_USER_SYNC_TIMEOUT", &configErrors),
			},
			Kafka: KafkaConfig{
				Enabled:          getRequiredBoolEnv("KAFKA_ENABLED", &configErrors),
				Brokers:          getListEnv("KAFKA_BROKERS"),
				UserCreatedTopic: getRequiredEnv("KAFKA_USER_CREATED_TOPIC", &configErrors),
			},
		},
	}

	if cfg.External.Kafka.Enabled && len(cfg.External.Kafka.Brokers) == 0 {
		configErrors = append(configErrors, "KAFKA_BROKERS is required when KAFKA_ENABLED=true")
	}

	if len(configErrors) > 0 {
		return Config{}, fmt.Errorf("invalid config: %s", strings.Join(configErrors, "; "))
	}

	return cfg, nil
}

func loadDotEnv() error {
	if err := godotenv.Load(); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil
		}

		return fmt.Errorf("load .env: %w", err)
	}

	return nil
}

func getRequiredEnv(key string, configErrors *[]string) string {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		*configErrors = append(*configErrors, key+" is required")
	}

	return value
}

func getOptionalEnv(key string) string {
	return strings.TrimSpace(os.Getenv(key))
}

func getEnv(key string, fallback string) string {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return fallback
	}

	return value
}

func getDurationEnv(key string, fallback time.Duration) time.Duration {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return fallback
	}

	duration, err := time.ParseDuration(value)
	if err != nil {
		return fallback
	}

	return duration
}

func getIntEnv(key string, fallback int) int {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return fallback
	}

	parsed, err := strconv.Atoi(value)
	if err != nil || parsed < 0 {
		return fallback
	}

	return parsed
}

func databaseDSN() string {
	if value := strings.TrimSpace(os.Getenv("DATABASE_URL")); value != "" {
		return value
	}

	if value := strings.TrimSpace(os.Getenv("DB_DSN")); value != "" {
		return value
	}

	user := getEnv("DB_USER", "postgres")
	password := getEnv("DB_PASSWORD", "postgres")
	host := getEnv("DB_HOST", "localhost")
	port := getEnv("DB_PORT", "5432")
	name := getEnv("DB_NAME", "fal_sync")
	sslMode := getEnv("DB_SSLMODE", "disable")

	dsn := url.URL{
		Scheme: "postgres",
		Host:   host + ":" + port,
		Path:   name,
	}

	if password == "" {
		dsn.User = url.User(user)
	} else {
		dsn.User = url.UserPassword(user, password)
	}

	query := dsn.Query()
	query.Set("sslmode", sslMode)
	dsn.RawQuery = query.Encode()

	return dsn.String()
}

func getRequiredDurationEnv(key string, configErrors *[]string) time.Duration {
	value := getRequiredEnv(key, configErrors)
	if value == "" {
		return 0
	}

	duration, err := time.ParseDuration(value)
	if err != nil {
		*configErrors = append(*configErrors, key+" must be a valid duration")
		return 0
	}

	return duration
}

func getRequiredBoolEnv(key string, configErrors *[]string) bool {
	value := getRequiredEnv(key, configErrors)
	if value == "" {
		return false
	}

	switch strings.ToLower(value) {
	case "1", "true", "yes", "on":
		return true
	case "0", "false", "no", "off":
		return false
	default:
		*configErrors = append(*configErrors, key+" must be a boolean")
		return false
	}
}

func getListEnv(key string) []string {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return nil
	}

	rawItems := strings.Split(value, ",")
	items := make([]string, 0, len(rawItems))
	for _, item := range rawItems {
		trimmed := strings.TrimSpace(item)
		if trimmed != "" {
			items = append(items, trimmed)
		}
	}

	return items
}

func getRequiredListEnv(key string, configErrors *[]string) []string {
	items := getListEnv(key)
	if len(items) == 0 {
		*configErrors = append(*configErrors, key+" is required")
	}

	return items
}
