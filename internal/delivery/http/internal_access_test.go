package httpdelivery

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestInternalAccessAllowsPrivateNetwork(t *testing.T) {
	next := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	})
	handler := newInternalAccessMiddleware(next, InternalAccessPolicy{
		AllowedCIDRs: []string{"10.0.0.0/8"},
	})

	request := httptest.NewRequest(http.MethodGet, "/internal/orders/order_123", nil)
	request.RemoteAddr = "10.1.2.3:12345"
	response := httptest.NewRecorder()

	handler.ServeHTTP(response, request)

	if response.Code != http.StatusNoContent {
		t.Fatalf("expected status %d, got %d", http.StatusNoContent, response.Code)
	}
}

func TestInternalAccessBlocksExternalNetwork(t *testing.T) {
	next := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	})
	handler := newInternalAccessMiddleware(next, InternalAccessPolicy{
		AllowedCIDRs: []string{"10.0.0.0/8"},
	})

	request := httptest.NewRequest(http.MethodGet, "/internal/orders/order_123", nil)
	request.RemoteAddr = "203.0.113.10:12345"
	response := httptest.NewRecorder()

	handler.ServeHTTP(response, request)

	if response.Code != http.StatusForbidden {
		t.Fatalf("expected status %d, got %d", http.StatusForbidden, response.Code)
	}
}
