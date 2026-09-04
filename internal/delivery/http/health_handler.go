package httpdelivery

import "net/http"

type healthHandler struct{}

func newHealthHandler() healthHandler {
	return healthHandler{}
}

func (h healthHandler) check(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{
		"status": "ok",
	})
}
