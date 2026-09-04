package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

var envKeys = []string{
	"APP_NAME",
	"APP_PORT",
	"APP_SHUTDOWN_TIMEOUT",
	"MONGO_URI",
	"MONGO_DATABASE",
	"MONGO_TIMEOUT",
	"DATABASE_URL",
	"DB_DSN",
	"DB_USER",
	"DB_PASSWORD",
	"DB_HOST",
	"DB_PORT",
	"DB_NAME",
	"DB_SSLMODE",
	"DB_MAX_OPEN_CONNS",
	"DB_MAX_IDLE_CONNS",
	"DB_CONN_MAX_LIFETIME",
	"DB_CONNECT_TIMEOUT",
	"PAYMENT_GATEWAY_BASE_URL",
	"PAYMENT_GATEWAY_TIMEOUT",
	"PAYMENT_SUCCESS_URL_TEMPLATE",
	"PAYMENT_FAILURE_URL_TEMPLATE",
	"PAYMENT_NOTIFICATION_URL",
	"ORDER_INVOICE_DUE_DURATION",
	"INTERNAL_ALLOWED_CIDRS",
	"INTERNAL_TRUST_PROXY_HEADERS",
	"EXTERNAL_USER_SYNC_BASE_URL",
	"EXTERNAL_USER_SYNC_PATH",
	"EXTERNAL_USER_SYNC_TIMEOUT",
	"KAFKA_ENABLED",
	"KAFKA_BROKERS",
	"KAFKA_USER_CREATED_TOPIC",
}

func TestLoadReadsDotEnv(t *testing.T) {
	cleanEnv(t)
	writeDotEnv(t, validDotEnv())

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	if cfg.AppName != "ms-order" {
		t.Fatalf("AppName = %q, want %q", cfg.AppName, "ms-order")
	}
	if cfg.HTTPPort != "8085" {
		t.Fatalf("HTTPPort = %q, want %q", cfg.HTTPPort, "8085")
	}
	if cfg.Mongo.Timeout != 10*time.Second {
		t.Fatalf("Mongo.Timeout = %s, want %s", cfg.Mongo.Timeout, 10*time.Second)
	}
	if cfg.Order.InvoiceDueDuration != 24*time.Hour {
		t.Fatalf("Order.InvoiceDueDuration = %s, want %s", cfg.Order.InvoiceDueDuration, 24*time.Hour)
	}
	if !cfg.InternalAPI.TrustProxyHeaders {
		t.Fatal("InternalAPI.TrustProxyHeaders = false, want true")
	}
	if len(cfg.InternalAPI.AllowedCIDRs) != 2 {
		t.Fatalf("AllowedCIDRs length = %d, want 2", len(cfg.InternalAPI.AllowedCIDRs))
	}
	if !cfg.External.Kafka.Enabled {
		t.Fatal("Kafka.Enabled = false, want true")
	}
	if len(cfg.External.Kafka.Brokers) != 2 {
		t.Fatalf("Kafka.Brokers length = %d, want 2", len(cfg.External.Kafka.Brokers))
	}
}

func TestLoadRejectsMissingRequiredConfig(t *testing.T) {
	cleanEnv(t)
	t.Chdir(t.TempDir())

	_, err := Load()
	if err == nil {
		t.Fatal("Load() error = nil, want error")
	}
	if !strings.Contains(err.Error(), "APP_NAME is required") {
		t.Fatalf("Load() error = %q, want missing APP_NAME", err.Error())
	}
}

func TestLoadRejectsInvalidDuration(t *testing.T) {
	cleanEnv(t)
	writeDotEnv(t, strings.Replace(validDotEnv(), "MONGO_TIMEOUT=10s", "MONGO_TIMEOUT=soon", 1))

	_, err := Load()
	if err == nil {
		t.Fatal("Load() error = nil, want error")
	}
	if !strings.Contains(err.Error(), "MONGO_TIMEOUT must be a valid duration") {
		t.Fatalf("Load() error = %q, want invalid MONGO_TIMEOUT", err.Error())
	}
}

func cleanEnv(t *testing.T) {
	t.Helper()

	previousValues := make(map[string]string, len(envKeys))
	presentKeys := make(map[string]bool, len(envKeys))

	for _, key := range envKeys {
		value, ok := os.LookupEnv(key)
		if ok {
			previousValues[key] = value
			presentKeys[key] = true
		}
		if err := os.Unsetenv(key); err != nil {
			t.Fatalf("unset %s: %v", key, err)
		}
	}

	t.Cleanup(func() {
		for _, key := range envKeys {
			if !presentKeys[key] {
				_ = os.Unsetenv(key)
				continue
			}

			_ = os.Setenv(key, previousValues[key])
		}
	})
}

func writeDotEnv(t *testing.T, content string) {
	t.Helper()

	tempDir := t.TempDir()
	t.Chdir(tempDir)

	if err := os.WriteFile(filepath.Join(tempDir, ".env"), []byte(content), 0o600); err != nil {
		t.Fatalf("write .env: %v", err)
	}
}

func validDotEnv() string {
	return strings.TrimSpace(`
APP_NAME=ms-order
APP_PORT=8085
APP_SHUTDOWN_TIMEOUT=10s
MONGO_URI=mongodb://localhost:27017
MONGO_DATABASE=ms_order
MONGO_TIMEOUT=10s
PAYMENT_GATEWAY_BASE_URL=http://localhost:8090
PAYMENT_GATEWAY_TIMEOUT=15s
PAYMENT_SUCCESS_URL_TEMPLATE=http://localhost:3000/payment/success?order_id={order_id}
PAYMENT_FAILURE_URL_TEMPLATE=http://localhost:3000/payment/failed?order_id={order_id}
PAYMENT_NOTIFICATION_URL=http://localhost:8085/internal/payments/notifications
ORDER_INVOICE_DUE_DURATION=24h
INTERNAL_ALLOWED_CIDRS=127.0.0.0/8,::1/128
INTERNAL_TRUST_PROXY_HEADERS=true
EXTERNAL_USER_SYNC_BASE_URL=http://localhost:9000
EXTERNAL_USER_SYNC_PATH=/internal/users/sync
EXTERNAL_USER_SYNC_TIMEOUT=3s
KAFKA_ENABLED=true
KAFKA_BROKERS=localhost:9092,localhost:9093
KAFKA_USER_CREATED_TOPIC=user.created
`) + "\n"
}
